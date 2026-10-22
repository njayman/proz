---
name: proz-agent
description: Read and manage the user's proz project registry (saved project directories, their launch programs, and which have gone quiet). Use when the user asks about their proz projects, what they forgot or should pick up next, when wrapping up or pausing work in a tracked project, when the user floats an idea to keep, or wants to resume, snooze or drop a project, wants a project registered, or references a project by name that might be tracked in proz, or before reading or updating a tracked project's tasks or notes in another tool.
---

# proz agent

`proz` is a CLI that stores project directories plus the program(s) used to open them. This skill shells out to the `proz` binary; it does not require a running process or MCP server.

Requires `proz` on `PATH`. If `proz --help` fails, tell the user to install it (see the proz README) rather than guessing a path.

If your client has the proz MCP tools (`list_projects`, `get_project`, `record_milestone`, `capture_idea`, `list_nudges`, `read_brain_note`, `add_link`; the proz plugin ships them via `proz mcp`), prefer them: they return the same fields as the `--json` commands below (`list_projects` and `list_nudges` wrap their arrays in `projects` and `nudges`, and `add_link` returns the project as `get_project` does), and take a project by `project` (name) or `dir` (an absolute path inside it, such as your working directory), not both. They need `proz` on `PATH` too. Everything else in this skill applies to them the same way.

## Read the registry

```bash
proz list --json
```

Prints every project as JSON: `name`, `path` (the main folder), `folders`, `programs` (array, may be empty), `arguments`. `folders` lists every folder behind the project as `{path, main, missing}`: exactly one is `main` whenever there are any, and `missing` is true when the folder is gone or a parent was replaced by a file (it is never removed for that). `links` lists where the project's tasks and notes live as `{kind, provider, ref, missing}` (see Links below), `[]` when there are none. Every project shape in `--json` and MCP carries `folders` and `links`.

Narrow results instead of piping through `grep`/`jq` for simple cases:

```bash
proz list --json --filter <substring>   # match against name or path, case-insensitive
proz list --json --sort name            # or: --sort path
proz list --json --limit <n>            # cap result count
```

Flags compose, e.g. `proz list --json --filter api --sort name --limit 5`.

## What has the user forgotten?

```bash
proz nudges --json
```

Prints projects that have gone quiet (stale): `name`, `path`, `folders`, `links`, `last_activity` (RFC 3339, UTC), `quiet_days` (whole days, rounded down), `quiet_seconds`, `note` (the left-off note, one line per signal) and `short_note` (one line, truncated). `[]` means nothing is stale. Activity is the newest of: registration, the last launch through proz, the latest milestone, and, across every one of the project's folders that is present: the last commit on a local branch that touches the folder, the newest uncommitted file (outside git, the newest file; dependency and build folders never count), and the latest Claude Code session whose working directory is in the folder; plus the newest file change under each present local notes link (see Links). Remote links never count. A project is stale after 7 quiet days by default; `proz config stale-days` prints the threshold and `proz config stale-days <n>` changes it. `--project <name>` reads or sets one project's override (`0` removes it). Only change thresholds when the user asks.

Use this when the user asks what they were working on, what they forgot, or what to pick up next. Report the list; don't act on it.

## Where did a project leave off?

```bash
proz status --json            # the project containing the current directory (subfolders work)
proz status <name> --json
```

