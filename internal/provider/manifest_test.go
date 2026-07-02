package provider

import (
	"reflect"
	"testing"
)

// sixBuiltinManifests delegates to the production Builtins() (M2.T3.S1), the
// single source of truth for the six §B.1–B.6 manifests (with the four §C
// corrections applied). The golden Render table below exercises those exact
// production manifests rather than a hand-maintained duplicate.
func sixBuiltinManifests() map[string]Manifest {
	return Builtins()
}

// assertRendered compares a *Rendered against the byte-exact expectation. The
// command token is deliberately excluded from wantArgs — Render's Args omit
// the command, which the executor supplies separately as m.Command.
func assertRendered(t *testing.T, got *Rendered, wantArgs []string, wantStdin string, wantVia bool) {
	t.Helper()
	if !reflect.DeepEqual(got.Args, wantArgs) {
		t.Errorf("Args mismatch:\n  got  %#v\n  want %#v", got.Args, wantArgs)
	}
	if got.StdinPayload != wantStdin {
		t.Errorf("StdinPayload = %q, want %q", got.StdinPayload, wantStdin)
	}
	if got.DeliverViaStdin != wantVia {
		t.Errorf("DeliverViaStdin = %v, want %v", got.DeliverViaStdin, wantVia)
	}
}

// TestRender_GoldenProviders asserts byte-exact Args (algorithm-derived order,
// NOT the §B.2/§B.6 illustrative ordering), StdinPayload, and DeliverViaStdin
// for all six providers from external_deps.md §B.1–B.6. sys="SYS", user="BODY"
// ⇒ the prepend payload is "SYS\n\nBODY" for the no-system-prompt-flag agents.
// The flag SET matches §B; for claude/cursor the ORDER follows the §12.2
// algorithm (print_flag AFTER bare_flags), which is functionally identical to
// §B since flag order is parse-order-independent for those CLIs.
func TestRender_GoldenProviders(t *testing.T) {
	manifests := sixBuiltinManifests()
	tests := []struct {
		name      string
		manifest  string
		model     string
		provider  string
		sys       string
		user      string
		wantArgs  []string
		wantStdin string
		wantVia   bool
	}{
		{
			name: "pi stdin, sys-flag, provider", manifest: "pi",
			model: "glm-5-turbo", provider: "zai", sys: "SYS", user: "BODY",
			// effUser="BODY" (sys-flag set ⇒ no prepend); -p last (§B.1 == algorithm).
			wantArgs: []string{
				"--provider", "zai", "--model", "glm-5-turbo", "--system-prompt", "SYS",
				"--no-tools", "--no-extensions", "--no-skills",
				"--no-prompt-templates", "--no-context-files", "--no-session",
				"-p",
			},
			wantStdin: "BODY", wantVia: true,
		},
		{
			name: "claude stdin, sys-flag, empty bare values", manifest: "claude",
			model: "sonnet", provider: "", sys: "SYS", user: "BODY",
			// effUser="BODY"; print_flag LAST per algorithm (NOT §B.2's -p-first).
			wantArgs: []string{
				"--model", "sonnet", "--system-prompt", "SYS",
				"--setting-sources", "", "--tools", "",
				"--disable-slash-commands", "--no-chrome", "--no-session-persistence",
				"-p",
			},
			wantStdin: "BODY", wantVia: true,
		},
		{
			name: "gemini positional, prepend", manifest: "gemini",
			model: "gemini-2.5-pro", provider: "", sys: "SYS", user: "BODY",
			// effUser="SYS\n\nBODY" (no sys-flag ⇒ prepend).
			wantArgs:  []string{"-m", "gemini-2.5-pro", "--approval-mode", "default", "SYS\n\nBODY"},
			wantStdin: "",
			wantVia:   false,
		},
		{
			name: "opencode positional, subcommand, prepend", manifest: "opencode",
			model: "anthropic/claude-sonnet-4", provider: "", sys: "SYS", user: "BODY",
			wantArgs:  []string{"run", "-m", "anthropic/claude-sonnet-4", "SYS\n\nBODY"},
			wantStdin: "",
			wantVia:   false,
		},
		{
			name: "codex stdin (CORRECTED), subcommand, prepend", manifest: "codex",
			model: "gpt-5", provider: "", sys: "SYS", user: "BODY",
			// effUser="SYS\n\nBODY" delivered via stdin (codex reads stdin).
			wantArgs: []string{
				"exec", "-m", "gpt-5",
				"--sandbox", "read-only", "--ask-for-approval", "never", "--ephemeral",
			},
			wantStdin: "SYS\n\nBODY", wantVia: true,
		},
		{
			name: "cursor positional, print last, prepend", manifest: "cursor",
			model: "gpt-5", provider: "", sys: "SYS", user: "BODY",
			// model BEFORE bare, print LAST (algorithm order, not §B.6's arrangement).
			wantArgs:  []string{"--model", "gpt-5", "--mode", "ask", "--trust", "-p", "SYS\n\nBODY"},
			wantStdin: "",
			wantVia:   false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			m := manifests[tc.manifest]
			got, err := m.Render(tc.model, tc.provider, tc.sys, tc.user)
			if err != nil {
				t.Fatalf("Render(%q) returned unexpected error: %v", tc.manifest, err)
			}
			if got == nil {
				t.Fatalf("Render(%q) returned nil *Rendered", tc.manifest)
			}
			assertRendered(t, got, tc.wantArgs, tc.wantStdin, tc.wantVia)
		})
	}
}

