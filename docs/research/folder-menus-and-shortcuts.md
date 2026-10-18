# Folder right-click entries and project shortcuts on each OS

Research for #55 (map #49). Question: how can proz put an "Add to proz" entry on folders in the file manager, which opens the project setup window, and create an optional per-project shortcut that resumes the project, per user and without admin rights?

Researched 2026-10-18. Command names such as `proz add --gui` and `proz open --id` are placeholders, not existing flags. Confidence markers: **[verified]** means read in the primary source (spec, official docs or upstream source code) linked in the row; **[local]** means checked on the author's machine (Pop!_OS 24.04, COSMIC Files 1.10.0, Nemo and Thunar installed); **[unverified]** means general knowledge or a secondary source, to be confirmed by a manual test before shipping.

## Summary

- **Linux has no single mechanism.** The one portable hook is a `.desktop` file with `MimeType=inode/directory` in `~/.local/share/applications`, which puts proz under "Open With" for folders in every freedesktop file manager. A real right-click entry needs one small file per file manager. COSMIC Files (the author's file manager) gained user-defined context actions in April 2026.
- **Windows** works per user through `HKCU\Software\Classes\Directory\shell`, with no admin rights needed. On Windows 11 these legacy verbs show only under "Show more options". The top-level menu needs `IExplorerCommand` plus package identity, and that needs a package signed with a trusted certificate.
- **macOS** is the weakest platform. Finder Services need an `.app` with a Cocoa service-provider object (Objective-C, so cgo). Quick Actions are Automator bundles whose `document.wflow` format Apple does not document. Finder Sync extensions need a signed app extension.
- **Per-project shortcuts** are easy everywhere: a `.desktop` launcher entry on Linux, a `.lnk` in the per-user Start Menu on Windows, and a stub `.app` or `.command` file on macOS. The hard part is cleanup. proz must record every file and registry key it writes, so that renaming or deleting a project can update or remove its shortcut.

## Per-OS mechanism table

### Folder entry ("Add to proz")

| OS / file manager | Mechanism | Per-user location | Owned file or shared file | Notes |
|---|---|---|---|---|
| Any freedesktop FM (Open With) | `.desktop` entry with `MimeType=inode/directory;` and `Exec=proz add --gui %f` | `~/.local/share/applications/` | Owned | Shows under "Open With", not top level. `NoDisplay=true` hides it from app menus but keeps it registered: the spec says NoDisplay means "this application exists, but don't display it in the menus" [verified, desktop-entry spec]. **Risk:** if the user has no `Default Applications` entry for `inode/directory`, the implementation "should pick the most-preferred .desktop files associated with the mimetype" [verified, mime-apps spec]. proz could then become the handler that opens folders. Check `xdg-mime query default inode/directory` first [unverified as a mitigation]. |
| COSMIC Files | `context_actions` key in the cosmic-config store: a RON list of `(name, confirm, selection: Folders, steps: ["proz add --gui %f"])` | `~/.config/cosmic/com.system76.CosmicFiles/v1/context_actions` (no extension) | **Shared** (one list for all user actions) | Added in pop-os/cosmic-files#1728, merged 2026-04-10 [verified]. The struct is `ContextActionPreset { name, confirm, selection: Any/Files/Folders, steps }` [verified, `src/context_action.rs`]. `steps` go through the desktop-entry Exec parser, so `%f` runs once per item and `%F` passes the whole selection; `~` and `$HOME` are not expanded [verified, #2083]. Entries appear in the right-click menu below Open/Open With [verified, `src/menu.rs`]. Issue #2083 says the file is read at startup only, but master subscribes to config changes (`Config::subscription()` in `src/app.rs`), so a live reload is likely [unverified, needs a manual test]. Not documented upstream (#2083 is open). COSMIC Files also lists Open With for a single selected folder [verified, `src/menu.rs`]. |
| Dolphin (KDE) | Service menu: `.desktop` with `Type=Service`, `MimeType=inode/directory;`, `Actions=` plus `[Desktop Action ...]` sections | `~/.local/share/kio/servicemenus/` (KF 5.85 and later; older versions used `kservices5/ServiceMenus`) | Owned | Must be executable (`chmod +x`) to count as authorized when installed to the local location [verified, KDE docs]. Exec takes `%u`/`%U` [verified]. |
| Nautilus (GNOME Files) | Script under Right-click > Scripts | `$XDG_DATA_HOME/nautilus/scripts/` (`~/.local/share/nautilus/scripts`; the Flatpak build hardcodes that path) | Owned, must be executable | Path from `nautilus_get_scripts_directory_path` [verified, `src/nautilus-file-utilities.c`]. The selection arrives in `NAUTILUS_SCRIPT_SELECTED_FILE_PATHS` (newline separated), plus `NAUTILUS_SCRIPT_SELECTED_URIS` and `NAUTILUS_SCRIPT_CURRENT_URI` [verified, `src/nautilus-files-view.c`]. Shows in a "Scripts" submenu for every selection type; you cannot filter it to folders [unverified]. A top-level entry needs a nautilus-python extension, which is an extra distro package [unverified]. |
| Nemo (Cinnamon) | `.nemo_action` keyfile with `Selection=s`, `Extensions=dir;`, `Exec=proz add --gui %F` | `~/.local/share/nemo/actions/` [unverified path; the sample only says "the actions folder"; system copies are in `/usr/share/nemo/actions` [local]] | Owned | `Extensions=dir` limits it to directories, and `%F`/`%P` are the tokens [verified, upstream `sample.nemo_action`]. |
| Thunar (Xfce) | Custom action with `<directories/>` appearance condition | `$XDG_CONFIG_HOME/Thunar/uca.xml` | **Shared** | Path and XML schema verified in `plugins/thunar-uca/thunar-uca-model.c` [verified]. Thunar rewrites the whole file from its in-memory model when the user saves actions [verified]. If Thunar is running and the user edits actions, it can drop an entry proz added in the meantime unless Thunar reloaded it [unverified]. Use a stable `<unique-id>` to find proz's entry again. |
| Windows 10 and 11 (legacy menu) | Static verb: `HKCU\Software\Classes\Directory\shell\proz.add` with `MUIVerb` or default value "Add to proz" and `command\(Default) = "C:\...\proz.exe" add --gui "%1"`. Also `Directory\Background\shell\proz.add` with `"%V"` for right-clicking inside a folder | HKCU, no elevation | Owned keys | "you can register any custom verbs under the HKEY_CURRENT_USER\Software\Classes subkey. The main advantage of doing so is that elevated permission is not required" [verified, MS Learn]. `Directory` is a predefined shell object that accepts verbs [verified]. `%V` for the background menu [unverified]. `Icon`, `Position=Top`, `MultiSelectModel` are optional [verified]. |
| Windows 11 (top-level menu) | `IExplorerCommand` COM handler plus package identity through a sparse package (packaging with external location) | Per-user registration with `Add-AppxPackage -Path ... -ExternalLocation ...` | Owned | Windows 11 shows legacy verbs under "Show more options", which "loads the Windows 10 context menu as-is" (also Shift+F10). Apps extend the new menu with "IExplorerCommand + app identity" [verified, Windows blog 2021]. The identity package "must be signed with a certificate that is trusted on the target computer", and a production certificate is needed for end users [verified, MS Learn]. Per-user registration itself needs no admin [verified]. Writing a COM DLL in Go is impractical [unverified]; this would mean a small C++ or Rust DLL. |
| macOS Finder Services | `NSServices` in an `.app`'s Info.plist with `NSSendFileTypes = [public.folder]`, `NSRequiredContext = {}`, `NSMessage` | `.app` in `~/Applications`, or a `.service` bundle in `Library/Services` | Owned bundle | `NSMessage` names an Objective-C method `name:userData:error:` on a provider object registered with `setServicesProvider:` or `NSRegisterServicesProvider` [verified, Apple Services guide]. Always include `NSRequiredContext`, even empty, or the service is not shown [verified]. "The list of available services ... is built each time a user logs in"; for `Library/Services`, no logout is needed, and `NSUpdateDynamicServices()` forces an update [verified]. So Go needs cgo plus an AppKit run loop. |
| macOS Quick Action | Automator `.workflow` bundle with a "Run Shell Script" action, "Workflow receives folders in Finder" | `~/Library/Services/` | Owned bundle | Appears under right-click > Quick Actions [unverified]. The `document.wflow` plist format is not documented by Apple, so generating it means copying a template workflow made in Automator [unverified]. Needs a prototype. |
| macOS Finder Sync extension | App extension (`FIFinderSync`) | Inside a signed app | Owned | Needs an Xcode-built, signed and sandboxed app extension that the user enables in System Settings [unverified]. Too heavy for a one-binary tool. |

### Per-project shortcut ("resume this project")

| OS | Mechanism | Per-user location | Notes |
|---|---|---|---|
| Linux | `.desktop` file, `Type=Application`, `Exec=proz open --id <id>`, `Path=<project dir>`, `Name=<project name>` | Launcher: `~/.local/share/applications/proz-<id>.desktop`. Desktop: a copy in the XDG `DESKTOP` dir (`xdg-user-dir DESKTOP`) | The launcher entry is what matters on Sway: launchers such as fuzzel and wofi, and the COSMIC launcher, read `~/.local/share/applications` [unverified per launcher]. Sway draws no desktop icons. GNOME's desktop-icons extension only launches a desktop file after "Allow Launching" (the `metadata::trusted` attribute) [unverified]. `Path` is the working directory [verified, spec]. |
| Windows | `.lnk` (Shell Link) with target `proz.exe`, arguments `open --id <id>`, working directory = project | Start Menu: `FOLDERID_Programs` (`%APPDATA%\Microsoft\Windows\Start Menu\Programs\proz\`). Desktop: `FOLDERID_Desktop` | Resolve both with `windows.KnownFolderPath` from `golang.org/x/sys/windows`, which proz already depends on (v0.49.0 has `FOLDERID_Desktop` and `FOLDERID_Programs`) [verified, local module cache]. Using the known folder API, rather than hardcoding `%USERPROFILE%\Desktop`, follows OneDrive folder redirection [unverified]. |
| macOS | Stub `.app` bundle (Info.plist plus an executable shell script in `Contents/MacOS/` that runs `proz open --id <id>`), or a `.command` file | `~/Applications/proz/<name>.app` (Spotlight, Launchpad, Dock); `~/Desktop/<name>.command` | A `.command` file opens Terminal, which suits terminal launches but not GUI ones [unverified]. Files proz writes locally carry no quarantine attribute, so Gatekeeper should not block an unsigned script stub [unverified, needs a test on Apple Silicon]. |

### Go libraries for `.lnk`

| Option | Status | Notes |
|---|---|---|
| `github.com/go-ole/go-ole` + `oleutil`, calling `WScript.Shell.CreateShortcut` (or `IShellLinkW` + `IPersistFile`) | MIT, about 1.3k stars, pushed 2026-10 [verified, GitHub API] | Pure Go, no cgo; Windows only. This is the usual route. |
| `github.com/jxeng/shortcut` | MIT, 19 stars, last push 2023-02 [verified] | A thin wrapper over go-ole `WScript.Shell`. `CreateDesktopShortcut` hardcodes `HomeDir\Desktop` [verified, source], so call `Create` with a known-folder path, or copy its 40 lines rather than add the dependency. |
| `github.com/parsiya/golnk` | Apache-2.0 | Parser only, cannot write [verified, repo description]. Useful in tests to check a generated `.lnk`. |
| Write [MS-SHLLINK] bytes directly | No dependency, cross-platform, testable on Linux | The format is a public Microsoft open specification. Whether a minimal link (header, LinkInfo, StringData, no IDList) resolves reliably in Explorer is [unverified]. |
| Shell out to PowerShell `New-Object -ComObject WScript.Shell` | No dependency | Slow (around a second of startup) and fragile under restricted execution policies [unverified]. |

## Coverage gaps

- **Linux right-click is per file manager.** No freedesktop spec covers context-menu actions. The FileManager-Actions/DES-EMA draft never got broad adoption [unverified]. proz must detect installed file managers and write one file per FM, plus the generic Open With entry.
- **Two shared config files.** COSMIC's `context_actions` (RON) and Thunar's `uca.xml` (XML) hold every user action in one file. proz must read, merge and write without clobbering the user's own entries, and it must find its own entry again by a stable key (COSMIC: `name`; Thunar: `unique-id`). No Go RON library turned up in a GitHub search [verified, empty search result], so a narrow hand-written reader and writer would be needed for COSMIC.
- **Nautilus** gets only a "Scripts" submenu entry with no folder filter, unless proz depends on nautilus-python.
- **Open With hijack.** Registering for `inode/directory` can make proz the default folder handler on systems with no explicit default. This needs a manual test on GNOME, KDE and COSMIC.
- **Windows 11 top-level menu** needs a signed identity package and a native COM DLL, which means a release-signing pipeline the project does not have. Until then the entry sits behind "Show more options".
- **macOS** has no cheap, documented, no-cgo path to a Finder right-click entry.
- **Sway** has no desktop surface, so "desktop shortcut" means a launcher entry there.

## Cleanup needs

1. **Record every artifact.** Keep a table of what proz installed (kind, path or registry key, project id, OS). Uninstall, rename and delete then work from the record, not from guesses.
2. **Name artifacts by project id, label by name.** `proz-<id>.desktop` and `Exec=proz open --id <id>` mean a rename only rewrites `Name=`. Shortcuts that call proz by id keep working even if a file was not updated. Windows `.lnk` and macOS `.app` names are the visible label, so rename means moving the file and updating the record.
3. **Delete removes.** Deleting or archiving a project removes its shortcuts and launcher entry. If the project folder moves, rewrite `Path=` or the working directory.
4. **Respect user deletions.** If a recorded shortcut is missing, treat it as turned off and never recreate it silently.
5. **Stale shortcuts degrade gracefully.** A copied shortcut can outlive its record, so `proz open --id <unknown>` should say the project is gone (a notification, not a silent failure).
6. **Global uninstall.** A `proz integrations remove` command (name to be decided) must remove the folder entries too: the Open With desktop file, the FM files, proz's entry in the shared COSMIC and Thunar files, the HKCU verbs and the macOS bundles. On Linux, refresh `mimeinfo.cache` with `update-desktop-database ~/.local/share/applications` where available [unverified that every FM needs it].

## Recommendation

1. **Linux first, for the author's setup.** Ship the generic Open With `.desktop` (with the default-handler check) and a COSMIC `context_actions` entry using `selection: Folders` and `steps: ["proz add --gui %f"]`. Add owned-file entries for Dolphin, Nemo and Nautilus scripts, which are cheap because proz owns those files. Add Thunar last, because of the shared `uca.xml`.
2. **Windows:** HKCU `Directory\shell` and `Directory\Background\shell` verbs now. Defer the Windows 11 top-level entry until releases are code-signed.
3. **macOS:** prototype an Automator Quick Action generated from a template in `~/Library/Services`. If it is too fragile, fall back to the GUI's own "Add folder" action and `proz add`, and revisit a cgo Services provider when the GUI app bundle exists.
4. **Shortcuts:** make the launcher entry the default form (a `.desktop` file, a Start Menu `.lnk`, `~/Applications/proz/<name>.app`) and a desktop copy opt-in. Use go-ole directly for `.lnk` (pure Go, MIT) and `golang.org/x/sys/windows` known folders, which proz already depends on.
5. **One installer seam in `core`:** an interface with per-OS implementations (install, update, remove, list) backed by the artifact record, so CLI, MCP and GUI share one code path (principle 3).

## Sources

- freedesktop Desktop Entry spec, recognized keys: https://specifications.freedesktop.org/desktop-entry/latest/recognized-keys.html
- freedesktop MIME Applications spec, default application: https://specifications.freedesktop.org/mime-apps/latest/default.html
- COSMIC Files PR #1728 "Add user-defined context actions": https://github.com/pop-os/cosmic-files/pull/1728
- COSMIC Files issue #2083 (context_actions undocumented, Exec semantics): https://github.com/pop-os/cosmic-files/issues/2083
- COSMIC Files source: https://github.com/pop-os/cosmic-files/blob/master/src/context_action.rs, `src/menu.rs`, `src/app.rs`, `src/config.rs`
- KDE Dolphin service menus: https://develop.kde.org/docs/apps/dolphin/service-menus/
- Nautilus scripts path: https://github.com/GNOME/nautilus/blob/main/src/nautilus-file-utilities.c (`nautilus_get_scripts_directory_path`); env vars in `src/nautilus-files-view.c`
- Nemo sample action: https://github.com/linuxmint/nemo/blob/master/files/usr/share/nemo/actions/sample.nemo_action
- Thunar custom actions: https://docs.xfce.org/xfce/thunar/custom-actions and https://github.com/xfce-mirror/thunar/blob/master/plugins/thunar-uca/thunar-uca-model.c
- Microsoft, Creating Shortcut Menu Handlers: https://learn.microsoft.com/en-us/windows/win32/shell/context-menu-handlers
- Windows blog, Extending the context menu and share dialog in Windows 11: https://blogs.windows.com/windowsdeveloper/2021/07/19/extending-the-context-menu-and-share-dialog-in-windows-11/
- Microsoft, Grant package identity by packaging with external location: https://learn.microsoft.com/en-us/windows/apps/desktop/modernize/grant-identity-to-nonpackaged-apps
- Microsoft [MS-SHLLINK] Shell Link Binary File Format: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-shllink/
- Apple Services Implementation Guide, properties: https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/SysServices/Articles/properties.html
- Apple Services Implementation Guide, providing a service: https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/SysServices/Articles/providing.html
- go-ole: https://github.com/go-ole/go-ole; jxeng/shortcut: https://github.com/jxeng/shortcut; parsiya/golnk: https://github.com/parsiya/golnk
- golang.org/x/sys/windows `KnownFolderPath`: https://pkg.go.dev/golang.org/x/sys/windows#KnownFolderPath