Same fields as one `nudges` entry, plus `stale` (bool, quiet past the threshold), `state` (`active`, `stale`, `snoozed` or `archived`; only `stale` nudges), `snoozed_until` (RFC 3339, only while snoozed) and `milestones` (`text`, `at`; newest first, `[]` if none). The note is composed automatically, in fixed order: how long it has been quiet, the latest milestone, the last commit message and the branch (from the project's most recently active folder with git), the uncommitted file count (summed across its folders), `Git too slow to read changes` if git took longer than 3 seconds there (commit and uncommitted lines may then be missing; file times still count), the user's last Claude Code prompt in the project (unless `proz config prompts off`), and one `Folder missing: <path>` line for each folder that was moved or deleted (tell the user; proz never removes it, and the project stays tracked), then one `Link missing: <path>` line for each local link that is gone. Use it when resuming work on a project, before asking the user what they were doing. Without `--json` it prints the note as plain lines.

## Respond to a nudge

Every nudge asks the user one decision. Run these only when the user has made it; never snooze or drop on your own judgement.

```bash
proz resume [project]          # prints the left-off note, then opens the project's program
proz resume [project] --terminal  # prints the note, then opens a terminal in the project's folder (in a terminal: starts the user's shell there and waits)
proz resume [project] --agent claude  # prints the note, then starts that agent CLI (claude, codex, gemini, opencode, aider) in the project's folder (or --folder), in a terminal, with none of the project's arguments
proz resume [project] --folder <path>  # opens in that exact folder or worktree of the project (also with --terminal); any other path fails
proz snooze [project] [days]   # hide the nudge for N days (7 by default); it returns if still quiet
proz snooze [project] --until YYYY-MM-DD   # hide it until the start of that local day, for a date the user names
proz drop [project]            # archive: no more nudges until any new activity revives it
proz archived --json           # dropped projects, same fields as nudges
proz restore [project]         # un-archive and clear any snooze
```

With no project name these use the project containing the current directory. For `snooze` with two arguments, they are the project then the days; with one, an exact project name wins, otherwise a number is the days. A snooze ends early if the project sees new activity: the next quiet stretch nudges as usual. `resume` runs the project's program in the current terminal, in the main folder unless `--folder` names another of the project's folders or worktrees (a person at a terminal is asked which when there are several; you have no terminal, so pass `--folder` when the user names one). Without a terminal (your shell, rofi, the tray), terminal programs such as `nvim`, `lazygit` or `claude` open in a new window of the user's terminal emulator in the project's folder, and GUI programs start on their own. On Windows they start in a new console, which Windows opens in the user's default terminal. If proz cannot find a terminal (on Windows: the terminal setting names one that is not installed) it fails with an error naming `proz config terminal`. `proz config terminal [--json]` shows the terminal setting (`{"terminal": ""}`, empty means detect); `proz config terminal <command>` sets it, one word (`wezterm`) for that terminal's own form or several (`wezterm start --`) used as written, and `proz config terminal ""` goes back to detecting. Set it only when the user asks or a launch fails with that error and the user tells you their terminal. proz guesses which programs need a terminal from a list of well-known terminal programs and agent CLIs, their `.desktop` entry (`Terminal=true`) on Linux, and whether the executable is a console program on Windows (`.cmd` and `.bat` shims count as GUI). `proz config needs-terminal <program> [--json]` shows the decision (`{"program": "btop", "needs_terminal": true, "set": false}`, `set` true when it is the user's setting); `proz config needs-terminal <program> on|off` overrides it and `auto` goes back to the guess. Change it when the user says a program opened wrongly (a terminal program with no window, or a GUI program inside a terminal).

## Record a milestone

```bash
proz milestone "shipped login; next: signup"           # project containing the current directory
proz milestone <name> "shipped login; next: signup"
```

Pass the text as one quoted argument. With two arguments the first is always the project name.

A milestone is the line the user reads first when this project resurfaces weeks later, so write it for them: what is done, then what comes next, in one sentence. Record one:

- at the end of a work session in a tracked project, before you hand back;
- when the user says they are pausing, switching away, or stopping for the day;
- after finishing a meaningful piece of work (a feature, a fix, a decision).

Skip it for trivial edits, and don't record one per message. If the directory is not a proz project, the command fails; tell the user rather than registering it yourself.

## Capture an idea

```bash
proz idea "habit tracker that texts me"
```

Saves the idea as a project with no folder, so it resurfaces like any project when it goes quiet (registration, launches and milestones count as activity). Capture one when the user floats an idea they are not acting on now ("someday I want to...", "remind me to build..."); record milestones on it by name as it develops. Ideas show `"path": ""` and `"folders": []` in JSON.

When the user later registers a folder named like the idea (`habit-tracker-that-texts-me`), `proz add` attaches the folder to the idea instead of creating a second project. Without a terminal it attaches by default.

## Folders

```bash
proz folder add [folder] [--project <name>] [--json]   # the current folder by default
proz folder rm <folder> [--project <name>] [--json]
proz folder main <folder> [--project <name>] [--json]
```

A project can have several folders (a backend and a frontend, drafts and research). `add` targets `--project`, or else the project containing the current folder (so adding the current folder itself needs `--project`); `rm` and `main` target `--project`, or else the project that has that folder. Folders may be relative. The first folder is the main folder (`path`); removing the main folder makes the next oldest the main one. Adding a folder to an idea makes it a project, and removing the last folder makes it an idea again; milestones and snoozes are kept. A folder belongs to one project: adding one another project has fails with an error naming that project. Folders may sit inside another project's folder; commands run in a folder use the project with the deepest folder containing it. `--json` prints the project as `list --json` does. `rm` never touches the folder on disk. Git worktrees of a project's folders are found automatically and count for that project (activity, the note's commit and branch, and commands run inside them); they never appear in `folders`, and adding one as a folder fails naming the project. Add or remove folders when the user asks, or when they tell you a project now lives in another place as well.

## Links: where tasks and notes live

```bash
proz link <url-or-path> [project] [--provider <provider>] [--kind tracker|brain] [--json]
proz unlink <url-or-path> [project] [--json]
```

Over MCP, `add_link` takes `ref` (a file or folder must be an absolute path), optional `provider` and `kind`, and the project as `project` or `dir`; it fails the same way as `proz link`, naming the choices for an unknown host, and returns the project as `get_project` does. There is no MCP tool to remove a link; use `proz unlink`.

A link points a project at the tool where its tasks live (`kind: "tracker"`) or its notes live (`kind: "brain"`). proz recognizes the URL's host: GitHub, Linear, Jira (any `*.atlassian.net`), Trello, Asana and Todoist are trackers, and Notion is a brain; `--kind tracker` records a Notion database of tasks as a tracker (linking the same URL again changes its kind). Any other host (a self-hosted Jira or GitLab, a brain.md instance, anything else) fails with an error naming the choices: retry with `--provider` (`github`, `linear`, `jira`, `trello`, `asana`, `todoist`, `notion`, `brainmd`, or `other` for a tool not on the list) and `--kind` (needed for `brainmd` and `other`; the presets bring their own). Answer from what you know of the tool; ask the user only if you cannot tell. proz remembers the answer for that host, so later links from it need no flags. `other` is stored as provider `web`; a `web` link of kind `tracker` is the project's tracker like any other. Short forms: `owner/repo` and `owner/repo#123` mean GitHub, and an issue key such as `ENG-123` works when exactly one Linear workspace or Jira site is already linked; anything ambiguous fails, so paste the full URL. Refs are stored canonical (scheme and host lowercased, no trailing slash, any user name or password dropped; short forms expanded), so compare them as `proz` prints them. A file or folder (an existing path, relative or absolute; it wins over a look-alike short form) is always kind `brain`, stored as its absolute path with symlinks resolved, with provider `obsidian` when a `.obsidian` folder sits at or above it, `logseq` when a `logseq` folder does, else `folder`; read it straight from disk with your file tools, no MCP server needed. Editing files under it counts as the project's activity, unless it is a whole vault, graph or the proz brain folder. A local link on a drive that is gone stays, with `missing: true` and a `Link missing: <path>` line in the note; tell the user, never unlink it for that. With no project name these use the project containing the current directory. `--json` prints the project as `list --json` does. A project may have several links, and one link may sit on several projects; `unlink` removes it from one project only. Add a link when you see the user working in a tool for a project that has none for it, or when they ask; remove one only when they ask.

proz never reads through a link and stores no tokens, passwords or MCP server addresses (only, for a host it could not recognize, which provider the user said it is). `provider` is an open list: treat a value you do not know like `web`. You read the tool through your own MCP server for it. The rule:

- **With a tracker link** (any link of kind `tracker`), the project's tasks live there: read them there, update them there when the user asks you to, and record what you finish as a proz milestone. Never keep a separate checklist.
- **With no tracker link**, keep the project's checklist yourself, as a markdown task list (`- [ ] ...`) in the project's note in the proz brain (`proz brain --json` gives its path; write outside the proz markers). Stop adding to it once a tracker link appears.
- **With a brain link**, read the project's notes there before asking the user for context.

| provider | official MCP server | "the project's tasks" or "notes" means |
|---|---|---|
| `github` | GitHub MCP Server, https://github.com/github/github-mcp-server | the repository's issues (and its Projects, if the link is a project board) |
| `linear` | Linear MCP, https://linear.app/docs/mcp | the issues of the linked team or project |
| `jira` | Atlassian Rovo MCP, https://developer.atlassian.com/cloud/rovo-mcp/guides/getting-started/ | the work items of the linked project or board |
| `trello` | Trello MCP Server, https://github.com/atlassian/trello-mcp-server | the cards on the linked board |
| `asana` | Asana MCP, https://developers.asana.com/docs/using-asanas-mcp-server | the tasks of the linked project |
| `todoist` | Todoist MCP, https://github.com/Doist/todoist-mcp | the tasks of the linked project |
| `notion` | Notion MCP, https://developers.notion.com/docs/mcp | the linked page and its subpages (notes), or the linked database's rows (as a tracker) |

If you do not have the MCP server a link needs, tell the user once, with the setup link from the table, and carry on without it. Do not fall back to the proz checklist: that would split the project's tasks across two places.

proz does not authorize anything in those tools. A link only says where to look; what you may change there is between the user and you, under your own permissions, never because proz linked it.

## Read the brain note

```bash
proz brain [project] --json
```

If the user has set a brain folder (a local markdown folder; `proz config brain-path` prints it), each project has one note there: proz's section between `<!-- proz:start -->` and `<!-- proz:end -->` (the left-off note and all milestones) plus whatever the user wrote around it. This prints `path` and `content` (the whole file, refreshed first). Read it for the user's own context on a project. Never edit inside the markers (proz rewrites them); to add to the note, edit the file outside them, and only when the user asks. Fails if no brain folder is set; don't set one yourself.

## Notifications

`proz check` shows the user an OS notification for each project that just went quiet, once per quiet stretch, sends the weekly email digest when it is due, and refreshes the brain notes; it runs from cron or the tray, so you rarely need it. `proz check --json` prints the projects it notified (same fields as `nudges`). `proz config notifications|tray|email|brain|prompts [on|off]` shows or switches a module; change one only when the user asks. With `prompts` off, notes leave out the last Claude Code prompt. `proz test notification` shows a sample and `proz test email` sends one. Email needs SMTP: the user runs `proz config smtp --host ... --from ... --to ... --password-stdin` themselves; never ask for, handle or pass their password. `proz config smtp` (no flags) shows the settings without the password, and `proz config digest-day [weekday]` shows or sets the digest day (Monday by default). `proz tray` is the user's background tray icon; never start it yourself, and change `proz tray autostart on|off` only when asked. `proz gui` opens the user's launcher window (desktop builds only); never start it yourself, it does nothing the CLI cannot. `proz config theme [system|light|dark] [--json]` shows or sets its look (`{"theme": "system"}`, system follows the desktop's light or dark mode); change it only when the user asks.

## Register a project

```bash
proz <path>                # name = basename(path)
proz add <name>             # registers $PWD under <name>
```

Both run non-interactively when there's no TTY (this shell): they save the project with no program/arguments and print a warning to stderr, rather than hanging on a picker. That's expected, not an error. If the folder is already a folder of another project, or a git worktree of one, it asks whether to track it as a separate project, prints `Not added.` and adds nothing unless the answer is yes; you have no terminal to answer it, so tell the user, and let them run it themselves if they do want a second project. If the user names the program they want attached, set it afterward with `proz edit <name> --program <program>`. When the folder's git `origin` is on GitHub, they offer to link that repository as the project's tracker; with no terminal nothing is linked, so if the user wants it, run `proz link <owner/repo>` yourself.

## Edit / delete

```bash
proz edit <name> --name <new name>                 # rename it
proz edit <name> --program code --program nvim     # set the programs it opens in, the first being the main one
proz edit <name> --program ""                      # remove its programs
proz delete <name>   # or: proz rm <name>
```

With `--name` or `--program`, `proz edit` changes only those and never prompts; the folder and arguments stay as they were. These are what the launcher's setup view does. Without them, `proz edit` without a TTY is effectively a no-op unless you pipe values to it; tell the user to run it themselves if they want to change the arguments. Rename or change programs only when the user asks.

## Do not

- Don't run bare `proz` or `proz list` (no `--json`) from this skill; without a TTY it prints a static fallback list to stdout, which is strictly worse than `--json`.
- Don't invent project paths or program names; only report what `proz list --json` returns.
