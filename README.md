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

### `proz nudges [--json]`

Lists projects that have gone quiet: no registration, no launch through proz, and no git commit touching the folder (on a local branch) for longer than the stale threshold, 7 days by default.

### `proz status [project] [--json]`

Prints where a project left off: how long it has been quiet, the latest milestone, the last commit message and the branch, followed by all milestones. With no name, uses the project containing the current directory. `proz nudges` shows the same note in short form.

### `proz idea <text>`

Captures an idea as a project with no folder yet, so it resurfaces too. Registering a folder named like the idea later (`proz add`, `proz <path>`) offers to attach the folder to it.

### `proz milestone [project] <text>`

Records a progress checkpoint, usually written by an agent at the end of a session ("shipped login; next: signup"). The latest milestone leads the left-off note and counts as activity. `proz status` lists them all.

### `proz resume | snooze | drop | restore [project]`

The answers to a nudge. `resume` prints the left-off note and opens the project's program. `snooze [project] [days]` hides the nudge for N days (7 by default); it comes back if the project is still quiet. `drop` archives the project until any new activity revives it, and `restore` brings it back by hand. `proz archived [--json]` lists dropped projects.

### `proz tray`

Runs in the background with a tray icon: a ring, with a red dot while any project is quiet. It checks every 15 minutes and notifies once per quiet stretch, like `proz check`. Its menu lists each quiet project with its short note; each has Resume (opens its first program, no terminal), Snooze 7 days and Drop. `proz tray autostart on|off` starts it at login (an autostart entry on Linux, a LaunchAgent on macOS, a registry Run value on Windows). `proz config tray off` removes it from login, and a running tray quits at its next refresh.

Release binaries include the tray. Building yourself: `go build -tags tray .` (on macOS this needs cgo). A build without the tag works fully, minus the tray. On GNOME the icon needs the AppIndicator extension.

### `proz mcp`

Serves proz to an agent over MCP on stdio, as a child process with no network listener. Tools: `list_projects`, `get_project`, `record_milestone`, `capture_idea` and `list_nudges`, returning the same fields as the `--json` commands. The proz Claude Code plugin registers it; for other clients, configure the command `proz mcp`.

### `proz check [--json]`

Runs one check: shows an OS notification for each project that just went quiet, at most once per quiet stretch (a snooze ending counts as a new one). Four or more at once collapse into one summary. Safe to run often; it never repeats itself. Without the tray, schedule it instead, for example with cron:

```cron
*/30 * * * * proz check
```

On Linux the notification goes through `notify-send`, which needs your desktop session; from cron you may need `DBUS_SESSION_BUS_ADDRESS` set. `proz test notification` shows a sample to confirm it works.

### `proz config notifications|tray|email|brain [on|off]`

Shows or switches a module. Notifications are on by default; the tray, email and brain stay off until you enable or configure them.

### `proz config stale-days [days] [--project <name>]`

Prints the stale threshold in days, or sets it. With `--project`, reads or sets that project's own threshold; `0` removes the override.

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
