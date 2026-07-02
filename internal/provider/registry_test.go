package provider

import (
	"reflect"
	"testing"
)

// These tests cover the provider.Registry merge/lookup/detect contract
// (decisions.md §6, PRD FR46–FR48). They mirror builtin_test.go conventions:
// white-box package provider, inline Manifest literals, reflect.DeepEqual, and
// the shared contains() helper defined in builtin_test.go.

// TestRegistry_MergeDefaultModelKeepsBareFlags asserts the decisions.md §6
// field-by-field contract: an override that sets ONLY default_model leaves
// every other built-in field (BareFlags, PrintFlag, ModelFlag, PromptDelivery)
// untouched.
func TestRegistry_MergeDefaultModelKeepsBareFlags(t *testing.T) {
	r := NewRegistry(Builtins(), map[string]Manifest{"pi": {DefaultModel: "x"}})

	got, ok := r.Get("pi")
	if !ok {
		t.Fatal(`r.Get("pi") ok = false, want true`)
	}
	if got.DefaultModel != "x" {
		t.Errorf("got.DefaultModel = %q, want %q", got.DefaultModel, "x")
	}

	base := Builtins()["pi"]
	if !reflect.DeepEqual(got.BareFlags, base.BareFlags) {
		t.Errorf("BareFlags not preserved: got %#v, want %#v", got.BareFlags, base.BareFlags)
	}
	if len(got.BareFlags) != 6 {
		t.Errorf("len(BareFlags) = %d, want 6 (override must not clear built-in flags)", len(got.BareFlags))
	}
	if got.PrintFlag != base.PrintFlag {
		t.Errorf("PrintFlag not preserved: got %q, want %q", got.PrintFlag, base.PrintFlag)
	}
	if got.ModelFlag != base.ModelFlag {
		t.Errorf("ModelFlag not preserved: got %q, want %q", got.ModelFlag, base.ModelFlag)
	}
	if got.PromptDelivery != base.PromptDelivery {
		t.Errorf("PromptDelivery not preserved: got %q, want %q", got.PromptDelivery, base.PromptDelivery)
	}
}

// TestRegistry_MergeBareFlagsReplaces asserts BareFlags is replaced WHOLESALE
// by a non-empty override (not appended/merged) and that the registry owns a
// defensive copy: mutating the override slice after construction cannot change
// the resolved manifest.
func TestRegistry_MergeBareFlagsReplaces(t *testing.T) {
	repl := []string{"--only-this"}
	r := NewRegistry(Builtins(), map[string]Manifest{"pi": {BareFlags: repl}})

	got, ok := r.Get("pi")
	if !ok {
		t.Fatal(`r.Get("pi") ok = false, want true`)
	}
	if !reflect.DeepEqual(got.BareFlags, repl) {
		t.Errorf("BareFlags = %#v, want %#v (wholesale replace)", got.BareFlags, repl)
	}
	if reflect.DeepEqual(got.BareFlags, Builtins()["pi"].BareFlags) {
		t.Errorf("BareFlags still equals the built-in; override should have replaced it")
	}

	// Defensive-copy check: mutating the override slice post-construction must
	// not leak into the registry's owned copy.
	repl[0] = "--mutated"
	if got.BareFlags[0] != "--only-this" {
		t.Errorf("registry aliased override slice: BareFlags[0] = %q, want %q", got.BareFlags[0], "--only-this")
	}
}

// TestRegistry_NewNameResolves asserts a user-defined provider with a name not
// present in the built-ins is added as-is (FR48) and resolvable via Get/List.
func TestRegistry_NewNameResolves(t *testing.T) {
	r := NewRegistry(Builtins(), map[string]Manifest{
		"myagent": {Command: "/opt/myagent/bin/agent", PromptDelivery: DeliveryStdin},
	})

	got, ok := r.Get("myagent")
	if !ok {
		t.Fatal(`r.Get("myagent") ok = false, want true`)
	}
	if got.Command != "/opt/myagent/bin/agent" {
		t.Errorf("got.Command = %q, want %q", got.Command, "/opt/myagent/bin/agent")
	}
	if got.PromptDelivery != DeliveryStdin {
		t.Errorf("got.PromptDelivery = %q, want %q", got.PromptDelivery, DeliveryStdin)
	}
	if !contains(r.List(), "myagent") {
		t.Errorf("myagent missing from List() = %#v", r.List())
	}
}

// TestRegistry_Detect asserts Detect uses exec.LookPath against the manifest's
// Detect field (or Command fallback): pi is installed in this environment
// (true) and a bogus command is not (false).
func TestRegistry_Detect(t *testing.T) {
	r := NewRegistry(Builtins(), map[string]Manifest{
		"definitely-not-an-agent-xyz": {Command: "definitely-not-an-agent-xyz"},
	})

	d := r.Detect()
	if !d["pi"] {
		t.Error(`d["pi"] = false, want true (pi must be on $PATH)`)
	}
	if d["definitely-not-an-agent-xyz"] {
		t.Error(`d["definitely-not-an-agent-xyz"] = true, want false`)
	}
}

