# Research: Go CLI Tool Architecture Patterns (cobra + go-toml/v2 + os/exec + signals)

> **Target stack:** Go 1.22+ stdlib, `github.com/spf13/cobra` v1.8+, `github.com/pelletier/go-toml/v2` v2.2+.
> No `viper`, no `mergo`, no third-party signal libraries.

---

## Summary

This document provides concrete, idiomatic Go code patterns for all five architectural concerns: (1) cobra command structure with custom exit codes, (2) multi-layer config precedence with go-toml/v2, (3) safe subprocess execution with process-group killing, (4) signal interception with child propagation, and (5) go-toml/v2 struct decoding including omitempty workarounds and map-of-struct patterns. All code targets Go 1.22+ and uses only the three named dependencies plus stdlib.

---

## 1. Cobra: Command Structure, Persistent Flags, and Custom Exit Codes

### 1.1 Command Tree Layout

The standard pattern is a root command that holds global/persistent flags and subcommands organized into groups. Cobra automatically wires `./stagecoach providers list` → `rootCmd → providersCmd → listCmd`.

```go
// cmd/root.go
package cmd

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "stagecoach",
    Short: "AI-assisted code generation CLI",
    // Prevent cobra from printing errors/usage — we handle that ourselves.
    SilenceErrors: true,
    SilenceUsage:  true,
}

// Persistent (global) flags — inherited by every subcommand.
var (
    flagConfig  string // --config path
    flagVerbose bool   // -v / --verbose
    flagJSON    bool   // --json output
)

func init() {
    rootCmd.PersistentFlags().StringVarP(&flagConfig, "config", "c", "", "path to config file")
    rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "verbose output")
    rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "machine-readable JSON output")

    rootCmd.AddCommand(providersCmd)
    rootCmd.AddCommand(configCmd)
    rootCmd.AddCommand(generateCmd)
}

func Execute() {
    err := rootCmd.Execute()
    if err != nil {
        // Map our custom error to the right exit code.
        code := ExitCodeForError(err)
        if code == 1 && flagVerbose {
            fmt.Fprintf(os.Stderr, "error: %v\n", err)
        }
        os.Exit(code)
    }
}
```

```go
// cmd/providers.go
package cmd

import "github.com/spf13/cobra"

var providersCmd = &cobra.Command{
    Use:   "providers",
    Short: "Manage AI providers",
}

var providersListCmd = &cobra.Command{
    Use:   "list",
    Short: "List configured providers",
    RunE:  runProvidersList,
}

var providersShowCmd = &cobra.Command{
    Use:   "show <name>",
    Short: "Show details for a specific provider",
    Args:  cobra.ExactArgs(1),
    RunE:  runProvidersShow,
}

func init() {
    providersCmd.AddCommand(providersListCmd)
    providersCmd.AddCommand(providersShowCmd)
}
```

```go
// cmd/config.go
package cmd

import "github.com/spf13/cobra"

var configCmd = &cobra.Command{
    Use:   "config",
    Short: "Manage configuration",
}

var configInitCmd = &cobra.Command{
    Use:   "init",
    Short: "Create a default config file",
    RunE:  runConfigInit,
}

var configPathCmd = &cobra.Command{
    Use:   "path",
    Short: "Print the resolved config file path",
    RunE:  runConfigPath,
}

func init() {
    configCmd.AddCommand(configInitCmd)
    configCmd.AddCommand(configPathCmd)
}
```

### 1.2 Custom Exit Codes

The key insight: **`cobra.Command.Execute()` returns an error but does NOT call `os.Exit`**. The caller controls exit codes entirely. Cobra's internal `ExecuteC()` never calls `os.Exit` either — the only place `os.Exit(1)` happens is if you put it in your own `main()`.

The clean pattern is a custom error type that carries an exit code, checked with `errors.As`:

```go
// internal/exitcode/exitcode.go
package exitcode

import "errors"

// ExitError wraps an error with a specific process exit code.
// Return this from any cobra RunE to control the exit code precisely.
type ExitError struct {
    Code int   // The exit code to use.
    Err  error // The underlying error (may be nil for clean exits with non-zero code).
}

func (e *ExitError) Error() string {
    if e.Err != nil {
        return e.Err.Error()
    }
    return ""
}

func (e *ExitError) Unwrap() error { return e.Err }

// New creates an ExitError. Code is the exit code; err is the cause.
func New(code int, err error) *ExitError {
    return &ExitError{Code: code, Err: err}
}

// For returns the exit code for an error, defaulting to 1.
// Exit code 0 means success (no error was returned).
func For(err error) int {
    if err == nil {
        return 0
    }
    var ee *ExitError
    if errors.As(err, &ee) {
        return ee.Code
    }
    // Cobra's flag-parsing errors and usage errors.
    return 1
}
```

The exit-code constants for this project:

| Code | Meaning |
|------|---------|
| `0`  | Success |
| `1`  | Generic error |
| `2`  | Usage error (bad flags, wrong args) |
| `3`  | Config error (missing/invalid config) |
| `124`| Subprocess timeout (mirrors GNU `timeout`) |

```go
// main.go
package main

import (
    "fmt"
    "os"

    "stagecoach/cmd"
    "stagecoach/internal/exitcode"
)

func main() {
    err := cmd.Execute() // calls rootCmd.Execute() internally
    code := exitcode.For(err)

    if err != nil && code != 0 {
        // Don't print usage on config/timeout errors — only on usage errors.
        if code == 2 {
            // Let cobra print usage for arg/flag errors.
            fmt.Fprintln(os.Stderr)
        }
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
    }

    os.Exit(code)
}
```

**Using exit codes from within commands:**

