# Finding the user's terminal on each OS

Research for #54 (part of map #49). Gathered 2026-10-18.

## Question

proz launches a project's programs, often a terminal program such as nvim or an
AI agent CLI. Today `launch` in `cmd/store.go` opens `/dev/tty` and hands it to
the program, falling back to proz's own stdio. From a launcher such as wofi,
the tray or an MCP call there is no controlling terminal, so a terminal program
has nothing to draw on. proz needs to open a terminal window itself. Which
terminal, how to pass it a command and a working directory, and how to know
when it cannot find one?

## Summary

- **Linux:** the only cross-desktop standard is the proposed XDG Default
  Terminal Execution spec, implemented by `xdg-terminal-exec`. GLib already
  tries it first for every `Terminal=true` app. Order for proz: user setting,
  `xdg-terminal-exec`, `$TERMINAL`, desktop setting (KDE `kdeglobals`), Sway
  `$term`, `x-terminal-emulator`, then a known-terminal probe list.
- **macOS:** there is no system "default terminal" setting. Terminal.app is
  always present. Use the user setting, else a known installed app (iTerm2,
  WezTerm, kitty, Ghostty, Alacritty), else Terminal.app via `osascript`.
- **Windows:** no detection needed in the common case. Starting a console
  program with `CREATE_NEW_CONSOLE` lets Windows pick the user's default
  terminal (Windows Terminal or conhost, per the Windows 11 setting). `wt.exe
  -d` is only needed to force Windows Terminal.
- **"Cannot detect"** is recognised when every step of the chain yields
  nothing runnable on `PATH` (or no app bundle on macOS). That is the point to
  warn and to suggest the user setting.
- **Is it a terminal program?** Reliable signals exist only in some places:
  `Terminal=true` in a `.desktop` entry, the PE subsystem field on Windows,
  and `.app` bundles on macOS (GUI). For a bare command on Linux or macOS
  there is no reliable signal, so proz needs a per-program flag with a
  known-names default list.

## 1. Linux and BSD

### 1.1 xdg-terminal-exec (proposed spec)

- A proposed XDG Default Terminal Execution Specification with a shell
  reference implementation, `xdg-terminal-exec`. It launches the user's
  default terminal and passes a command using that terminal's own exec
  argument. [xte]
- Config: `${desktop}-xdg-terminals.list` or `xdg-terminals.list`, where
  `${desktop}` matches entries of `$XDG_CURRENT_DESKTOP`. Looked up in
  `~/.config` then `/etc/xdg`; distro fallbacks in
  `/usr/local/share/xdg-terminal-exec/` and `/usr/share/xdg-terminal-exec/`.
  Each line names a `.desktop` entry (`-entry` excludes it, `+entry` protects
  it). [xte]
- Terminals declare themselves in their `.desktop` file with
  `X-TerminalArgExec` (required), `X-TerminalArgDir`, `X-TerminalArgAppId`,
  `X-TerminalArgTitle`, `X-TerminalArgHold`. The `X-` prefix stays while the
  spec is proposed. [xte]
- `xdg-terminal-exec --dir=DIR cmd args...` sets the working directory; if the
  terminal has no `X-TerminalArgDir`, the tool changes directory before
  launching it. [xte]
- If no listed or fallback entry applies, it returns an error. The README does
  not document a specific exit code (uncertain; treat any non-zero exit as
  failure). [xte]
- Status: still a proposal on freedesktop GitLab (the merge request page was
  behind bot protection and could not be read, so its exact state is
  unverified). [xte]
- Adoption: GLib tries `xdg-terminal-exec` first, before `kgx`,
  `gnome-terminal` and others, when launching any `Terminal=true` app
  [glib-src]. It was added via GLib MR !2839 in October 2022 as a stop-gap
  until the default-apps spec settles [glib-search]. Debian's GLib README says
  GLib uses it by default when the `xdg-terminal-exec` package is installed
  [debian-glib]. Per-distro packaging beyond that was not verified.

**Caveat seen on the author's machine:** `~/.local/bin/xdg-terminal-exec` is a
hand-written two-line wrapper (`exec wezterm start -- "$@"`), not the
reference implementation. It ignores `--dir`. So proz cannot assume `--dir` is
honoured; setting `cmd.Dir` on the spawned process as well is the safe choice
(the reference implementation also falls back to the launch cwd).

