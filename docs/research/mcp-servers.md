# MCP servers for trackers and brains

Research for #57, part of map #49. Checked 2026-10-18.

## Question

Which project trackers and knowledge tools have official or well-maintained MCP servers? For each: what an agent can read and write, the auth model, local or hosted, and how a proz project could reference a resource there.

## Short answer

Every major hosted tracker now runs an **official, vendor-hosted, remote MCP server** over Streamable HTTP with OAuth 2.1 in the browser. GitHub, Linear, Jira/Confluence, Notion, Todoist, Trello and Asana all have one. Google Drive's is still in Developer Preview. Local-first note tools (Obsidian, Logseq) have **no official server**. Community servers reach them through a local HTTP plugin or through the files themselves. brain.md ships its own MCP endpoint per instance.

So the model holds: proz stores a link, meaning a kind (`brain` or `tracker`), a provider, and a reference. The agent's own MCP client already has the credentials and does the reads and writes. proz needs no OAuth flows, tokens or API calls of its own.

## Providers

"Official" means published by the vendor. Status was taken from vendor docs or the vendor's GitHub org on the date above.

### Trackers

| Provider | Official? | Read / write | Auth | Local or hosted | Reference shape |
|---|---|---|---|---|---|
| GitHub | Yes, `github/github-mcp-server` [1] | Read and write issues, PRs, repos, Projects (`projects_write`). Toolsets are selectable, and `--read-only` drops write tools [1] | OAuth or PAT (Bearer) on the remote server. PAT, OAuth or GitHub App on the local server [1] | Hosted at `https://api.githubcopilot.com/mcp/`. Local option is stdio via Docker `ghcr.io/github/github-mcp-server` or a source build [1] | `owner/repo`, `owner/repo#123`, or the issue/project URL `https://github.com/owner/repo/issues/123` (a) |
| Linear | Yes, run by Linear [2] | Find, create and update issues, projects and comments. `/mcp/readonly` exposes reads only [2] | OAuth 2.1 with dynamic client registration, or an API key / OAuth token as Bearer [2] | Hosted only: `https://mcp.linear.app/mcp`. The SSE endpoint is deprecated [2] | Issue key `ENG-123`, or the URL `https://linear.app/<workspace>/issue/ENG-123/...` and `.../project/<slug>` (a) |
| Jira, Confluence (Atlassian Rovo MCP) | Yes, Atlassian [3][4] | Search and summarize. Create and update Jira work items and Confluence pages. Also covers JSM and Bitbucket [4] | OAuth 2.1 recommended. API token for headless use if an org admin enables it [3][4] | Hosted only: `https://mcp.atlassian.com/v2/mcp`. Cloud only. For Server/Data Center there is the community `sooperset/mcp-atlassian` (b) [4][9] | Jira key `PROJ-123`, or `https://<site>.atlassian.net/browse/PROJ-123`. Confluence page URL or page id (a) |
| Trello | Yes, Atlassian, `atlassian/trello-mcp-server` [5] | Read boards, lists, cards, checklists and Planner. Create boards, cards and checklists. Move, archive and mark done. No hard delete. No comments, attachments or custom fields yet [5] | OAuth 2.0 only, one workspace per connection. API tokens are not supported [5] | Hosted only: `https://mcp.trello.com/v1` [5] | Board `https://trello.com/b/<shortLink>/...`, card `https://trello.com/c/<shortLink>/...` (a) |
| Asana | Yes, Asana (V2) [6] | Read tasks and project status. Create and manage tasks and projects. Tool list via `tools/list` [6] | OAuth. Some clients need a pre-registered MCP app with client id and secret [6] | Hosted only: `https://mcp.asana.com/v2/mcp`. V1 (`/sse`) has been shut down [6] | Project or task gid, or the URL `https://app.asana.com/...` that carries the gid (a) |
| Todoist | Yes, Doist org, `Doist/todoist-mcp` (the page itself does not say "official") [7] | Find tasks by date, add tasks, search and fetch. Full coverage is in `src/tools` and was not verified [7] | OAuth on the hosted server. The local npx run uses `TODOIST_API_KEY` in dev setup (unclear for end users) [7] | Hosted at `https://ai.todoist.net/mcp`. Local `npx @doist/todoist-mcp` (formerly `@doist/todoist-ai`) [7] | Project id, or the URL `https://app.todoist.com/app/project/<id>` (a) |

