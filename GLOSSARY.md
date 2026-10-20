# proz

proz tracks projects and ideas for solo developers and non-developers, mostly through what AI agents record, and reminds people about the ones gone quiet.

## Language

**Project**:
Something a person is working on, tracked by proz. It has zero or more folders.

**Idea**:
A project with no folders yet. Adding a folder makes it a project; it is never a separate record.

**Folder**:
A directory on disk that belongs to exactly one project. Folders may nest; a place inside nested folders belongs to the deepest one.
_Avoid_: Path, directory (in anything a person reads)

**Main folder**:
The folder a project opens in by default: the first one added, unless changed.

**Missing folder**:
A folder proz can no longer find, such as one on an unplugged drive. It stays with its project until someone removes it.

**Worktree**:
An extra checkout of a git folder. proz finds worktrees from git and never stores them; their activity counts for the folder's project.

**Checkout**:
Any place a project can be opened in: one of its folders, or the same place in a worktree of one. Resuming asks which when there are several.

**Workspace**:
The internal scope every record belongs to. Never shown to people.

**Milestone**:
A past result recorded by an agent, with an optional "next" part, such as "shipped login; next: signup".
_Avoid_: Task, step

**Left-off note**:
A project's automatic summary of where it stopped, built from signals and the latest milestone.
_Avoid_: Status note, summary

**Checklist**:
A project's list of open items in the proz brain, kept by agents only when the project has no linked tracker.
_Avoid_: Tasks, todo list, backlog

**Brain**:
The knowledge store proz reads from and writes to, a local markdown folder by default.
_Avoid_: Vault, notebook

**Tracker**:
The user's own tool for tasks, such as GitHub Issues, Linear or Trello, which proz links to but never stores.

**Link**:
A project's pointer to a tracker or brain it uses. proz never reads through it; agents do.
_Avoid_: Integration, connection

**Provider**:
The tool a link points at, such as GitHub, Notion or a local folder.
_Avoid_: Integration, service

**Launcher**:
The desktop window for finding and opening projects. A place to act, never to maintain.
_Avoid_: Dashboard, app

**Hosted cloud**:
The v2 server run by us, the only paid part of proz.
_Avoid_: Pro, premium