### 1.2 $TERMINAL

- Not a standard. It is a convention honoured by tools such as
  `i3-sensible-terminal`, which tries `$TERMINAL` first and then a fixed list
  (`x-terminal-emulator`, `mate-terminal`, `gnome-terminal`, ... `kitty`,
  `alacritty`, `wezterm`, `ghostty`) and shows `i3-nagbar` if none is found.
  [i3]
- It holds a bare command name. How to pass a command and cwd still depends on
  which terminal it names (see the table in section 4).

### 1.3 x-terminal-emulator (Debian and derivatives)

- Debian policy 11.8.3: terminal packages provide the virtual package
  `x-terminal-emulator` and register `/usr/bin/x-terminal-emulator` as an
  alternative. It must support `-e command` (remaining arguments form the argv,
  no shell) and `-T title`. [debian-policy]
- Policy says nothing about a working directory, so proz must set `cmd.Dir`.
- Caveat: what the alternative points to is not always policy-compliant. On
  the author's machine it resolves to `/usr/bin/open-wezterm-here`, which runs
  `wezterm start --cwd "$PWD" -- "$@"`. Passing `-e nvim` there would make
  wezterm try to run a program named `-e` (inferred from the script, not
  tested). Prefer the terminal-specific syntax when the target is a known
  terminal.

### 1.4 GNOME

- `org.gnome.desktop.default-applications.terminal` keys `exec` and `exec-arg`
  are marked "DEPRECATED: This key is deprecated and ignored. The default
  terminal is handled in GIO." [gsettings] Do not read them.
- GIO's behaviour is the `known_terminals` list in `gdesktopappinfo.c`:
  `xdg-terminal-exec`, `kgx -e`, `gnome-terminal --`, `mate-terminal -x`,
  `xfce4-terminal -x`, `tilix -e`, `konsole -e`, `nxterm -e`, `color-xterm -e`,
  `rxvt -e`, `dtterm -e`, `xterm -e`. If none is found it fails with "Unable to
  find terminal required for application." [glib-src]
- Red Hat ships a downstream GLib patch adding `ptyxis --` right after
  `xdg-terminal-exec` (RHEL-60786). [alma-patch]

### 1.5 KDE

- KIO's `KTerminalLauncherJob` reads the `General` group of the default shared
  config (in practice `kdeglobals`): first `TerminalService` (a desktop file
  storage id), then `TerminalApplication` (a raw command), then
  `org.kde.konsole`, then `konsole` or `xterm` on `PATH`; otherwise "No
  terminal emulator found". [kio]
- For `konsole` it appends `--workdir DIR`; on Windows it uses `wt.exe
  --startingDirectory`. [kio]
- proz can read `~/.config/kdeglobals` `[General] TerminalApplication` /
  `TerminalService` when `XDG_CURRENT_DESKTOP` contains `KDE`.

### 1.6 Sway and other wlroots compositors

- Sway's default config has `set $term foot` and `set $menu wmenu-run`
  [sway-config]. The author's config has `set $term wezterm`.
- `$term` is a sway config variable, not an environment variable, so a child
  process never sees it. Two ways to read it: parse `~/.config/sway/config`
  (and its `include`s) for `set $term`, or ask the running compositor with
  `swaymsg -t get_config`, which returns the loaded config text (sway-ipc
  `GET_CONFIG`). Either way the value can be a full command line with flags,
  and it may be an arbitrary script. Treat it as a best-effort hint only, used
  when `$SWAYSOCK` is set.
- Other wlroots compositors (Hyprland, river, niri) have no shared setting.
  `xdg-terminal-exec` is the only portable hook there.

### 1.7 Recommended Linux order

1. proz user setting (exact command template; always wins).
2. `xdg-terminal-exec` on `PATH` (passes `--dir`, also set `cmd.Dir`).
3. `$TERMINAL` if set and on `PATH`.
4. Desktop setting: KDE `kdeglobals` `TerminalService`/`TerminalApplication`.
5. Sway `$term` when `$SWAYSOCK` is set.
6. `x-terminal-emulator` on `PATH` (use `-e`; known caveat above).
7. Probe list of known terminals on `PATH`, desktop-aware first (`ptyxis`,
   `kgx`, `gnome-terminal` on GNOME; `konsole` on KDE), then `foot`, `kitty`,
   `wezterm`, `alacritty`, `ghostty`, `xfce4-terminal`, `xterm`.