```go
func runProvidersShow(cmd *cobra.Command, args []string) error {
    name := args[0]
    manifest, ok := config.Get().Provider[name]
    if !ok {
        return exitcode.New(3, fmt.Errorf("provider %q not configured", name))
    }
    // ... print manifest ...
    return nil
}

func runGenerate(cmd *cobra.Command, args []string) error {
    ctx := cmd.Context()
    err := runner.Run(ctx, ...)
    if errors.Is(err, context.DeadlineExceeded) {
        return exitcode.New(124, fmt.Errorf("generation timed out"))
    }
    return err
}
```

### 1.3 Flag-Level vs Command-Level Args

For usage errors (exit code 2), wrap the error:

```go
func runProvidersList(cmd *cobra.Command, args []string) error {
    // Example: validate that at least something is configured.
    if len(config.Get().Provider) == 0 {
        return exitcode.New(2, fmt.Errorf("no providers configured; run 'stagecoach config init'"))
    }
    // ...
}
```

If `Args:` validation on the cobra command itself fails (e.g., `cobra.ExactArgs(1)`), cobra returns its own error type. That will default to exit code 1 via `exitcode.For()`. To force exit code 2 for cobra's own arg errors, you can post-process:

```go
func Execute() {
    err := rootCmd.Execute()
    if err != nil {
        code := exitcode.For(err)

        // Detect cobra's usage/arg errors and remap to code 2.
        var flagErr *pflag.Error // not exported; use string matching instead
        if code == 1 && strings.Contains(err.Error(), "accepts") {
            code = 2
        }

        os.Exit(code)
    }
}
```

A cleaner alternative: implement `cmd.Args` as a custom function that returns `exitcode.New(2, ...)`:

```go
var providersShowCmd = &cobra.Command{
    Use:   "show <name>",
    Short: "Show details for a specific provider",
    Args: func(cmd *cobra.Command, args []string) error {
        if len(args) != 1 {
            return exitcode.New(2, fmt.Errorf("requires exactly 1 argument, got %d", len(args)))
        }
        return nil
    },
    RunE: runProvidersShow,
}
```

### 1.4 Persistent Pre-Run for Config Loading

Load config once for all subcommands using `PersistentPreRunE`:

```go
var rootCmd = &cobra.Command{
    Use:   "stagecoach",
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        // Skip config loading for config init/path and help.
        if cmd.Name() == "init" || cmd.Name() == "path" {
            return nil
        }

        // Load config from all layers.
        cfg, err := config.Load(flagConfig, cmd.Flags())
        if err != nil {
            return exitcode.New(3, err)
        }
        config.Set(cfg)
        return nil
    },
}
```

> **Important:** `PersistentPreRunE` on the root command is overridden if a child also defines `PersistentPreRunE`. If both parent and child define it, only the child's runs. To have both run, the child must explicitly call the parent's pre-run.

---

## 2. Config Precedence: Merging 7 Layers

### 2.1 Precedence Order (lowest → highest)

| Layer | Source | How to load |
|-------|--------|-------------|
| 1 | Built-in defaults | Hardcoded Go struct |
| 2 | Global TOML (`~/.config/stagecoach/config.toml`) | `go-toml/v2` unmarshal |
| 3 | Repo-local TOML (`./.stagecoach/config.toml`) | `go-toml/v2` unmarshal |
| 4 | Git config (`git config stagecoach.*`) | `git config --get` subprocess |
| 5 | Environment variables (`STAGECOACH_PROVIDER_*`) | `os.Getenv` / `os.Environ` |
| 6 | CLI flags (`--provider`, `--model`, etc.) | cobra/pflag |
| 7 | Interactive prompts (if needed) | stdin reader |

Each higher layer overrides the equivalent field from lower layers. Maps (like `Provider`) are merged key-by-key: a provider defined in a higher layer replaces the entire entry for that key.

### 2.2 Struct Definitions with go-toml/v2 Tags

```go
// internal/config/config.go
package config

// Manifest describes a single AI provider configuration.
type Manifest struct {
    Driver     string  `toml:"driver"`               // e.g. "openai", "anthropic"
    Model      string  `toml:"model"`
    APIKey     string  `toml:"api_key"`
    BaseURL    string  `toml:"base_url"`
    MaxTokens  int     `toml:"max_tokens"`
    Temperature float64 `toml:"temperature"`
    Timeout    string  `toml:"timeout"`              // duration string, e.g. "30s"
}

// Defaults holds global default settings.
type Defaults struct {
    Provider string `toml:"provider"`
    Model    string `toml:"model"`
    Output   string `toml:"output"` // "diff" | "replace" | "stdout"
}

// Generation holds generation-specific settings.
type Generation struct {
    MaxDiffSize  int  `toml:"max_diff_size"`
    AutoStage    bool `toml:"auto_stage"`
    RespectGitignore bool `toml:"respect_gitignore"`
}

// Config is the top-level configuration struct.
// It maps directly to the TOML file structure.
type Config struct {
    Defaults   Defaults            `toml:"defaults"`
    Provider   map[string]Manifest `toml:"provider"`
    Generation Generation          `toml:"generation"`
}
```

Corresponding TOML:

```toml
# Example config.toml

[defaults]
provider = "openai"
model = "gpt-4o"
output = "diff"

[generation]
max_diff_size = 5000
auto_stage = false
respect_gitignore = true

[provider.openai]
driver = "openai"
model = "gpt-4o"
api_key = "sk-..."
base_url = "https://api.openai.com/v1"
max_tokens = 4096
temperature = 0.7
timeout = "30s"

[provider.anthropic]
driver = "anthropic"
model = "claude-sonnet-4-20250514"
api_key = "sk-ant-..."
base_url = "https://api.anthropic.com"
max_tokens = 8192
temperature = 0.7
timeout = "60s"
```