// TestRender_NoModel verifies that an empty model omits the --model token
// (and its value) entirely; the rest of the arg order is unchanged.
func TestRender_NoModel(t *testing.T) {
	m := sixBuiltinManifests()["pi"]
	got, err := m.Render("", "zai", "SYS", "BODY")
	if err != nil {
		t.Fatalf("Render returned unexpected error: %v", err)
	}
	wantArgs := []string{
		"--provider", "zai", "--system-prompt", "SYS",
		"--no-tools", "--no-extensions", "--no-skills",
		"--no-prompt-templates", "--no-context-files", "--no-session",
		"-p",
	}
	assertRendered(t, got, wantArgs, "BODY", true)
	for _, a := range got.Args {
		if a == m.ModelFlag {
			t.Errorf("Args = %#v unexpectedly contains model flag %q", got.Args, m.ModelFlag)
		}
	}
}

// TestRender_NoProvider verifies that an empty provider omits the --provider
// token (and its value) entirely.
func TestRender_NoProvider(t *testing.T) {
	m := sixBuiltinManifests()["pi"]
	got, err := m.Render("glm-5-turbo", "", "SYS", "BODY")
	if err != nil {
		t.Fatalf("Render returned unexpected error: %v", err)
	}
	wantArgs := []string{
		"--model", "glm-5-turbo", "--system-prompt", "SYS",
		"--no-tools", "--no-extensions", "--no-skills",
		"--no-prompt-templates", "--no-context-files", "--no-session",
		"-p",
	}
	assertRendered(t, got, wantArgs, "BODY", true)
	for _, a := range got.Args {
		if a == m.ProviderFlag {
			t.Errorf("Args = %#v unexpectedly contains provider flag %q", got.Args, m.ProviderFlag)
		}
	}
}

// TestRender_FlagDelivery verifies the "flag" delivery branch: the payload is
// appended as (PromptFlag, effUser), DeliverViaStdin is false, and StdinPayload
// is empty. With a system-prompt flag present, no prepend occurs.
func TestRender_FlagDelivery(t *testing.T) {
	m := Manifest{
		Name:             "flag-test",
		Command:          "x",
		PromptDelivery:   DeliveryFlag,
		PromptFlag:       "--prompt",
		ProviderFlag:     "--provider",
		ModelFlag:        "--model",
		SystemPromptFlag: "--sys",
		BareFlags:        []string{"--x"},
		PrintFlag:        "-p",
	}
	got, err := m.Render("m", "p", "S", "U")
	if err != nil {
		t.Fatalf("Render returned unexpected error: %v", err)
	}
	wantArgs := []string{"--provider", "p", "--model", "m", "--sys", "S", "--x", "-p", "--prompt", "U"}
	assertRendered(t, got, wantArgs, "", false)
}

// TestRender_FlagDeliveryPrepend confirms the system-prompt prepend fallback
// also fires for flag delivery: with SystemPromptFlag=="" && sys!="", the
// (PromptFlag, effUser) pair carries sys+"\n\n"+user.
func TestRender_FlagDeliveryPrepend(t *testing.T) {
	m := Manifest{
		Name:             "flag-prepend",
		Command:          "x",
		PromptDelivery:   DeliveryFlag,
		PromptFlag:       "--prompt",
		ModelFlag:        "--model",
		SystemPromptFlag: "", // none ⇒ prepend
	}
	got, err := m.Render("m", "", "SYS", "BODY")
	if err != nil {
		t.Fatalf("Render returned unexpected error: %v", err)
	}
	wantArgs := []string{"--model", "m", "--prompt", "SYS\n\nBODY"}
	assertRendered(t, got, wantArgs, "", false)
}