8. Nothing found: "cannot detect" (section 5).

Steps 3 to 7 resolve to a binary name. proz then builds the command line from
the table in section 4, using a generic `-e` form for unknown names.

## 2. macOS

- There is no OS-level "default terminal" preference to read (no source
  found for one; the Finder path bar offers "Open in Terminal" for Terminal.app
  only [apple-terminal]). Terminal.app is always installed.
- `TERM_PROGRAM` names the terminal only when proz itself runs inside one, so it
  is useless from a launcher, the tray or MCP.
- Terminal.app: AppleScript `tell application "Terminal" to do script "cd DIR
  && nvim"` opens a new window and runs the command, as shown by an Apple DTS
  answer [apple-forum]. The command is a shell string, so DIR and args must be
  shell-quoted. Terminal's AppleScript cannot open a new tab (long-standing
  bug per the same forums).
- `open -a Terminal DIR` opens a window at a folder (widely used, not found in
  Apple docs; uncertain). `open -a APP --args ...` passes argv to the app, and
  `-n` forces a new instance. [open]
- iTerm2: `tell application "iTerm" to create window with default profile
  command "..."`. [iterm2]
- Uncertain: the first `osascript` call that drives Terminal or iTerm2 likely
  triggers the macOS Automation (Apple Events) consent prompt for the calling
  app. Not verified against Apple docs here.
- Recommended order: user setting, then the first installed of iTerm2,
  WezTerm, kitty, Ghostty, Alacritty (check `/Applications` and
  `~/Applications` bundles, or binaries on `PATH`), then Terminal.app. Since
  Terminal.app always exists, "cannot detect" should not happen on macOS
  unless `osascript` fails.

## 3. Windows

- `CREATE_NEW_CONSOLE`: "The new process has a new console, instead of
  inheriting its parent's console." [win-flags] In Go:
  `cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x10}`.
- On Windows 11 22H2 and later, a new console is hosted by the user's default
  terminal application, stored at `HKCU\Console\%%Startup` as
  `DelegationConsole` / `DelegationTerminal`. All zeros means "Let Windows
  decide"; `{B23D10C0-E52E-411E-9D5B-C09FDF709C7D}` is conhost;
  `{2EACA947-...}` / `{E12CFF52-...}` is Windows Terminal. [wt-policy]
  [ms-support] So proz does not need to pick a terminal: spawning the program
  with a new console respects the user's choice. Reading the key is only
  useful for diagnostics.
- If "Use legacy console" is enabled, conhost may still open even when
  Windows Terminal is the default (community source, uncertain).
  [winhelponline]
- Forcing Windows Terminal: `wt.exe [-w new] new-tab -d DIR cmd args`. `wt`
  is an app execution alias in `%LOCALAPPDATA%\Microsoft\WindowsApps`.
  `;` separates wt sub-commands, so any `;` in the program's arguments must be
  escaped as `\;`. Execution aliases do not work from inside WSL.
  [wt-cli]
- "Cannot detect" on Windows: effectively never with `CREATE_NEW_CONSOLE`.
  With a forced `wt.exe`, it is `exec.LookPath("wt.exe")` failing.

## 4. Command and working directory syntax

All forms below take argv directly (no shell) unless noted. Always set
`cmd.Dir` too, as a fallback for terminals or wrappers that ignore the flag.

