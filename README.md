# proz

CLI/TUI tool that stores project directories and launches them with configured programs.

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

Opens a program picker. Select one or more programs (code, nvim, claude, etc.), then enter optional arguments. `q`/`esc` skips the picker and saves a path-only project.

```bash
proz
```

Opens the project picker. Selecting a project launches its configured program(s) in that directory. A project with no program prints the path only, for `cd $(proz)`-style shell integration.

```bash
proz ~/projects/my-app
```

Adds `~/projects/my-app` as a project (name = directory basename), without `cd`.

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

A project with 2+ configured programs prompts a second picker to choose which one to launch.

`--json` prints all projects as JSON instead of opening the picker.

### `proz <path>`

Adds `path` as a project (name = basename), same flow as `proz add`, without `cd`.

### `proz add <name>`

Opens the program picker: desktop applications by default, Tab for all PATH binaries. Space toggles a program for multi-select (toggled items pin to top); Enter with none toggled selects the highlighted item. Recent programs sort first. Prompts for arguments after selection.

### `proz delete <name>`

Deletes a project. Without a name, opens the picker to select one.

### `proz edit <name>`

TUI form to update name, programs (space-separated), or arguments. Falls back to text prompts if the TUI is unavailable.

### `proz rm <name>`

Alias for `delete`.

### `proz completion [bash|zsh|fish|powershell]`

Prints a shell completion script. Detects the shell from `$SHELL` if no argument is given.

```bash
source <(proz completion)
```

### `proz help`

Prints usage info.

## Desktop app source

| Platform | Source |
|----------|--------|
| Linux | `.desktop` files from XDG data directories |
| macOS | `.app` bundles in `/Applications` |
| Windows | Registry App Paths and Uninstall entries |

## Recent programs

The last 4 programs picked or launched sort to the top of the picker. Re-running one moves it back to position 1. Stored in `~/.config/proz/recent.json`.

## Coding agent plugin

`.claude-plugin/plugin.json` + `skills/proz-agent/SKILL.md` in this repo provide a Claude Code skill that shells out to `proz list --json` (with `--filter`/`--sort`/`--limit`) to read the project registry, and `proz <path>`/`proz add` to register projects. No daemon or MCP server; install the plugin from this repo and it invokes the `proz` binary directly.

## Contributing

Open a pull request.

## License

GPL v3. See [LICENSE](LICENSE).