### Brains (knowledge stores)

| Provider | Official? | Read / write | Auth | Local or hosted | Reference shape |
|---|---|---|---|---|---|
| Notion | Yes, hosted by Notion [8][10] | Search. Read, create and update pages. Create databases [8] | OAuth by default. Bearer "MCP token" on Business and Enterprise. Enterprise SSO [10] | Hosted: `https://mcp.notion.com/mcp` (SSE fallback `/sse`). The local `makenotion/notion-mcp-server` is **no longer maintained** [10][11] | Page or database URL ending in a 32-hex id, or that id alone (a) |
| Google Drive | Yes, Google, but **Developer Preview**, not GA [12][13] | 8 tools. Read: search, list recent, metadata, permissions, read and download content. Write: `create_file`, `copy_file` [12] | Google Cloud project with the Drive MCP API enabled. Inherits the user's permissions [13] | Hosted: `https://drivemcp.googleapis.com/mcp/v1` [12] | File or folder id, from `https://drive.google.com/drive/folders/<id>` or `https://docs.google.com/document/d/<id>` (a) |
| Obsidian | **No official server.** Community servers only. Most popular is `MarkusPfundstein/mcp-obsidian` (4.5k stars, 69 open issues and 38 open PRs, so maintenance is uncertain) [14]. Newer community servers wrap the official Obsidian CLI (1.12+) [15] | List, read and search. Patch, append and delete files [14] | API key from the community Local REST API plugin, on `127.0.0.1:27124` [14] | Local only. Obsidian must be running | Vault plus a vault-relative path such as `projects/proz.md`. `obsidian://open?vault=<v>&file=<path>` also works (a) |
| Logseq | **No official server** found (secondary source, uncertain) [16] | Community servers (`ergut/mcp-logseq`, `dailydaniel/logseq-mcp` and others) read and write pages. Some limit writes to a namespace [16][17] | Token for Logseq's local HTTP API server [17] | Local only. Logseq must be running. Support for file graphs and DB graphs varies by server [16] | Page name in a graph, for example `proz` (a) |
| brain.md | Yes, ships with brain.md (same author as proz) [18] | `search_notes`, `similar_notes`, `read_note`, `list_notes`, `get_backlinks`, `get_tasks`, `write_note`, `append_note` and more [18] | API key, session token, or OAuth 2.1, as Bearer [18] | Per-instance HTTP: `<instance>/mcp`, Streamable HTTP. Self-hosted or remote [18] | Vault-relative note path, for example `work/proz.md`, plus the instance URL (a) |
| proz local brain folder | n/a (proz's default brain) | Agents use their filesystem tools, or proz CLI/MCP (`proz brain`) | None | Local folder | Path inside `brain-path` |

Notes:
- (a) Reference shapes come from each product's public URL scheme, from general knowledge. They were not re-checked against vendor docs for this note.
- (b) Community project. Not checked in depth.

## Patterns that matter for proz

1. **Hosted plus OAuth is now the norm for SaaS.** Seven of the nine SaaS providers are hosted-only or hosted-first, with OAuth in the agent's client [1][2][4][5][6][7][10]. A server-side bearer token is the exception (Linear API key, GitHub PAT, Notion MCP token, Atlassian API token behind an admin switch). This favors proz never holding credentials: the agent's MCP client already owns the auth.
2. **URLs are the one shape every provider shares.** Every provider has a stable web URL that its MCP tools accept or that an agent can turn into an id. Short forms (`owner/repo#123`, `ENG-123`, `PROJ-123`) exist only for some.
3. **Local note tools have no stable MCP endpoint.** Obsidian and Logseq need the desktop app running plus a community plugin. A filesystem path is the more reliable reference, and it fits proz's local markdown brain default (principle 5).
4. **Scope is account-wide, not per-project.** No server lets you scope a connection to one project. A proz link only tells the agent where to look, and the agent's own permissions decide what it can do.
5. **Endpoints churn.** In 2026 alone, Asana V1, Atlassian `/v1/sse` and the Linear SSE endpoint were deprecated [2][6][9]. proz should not store MCP endpoint URLs. It should store resource references and let the agent's config own the endpoints.

## Implications for a proz link format

A suggestion for the spec, not a decision:

```
link: { kind: "tracker" | "brain", provider: "<slug>", ref: "<string>", label?: "<string>" }
```

- `provider` is a free-form slug with a known list (`github`, `linear`, `jira`, `confluence`, `trello`, `asana`, `todoist`, `notion`, `gdrive`, `obsidian`, `logseq`, `brainmd`, `folder`). Unknown slugs are allowed so new tools need no release.
- `ref` should be the **canonical web URL** where one exists. A short form (`owner/repo#123`, `ENG-123`) can be accepted on input, but an agent with that provider's MCP can always act on the URL. For local brains, `ref` is a path (vault-relative plus the vault, or absolute). For brain.md it is instance URL plus note path.
- proz can infer `provider` from a URL host (`github.com`, `linear.app`, `*.atlassian.net`, `trello.com`, `app.asana.com`, `app.todoist.com`, `notion.so`, `drive.google.com` and `docs.google.com`), so `proz link <url>` needs no flags.
- No credentials, endpoints or tool names go in proz. `skills/proz-agent/SKILL.md` would tell agents: "for a link with provider X, use your X MCP server. If you have none, tell the user."
- `--json` and MCP should expose links as a list on the project, so adding fields later is non-breaking.

## Open uncertainties

- The exact tool lists for Linear, Atlassian, Asana and Todoist were not enumerated. Their docs defer to `tools/list`.
- Whether Logseq ships an official server: none was found, but the only source for that is secondary.
- Google Drive MCP is preview only. Access and auth details may change before GA.
- The Atlassian `/v1/sse` sunset date (2026-06-30) comes from a third-party guide [9].

## Sources

1. GitHub MCP Server README: https://github.com/github/github-mcp-server
2. Linear MCP docs: https://linear.app/docs/mcp
3. Atlassian Rovo MCP, supported clients: https://support.atlassian.com/atlassian-rovo-mcp-server/docs/using-with-other-supported-mcp-clients/
4. Atlassian Rovo MCP, getting started: https://developer.atlassian.com/cloud/rovo-mcp/guides/getting-started/
5. Trello MCP Server README (Atlassian): https://github.com/atlassian/trello-mcp-server
6. Asana, using Asana's MCP server: https://developers.asana.com/docs/using-asanas-mcp-server
7. Doist todoist-mcp README: https://github.com/Doist/todoist-mcp
8. Notion MCP overview: https://developers.notion.com/docs/mcp
9. Third-party guide, Jira MCP 2026 (secondary): https://mcp.directory/blog/jira-mcp-complete-guide-2026
10. Notion, get started with MCP: https://developers.notion.com/docs/get-started-with-mcp
11. makenotion/notion-mcp-server README: https://github.com/makenotion/notion-mcp-server
12. Google Drive MCP reference: https://developers.google.com/workspace/drive/api/reference/mcp
13. Google Workspace, configure MCP servers: https://developers.google.com/workspace/guides/configure-mcp-servers
14. mcp-obsidian README (community): https://github.com/MarkusPfundstein/mcp-obsidian
15. Obsidian CLI MCP servers, directory listing (community, secondary): https://glama.ai/mcp/servers/fodaveg/obsidian-mcp
16. Logseq MCP guide (secondary): https://contextbolt.com/blog/logseq-mcp/
17. ergut/mcp-logseq listing (community): https://lobehub.com/en/mcp/ergut-mcp-logseq
18. brain.md Claude Code plugin (`brain-md` 1.0.0, local `plugin.json` and `SKILL.md`), author njayman