| Terminal | Command form | Working directory | Hold open | Source |
| --- | --- | --- | --- | --- |
| xdg-terminal-exec | `xdg-terminal-exec CMD ARGS...` | `--dir=DIR` | via entry's `X-TerminalArgHold` | [xte] |
| x-terminal-emulator | `x-terminal-emulator -e CMD ARGS...` | none in policy | none in policy | [debian-policy] |
| WezTerm | `wezterm start -- CMD ARGS...` | `--cwd DIR` | none (config) | [wezterm] |
| foot | `foot CMD ARGS...` | `-D DIR` / `--working-directory=DIR` | `-H` | [foot] |
| kitty | `kitty CMD ARGS...` | `-d DIR` / `--directory DIR` | `--hold` | [kitty] |
| Alacritty | `alacritty -e CMD ARGS...` (`-e` must be last) | `--working-directory DIR` | `--hold` | [alacritty] |
| GNOME Terminal | `gnome-terminal -- CMD ARGS...` (`-e`, `-x` deprecated) | `--working-directory=DIR` | none | [gnome-terminal] |
| Konsole | `konsole -e CMD ARGS...` (`-e` last) | `--workdir DIR` | `--hold` / `--noclose` | [konsole] |
| Ghostty (Linux) | `ghostty -e CMD ARGS...` | `--working-directory=DIR` | none | [ghostty] |
| Console (kgx), Ptyxis | `kgx -e CMD`, `ptyxis -- CMD` | not verified | not verified | [glib-src] [alma-patch] |
| Windows Terminal | `wt.exe new-tab -d DIR CMD ARGS` (escape `;`) | `-d DIR` / `--startingDirectory` | profile setting | [wt-cli] |
| Windows default | spawn CMD with `CREATE_NEW_CONSOLE` | `cmd.Dir` | none | [win-flags] |
| Terminal.app | `osascript -e 'tell application "Terminal" to do script "cd DIR && CMD"'` (shell string) | inside the script | stays open by default | [apple-forum] |
| iTerm2 | `osascript -e 'tell application "iTerm" to create window with default profile command "..."'` | inside the command | profile setting | [iterm2] |

Ghostty on macOS was not verified; its docs say that when launched via `open`
the working directory defaults to `home`, so pass it explicitly. [ghostty]

## 5. How "cannot detect" is recognised

- Each step of the chain yields a candidate binary; the step counts only if
  `exec.LookPath` finds it. Unset variables, missing config keys and missing
  binaries all fall through.
