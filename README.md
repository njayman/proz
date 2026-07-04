# proz

Browse and launch projects from a terminal TUI.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/njayman/proz/main/install.sh | bash
```

Windows:

```powershell
powershell -c "irm https://raw.githubusercontent.com/njayman/proz/main/install.ps1 | iex"
```

Or build from source:

```bash
go install github.com/njayman/proz@latest
```

## Quick start

```bash
cd ~/projects/my-app
proz add my-app
```

A program picker opens. Pick an editor (code, nvim, etc.) and add optional arguments. The project is saved.

```bash
proz
```

Opens the project picker. Select a project and it launches your editor in that directory.

## Commands

### `proz` (or `proz list`)

Opens the project picker TUI. Shows all saved projects with their editor and path.

```
/ search

▸ my-app  (nvim  -> ~/projects/my-app)
  web-app (code  -> ~/projects/web)

j/↓ move down · k/↑ move up · / search · enter select · esc cancel
```

The editor launches in the project directory.

### `proz add <name>`

Opens the program picker — desktop applications by default, Tab toggles to all PATH binaries. Recent executables appear first. After selecting an editor, you can add arguments.

### `proz delete <name>`

Removes a project. Without a name, opens the picker to select one.

### `proz edit <name>`

Opens a TUI form to update the name, executable, or arguments. Falls back to text prompts if the TUI is unavailable.

### `proz rm <name>`

Alias for `delete`.

### `proz completion`

Prints a shell completion script for your shell (bash, zsh, fish, powershell). Detects the shell from `$SHELL`.

```bash
source <(proz completion)
```

### `proz help`

Prints usage info.

## Program picker

When running `proz add`, a picker shows desktop applications:

| Platform | Source |
|----------|--------|
| Linux | `.desktop` files from XDG data directories |
| macOS | `.app` bundles in `/Applications` |
| Windows | Registry App Paths and Uninstall entries |

Press Tab to switch from desktop apps to all binaries in your PATH.

## Recent programs

The last 4 executables you pick or launch are sorted to the top of the list. Re-running an executable moves it back to position 1. Data is stored in `~/.config/proz/recent.json`.

## Contributing

Open a pull request.

## License

GPL v3. See [LICENSE](LICENSE).
