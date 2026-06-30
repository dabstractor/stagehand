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

// loadGitConfig reads Stagehand's per-repo git-config layer (PRD §16.3, FR36, §16.1 layer 4) from the
// repo at repoDir and returns a PARTIAL *Config carrying ONLY the keys that were found set (all others
// remain at their zero value). Missing keys are NOT errors (git config --get exits 1 for a missing
// key, FINDING B). A non-integer timeout, a missing git binary, or any unexpected git exit yields a
// wrapped error.
//
// KEY NAMES ARE CAMELCASE (FINDING A): git config rejects underscores ("invalid key"). The multi-word
// keys follow the PRD §16.3 example (autoStageAll, maxDiffBytes, …), NOT FR36's snake_case spelling.
//
// The returned *Config is designed for S2's NON-ZERO overlay(): unset fields are zero, so overlay
// copies only the fields the user actually set. Do NOT pre-fill with Defaults() (that would make every
// field "set" and clobber lower layers — see the GOTCHA). Because overlay is non-zero, a found-but-
// false bool (autoStageAll=false) is a documented no-op (FINDING G); force false via env (S4)/CLI (S4).
func loadGitConfig(repoDir string) (*Config, error) {
	c := &Config{} // ALL fields zero; only found keys are set below.

	// --- strings (plain --get) ---
	if v, found, err := gitConfigGet(repoDir, "stagehand.provider"); err != nil {
		return nil, err
	} else if found {
		c.Provider = v
	}
	if v, found, err := gitConfigGet(repoDir, "stagehand.model"); err != nil {
		return nil, err
	} else if found {
		c.Model = v
	}
	if v, found, err := gitConfigGet(repoDir, "stagehand.output"); err != nil {
		return nil, err
	} else if found {
		c.Output = v
	}

	// --- timeout: accepts both "90" (seconds) and "90s" (Go duration) forms. ---
	if v, found, err := gitConfigGet(repoDir, "stagehand.timeout"); err != nil {
		return nil, err
	} else if found {
		d, perr := parseTimeout(v) // parseTimeout handles both "90" and "90s"
		if perr != nil {
			return nil, fmt.Errorf("git config stagehand.timeout: %w", perr)
		}
		c.Timeout = d
	}

	// --- booleans (--bool canonicalizes; FINDING C) ---
	if v, found, err := gitConfigBool(repoDir, "stagehand.autoStageAll"); err != nil { // camelCase!
		return nil, err
	} else if found {
		c.AutoStageAll = v
	}
	if v, found, err := gitConfigBool(repoDir, "stagehand.verbose"); err != nil {
		return nil, err
	} else if found {
		c.Verbose = v
	}
	if v, found, err := gitConfigBool(repoDir, "stagehand.stripCodeFence"); err != nil { // camelCase!
		return nil, err
	} else if found {
		c.StripCodeFence = &v
	}

	// --- ints (plain --get -> Atoi) ---
	if v, found, err := gitConfigGet(repoDir, "stagehand.maxDiffBytes"); err != nil { // camelCase!
		return nil, err
	} else if found {
		if err := parseInt(repoDir, "stagehand.maxDiffBytes", v, &c.MaxDiffBytes); err != nil {
			return nil, err
		}
	}
	if v, found, err := gitConfigGet(repoDir, "stagehand.maxMdLines"); err != nil { // camelCase!
		return nil, err
	} else if found {
		if err := parseInt(repoDir, "stagehand.maxMdLines", v, &c.MaxMdLines); err != nil {
			return nil, err
		}
	}
	if v, found, err := gitConfigGet(repoDir, "stagehand.maxDuplicateRetries"); err != nil { // camelCase!
		return nil, err
	} else if found {
		if err := parseInt(repoDir, "stagehand.maxDuplicateRetries", v, &c.MaxDuplicateRetries); err != nil {
			return nil, err
		}
	}
	if v, found, err := gitConfigGet(repoDir, "stagehand.subjectTargetChars"); err != nil { // camelCase!
		return nil, err
	} else if found {
		if err := parseInt(repoDir, "stagehand.subjectTargetChars", v, &c.SubjectTargetChars); err != nil {
			return nil, err
		}
	}

	return c, nil // non-nil; all-zero if nothing was set (overlay is then a no-op)
}
