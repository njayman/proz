---
name: proz-agent
description: Read and manage the user's proz project registry (saved project directories, their launch programs, and which have gone quiet). Use when the user asks about their proz projects, what they forgot or should pick up next, when wrapping up or pausing work in a tracked project, when the user floats an idea to keep, or wants to resume, snooze or drop a project, wants a project registered, or references a project by name that might be tracked in proz.
---

# proz agent

`proz` is a CLI that stores project directories plus the program(s) used to open them. This skill shells out to the `proz` binary; it does not require a running process or MCP server.

Requires `proz` on `PATH`. If `proz --help` fails, tell the user to install it (see the proz README) rather than guessing a path.

If your client has the proz MCP tools (`list_projects`, `get_project`, `record_milestone`, `capture_idea`, `list_nudges`; the proz plugin ships them via `proz mcp`), prefer them: they return the same fields as the `--json` commands below (`list_projects` and `list_nudges` wrap their arrays in `projects` and `nudges`), and take a project by `project` (name) or `dir` (an absolute path inside it, such as your working directory), not both. They need `proz` on `PATH` too. Everything else in this skill applies to them the same way.

## Read the registry

```bash
proz list --json
```

Prints every project as JSON: `name`, `path`, `programs` (array, may be empty), `arguments`.

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

Prints projects that have gone quiet (stale): `name`, `path`, `last_activity` (RFC 3339, UTC), `quiet_days` (whole days, rounded down), `quiet_seconds`, `note` (the left-off note, one line per signal) and `short_note` (one line, truncated). `[]` means nothing is stale. Activity is the newest of: registration, the last launch through proz, the latest milestone, the last commit on a local branch that touches the project folder, the newest uncommitted file (outside git, the newest file; dependency and build folders never count), and the latest Claude Code session whose working directory is in the project. A project is stale after 7 quiet days by default; `proz config stale-days` prints the threshold and `proz config stale-days <n>` changes it. `--project <name>` reads or sets one project's override (`0` removes it). Only change thresholds when the user asks.

Use this when the user asks what they were working on, what they forgot, or what to pick up next. Report the list; don't act on it.

## Where did a project leave off?

```bash
proz status --json            # the project containing the current directory (subfolders work)
proz status <name> --json
```

Same fields as one `nudges` entry, plus `stale` (bool, quiet past the threshold), `state` (`active`, `stale`, `snoozed` or `archived`; only `stale` nudges), `snoozed_until` (RFC 3339, only while snoozed) and `milestones` (`text`, `at`; newest first, `[]` if none). The note is composed automatically, in fixed order: how long it has been quiet, the latest milestone, the last commit message, the branch, the uncommitted file count, the user's last Claude Code prompt in the project, and `Folder missing: <path>` if the folder was moved or deleted (tell the user; the project stays tracked). Use it when resuming work on a project, before asking the user what they were doing. Without `--json` it prints the note as plain lines.

## Respond to a nudge

Every nudge asks the user one decision. Run these only when the user has made it; never snooze or drop on your own judgement.

```bash
proz resume [project]          # prints the left-off note, then opens the project's program
proz snooze [project] [days]   # hide the nudge for N days (7 by default); it returns if still quiet
proz drop [project]            # archive: no more nudges until any new activity revives it
proz archived --json           # dropped projects, same fields as nudges
proz restore [project]         # un-archive and clear any snooze
```

With no project name these use the project containing the current directory. For `snooze` with two arguments, they are the project then the days; with one, an exact project name wins, otherwise a number is the days. A snooze ends early if the project sees new activity: the next quiet stretch nudges as usual. `resume` opens an interactive program in a terminal; from a shell without one, prefer `proz status --json` to read the note and let the user open it.

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

Saves the idea as a project with no folder, so it resurfaces like any project when it goes quiet (registration, launches and milestones count as activity). Capture one when the user floats an idea they are not acting on now ("someday I want to...", "remind me to build..."); record milestones on it by name as it develops. Ideas show `"path": ""` in JSON.

When the user later registers a folder named like the idea (`habit-tracker-that-texts-me`), `proz add` attaches the folder to the idea instead of creating a second project. Without a terminal it attaches by default.

## Notifications

`proz check` shows the user an OS notification for each project that just went quiet, once per quiet stretch; it runs from cron or the tray, so you rarely need it. `proz check --json` prints the projects it notified (same fields as `nudges`). `proz config notifications|tray|email|brain [on|off]` shows or switches a module; change one only when the user asks. `proz test notification` shows a sample. `proz tray` is the user's background tray icon; never start it yourself, and change `proz tray autostart on|off` only when asked.

## Register a project

```bash
proz <path>                # name = basename(path)
proz add <name>             # registers $PWD under <name>
```

Both run non-interactively when there's no TTY (this shell): they save the project with no program/arguments and print a warning to stderr, rather than hanging on a picker. That's expected, not an error. If the user wants a program attached, tell them to run `proz add <name>` themselves in a terminal, or run `proz edit <name>` afterward.

## Edit / delete

```bash
proz edit <name>     # non-interactive: only fills in fields it can read from stdin, otherwise leaves them unchanged
proz delete <name>   # or: proz rm <name>
```

`proz edit` without a TTY is effectively a no-op unless you pipe values to it; prefer telling the user to run it interactively if they want to change the program or arguments.

## Do not

- Don't run bare `proz` or `proz list` (no `--json`) from this skill; without a TTY it prints a static fallback list to stdout, which is strictly worse than `--json`.
- Don't invent project paths or program names; only report what `proz list --json` returns.