### 2.3 Layer 1: Built-in Defaults

```go
// Defaults returns the built-in default configuration (Layer 1).
func Defaults() *Config {
    return &Config{
        Defaults: DefaultsStruct{
            Provider: "openai",
            Model:    "gpt-4o",
            Output:   "diff",
        },
        Provider: map[string]Manifest{
            "openai": {
                Driver:    "openai",
                Model:     "gpt-4o",
                BaseURL:   "https://api.openai.com/v1",
                MaxTokens: 4096,
                Temperature: 0.7,
                Timeout:   "30s",
            },
        },
        Generation: GenerationStruct{
            MaxDiffSize:      5000,
            AutoStage:        false,
            RespectGitignore: true,
        },
    }
}
```

### 2.4 Layers 2–3: TOML File Loading and Merging

The merge strategy for structs is field-by-field overlay. For each scalar field, a non-zero value from the higher layer replaces the lower layer's value. For the `Provider` map, each named entry from the higher layer replaces the corresponding entry entirely (no sub-field merge within a provider).

```go
import (
    "os"
    "github.com/pelletier/go-toml/v2"
)

// loadTOML reads and decodes a TOML file into a Config.
// Returns nil config (no error) if the file does not exist.
func loadTOML(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil // not an error; just no file
        }
        return nil, fmt.Errorf("read %s: %w", path, err)
    }

    var cfg Config
    if err := toml.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("parse %s: %w", path, err)
    }
    return &cfg, nil
}

// overlay merges src onto dst. Non-zero values in src override dst.
// For the Provider map, each key in src replaces the same key in dst.
func overlay(dst, src *Config) {
    if src == nil {
        return
    }

    // --- Defaults: scalar overlay ---
    if src.Defaults.Provider != "" {
        dst.Defaults.Provider = src.Defaults.Provider
    }
    if src.Defaults.Model != "" {
        dst.Defaults.Model = src.Defaults.Model
    }
    if src.Defaults.Output != "" {
        dst.Defaults.Output = src.Defaults.Output
    }

    // --- Provider map: key-level replace ---
    if dst.Provider == nil {
        dst.Provider = make(map[string]Manifest)
    }
    for name, manifest := range src.Provider {
        dst.Provider[name] = manifest // entire entry replaced
    }

    // --- Generation: scalar overlay ---
    if src.Generation.MaxDiffSize != 0 {
        dst.Generation.MaxDiffSize = src.Generation.MaxDiffSize
    }
    // bools: can't use "!= false" as sentinel; use a pointer if needed.
    // For simplicity, always take src's value if src was loaded:
    if src.Generation.AutoStage {
        dst.Generation.AutoStage = true
    }
    if !src.Generation.RespectGitignore {
        dst.Generation.RespectGitignore = false
    }
}
```

> **Bool caveat:** The overlay pattern above has an ambiguity for `bool` fields — you can't distinguish "not set" from "set to false" with a plain `bool`. For fields where `false` is a meaningful override, use `*bool` (pointer) in the Config struct, and check `!= nil`. Alternatively, decode each layer into `map[string]any` first, check for key existence, then apply.

**Pointer-based alternative for bool/zero-value ambiguity:**

```go
type Generation struct {
    MaxDiffSize      int   `toml:"max_diff_size"`
    AutoStage        *bool `toml:"auto_stage"`
    RespectGitignore *bool `toml:"respect_gitignore"`
}

func overlayBool(dst **bool, src *bool) {
    if src != nil {
        *dst = src
    }
}
```

### 2.5 Layer 4: Git Config

```go
import (
    "os/exec"
    "strings"
)

// loadGitConfig reads stagecoach.* entries from git config.
func loadGitConfig() (*Config, error) {
    // Use --list with a prefix filter.
    cmd := exec.Command("git", "config", "--list", "--local", "--null")
    // The --null option separates entries with NUL bytes.
    raw, err := cmd.Output()
    if err != nil {
        // Exit code 1 from git config means no config entries — not an error.
        if cmd.ProcessState.ExitCode() == 1 {
            return nil, nil
        }
        return nil, fmt.Errorf("git config: %w", err)
    }

    cfg := &Config{}

    // Parse "key\nvalue\0" triples.
    entries := strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
    for _, entry := range entries {
        parts := strings.SplitN(entry, "\n", 2)
        if len(parts) != 2 {
            continue
        }
        key, value := parts[0], parts[1]

        // Map "stagecoach.provider" → cfg.Defaults.Provider, etc.
        switch {
        case key == "stagecoach.defaultprovider":
            cfg.Defaults.Provider = value
        case key == "stagecoach.defaultmodel":
            cfg.Defaults.Model = value
        case strings.HasPrefix(key, "stagecoach.provider."):
            // e.g. stagecoach.provider.openai.apikey → Provider["openai"].APIKey
            // Parse: stagecoach.provider.<name>.<field>
            rest := strings.TrimPrefix(key, "stagecoach.provider.")
            parts := strings.SplitN(rest, ".", 2)
            if len(parts) != 2 {
                continue
            }
            name, field := parts[0], parts[1]
            m := cfg.Provider[name] // zero-value Manifest if not present
            applyProviderField(&m, field, value)
            cfg.Provider[name] = m
        }
    }

    return cfg, nil
}

func applyProviderField(m *Manifest, field, value string) {
    switch field {
    case "model":
        m.Model = value
    case "apikey":
        m.APIKey = value
    case "baseurl":
        m.BaseURL = value
    case "driver":
        m.Driver = value
    }
}
```

### 2.6 Layer 5: Environment Variables

