package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestConfigGlobalGet_FoundAndMissing verifies get on present and absent keys.
// Isolation: t.Setenv("GIT_CONFIG_GLOBAL", tmpfile) replaces ~/.gitconfig.
func TestConfigGlobalGet_FoundAndMissing(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)

	g := New(t.TempDir())
	ctx := context.Background()

	// Set a key first via ConfigGlobalSet.
	if err := g.ConfigGlobalSet(ctx, "alias.testkey", "!stagehand"); err != nil {
		t.Fatalf("ConfigGlobalSet: %v", err)
	}

	// Get the key back — should be found.
	val, found, err := g.ConfigGlobalGet(ctx, "alias.testkey")
	if err != nil {
		t.Fatalf("ConfigGlobalGet (found): %v", err)
	}
	if !found {
		t.Fatal("ConfigGlobalGet found=false, want true")
	}
	if val != "!stagehand" {
		t.Fatalf("ConfigGlobalGet value = %q, want %q", val, "!stagehand")
	}

	// Get a missing key — found=false, nil err.
	val2, found2, err := g.ConfigGlobalGet(ctx, "alias.nonexistent")
	if err != nil {
		t.Fatalf("ConfigGlobalGet (missing): %v", err)
	}
	if found2 {
		t.Fatal("ConfigGlobalGet found=true for missing key, want false")
	}
	if val2 != "" {
		t.Fatalf("ConfigGlobalGet value = %q, want empty", val2)
	}
}

// TestConfigGlobalSet_WritesValue verifies the value is persisted (the `!` is preserved).
func TestConfigGlobalSet_WritesValue(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)

	g := New(t.TempDir())
	ctx := context.Background()

	if err := g.ConfigGlobalSet(ctx, "alias.testset", "!stagehand"); err != nil {
		t.Fatalf("ConfigGlobalSet: %v", err)
	}

	// Read back to confirm the `!` is preserved (proves single-argv, not sh -c).
	val, found, err := g.ConfigGlobalGet(ctx, "alias.testset")
	if err != nil {
		t.Fatalf("ConfigGlobalGet: %v", err)
	}
	if !found {
		t.Fatal("key not found after set")
	}
	if val != "!stagehand" {
		t.Fatalf("value = %q, want %q (the `!` must be preserved)", val, "!stagehand")
	}
}

// TestConfigGlobalUnset_PresentAndMissing verifies unset on present and absent keys.
func TestConfigGlobalUnset_PresentAndMissing(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)

	g := New(t.TempDir())
	ctx := context.Background()

	// Set then unset a key.
	if err := g.ConfigGlobalSet(ctx, "alias.testunset", "!stagehand"); err != nil {
		t.Fatalf("ConfigGlobalSet: %v", err)
	}

	found, err := g.ConfigGlobalUnset(ctx, "alias.testunset")
	if err != nil {
		t.Fatalf("ConfigGlobalUnset (present): %v", err)
	}
	if !found {
		t.Fatal("ConfigGlobalUnset found=false for present key, want true")
	}

	// Confirm it's now gone.
	_, stillFound, err := g.ConfigGlobalGet(ctx, "alias.testunset")
	if err != nil {
		t.Fatalf("ConfigGlobalGet after unset: %v", err)
	}
	if stillFound {
		t.Fatal("key still found after unset")
	}

	// Unset again — exit 5 ⇒ found=false, NOT an error.
	found2, err := g.ConfigGlobalUnset(ctx, "alias.testunset")
	if err != nil {
		t.Fatalf("ConfigGlobalUnset (missing): %v", err)
	}
	if found2 {
		t.Fatal("ConfigGlobalUnset found=true for missing key, want false")
	}
}

// TestConfigGlobal_Isolation verifies that writing via GIT_CONFIG_GLOBAL does NOT touch the
// real ~/.gitconfig. The test config file must exist and contain the entry; the real global
// config is checked for a sentinel key before and after.
func TestConfigGlobal_Isolation(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)

	// Capture real global config state for a sentinel key (should never exist).
	realBefore, _, _ := (&gitRunner{workDir: os.TempDir()}).ConfigGlobalGet(context.Background(), "stagehand.t2s1.isolation.sentinel")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg) // re-set after the above (the getter inherits env)

	g := New(t.TempDir())
	ctx := context.Background()

	if err := g.ConfigGlobalSet(ctx, "alias.isolated", "!stagehand"); err != nil {
		t.Fatalf("ConfigGlobalSet: %v", err)
	}

	// Read from the isolated config — should be found.
	val, found, err := g.ConfigGlobalGet(ctx, "alias.isolated")
	if err != nil {
		t.Fatalf("ConfigGlobalGet: %v", err)
	}
	if !found || val != "!stagehand" {
		t.Fatalf("isolated config: found=%v val=%q, want true %q", found, val, "!stagehand")
	}

	// Verify the real global config is untouched (sentinel unchanged).
	t.Setenv("GIT_CONFIG_GLOBAL", "") // clear so the next read hits the real config
	realAfter, _, _ := (&gitRunner{workDir: os.TempDir()}).ConfigGlobalGet(context.Background(), "stagehand.t2s1.isolation.sentinel")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg) // restore for cleanup

	if realBefore != realAfter {
		t.Errorf("real global config changed: before=%q after=%q", realBefore, realAfter)
	}
}
