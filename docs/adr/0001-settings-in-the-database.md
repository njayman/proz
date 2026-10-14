# Settings live in the database, secrets in a config file

The v1 spec put every setting in a config file. Settings like the stale threshold live in the SQLite store instead, scoped by workspace and changed through `proz config` (and later MCP), because agents are the main writers and must change settings without editing a file, and hosted sync can then carry them like any other workspace data. Only credentials (SMTP, later API tokens) go in a config file in the config directory, written with owner-only permissions, so secrets never travel through sync.