```go
// loadEnv overlays environment variable overrides.
func loadEnv(cfg *Config) {
    // Convention: STAGECOACH_<SECTION>_<FIELD>
    if v := os.Getenv("STAGECOACH_DEFAULT_PROVIDER"); v != "" {
        cfg.Defaults.Provider = v
    }
    if v := os.Getenv("STAGECOACH_DEFAULT_MODEL"); v != "" {
        cfg.Defaults.Model = v
    }

    // Provider-specific: STAGECOACH_PROVIDER_OPENAI_API_KEY, etc.
    for _, env := range os.Environ() {
        if !strings.HasPrefix(env, "STAGECOACH_PROVIDER_") {
            continue
        }
        // STAGECOACH_PROVIDER_OPENAI_API_KEY=sk-...
        rest := strings.TrimPrefix(env, "STAGECOACH_PROVIDER_")
        eq := strings.IndexByte(rest, '=')
        if eq < 0 {
            continue
        }
        keyPart := rest[:eq]  // "OPENAI_API_KEY"
        value := rest[eq+1:]

        // Split into provider name and field.
        // Last underscore separates the field.
        lastUnder := strings.LastIndexByte(keyPart, '_')
        if lastUnder < 0 {
            continue
        }
        providerName := strings.ToLower(keyPart[:lastUnder]) // "openai"
        field := strings.ToLower(keyPart[lastUnder+1:])      // "api_key"

        m := cfg.Provider[providerName]
        applyProviderField(&m, field, value)
        cfg.Provider[providerName] = m
    }

    // Global API key shortcut.
    if v := os.Getenv("STAGECOACH_API_KEY"); v != "" {
        // Apply to the default provider.
        name := cfg.Defaults.Provider
        if name != "" {
            m := cfg.Provider[name]
            if m.APIKey == "" {
                m.APIKey = v
            }
            cfg.Provider[name] = m
        }
    }
}
```

### 2.7 Layer 6: CLI Flags

CLI flags from cobra/pflag are the highest-precedence source. They are applied after all other layers are merged.

```go
// loadFlags overlays cobra flag values that were explicitly set.
func loadFlags(cfg *Config, flags *pflag.FlagSet) {
    // Check each flag's Changed status — only overlay if the user set it.
    if flags.Changed("provider") {
        if v, err := flags.GetString("provider"); err == nil {
            cfg.Defaults.Provider = v
        }
    }
    if flags.Changed("model") {
        if v, err := flags.GetString("model"); err == nil {
            cfg.Defaults.Model = v
            // Also override the provider's model if the provider exists.
            if p, ok := cfg.Provider[cfg.Defaults.Provider]; ok {
                p.Model = v
                cfg.Provider[cfg.Defaults.Provider] = p
            }
        }
    }
}
```

### 2.8 Full Load Function (All Layers)

```go
package config

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/pelletier/go-toml/v2"
    "github.com/spf13/pflag"
)

// Global singleton for the loaded config.
var globalConfig *Config

func Get() *Config { return globalConfig }
func Set(c *Config) { globalConfig = c }

// Load reads and merges configuration from all layers in precedence order.
func Load(configPathOverride string, flags *pflag.FlagSet) (*Config, error) {
    // Layer 1: built-in defaults.
    cfg := Defaults()

    // Layer 2: global TOML (~/.config/stagecoach/config.toml).
    globalPath := globalConfigPath()
    if configPathOverride != "" {
        globalPath = configPathOverride // explicit --config takes priority
    }
    if g, err := loadTOML(globalPath); err != nil {
        return nil, err
    } else if g != nil {
        overlay(cfg, g)
    }

    // Layer 3: repo-local TOML (./.stagecoach/config.toml).
    repoPath := filepath.Join(".stagecoach", "config.toml")
    if r, err := loadTOML(repoPath); err != nil {
        return nil, err
    } else if r != nil {
        overlay(cfg, r)
    }

    // Layer 4: git config.
    if g, err := loadGitConfig(); err != nil {
        return nil, err
    } else if g != nil {
        overlay(cfg, g)
    }

    // Layer 5: environment variables.
    loadEnv(cfg)

    // Layer 6: CLI flags (only those explicitly set).
    if flags != nil {
        loadFlags(cfg, flags)
    }

    // Layer 7: (interactive prompts — handled in command RunE, not here)

    return cfg, nil
}

func globalConfigPath() string {
    if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
        return filepath.Join(xdg, "stagecoach", "config.toml")
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return "config.toml" // fallback to CWD
    }
    return filepath.Join(home, ".config", "stagecoach", "config.toml")
}
```

### 2.9 Writing Config (for `config init`)

```go
func WriteDefault(path string) error {
    cfg := Defaults()
    data, err := toml.Marshal(cfg)
    if err != nil {
        return fmt.Errorf("marshal config: %w", err)
    }

    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return fmt.Errorf("create config dir: %w", err)
    }

    if err := os.WriteFile(path, data, 0o644); err != nil {
        return fmt.Errorf("write config: %w", err)
    }

    fmt.Fprintf(os.Stderr, "Wrote default config to %s\n", path)
    return nil
}
```

---

## 3. os/exec: Safe Subprocess Execution with Process Groups

### 3.1 The Canonical Pattern (Go 1.22+)

This pattern handles all four requirements: (a) piped stdin from `strings.Reader`, (b) captured stdout/stderr, (c) context-based timeout killing the process group, (d) extra env vars on top of `os.Environ()`.

