# proz

proz is a local-first tracker for projects, ideas and milestones. **AI agents are its main writers**, and humans receive what matters through channels they already watch. It is a tool that agents use. It is not an AI agent or an AI workspace itself.

The problem it solves: people start ideas, get busy, pause them, and forget. They will not keep a notebook or open a notes app, but they do read email, notifications and tray dots.

## Product principles

Every feature must pass all of these. If it doesn't, cut it or redesign it.

1. **Zero maintenance.** The user never has to open or update anything. State comes from agents and from automatic signals (git, file activity, agent sessions).
2. **Push, don't pull.** Humans are reached through a tray dot, an OS notification or email. A dashboard is optional and never required.
3. **Agents first.** Every capability is reachable by agents (CLI `--json`, MCP over stdio) before it gets a human UI.
4. **Local-first.** One binary, works offline. Hosted sync, relay and email are an optional paid layer on top, never a requirement.
5. **Pluggable brains.** A "brain" is the knowledge store proz reads from and writes to. The default is a local markdown folder accessed through the filesystem, with no network calls. Remote brains (for example brain.md over HTTP MCP, or a git remote) are adapters that sync to a local copy in the background, so agents never wait on the network.
6. **Modular.** Modules can be switched on or off in config. Use build tags only where a dependency forces it (systray and cgo).

## Architecture

- `core/`: domain logic and storage. It must not import UI, network or frontend packages.
- Frontends call `core` directly and stay thin: `cmd/` (CLI/TUI), and later `tray`, `serve` (HTTP), `mcp`.
- External frontends (dashboard, mobile, browser addon) talk to `proz serve`, never to the CLI.
- Current state: the logic still lives in `cmd/` and `utils/`. Move it into `core/` whenever a feature touches it, not as a separate big-bang refactor.
- Storage today is `os.UserConfigDir()/proz/store.json`. The plan is SQLite (`modernc.org/sqlite`, pure Go) with `workspace_id` on every table.

## Workflow (mandatory order)

Each stage has a skill. Slash-prefixed ones are user-invoked, so suggest them to the user.

1. **Shape.** `/grill-with-docs` stress-tests the idea and records terms in `GLOSSARY.md` and decisions in `docs/adr/`. Work too big for one session starts with `/wayfinder`.
2. **Research.** The `research` skill writes findings into `docs/research/`.
3. **Spec.** `/to-spec` publishes the spec as a GitHub issue: the problem, the behavior, what is out of scope, and numbered given/when/then scenarios. `/to-tickets` cuts it into tracer-bullet tickets.
4. **Red.** The ticket's acceptance criteria list every scenario up front. The `tdd` skill then works them one at a time: one Go test goes red for the expected reason, at a seam agreed with the user.
5. **Green.** Just enough code to pass that test, then the next scenario. `/implement` drives the loop until every scenario passes.
6. **Bugs.** The `diagnosing-bugs` skill. A regression test goes red on the bug before the fix lands.
7. **Done** means `go vet ./...` and `go test ./...` pass, every scenario has a test, and the `code-review` skill has checked the change against its spec.

Product scope (target users, problems, what is in and out) lives in `docs/product.md`.

## Commands

```bash
go build -o proz .     # local build
go test ./...          # all tests
go vet ./...
./build.sh             # release binaries for all platforms into ./bin
```

## Conventions

- Go 1.27 (`go.mod` is the source of truth; CI reads it), cobra for commands, bubbletea/bubbles for the TUI.
- Agents rely on `skills/proz-agent/SKILL.md`. Update it in the same change whenever CLI flags or output change.
- `--json` output is an API contract with agents. Adding fields is fine; renaming or removing fields is a breaking change.
- Never use em dashes in docs, code comments or CLI output.

## Agent skills

### Issue tracker

GitHub issues via `gh`. See `docs/agents/issue-tracker.md`.

### Triage labels

Default five roles (needs-triage, needs-info, ready-for-agent, ready-for-human, wontfix). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `GLOSSARY.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
