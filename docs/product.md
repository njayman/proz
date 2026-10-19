# proz product scope

proz is project management that maintains itself. People start projects and ideas, get busy, pause them and forget. proz keeps track for them from automatic signals and from what their AI agents record, and reminds them through channels they already watch.

## Who it is for

- **Solo developers and non-developers**, each tracking their own projects.
- Non-developers set the floor: everything works with no terminal and no git, on a plain folder.
- Developers get extra automatic signals on top: git, worktrees, agent sessions.
- AI agents are the main writers. Humans mostly read.

## What proz holds

- **Projects**, their **milestones** and an automatic **left-off note**.
- A **checklist** per project, kept by agents in the proz brain, only when the project has no linked tracker. Reminders show its first open item.
- **Links** to the brains and trackers the user already has. Tasks live there, not in proz.

## Principles

1. **Zero maintenance.** The user never has to open or update anything.
2. **Push to remind, pull to act.** proz reaches people through a tray dot, an OS notification, email, or a phone notification once the v2 server exists. It never depends on anyone opening it to stay current. Every surface someone opens (TUI, desktop GUI, phone app) is a place to act: open, launch, check on, set up. None of them is a place to maintain. Agents and the CLI can reach every action those surfaces offer.
3. **Agents first.**
4. **Local-first.**
5. **Pluggable brains.**
6. **Modular.**

`CLAUDE.md` holds the full wording of each principle.

## Paid layer

You pay for hosting, not for features (see [ADR 0002](adr/0002-pay-for-hosting-not-features.md)).

- The binary is fully GPL v3, with no license keys, no feature flags tied to payment, and no calls home.
- The v2 server can be self-hosted for free with every feature.
- The hosted cloud is the same server run by us: sync across devices, email relay without SMTP setup, and the phone app backend. You use it by signing in.

## In scope for v2

- The folder model: projects, ideas and worktrees.
- Links to brains and trackers, used through agents.
- Opening terminal programs when proz runs without a terminal.
- A GUI launcher.
- Installers for Linux, Windows and macOS.
- The v2 server and phone app, for checking on projects on the go.

## Out of scope

- Teams, shared projects, accounts with roles.
- Kanban, boards, assignees, per-task due dates.
- proz holding full tasks or fetching tracker items itself.
- Features gated behind payment.
