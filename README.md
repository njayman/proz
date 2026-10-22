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

Or, on Windows, download `proz-setup-windows-amd64.exe` from the [latest release](https://github.com/njayman/proz/releases/latest) and run it. It installs the desktop build (launcher, tray and CLI) for you only, without admin rights: a "proz" entry in the Start menu, `proz` on your PATH (open a new terminal), and an uninstaller in Settings, Apps. Running a newer installer upgrades in place; uninstalling keeps your projects and settings. Every release lists the SHA-256 of each download in `checksums.txt`.

Or build from source:

```bash
go install github.com/njayman/proz@latest
```

Linux packages (`.deb`, `.rpm`, Arch) hold the desktop build: the launcher window (`proz gui`), the tray and the CLI, plus a "proz" entry in the app menu. Build them for your machine's architecture with [nfpm](https://nfpm.goreleaser.com) and the launcher's build headers (on Debian and Ubuntu: `gcc libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev`), then install one with your package manager:

```bash
scripts/package-linux.sh            # writes dist/proz_*.deb, dist/proz-*.rpm, dist/proz-*.pkg.tar.zst
sudo apt install ./dist/proz_*.deb  # or: sudo dnf install ./dist/proz-*.rpm, sudo pacman -U dist/proz-*.pkg.tar.zst
```

Removing the package keeps your projects and settings. `proz --version` prints the installed version.

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

If the folder is already a folder of another project, or a git worktree of one, it first asks whether to track it as a separate project; anything but yes adds nothing.

When the folder's git `origin` is on GitHub, it then offers to link the repository's issues as the project's tracker (see `proz link`): Enter or `y` links it, anything else does not.

### `proz delete <name>`

Deletes a project. Without a name, opens the picker to select one.

### `proz edit <name> [--name <new name>] [--program <program>]...`

TUI form to update name, programs (space-separated), or arguments. Falls back to text prompts if the TUI is unavailable. With `--name` or `--program` (repeat it for several; `--program ""` removes them) it changes just those without prompting, the way the launcher's setup does.

### `proz rm <name>`

Alias for `delete`.

### `proz nudges [--json]`

Lists projects that have gone quiet: no registration, no launch through proz, no milestone, no git commit touching the folder (on a local branch), no uncommitted edit (or, outside git, no file change) and no Claude Code session in it, for longer than the stale threshold, 7 days by default. Claude Code logs are read from `$CLAUDE_CONFIG_DIR` (or `~/.claude`).

### `proz status [project] [--json]`

Prints where a project left off: how long it has been quiet, the latest milestone, the last commit message and the branch, how many files are uncommitted, the last thing you asked Claude Code there, and a warning if the folder is gone, followed by all milestones. With no name, uses the project containing the current directory. `proz nudges` shows the same note in short form.

### `proz idea <text>`

Captures an idea as a project with no folder yet, so it resurfaces too. Registering a folder named like the idea later (`proz add`, `proz <path>`) offers to attach the folder to it.

### `proz milestone [project] <text>`

Records a progress checkpoint, usually written by an agent at the end of a session ("shipped login; next: signup"). The latest milestone leads the left-off note and counts as activity. `proz status` lists them all.

### `proz link <url-or-path> [project]` and `proz unlink <url-or-path> [project]`

Links a project to the tracker or brain it already uses, so agents know where its tasks and notes live: paste its GitHub repository, Linear project, Jira board, Trello board, Asana or Todoist project, or Notion page. proz recognizes the tool from the address; Notion counts as notes unless you add `--kind tracker`. For any other address, such as a self-hosted Jira or GitLab or a brain.md instance, proz asks you to say what it is once with `--provider` (one of the above, `brainmd`, or `other`) and `--kind tracker|brain`, and remembers it for that host. `owner/repo` and `owner/repo#123` are taken as GitHub, and an issue key such as `ENG-123` works once one Linear workspace or Jira site is linked. A file or folder of notes, such as a note or folder in an Obsidian vault or a Logseq graph, links as the project's notes with no server running, and editing it counts as activity (a whole vault or graph, or the brain folder, does not). A linked folder on an unplugged drive stays linked and the left-off note says it is missing. A project can have several links and one link can sit on several projects. proz never reads through a link and stores no passwords or tokens; your agent reads the tool with its own MCP server. With no project name, uses the project containing the current folder. `--json` prints the project as `list --json` does.

### `proz resume | snooze | drop | restore [project]`

The answers to a nudge. `resume` prints the left-off note and opens the project's program. `resume --terminal` opens a terminal in its folder instead, and `resume --agent claude` (or `codex`, `gemini`, `opencode`, `aider`) starts that agent CLI there. `snooze [project] [days]` hides the nudge for N days (7 by default), and `snooze [project] --until 2026-11-09` hides it until the start of that day; either way it comes back if the project is still quiet. `drop` archives the project until any new activity revives it, and `restore` brings it back by hand. `proz archived [--json]` lists dropped projects.

### `proz tray`

Runs in the background with a tray icon: a ring, with a red dot while any project is quiet. It checks every 15 minutes and notifies once per quiet stretch, like `proz check`. In desktop builds its menu starts with Open proz, which opens the launcher (`proz gui`). The menu lists each quiet project with its short note; each has Resume (opens its first program, no terminal), five Snooze lengths (until tomorrow, 3 days, 1 week, 2 weeks or 1 month) and Drop. `proz tray autostart on|off` starts it at login (an autostart entry on Linux, a LaunchAgent on macOS, a registry Run value on Windows). `proz config tray off` removes it from login, and a running tray quits within 30 seconds.

Release binaries include the tray. Building yourself: `go build -tags tray .` (on macOS this needs cgo). A build without the tag works fully, minus the tray. On GNOME the icon needs the AppIndicator extension.

#### Sway and other bare Wayland compositors

Sway ignores XDG autostart, so `proz tray autostart on` does nothing there. Run the tray as a systemd user service that Sway starts instead; it restarts after a crash and logs to `journalctl --user -u proz-tray`. You also need a bar with a tray (swaybar, or waybar or eww with a systray widget).

```bash
mkdir -p ~/.config/systemd/user
cp contrib/sway/proz-tray.service ~/.config/systemd/user/
systemctl --user daemon-reload
cp contrib/sway/proz.conf ~/.config/sway/
grep -qxF 'include ~/.config/sway/proz.conf' ~/.config/sway/config ||
  echo 'include ~/.config/sway/proz.conf' >> ~/.config/sway/config
```

Log in again to start it (Sway runs `exec` at login, not on reload). The service expects the binary at `~/.local/bin/proz`. `proz.conf` hands Sway's Wayland session to systemd first, so programs opened from Resume can show windows. Avoid `dex --autostart`: it launches every XDG autostart entry on the system, not just proz, and nothing restarts the tray if it dies.

### `proz mcp`

Serves proz to an agent over MCP on stdio, as a child process with no network listener. Tools: `list_projects`, `get_project`, `record_milestone`, `capture_idea`, `list_nudges`, `read_brain_note` and `add_link`, returning the same fields as the `--json` commands. The proz Claude Code plugin registers it; for other clients, configure the command `proz mcp`.

### `proz check [--json]`

Runs one check: shows an OS notification for each project that just went quiet, at most once per quiet stretch (a snooze ending counts as a new one), sends the weekly email digest when it is due, and refreshes the brain notes. Four or more at once collapse into one summary. Safe to run often; it never repeats itself. Without the tray, schedule it instead, for example with cron:

```cron
*/30 * * * * proz check
```

On Linux the notification goes through `notify-send`, which needs your desktop session; from cron you may need `DBUS_SESSION_BUS_ADDRESS` set. `proz test notification` shows a sample to confirm it works.

### `proz config notifications|tray|email|brain|prompts [on|off]`

Shows or switches a module. Notifications are on by default; the tray, email and brain stay off until you enable or configure them. Prompts is on: notes show your last Claude Code prompt in the project, and so do notifications, the email digest and brain notes. Switch it off to keep prompts private; brain notes drop theirs at once, and the session still counts as activity.

### `proz config brain-path [dir]` and `proz brain [project] [--json]`

The brain is a local markdown folder, such as an Obsidian vault. Once you set it, proz keeps one note per project there (`habit-tracker.md`) with its last activity date, left-off note and milestones, refreshed on every check and whenever a milestone is recorded. A note changes only when something happened, so a synced vault stays quiet. proz only rewrites the part between `<!-- proz:start -->` and `<!-- proz:end -->`; write anything you like around it. A note keeps its file name if you rename the project, and an existing note with that name is used, not replaced. A linked note is written where the link points; a broken link is reported, never replaced. If you save a note at the moment proz rewrites it, proz leaves your save and tries again at the next check. Changing the folder starts fresh notes there; the old ones stay where they are. A brain folder inside a tracked project never counts as that project's activity. `proz brain` prints a project's note (`--json` gives `path` and `content`). Without a brain folder, everything else works as before.

### `proz config smtp` and `proz config digest-day [weekday]`

Once a week proz emails you every quiet project with its full left-off note and the exact `proz resume`, `snooze` and `drop` commands. It is a simple HTML email with a plain-text copy for clients that prefer it; it loads nothing from the network, so there are no images or tracking. Email stays off until you set an SMTP server:

```bash
proz config smtp --host smtp.gmail.com --username me@gmail.com \
  --from me@gmail.com --to me@gmail.com --password-stdin
proz test email
```

The password is read from stdin (hidden in a terminal) and saved in `smtp.json` in the config folder, readable only by you; nothing else about email is stored there. Port 465 uses TLS; any other port (587 by default) must offer STARTTLS, so nothing is sent unencrypted except to a relay on localhost. proz refuses `smtp.json` if others can read it. `proz config smtp` with no flags shows the settings without the password; flags you leave out keep their saved value, and a new host or username needs the password again. Setting SMTP the first time switches email on; later edits leave it as it is. For Gmail, use an app password.

The digest goes out on Monday by default (`proz config digest-day friday` changes it), from the tray or `proz check`. If the machine was off that day, it goes out at the next check that week. The first digest waits for the digest day. A week with nothing quiet sends nothing, and a restart never sends twice. A failed send is retried an hour later, not at every check, so a wrong password can't get your account locked.

### `proz config stale-days [days] [--project <name>]`

Prints the stale threshold in days, or sets it. With `--project`, reads or sets that project's own threshold; `0` removes the override.

### `proz config theme [system|light|dark] [--json]`

Shows or sets how the launcher (`proz gui`) looks. `system` (the default) follows your desktop's light or dark mode; the launcher's Dark check box sets `light` or `dark`.

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

Open a pull request. Every pull request also runs the packaging check: it builds the Linux packages and installs and removes each one in a clean Debian, Ubuntu, Fedora and Arch container ([`scripts/check-linux-package.sh`](scripts/check-linux-package.sh), which needs docker), and builds the Windows installer on Windows and installs, upgrades and uninstalls it silently ([`scripts/check-windows-installer.ps1`](scripts/check-windows-installer.ps1)). On Linux, [`scripts/check-windows-installer-wine.sh`](scripts/check-windows-installer-wine.sh) runs a rougher version of that check under wine. Before a release, run the manual end to end test, [`scripts/manual-test.sh`](scripts/manual-test.sh). It walks you through every feature in a throwaway config (the email stage needs [Mailtrap Local](https://github.com/mailtrap/mailtrap-local)) and writes a report; only its last stage touches your real setup, and it asks first.

## License

GPL v3. See [LICENSE](LICENSE).
