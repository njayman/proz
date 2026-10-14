---
name: proz-agent
description: Read and manage the user's proz project registry (saved project directories, their launch programs, and which have gone quiet). Use when the user asks about their proz projects, what they forgot or should pick up next, wants a project registered, or references a project by name that might be tracked in proz.
---

# proz agent

`proz` is a CLI that stores project directories plus the program(s) used to open them. This skill shells out to the `proz` binary; it does not require a running process or MCP server.

Requires `proz` on `PATH`. If `proz --help` fails, tell the user to install it (see the proz README) rather than guessing a path.

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

Prints projects that have gone quiet (stale): `name`, `path`, `last_activity` (RFC 3339, UTC), `quiet_days` (whole days, rounded down), `quiet_seconds`, `note` (the left-off note, one line per signal) and `short_note` (one line, truncated). `[]` means nothing is stale. Activity is the newest of: registration, the last launch through proz, and the last commit on a local branch that touches the project folder. A project is stale after 7 quiet days by default; `proz config stale-days` prints the threshold and `proz config stale-days <n>` changes it. `--project <name>` reads or sets one project's override (`0` removes it). Only change thresholds when the user asks.

Use this when the user asks what they were working on, what they forgot, or what to pick up next. Report the list; don't act on it.

## Where did a project leave off?

```bash
proz status --json            # the project containing the current directory (subfolders work)
proz status <name> --json
```

Same fields as one `nudges` entry, plus `stale` (bool). The note is composed automatically, in fixed order: how long it has been quiet, the last commit message, the branch. Use it when resuming work on a project, before asking the user what they were doing. Without `--json` it prints the note as plain lines.

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
