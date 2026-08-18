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

A program picker opens. Pick one or more editors/programs (code, nvim, claude, etc.) and add optional arguments. The project is saved. You can also skip the picker entirely (press `q`/`esc`) to save a path-only project.

```bash
proz
```

Opens the project picker. Select a project and it launches its program(s) in that directory. A project with no program configured just prints the path — that's also useful for `cd $(proz)`-style shell integration.

```bash
proz ~/projects/my-app
```

Adds a project directly from a given path (name defaults to the directory's basename), without needing to `cd` there first.

## Commands

### `proz` (or `proz list`)

Opens the project picker TUI. Shows all saved projects with their program(s) and path, plus a bottom "+ Add project" row that starts the add flow using the current directory.

```
/ search

▸ my-app  (nvim  -> ~/projects/my-app)
  web-app (code  -> ~/projects/web)
  + Add project (current dir)

j/↓ move down · k/↑ move up · / search · enter select · q/esc cancel
```

If a project has more than one program configured, selecting it opens a second picker to choose which one to launch with this time.

`proz list --json` prints all projects as JSON instead of opening the picker — useful for scripting or feeding into other tools.

### `proz <path>`

Adds `path` as a project (basename as the name), same picker flow as `proz add`, without changing directory first.

### `proz add <name>`

Opens the program picker — desktop applications by default, Tab toggles to all PATH binaries. Press space to toggle multiple programs for one project (selected ones pin to the top); Enter with nothing toggled picks just the highlighted one. Recent programs appear first. After selecting, you can add arguments.

### `proz delete <name>`

Removes a project. Without a name, opens the picker to select one.

### `proz edit <name>`

Opens a TUI form to update the name, programs (space-separated), or arguments. Falls back to text prompts if the TUI is unavailable.

### `proz rm <name>`

Alias for `delete`.

### `proz completion [bash|zsh|fish|powershell]`

Prints a shell completion script. Detects the shell from `$SHELL` if no argument is given.

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

Press Tab to switch from desktop apps to all binaries in your PATH. Press space to select multiple programs for one project; press `q`/`esc` to cancel without picking one.

## Recent programs

The last 4 executables you pick or launch are sorted to the top of the list. Re-running an executable moves it back to position 1. Data is stored in `~/.config/proz/recent.json`.

## Contributing

Open a pull request.

## License

GPL v3. See [LICENSE](LICENSE).
