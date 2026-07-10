package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// gitExec is the SOLE place internal/config shells out to git. It mirrors internal/git.run()'s proven
// pattern but is self-contained here (internal/git.run is unexported and unreachable from this
// package; importing internal/git would also risk a cycle once S4's Load() calls loadGitConfig).
//
// INVARIANT (copied from internal/git.run): NON-ZERO git exit -> err == nil, exitCode = the code.
func gitExec(repo string, args ...string) (stdout string, stderr string, exitCode int, err error) {
	gitPath, lerr := exec.LookPath("git")
	if lerr != nil {
		return "", "", -1, fmt.Errorf("git binary not found in PATH: %w", lerr)
	}

	full := make([]string, 0, len(args)+2)
	full = append(full, "-C", repo) // repo via flag, not cmd.Dir
	full = append(full, args...)

	cmd := exec.CommandContext(context.Background(), gitPath, full...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb // captured separately so callers can build precise error messages

	runErr := cmd.Run()
	stdout, stderr = out.String(), errb.String()
	if runErr == nil {
		return stdout, stderr, 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) { // non-zero git exit -> capture code, err stays nil
		return stdout, stderr, exitErr.ExitCode(), nil
	}
	return stdout, stderr, -1, runErr // start / I/O failure (e.g. context cancel — dormant for Background)
}

// gitConfigGet runs `git -C <repo> config --get <key>` and returns the value iff the key is present.
// Exit-code semantics (FINDING B): 0 -> found (trimmed value); 1 -> missing (found=false, NOT an
// error); anything else -> wrapped error (incl. stderr text). Used for STRING and INT keys.
func gitConfigGet(repo, key string) (value string, found bool, err error) {
	stdout, stderr, code, err := gitExec(repo, "config", "--get", key)
	if err != nil {
		return "", false, err // LookPath miss / start-I/O failure (exitCode == -1)
	}
	switch code {
	case 0:
		return strings.TrimSpace(stdout), true, nil // found
	case 1:
		return "", false, nil // missing key — NOT an error (FINDING B)
	default:
		return "", false, fmt.Errorf("git config --get %s: failed (exit %d): %s", key, code, strings.TrimSpace(stderr))
	}
}

// gitConfigBool runs `git -C <repo> config --bool --get <key>` and returns the parsed bool iff present.
// `--bool` canonicalizes any git-boolean (on/off/yes/no/1/0/true/false) to "true"/"false" (FINDING C),
// so strconv.ParseBool never fails on the output. Same exit-code semantics as gitConfigGet.
func gitConfigBool(repo, key string) (value bool, found bool, err error) {
	stdout, stderr, code, err := gitExec(repo, "config", "--bool", "--get", key)
	if err != nil {
		return false, false, err
	}
	switch code {
	case 0:
		b, perr := strconv.ParseBool(strings.TrimSpace(stdout)) // "true"/"false" — never fails in practice
		if perr != nil {
			return false, false, fmt.Errorf("git config --bool --get %s: unparseable output %q: %w", key, stdout, perr)
		}
		return b, true, nil
	case 1:
		return false, false, nil // missing key — NOT an error
	default:
		return false, false, fmt.Errorf("git config --bool --get %s: failed (exit %d): %s", key, code, strings.TrimSpace(stderr))
	}
}

// parseInt is a helper that parses a string value as an integer and stores it via dst.
// On parse failure it returns a wrapped error identifying the git key.
func parseInt(repo, key, value string, dst *int) error {
	_ = repo // unused — key is already the full git key name
	n, perr := strconv.Atoi(value)
	if perr != nil {
		return fmt.Errorf("git config %s: invalid integer %q: %w", key, value, perr)
	}
	*dst = n
	return nil
}

// loadGitConfig reads Stagecoach's per-repo git-config layer (PRD §16.3, FR36, §16.1 layer 4) from the
// repo at repoDir and returns a PARTIAL *Config carrying ONLY the keys that were found set (all others
// remain at their zero value). Missing keys are NOT errors (git config --get exits 1 for a missing
// key, FINDING B). A non-integer timeout, a missing git binary, or any unexpected git exit yields a
// wrapped error.
//
// KEY NAMES ARE CAMELCASE (FINDING A): git config rejects underscores ("invalid key"). The multi-word
// keys follow the PRD §16.3 example (autoStageAll, maxDiffBytes, …), NOT FR36's snake_case spelling.
//
// The returned *Config is designed for S2's *bool/*int nil-overlay(): unset fields are nil/zero, so
// overlay copies only the fields the user actually set. Do NOT pre-fill with Defaults() (that would
// make every field "set" and clobber lower layers — see the GOTCHA). AutoStageAll is now *bool, so a
// found-but-false bool (autoStageAll=false) propagates as *false and survives overlay (the *bool
// conversion fixed this; force false via env (S4)/CLI (S4) remains available too).
func loadGitConfig(repoDir string) (*Config, error) {
	c := &Config{} // ALL fields zero; only found keys are set below.

	// --- strings (plain --get) ---
	if v, found, err := gitConfigGet(repoDir, "stagecoach.provider"); err != nil {
		return nil, err
	} else if found {
		c.Provider = v
	}
	if v, found, err := gitConfigGet(repoDir, "stagecoach.model"); err != nil {
		return nil, err
	} else if found {
		c.Model = v
	}
	if v, found, err := gitConfigGet(repoDir, "stagecoach.output"); err != nil {
		return nil, err
	} else if found {
		c.Output = &v
	}
	// §9.19 FR-F1/FR-F6 — format/locale: single-word string keys, raw copy (no validation here).
	if v, found, err := gitConfigGet(repoDir, "stagecoach.format"); err != nil {
		return nil, err
	} else if found {
		c.Format = v
	}
	if v, found, err := gitConfigGet(repoDir, "stagecoach.locale"); err != nil {
		return nil, err
	} else if found {
		c.Locale = v
	}
	if v, found, err := gitConfigGet(repoDir, "stagecoach.template"); err != nil {
		return nil, err
	} else if found {
		c.Template = v
	}

	// --- timeout: accepts both "90" (seconds) and "90s" (Go duration) forms. ---
	if v, found, err := gitConfigGet(repoDir, "stagecoach.timeout"); err != nil {
		return nil, err
	} else if found {
		d, perr := parseTimeout(v) // parseTimeout handles both "90" and "90s"
		if perr != nil {
			return nil, fmt.Errorf("git config stagecoach.timeout: %w", perr)
		}
		c.Timeout = d
	}

	// §9.15 FR-R7 / §9.8 FR36 / §16.1 layer 4 — per-role generation timeout via git config
	// (NEW infrastructure: git.go read NO per-role keys before this). Mirrors the global
	// stagecoach.timeout block above EXACTLY: gitConfigGet → parseTimeout → wrapped error →
	// setRoleTimeout. gitConfigGet maps a missing key (git exit 1) to found=false (no-op), so a
	// role with no stagecoach.role.<role>.timeout key is untouched. parseTimeout accepts "600s"
	// and bare "600". A malformed value is a HARD ERROR (loadGitConfig has an error return) —
	// the OPPOSITE of S2's loadFlags silent-ignore. Per-role provider/model/reasoning git keys
	// are intentionally NOT read here (file/env/flag only); this loop is timeout-only.
	for _, role := range roleNames {
		key := "stagecoach.role." + role + ".timeout"
		if v, found, err := gitConfigGet(repoDir, key); err != nil {
			return nil, err
		} else if found {
			d, perr := parseTimeout(v)
			if perr != nil {
				return nil, fmt.Errorf("git config %s: %w", key, perr)
			}
			c.setRoleTimeout(role, d)
		}
	}

	// --- booleans (--bool canonicalizes; FINDING C) ---
	if v, found, err := gitConfigBool(repoDir, "stagecoach.autoStageAll"); err != nil { // camelCase!
		return nil, err
	} else if found {
		c.AutoStageAll = boolPtr(v) // *bool: wrap the plain bool v; omitted ⇒ stays nil (overlay inherits). An explicit "git config stagecoach.autoStageAll off" now propagates *false end-to-end (was a no-op under the old only-true-propagates bool — fixed by the *bool conversion). Mirrors the DiffContext intPtr(n) pattern below.
	}
	if v, found, err := gitConfigBool(repoDir, "stagecoach.verbose"); err != nil {
		return nil, err
	} else if found {
		c.Verbose = v
	}
	if v, found, err := gitConfigBool(repoDir, "stagecoach.stripCodeFence"); err != nil { // camelCase!
		return nil, err
	} else if found {
		c.StripCodeFence = &v
	}
	// §9.22 FR-P1 — push via git config (lowercase single-word key — no camelCase needed).
	if v, found, err := gitConfigBool(repoDir, "stagecoach.push"); err != nil {
		return nil, err
	} else if found {
		c.Push = v
	}
	// §9.25 FR-V5 — noVerify via git config (camelCase key: git rejects underscores in the final segment,
	// matching the autoStageAll/maxDiffBytes/stripCodeFence convention).
	if v, found, err := gitConfigBool(repoDir, "stagecoach.noVerify"); err != nil {
		return nil, err
	} else if found {
		c.NoVerify = v
	}
	// §9.27 FR-K6 — noParentWatchdog via git config (camelCase key, same convention as noVerify).
	if v, found, err := gitConfigBool(repoDir, "stagecoach.noParentWatchdog"); err != nil {
		return nil, err
	} else if found {
		c.NoParentWatchdog = v
	}

	// --- ints (plain --get -> Atoi) ---
	if v, found, err := gitConfigGet(repoDir, "stagecoach.maxDiffBytes"); err != nil { // camelCase!
		return nil, err
	} else if found {
		if err := parseInt(repoDir, "stagecoach.maxDiffBytes", v, &c.MaxDiffBytes); err != nil {
			return nil, err
		}
	}
	if v, found, err := gitConfigGet(repoDir, "stagecoach.maxMdLines"); err != nil { // camelCase!
		return nil, err
	} else if found {
		if err := parseInt(repoDir, "stagecoach.maxMdLines", v, &c.MaxMdLines); err != nil {
			return nil, err
		}
	}
	// §9.1 FR3d — token_limit via git config (camelCase key). 0 = unset ⇒ legacy caps (no meaningful explicit 0).
	if v, found, err := gitConfigGet(repoDir, "stagecoach.tokenLimit"); err != nil { // camelCase!
		return nil, err
	} else if found {
		if err := parseInt(repoDir, "stagecoach.tokenLimit", v, &c.TokenLimit); err != nil {
			return nil, err
		}
	}
	// §9.1 FR3f — diff_context via git config (camelCase key, integer 0–3). Config.DiffContext is *int
	// (S2): nil when the key is absent (found=false → overlay inherits the default *1); non-nil — incl.
	// *0 — when found. The write is UNCONDITIONAL inside `found`: an explicit "git config
	// stagecoach.diffContext 0" must survive as intPtr(0) (0 = changed-lines-only is a first-class value).
	if v, found, err := gitConfigGet(repoDir, "stagecoach.diffContext"); err != nil { // camelCase!
		return nil, err
	} else if found {
		var n int
		if err := parseInt(repoDir, "stagecoach.diffContext", v, &n); err != nil {
			return nil, err
		}
		c.DiffContext = intPtr(n) // *int: parse into local n (NOT &c.DiffContext which is **int), then wrap.
	}
	if v, found, err := gitConfigGet(repoDir, "stagecoach.maxDuplicateRetries"); err != nil { // camelCase!
		return nil, err
	} else if found {
		if err := parseInt(repoDir, "stagecoach.maxDuplicateRetries", v, &c.MaxDuplicateRetries); err != nil {
			return nil, err
		}
	}
	if v, found, err := gitConfigGet(repoDir, "stagecoach.subjectTargetChars"); err != nil { // camelCase!
		return nil, err
	} else if found {
		if err := parseInt(repoDir, "stagecoach.subjectTargetChars", v, &c.SubjectTargetChars); err != nil {
			return nil, err
		}
	}

	return c, nil // non-nil; all-zero if nothing was set (overlay is then a no-op)
}