// TestRegistry_GetUnknownReturnsFalse asserts Get of an unknown name returns
// the zero Manifest and ok=false (used by FR47 providers show error path and
// M5 config Load validation).
func TestRegistry_GetUnknownReturnsFalse(t *testing.T) {
	r := NewRegistry(Builtins(), nil)
	m, ok := r.Get("nope")
	if ok {
		t.Error(`Get("nope") ok = true, want false`)
	}
	if !reflect.DeepEqual(m, Manifest{}) {
		t.Errorf(`Get("nope") = %+v, want zero-value Manifest{}`, m)
	}
}

// TestRegistry_ListSortedAndUnion asserts List is sorted and is exactly the
// union of the six built-in names plus any new override names, with no dupes.
func TestRegistry_ListSortedAndUnion(t *testing.T) {
	r := NewRegistry(Builtins(), map[string]Manifest{
		"myagent": {Command: "myagent"}, // new name
		"pi":      {DefaultModel: "x"},  // override an existing builtin
	})

	list := r.List()

	// Sorted ascending.
	for i := 1; i < len(list); i++ {
		if list[i-1] > list[i] {
			t.Errorf("List() not sorted: %#v", list)
			break
		}
	}

	// Exact union: 6 builtins + the one new name, each exactly once.
	seen := make(map[string]int, len(list))
	for _, n := range list {
		seen[n]++
	}
	want := map[string]struct{}{
		"pi": {}, "claude": {}, "gemini": {}, "opencode": {}, "codex": {}, "cursor": {},
		"myagent": {},
	}
	for name, count := range seen {
		if count > 1 {
			t.Errorf("List() has duplicate %q (%d times)", name, count)
		}
		if _, ok := want[name]; !ok {
			t.Errorf("List() has unexpected name %q", name)
		}
	}
	for name := range want {
		if seen[name] == 0 {
			t.Errorf("List() missing expected name %q", name)
		}
	}
}

// TestRegistry_EnvReplaceWholesale asserts a non-empty override Env replaces
// the base Env WHOLESALE (never deep-merged) and that the registry owns a copy
// of the map: mutating the override map after construction cannot change Get.
func TestRegistry_EnvReplaceWholesale(t *testing.T) {
	ov := map[string]string{"X": "1"}
	r := NewRegistry(Builtins(), map[string]Manifest{"pi": {Env: ov}})

	got, ok := r.Get("pi")
	if !ok {
		t.Fatal(`r.Get("pi") ok = false, want true`)
	}
	want := map[string]string{"X": "1"}
	if !reflect.DeepEqual(got.Env, want) {
		t.Errorf("Env = %#v, want %#v (wholesale replace of built-in nil Env)", got.Env, want)
	}

	// Mutate the override map post-construction: the registry must own a copy.
	ov["X"] = "mutated"
	ov["Y"] = "added"
	got2, _ := r.Get("pi")
	if !reflect.DeepEqual(got2.Env, want) {
		t.Errorf("registry aliased override Env map: Env = %#v, want %#v", got2.Env, want)
	}
}

// TestRegistry_NewRegistryDoesNotMutateBuiltins asserts the registry never
// mutates the Builtins() backing map: a fresh Builtins() call after building a
// registry with overrides returns the unmodified canonical manifests.
func TestRegistry_NewRegistryDoesNotMutateBuiltins(t *testing.T) {
	wantPi := Builtins()["pi"]

	_ = NewRegistry(Builtins(), map[string]Manifest{"pi": {DefaultModel: "merged-model"}})

	gotPi := Builtins()["pi"]
	if !reflect.DeepEqual(wantPi, gotPi) {
		t.Errorf("NewRegistry mutated a fresh Builtins() map: want %+v, got %+v", wantPi, gotPi)
	}
	if gotPi.DefaultModel != "glm-5-turbo" {
		t.Errorf("Builtins()[pi].DefaultModel = %q, want %q (must be unchanged)", gotPi.DefaultModel, "glm-5-turbo")
	}
}

// TestRegistry_DetectUsesCommandWhenDetectEmpty asserts Detect falls back to
// Command when Detect is empty (PRD §12.1): a provider with Command="pi" (on
// $PATH) is reported as installed.
func TestRegistry_DetectUsesCommandWhenDetectEmpty(t *testing.T) {
	r := NewRegistry(Builtins(), map[string]Manifest{
		"viapistdin": {Command: "pi"}, // no Detect → fall back to Command
	})
	d := r.Detect()
	if !d["viapistdin"] {
		t.Errorf(`d["viapistdin"] = false, want true (Detect empty → fall back to Command "pi")`)
	}
}