- After the probe list, nothing on `PATH` means "cannot detect". GLib ("Unable
  to find terminal required for application"), KIO ("No terminal emulator
  found") and `i3-sensible-terminal` (`i3-nagbar`) all treat it the same way:
  the end of a fixed list. [glib-src] [kio] [i3]
- A found terminal can still fail to start: the spawn returns an error, or the
  terminal process exits non-zero almost immediately (for example a wrapper
  that rejects the arguments). proz cannot easily tell "terminal failed" from
  "program inside exited" once the window opens, so only spawn errors and fast
  non-zero exits (within roughly a second, a heuristic) should count as
  failure.
- When proz has no tty and cannot detect a terminal, the warning should name
  the setting to fill in and should go to the user through push channels
  (notification), since a launcher has nowhere to print stderr.
- Knowing there is no tty: `os.OpenFile("/dev/tty", ...)` fails (ENXIO) when
  the process has no controlling terminal, which is exactly what `launch`
  already tests. On Windows the equivalent is that proz has no console (GUI or
  detached process).

## 6. Is the program a terminal program?

Only a terminal program needs a terminal window; a GUI program (VS Code, a
browser) should be spawned directly and detached.

- **Desktop entries:** `Terminal=true` means "the program runs in a terminal
  window"; `Path=` is the working directory for an `Application` entry.
  [desktop-entry] If the program matches a `.desktop` entry in
  `$XDG_DATA_HOME/applications` or `$XDG_DATA_DIRS/*/applications`, this is
  authoritative. Most CLI tools (nvim excepted, which ships `nvim.desktop`
  with `Terminal=true`) have no entry at all.
- **Windows:** the PE optional header `Subsystem` field (offset 68) is
  `IMAGE_SUBSYSTEM_WINDOWS_CUI` (3) for console programs and
  `IMAGE_SUBSYSTEM_WINDOWS_GUI` (2) for GUI programs. [pe] Go's `debug/pe`
  exposes it via `OptionalHeader32/64.Subsystem`. This is reliable.
- **macOS:** a `.app` bundle (or `open -a`) is GUI; a bare executable on `PATH`
  is usually a CLI. Not a guarantee (inference).
- **Linux ELF:** nothing in the binary says "needs a tty". Linking against
  GTK/Qt/X11/Wayland libraries hints at GUI, but many GUI apps are scripts or
  Electron wrappers. Not reliable.
- **Recommendation:** a per-program setting (`terminal: true|false`) with a
  default from, in order: matching `.desktop` entry, PE subsystem on Windows,
  a built-in list of known terminal programs (`nvim`, `vim`, `vi`, `nano`,
  `emacs -nw`, `helix`/`hx`, `claude`, `codex`, `gemini`, `aider`, `opencode`,
  `lazygit`, shells), else assume terminal when proz has no tty (a stray
  terminal window is less harmful than a CLI that silently dies).

## Sources

- [xte] xdg-terminal-exec README and proposed spec: https://github.com/Vladimir-csp/xdg-terminal-exec
- [glib-src] GLib `gio/gdesktopappinfo.c`, `known_terminals` and `prepend_terminal_to_vector`: https://gitlab.gnome.org/GNOME/glib/-/blob/main/gio/gdesktopappinfo.c
- [glib-search] GLib commit history for gdesktopappinfo.c (MR !2839): https://src.opensuse.org/luc14n0/glib/commits/tag/2.79.3/gio/gdesktopappinfo.c
- [debian-glib] Debian libglib2.0-0 README.Debian: https://metadata-backend.ftp-master.debian.org/changelogs/main/g/glib2.0/testing_libglib2.0-0.README.Debian
- [alma-patch] AlmaLinux/RHEL glib2 Ptyxis patch: https://git.almalinux.org/rpms/glib2/commit/d05bc5c500e1db9c589a62e93ab9585354baeb0c
- [debian-policy] Debian Policy 11.8.3: https://www.debian.org/doc/debian-policy/ch-customized-programs.html
- [gsettings] gsettings-desktop-schemas default-applications schema: https://gitlab.gnome.org/GNOME/gsettings-desktop-schemas/-/blob/master/schemas/org.gnome.desktop.default-applications.gschema.xml.in
- [kio] KIO `kterminallauncherjob.cpp`: https://invent.kde.org/frameworks/kio/-/blob/master/src/gui/kterminallauncherjob.cpp
- [sway-config] Sway default config: https://github.com/swaywm/sway/blob/master/config.in
- [i3] i3-sensible-terminal: https://github.com/i3/i3/blob/next/i3-sensible-terminal
- [desktop-entry] Desktop Entry Specification, recognized keys: https://specifications.freedesktop.org/desktop-entry/latest/recognized-keys.html
- [wezterm] `wezterm start`: https://wezterm.org/cli/start.html
- [foot] foot(1): https://codeberg.org/dnkl/foot/src/branch/master/doc/foot.1.scd
- [kitty] kitty invocation: https://sw.kovidgoyal.net/kitty/invocation/
- [alacritty] alacritty(1): https://github.com/alacritty/alacritty/blob/master/extra/man/alacritty.1.scd
- [gnome-terminal] gnome-terminal(1): https://manpages.debian.org/testing/gnome-terminal/gnome-terminal.1.en.html
- [konsole] Konsole command-line options: https://docs.kde.org/stable_kf6/en/konsole/konsole/command-line-options.html
- [ghostty] Ghostty config reference: https://ghostty.org/docs/config/reference
- [wt-cli] Windows Terminal command line arguments: https://learn.microsoft.com/en-us/windows/terminal/command-line-arguments
- [wt-policy] Windows Terminal group policy (default terminal): https://learn.microsoft.com/windows/terminal/group-policy
- [ms-support] Microsoft support, Command Prompt and Windows PowerShell: https://support.microsoft.com/en-us/windows/apps/command-prompt-and-windows-powershell
- [winhelponline] Set default terminal in Windows 11 (community): https://www.winhelponline.com/blog/set-default-terminal-windows-11/
- [win-flags] Process creation flags: https://learn.microsoft.com/en-us/windows/win32/procthread/process-creation-flags
- [pe] PE format, Windows Subsystem: https://learn.microsoft.com/en-us/windows/win32/debug/pe-format
- [apple-terminal] Terminal User Guide, open windows and tabs: https://support.apple.com/guide/terminal/open-new-terminal-windows-and-tabs-trmlb20c7888/mac
- [apple-forum] Apple Developer Forums, running a command in Terminal: https://developer.apple.com/forums/thread/681647
- [open] open(1) reference: https://ss64.com/mac/open.html
- [iterm2] iTerm2 scripting: https://iterm2.com/documentation-scripting.html