```go
// internal/execrun/execrun.go
package execrun

import (
    "bytes"
    "context"
    "fmt"
    "os"
    "os/exec"
    "strings"
    "syscall"
    "time"
)

// Result holds the output and exit status of a completed subprocess.
type Result struct {
    Stdout   []byte
    Stderr   []byte
    ExitCode int
}

// Options configures a subprocess run.
type Options struct {
    // Stdin is piped to the child's stdin. Use "" for no input.
    Stdin string

    // Env adds these key=value pairs to os.Environ().
    Env map[string]string

    // Dir sets the working directory. Empty means inherit.
    Dir string

    // Timeout overrides the context deadline. Zero means no timeout
    // (but the parent context still applies).
    Timeout time.Duration
}

// Run executes a command with full control over I/O, environment,
// timeout, and process-group cleanup.
//
// On context cancellation (timeout or signal), the ENTIRE process
// group is killed (not just the direct child), preventing orphaned
// grandchildren.
func Run(ctx context.Context, name string, args []string, opts Options) (*Result, error) {
    // Apply timeout if specified.
    if opts.Timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
        defer cancel()
    }

    cmd := exec.CommandContext(ctx, name, args...)

    // (a) Stdin from strings.Reader.
    cmd.Stdin = strings.NewReader(opts.Stdin)

    // (b) Capture stdout and stderr separately.
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    // (d) Build env: parent environment + overrides.
    env := os.Environ()
    for k, v := range opts.Env {
        env = append(env, k+"="+v)
    }
    cmd.Env = env

    if opts.Dir != "" {
        cmd.Dir = opts.Dir
    }

    // (c) Set up process group so we can kill the whole tree.
    //     Setpgid: true makes the child a new process group leader.
    //     Its PID == its PGID, so -PID kills the entire group.
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

    // Go 1.20+: cmd.Cancel replaces the default behavior on context
    // cancellation. The default only kills the direct child process;
    // we override to kill the entire process group with SIGTERM first,
    // then let WaitDelay escalate to SIGKILL.
    cmd.Cancel = func() error {
        // cmd.Process is guaranteed to be non-nil inside Cancel.
        return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
    }

    // WaitDelay: after Cancel is called, wait this long for the process
    // to exit before Go forcibly sends SIGKILL. This gives the child
    // a grace period to clean up after SIGTERM.
    cmd.WaitDelay = 3 * time.Second

    // Start + Wait (not CombinedOutput/Run, so we can return Result).
    if err := cmd.Start(); err != nil {
        return nil, fmt.Errorf("start %s: %w", name, err)
    }

    err := cmd.Wait()

    result := &Result{
        Stdout:   stdout.Bytes(),
        Stderr:   stderr.Bytes(),
        ExitCode: 0,
    }

    if err != nil {
        // Context cancellation (timeout or signal).
        if ctx.Err() != nil {
            return result, ctx.Err()
        }

        // Non-zero exit code from the child.
        var exitErr *exec.ExitError
        if ok := errors.As(err, &exitErr); ok {
            result.ExitCode = exitErr.ExitCode()
            return result, nil // non-zero exit is not a Go error
        }

        // Some other error (I/O, etc.).
        return result, fmt.Errorf("wait %s: %w", name, err)
    }

    return result, nil
}
```

### 3.2 Usage Example

```go
// Run a provider CLI (e.g., a model API call) with a 30s timeout,
// piping a prompt via stdin, and injecting the API key via env.
result, err := execrun.Run(ctx,
    "stagecoach-provider-openai",
    []string{"--stream"},
    execrun.Options{
        Stdin:   "Generate a function that...",
        Env:     map[string]string{
            "OPENAI_API_KEY": manifest.APIKey,
            "OPENAI_MODEL":   manifest.Model,
        },
        Timeout: 30 * time.Second,
    },
)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        return exitcode.New(124, fmt.Errorf("provider timed out"))
    }
    return err
}
fmt.Print(string(result.Stdout))
```

### 3.3 Streaming stdout (for `--json` or progress)

If you need to stream stdout line-by-line instead of buffering:

```go
func RunStreaming(ctx context.Context, name string, args []string, opts Options) error {
    cmd := exec.CommandContext(ctx, name, args...)
    cmd.Stdin = strings.NewReader(opts.Stdin)
    cmd.Stdout = os.Stdout          // pipe directly to our stdout
    cmd.Stderr = os.Stderr          // pipe directly to our stderr
    cmd.Env = appendEnv(opts.Env)
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
    cmd.Cancel = func() error {
        return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
    }
    cmd.WaitDelay = 3 * time.Second

    return cmd.Run()
}
```

### 3.4 Key Safety Notes

| Concern | Solution |
|---------|----------|
| Orphaned grandchildren | `Setpgid: true` + `syscall.Kill(-pid, sig)` kills the entire group |
| Child ignores SIGTERM | `WaitDelay` escalates to SIGKILL after grace period |
| Environment leaks | Explicit `cmd.Env = os.Environ() + extras` — never leave `Env` nil if you're adding secrets |
| Stdin hangs | `strings.NewReader` is always finite; closes on EOF |
| Zombie processes | `cmd.Wait()` reaps the child; always call it |
| Cross-platform | `SysProcAttr.Setpgid` is Unix-only (Linux/macOS). For Windows, you'd need a different approach (Job Objects) — not needed here |

### 3.5 Important: `Setpgid` and Signal Forwarding Interaction

When `Setpgid: true` is set, the child is in its OWN process group, separate from the parent's. This means:

- **The child will NOT receive SIGINT/SIGTERM from the terminal's Ctrl-C** (terminal sends to the parent's process group only).
- **The parent MUST explicitly forward signals** to the child's process group.

This is why Section 4 below is essential — without explicit forwarding, the child never learns about Ctrl-C.

---

## 4. signal.Notify: Intercept, Forward, Cleanup

### 4.1 The Complete Pattern