// TestRender_EnvSorted verifies m.Env renders as sorted "KEY=VALUE" strings
// (Go map iteration is random, so sorting is required for determinism).
func TestRender_EnvSorted(t *testing.T) {
	m := Manifest{
		Name:           "env-test",
		Command:        "x",
		PromptDelivery: DeliveryStdin,
		Env:            map[string]string{"B": "2", "A": "1", "C": "3"},
	}
	got, err := m.Render("m", "", "", "BODY")
	if err != nil {
		t.Fatalf("Render returned unexpected error: %v", err)
	}
	wantEnv := []string{"A=1", "B=2", "C=3"}
	if !reflect.DeepEqual(got.Env, wantEnv) {
		t.Errorf("Env = %#v, want %#v", got.Env, wantEnv)
	}
}

// TestRender_UnknownDeliveryErrors verifies an unknown prompt_delivery value
// yields a non-nil error AND a nil *Rendered (no partial result leaks).
func TestRender_UnknownDeliveryErrors(t *testing.T) {
	m := Manifest{
		Name:           "bogus",
		Command:        "x",
		PromptDelivery: "telepathy",
	}
	got, err := m.Render("m", "p", "SYS", "BODY")
	if err == nil {
		t.Fatalf("Render with unknown delivery: want error, got nil (result=%#v)", got)
	}
	if got != nil {
		t.Errorf("Render with unknown delivery: want nil *Rendered, got %#v", got)
	}
}

// TestRender_EmptySysNoPrepend verifies that with SystemPromptFlag=="" AND
// sys=="", the stdin payload is exactly the user payload — no stray "\n\n"
// is injected by the prepend fallback guard.
func TestRender_EmptySysNoPrepend(t *testing.T) {
	m := Manifest{
		Name:           "no-sys",
		Command:        "x",
		PromptDelivery: DeliveryStdin,
		// SystemPromptFlag empty + sys empty ⇒ no prepend.
	}
	got, err := m.Render("m", "", "", "BODY")
	if err != nil {
		t.Fatalf("Render returned unexpected error: %v", err)
	}
	if got.StdinPayload != "BODY" {
		t.Errorf("StdinPayload = %q, want %q (no stray \\n\\n)", got.StdinPayload, "BODY")
	}
}

// TestRender_DefaultEmptyDeliveryIsStdin confirms an empty PromptDelivery
// (the §12.1 default) is treated as stdin delivery.
func TestRender_DefaultEmptyDeliveryIsStdin(t *testing.T) {
	m := Manifest{
		Name:           "default-delivery",
		Command:        "x",
		PromptDelivery: "", // omitted ⇒ stdin
	}
	got, err := m.Render("m", "", "", "BODY")
	if err != nil {
		t.Fatalf("Render returned unexpected error: %v", err)
	}
	if !got.DeliverViaStdin {
		t.Errorf("DeliverViaStdin = false, want true (empty delivery ⇒ stdin)")
	}
	if got.StdinPayload != "BODY" {
		t.Errorf("StdinPayload = %q, want %q", got.StdinPayload, "BODY")
	}
}

// TestRender_DoesNotMutateManifest verifies the defensive copy: calling
// Render (twice, to be sure) never alters the source Manifest's slices or
// env map. Without the fresh-slice allocation, appending BareFlags could
// alias and clobber the caller's Subcommand backing array.
func TestRender_DoesNotMutateManifest(t *testing.T) {
	m := Manifest{
		Name:           "immut",
		Command:        "x",
		Subcommand:     []string{"run"},
		PromptDelivery: DeliveryStdin,
		BareFlags:      []string{"--x", "--y"},
		Env:            map[string]string{"K": "v"},
	}
	wantSub := []string{"run"}
	wantBare := []string{"--x", "--y"}
	wantEnv := map[string]string{"K": "v"}

	for i := 0; i < 2; i++ {
		if _, err := m.Render("m", "p", "SYS", "BODY"); err != nil {
			t.Fatalf("Render pass %d returned unexpected error: %v", i, err)
		}
	}
	if !reflect.DeepEqual(m.Subcommand, wantSub) {
		t.Errorf("m.Subcommand mutated: got %#v, want %#v", m.Subcommand, wantSub)
	}
	if !reflect.DeepEqual(m.BareFlags, wantBare) {
		t.Errorf("m.BareFlags mutated: got %#v, want %#v", m.BareFlags, wantBare)
	}
	if !reflect.DeepEqual(m.Env, wantEnv) {
		t.Errorf("m.Env mutated: got %#v, want %#v", m.Env, wantEnv)
	}
}
