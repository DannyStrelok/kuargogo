# AGENTS.md - kuargogo

## 🎯 Purpose
Rules and patterns for AI to generate consistent, production-level Go code.

---

## ⚡ Execution Mode (MANDATORY)

1. Prefer existing patterns over new ones  
2. Minimize output text, maximize correctness  
3. Do not explain unless asked  
4. Avoid overengineering  
5. Follow repository structure strictly  

---

## 🏗️ Architecture

- Business logic → `internal/*`
- CLI orchestration → `cmd/kgg/*`
- TUI wrappers → `internal/ui/menu/*`

❌ NEVER put logic in `cmd/kgg/`

---

## 🚫 Hard Rules (DO NOT VIOLATE)

### Output
- ❌ `fmt.Println()` in `internal/*`
- ✅ use `io.Writer`

### Process
- ❌ `os.Exit()` in `internal/*`
- ✅ return `error`

### TUI
- MUST return `tea.Cmd`
- MUST capture ALL output with `bytes.Buffer`

### CLI
- MUST pass `os.Stdout`

---

## 📐 Core Patterns

### Service
```go
type Service struct {
    Output io.Writer
    DryRun bool
}

func New(...) *Service {
    return &Service{Output: os.Stdout}
}

func (s *Service) Execute() error {
    fmt.Fprintln(s.Output, "...")
    return nil
}
```

### TUI Action
```go
func Action() tea.Cmd {
    return func() tea.Msg {
        var buf bytes.Buffer

        svc := service.New(...)
        svc.Output = &buf

        err := svc.Execute()

        if err != nil {
            return ResultMsg{Output: buf.String() + "\n\n❌ " + err.Error()}
        }
        return ResultMsg{Output: buf.String() + "\n\n✅ Success"}
    }
}
```

### CLI
```go
svc := service.New(...)
svc.Output = os.Stdout

if err := svc.Execute(); err != nil {
    fmt.Println(err)
    os.Exit(1)
}
```

### Config
cfg := config.GetConfig()

### SSH & WSL Bridge (CRITICAL)
- Use `internal/provision/executor.go` for standard SSH execution (DO NOT reimplement).
- On Windows, Ansible playbooks run under WSL. You MUST synchronize SSH keys to the native Linux filesystem using `deps.SyncSSHKeyToWSL(key)` and convert Windows paths to WSL using `deps.ConvertToWSLPath(path)` to prevent permission (0777) or file access failures.

### Bóveda de Seguridad (Vault)
- Sensitive config variables (S3 tokens, telegram bots, API keys) are vault-encrypted under `!vaultENC:` tags.
- NEVER write plain-text secrets directly to `kuargogo.yaml`. Use the atomic configuration writers in `internal/config` that natively handle encryption/decryption.

---

## 🧪 Testing

- Use `bytes.Buffer`
- Always support `DryRun`

---

## ⚠️ Common Errors

- Partial output capture
- Ignored errors
- Hardcoded values
- Mixing CLI and TUI logic

---

## 🎨 Style

### Errors
```go
return fmt.Errorf("context: %w", err)
```

### Imports order
1. stdlib  
2. third-party  
3. internal  

---

## 🔄 Feature Flow

1. `internal/` → logic  
2. `cmd/kgg/` → CLI  
3. `actions/` → TUI  
4. `tree.go` → menu  

---

## ✅ Pre-Commit

Before marking any task as complete, you MUST run verification via the [Makefile](file:///e:/Development/kuargogo/Makefile):
1. Run `make lint` to ensure there are no linter errors.
2. Run `make test` to ensure all tests pass.
3. Run `make build` to verify it compiles successfully.
Alternatively, you can run `make audit` for a combined check of linter, tests, and security scans.

Checklist:
- No `fmt.Println()` in `internal/`
- No `os.Exit()` in `internal/`
- Uses `io.Writer`
- TUI captures ALL output
- CLI uses `os.Stdout`
- Errors wrapped
- Lint clean (`make lint`)
- Build OK (`make build`)
- Tests passing (`make test`)

---

## 🧠 Principles

- No duplication
- Dependency injection
- Errors > logs
- Deterministic code