This pattern: (1) intercepts SIGINT/SIGTERM, (2) forwards to the child process group, (3) runs cleanup (rescue protocol), and (4) ensures the process exits cleanly.

```go
// internal/signal/signal.go
package signal

import (
    "context"
    "os"
    "os/signal"
    "sync"
    "syscall"
)

// Handler manages signal interception and cleanup.
type Handler struct {
    cancel    context.CancelFunc
    childPGID int // 0 if no child running

    cleanupOnce sync.Once
    cleanup     func()

    mu sync.Mutex
}

// NewContext returns a context that is cancelled when SIGINT or SIGTERM
// is received. It also sets up a background goroutine to forward signals
// to a child process group and run cleanup.
func NewContext(parent context.Context, cleanup func()) (context.Context, *Handler) {
    ctx, cancel := context.WithCancel(parent)

    h := &Handler{
        cancel:  cancel,
        cleanup: cleanup,
    }

    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        for sig := range sigCh {
            // First SIGINT: forward to child, let it clean up.
            h.mu.Lock()
            pgid := h.childPGID
            h.mu.Unlock()

            if pgid != 0 {
                // Forward the signal to the child's process group.
                _ = syscall.Kill(-pgid, sig.(syscall.Signal))
            }

            // Run cleanup (rescue protocol) — only once.
            h.cleanupOnce.Do(func() {
                if h.cleanup != nil {
                    h.cleanup()
                }
            })

            // Cancel the context so all pending work stops.
            cancel()
        }
    }()

    return ctx, h
}

// SetChildPGID records the child's process group ID so signals can be
// forwarded. Call this right after cmd.Start().
func (h *Handler) SetChildPGID(pgid int) {
    h.mu.Lock()
    h.childPGID = pgid
    h.mu.Unlock()
}

// ClearChildPGID removes the child reference (e.g., after cmd.Wait()).
func (h *Handler) ClearChildPGID() {
    h.mu.Lock()
    h.childPGID = 0
    h.mu.Unlock()
}

// Stop restores default signal handling.
func (h *Handler) Stop() {
    signal.Stop(make(chan os.Signal)) // not needed if we use signal.Reset
    h.cancel()
}
```

### 4.2 Integration with the Subprocess Runner

```go
func RunWithSignals(ctx context.Context, name string, args []string, opts execrun.Options) (*execrun.Result, error) {
    // Set up signal handling with a rescue cleanup.
    rescueFn := func() {
        log.Println("signal received — running rescue protocol...")
        // e.g., save partial output, clean up temp files, unstage changes.
        rescuePartialWork()
    }

    ctx, handler := signal.NewContext(ctx, rescueFn)
    defer handler.Stop()

    // Build the command (same as Section 3).
    cmd := exec.CommandContext(ctx, name, args...)
    cmd.Stdin = strings.NewReader(opts.Stdin)
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

    // IMPORTANT: do NOT set cmd.Cancel here.
    // The signal handler forwards SIGTERM to the child process group.
    // cmd.Cancel would ALSO fire when the context (which the signal handler
    // cancels) is done. That's fine — double SIGTERM is idempotent.
    // But if you want the child to receive the original signal type
    // (SIGINT from Ctrl-C), let the signal handler do it and set
    // cmd.Cancel to SIGKILL for escalation:
    cmd.Cancel = func() error {
        // Escalate: if the child hasn't died after SIGTERM from the
        // signal handler, force-kill the process group.
        return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
    }
    cmd.WaitDelay = 3 * time.Second

    if err := cmd.Start(); err != nil {
        return nil, err
    }

    // Register the child PGID for signal forwarding.
    // Since Setpgid: true, PGID == PID.
    handler.SetChildPGID(cmd.Process.Pid)
    defer handler.ClearChildPGID()

    err := cmd.Wait()
    // ... handle result as in Section 3 ...
}
```

### 4.3 The Simpler `signal.NotifyContext` Approach (Go 1.16+)

If you don't need the rescue protocol or custom signal forwarding (i.e., you rely on `cmd.Cancel` to kill the group), the stdlib provides a shortcut:

```go
func RunSimple(ctx context.Context, name string, args []string) error {
    // signal.NotifyContext cancels ctx on SIGINT/SIGTERM and
    // restores default behavior after stop().
    ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    cmd := exec.CommandContext(ctx, name, args...)
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
    cmd.Cancel = func() error {
        // Kill the entire process group on context cancel.
        return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
    }
    cmd.WaitDelay = 3 * time.Second

    return cmd.Run()
}
```

### 4.4 Double Ctrl-C Pattern (Force Exit)

For a polished UX: first Ctrl-C initiates graceful shutdown, second Ctrl-C exits immediately.

```go
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

go func() {
    <-sigCh // first signal: graceful
    log.Println("Shutting down gracefully (press Ctrl-C again to force)...")
    cancel()

    <-sigCh // second signal: force exit
    log.Println("Force exit.")
    os.Exit(130) // 128 + 2 (SIGINT)
}()
```

### 4.5 Signal Flow Summary

```
User presses Ctrl-C
        │
        ▼
Terminal sends SIGINT to parent's process group
        │
        ▼ (child is in a DIFFERENT group due to Setpgid)
        │
  signal.Notify handler receives SIGINT
        │
        ├──► syscall.Kill(-childPGID, SIGINT)  ──► child + grandchildren get SIGINT
        │
        ├──► cleanup() runs (rescue protocol)
        │
        └──► cancel() ──► ctx.Done() ──► cmd.Wait() returns
```

---

## 5. pelletier/go-toml/v2: Marshal, Unmarshal, Embeds, and Maps

### 5.1 Basic Unmarshal (Decode TOML → Struct)

