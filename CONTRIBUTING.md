# Contributing to Kuargogo (kuargogo)

Thank you for your interest in contributing to **kuargogo**! We welcome improvements, bug fixes, and new features to make homelab management easier.

## 🛠️ Development Setup

1.  **Prerequisites**:
    *   Go 1.26+
    *   Git

2.  **Clone the repo**:
    ```bash
    git clone https://github.com/DannyStrelok/kuargogo.git
    cd kuargogo
    ```

3.  **Run Tests**:
    Ensure the tests pass before making changes:
    ```bash
    go test ./...
    ```

## 🚀 How to Contribute

1.  **Fork** the repository and clone it locally.
2.  Create a **new branch** for your feature or fix:
    ```bash
    git checkout -b feature/my-new-command
    ```
3.  **Add your command**:
    *   Create a new file in `cmd/kgg/` (e.g., `cmd/kgg/my_cmd.go`).
    *   Register it with `rootCmd`.
4.  **Test your changes**:
    *   Run `go build` to verify compilation.
    *   Add unit tests if applicable.
5.  **Commit** your changes with clear messages.
6.  **Push** to your fork and submit a **Pull Request**.

## 📝 Code Style & Guidelines

*   **Go**: Follow standard Go idioms (Effective Go).
*   **Config**: Use `internal/config` for global settings. Avoid hardcoding values.
*   **Documentation**: If you add a command, please update `COMMANDS.md` and add a brief mention in `README.md` if it's a core feature.

## 🐛 Reporting Bugs

If you find a bug, please open an issue describing:
1.  What you ran (`kgg ...`).
2.  What happened.
3.  What you expected.
4.  Your OS and `kgg` version.

---

Happy Hacking! 🚀