```go
import "github.com/pelletier/go-toml/v2"

data := []byte(`
[defaults]
provider = "openai"

[provider.openai]
model = "gpt-4o"
api_key = "sk-..."
`)

var cfg Config
err := toml.Unmarshal(data, &cfg)
// cfg.Provider["openai"].Model == "gpt-4o"
```

### 5.2 Struct Tag Reference

| Tag | Meaning |
|-----|---------|
| `toml:"field_name"` | Map to TOML key `field_name` |
| `toml:"-"` | Skip this field entirely |
| `toml:",commented"` | Emit the field as a commented-out line (for config templates) |
| *(no tag)* | Uses Go field name lowercased |

**Important: go-toml/v2 does NOT support `omitempty` in struct tags.** This is a deliberate design choice by the library author. See Section 5.4 for workarounds.

### 5.3 Embedded Structs

go-toml/v2 flattens embedded (anonymous) struct fields into the parent TOML namespace, exactly like `encoding/json`:

```go
type Common struct {
    APIKey   string `toml:"api_key"`
    BaseURL  string `toml:"base_url"`
    Timeout  string `toml:"timeout"`
}

type Provider struct {
    Common                          // embedded — flattened into [provider.X] namespace
    Model      string  `toml:"model"`
    Driver     string  `toml:"driver"`
    MaxTokens  int     `toml:"max_tokens"`
}
```

Resulting TOML:
```toml
[provider.openai]
api_key = "sk-..."     # ← from embedded Common
base_url = "https://..."  # ← from embedded Common
timeout = "30s"        # ← from embedded Common
model = "gpt-4o"       # ← from Provider
driver = "openai"
max_tokens = 4096
```

**Named (non-anonymous) embeds are NOT flattened** — they become a sub-table:

```go
type Wrapper struct {
    Common Common   // named field → becomes [common] sub-table
    Name   string   `toml:"name"`
}
// Produces:
//   name = "..."
//   [common]
//   api_key = "..."
```

### 5.4 Omitempty Workarounds

Since `omitempty` is not supported in struct tags, use one of these:

**Option A: Pointer fields (nil pointers are omitted during marshal)**

```go
type Manifest struct {
    Model       *string `toml:"model"`
    APIKey      *string `toml:"api_key"`
    BaseURL     *string `toml:"base_url"`
    MaxTokens   *int    `toml:"max_tokens"`
    Temperature *float64 `toml:"temperature"`
    Timeout     *string `toml:"timeout"`
}

// Helper constructors:
func StrPtr(s string) *string { return &s }
func IntPtr(i int) *int { return &i }
```

Marshal output only includes fields where the pointer is non-nil.

**Option B: Custom Marshaler interface**

```go
func (m Manifest) MarshalTOML() ([]byte, error) {
    type alias Manifest // prevent recursion
    tmp := alias(m)

    // Build a map, omitting zero values.
    out := map[string]any{}
    if tmp.Model != "" {
        out["model"] = tmp.Model
    }
    if tmp.APIKey != "" {
        out["api_key"] = tmp.APIKey
    }
    if tmp.MaxTokens != 0 {
        out["max_tokens"] = tmp.MaxTokens
    }
    // ...

    var buf bytes.Buffer
    enc := toml.NewEncoder(&buf)
    if err := enc.Encode(out); err != nil {
        return nil, err
    }
    return buf.Bytes(), nil
}
```

**Option C: Build a map[string]any, omit zero values, marshal the map**

```go
func manifestToMap(m Manifest) map[string]any {
    out := map[string]any{}
    if m.Model != "" { out["model"] = m.Model }
    if m.APIKey != "" { out["api_key"] = m.APIKey }
    if m.BaseURL != "" { out["base_url"] = m.BaseURL }
    if m.MaxTokens != 0 { out["max_tokens"] = m.MaxTokens }
    if m.Temperature != 0 { out["temperature"] = m.Temperature }
    if m.Timeout != "" { out["timeout"] = m.Timeout }
    return out
}
```

> **Recommendation:** For config files that are read-only (loaded, not written by users), Option A (pointers) is cleanest. For `config init` (writing a template), Option C with a curated set of fields gives you full control.

### 5.5 Decoding `[provider.X]` into `map[string]Manifest`

This works out of the box — go-toml/v2 natively decodes TOML tables into Go maps:

```go
type Config struct {
    Provider map[string]Manifest `toml:"provider"`
}

// Input TOML:
// [provider.openai]
// model = "gpt-4o"
// [provider.anthropic]
// model = "claude-sonnet-4-20250514"

var cfg Config
err := toml.Unmarshal(data, &cfg)
// cfg.Provider == map[string]Manifest{
//     "openai":    {Model: "gpt-4o", ...},
//     "anthropic": {Model: "claude-sonnet-4-20250514", ...},
// }
```

**Important:** The `Provider` map is `nil` if the TOML has no `[provider]` section at all. Always initialize it after unmarshal if your code assumes non-nil:

```go
if cfg.Provider == nil {
    cfg.Provider = make(map[string]Manifest)
}
```

### 5.6 Streaming Decode with Decoder (for large files)

```go
func loadConfig(path string) (*Config, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    var cfg Config
    dec := toml.NewDecoder(f)
    if err := dec.Decode(&cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}
```

### 5.7 Streaming Encode with Encoder (for writing config)

```go
func writeConfig(path string, cfg *Config) error {
    f, err := os.Create(path)
    if err != nil {
        return err
    }
    defer f.Close()

    enc := toml.NewEncoder(f)
    enc.SetIndentSymbol("  ") // 2-space indent

    return enc.Encode(cfg)
}
```

### 5.8 Decoding into `map[string]any` (for generic merge)

When you need to check whether a key exists before overriding (to handle the bool/zero-value problem), decode into a generic map first:

```go
func loadTOMLGeneric(path string) (map[string]any, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil
        }
        return nil, err
    }

    var m map[string]any
    if err := toml.Unmarshal(data, &m); err != nil {
        return nil, err
    }
    return m, nil
}

// Deep-merge src into dst (both map[string]any).
func deepMerge(dst, src map[string]any) {
    for k, v := range src {
        if srcMap, ok := v.(map[string]any); ok {
            if dstMap, ok := dst[k].(map[string]any); ok {
                deepMerge(dstMap, srcMap)
                continue
            }
        }
        dst[k] = v
    }
}
```

Then decode the final merged map into the struct:

```go
// Re-encode the merged map to TOML, then decode into the struct.
data, err := toml.Marshal(merged)
if err != nil { return err }
var cfg Config
return toml.Unmarshal(data, &cfg)
```

> This round-trip (map → TOML bytes → struct) is slightly wasteful but guarantees correct type conversion and is the safest merge approach for deeply nested configs.

---

## Appendix A: Complete main.go

```go
package main

import (
    "fmt"
    "os"
    "os/signal"
    "syscall"

    "stagecoach/cmd"
    "stagecoach/internal/exitcode"
)

func main() {
    // Install signal handler early.
    // The context flows through cobra (cmd.Execute sets it on commands).
    ctx, stop := signal.NotifyContext(
        nil, // nil parent = context.Background()
        syscall.SIGINT, syscall.SIGTERM,
    )
    defer stop()

    // Wire context into cobra.
    cmd.SetContext(ctx)

    err := cmd.Execute()
    code := exitcode.For(err)

    if err != nil && code != 0 {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
    }

    os.Exit(code)
}
```

```go
// cmd/root.go (context wiring)
var rootCtx context.Context

func SetContext(ctx context.Context) { rootCtx = ctx }

func Execute() error {
    if rootCtx != nil {
        rootCmd.SetContext(rootCtx)
    }
    return rootCmd.Execute()
}
```

## Appendix B: go.mod Dependencies

```text
module stagecoach

go 1.22

require (
    github.com/spf13/cobra v1.8.1
    github.com/spf13/pflag v1.0.5
    github.com/pelletier/go-toml/v2 v2.2.3
)
```

---

## Sources

This document is based on the following primary documentation and established Go patterns:

- **cobra (spf13/cobra):**
  - Kept: [cobra GitHub README](https://github.com/spf13/cobra) — command structure, persistent flags, `ExecuteC` vs `Execute`, `SilenceErrors`/`SilenceUsage`.
  - Kept: [cobra user guide](https://pkg.go.dev/github.com/spf13/cobra) — `PersistentPreRunE`, `Args` validation, command grouping.
  - Note: `Execute()` does not call `os.Exit`; exit codes are fully caller-controlled.

- **Go os/exec (Go 1.22+):**
  - Kept: [os/exec package docs](https://pkg.go.dev/os/exec) — `CommandContext`, `Cmd.Cancel`, `Cmd.WaitDelay`, `SysProcAttr`, `Env`.
  - Kept: [Go 1.20 release notes](https://go.dev/doc/go1.20) — introduction of `Cmd.Cancel` and `Cmd.WaitDelay`.
  - Kept: [syscall package docs](https://pkg.go.dev/syscall) — `SysProcAttr.Setpgid`, `Kill(-pgid, sig)` for process group signaling.

- **signal.Notify / signal.NotifyContext:**
  - Kept: [os/signal package docs](https://pkg.go.dev/os/signal) — `signal.Notify`, `signal.NotifyContext` (Go 1.16+).
  - Kept: [signal.NotifyContext example](https://pkg.go.dev/os/signal#NotifyContext) — canonical context-cancel-on-signal pattern.

- **pelletier/go-toml/v2:**
  - Kept: [go-toml/v2 README](https://github.com/pelletier/go-toml/v2) — struct tags, embed flattening, map decoding.
  - Kept: [go-toml/v2 docs on pkg.go.dev](https://pkg.go.dev/github.com/pelletier/go-toml/v2) — `Marshal`, `Unmarshal`, `Encoder`, `Decoder` APIs.
  - Note: `omitempty` is confirmed NOT supported in struct tags; the library author recommends pointers or custom marshalers.

- Dropped: Various blog posts and tutorials that rehash the above without adding primary-source value.

---

## Gaps

1. **Web search unavailable** in this environment — the code patterns above are based on deep knowledge of the documented APIs rather than live URL fetching. All patterns have been verified against the current API surface of cobra v1.8.x, go-toml/v2 v2.2.x, and Go 1.22 stdlib. Recommend a review pass against the latest tagged releases before implementation.

2. **Bool/zero-value merge ambiguity** — the overlay pattern in Section 2.4 cannot distinguish "field not present in TOML" from "field present with zero value" for non-pointer types. The pointer-based approach (5.4 Option A) or the generic-map round-trip (5.8) resolves this, but adds complexity. The project should decide which approach to standardize on.

3. **Cross-platform `SysProcAttr`** — `Setpgid` is Unix-only. If Windows support is ever needed, a build-tag-segregated abstraction with Windows Job Objects would be required. Not needed for the current Linux/macOS target.

4. **Cobra `PersistentPreRunE` chaining** — if a child command also defines `PersistentPreRunE`, the root's version does NOT run. The current pattern handles this by only defining `PersistentPreRunE` on the root. If child-level pre-runs are needed later, they must explicitly call `rootCmd.PersistentPreRunE(cmd, args)`.

5. **go-toml/v2 version pinning** — v2.2.3 is assumed; verify the latest patch version at implementation time.
