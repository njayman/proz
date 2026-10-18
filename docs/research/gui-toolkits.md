# GUI toolkits for a Go desktop app

Ticket: #53 (part of map #49). Researched 2026-10-18.

Question: which toolkit should back an optional launcher-style window in proz? Criteria: screen reader accessibility, running as a cobra subcommand inside the single binary, Linux runtime dependencies, cgo per OS, coexistence with `fyne.io/systray` (v1.12.2, `tray` build tag, started with `systray.Run` in `tray/run.go`), binary size, startup time, and maturity in 2026.

Tags: **[src]** is backed by a cited primary source. **[inf]** is an inference from sources. **[unverified]** was not measured or found.

## Comparison

| | Fyne v2.8 | Wails v3 (beta) / v2 | webview_go | Gio | Qt via miqt |
|---|---|---|---|---|---|
| Rendering | Own widgets on OpenGL (GLFW) [src 3] | System webview: WebView2, WKWebView, WebKitGTK [src 6] | Same system webviews [src 9] | Own immediate-mode renderer | Native Qt widgets |
| Screen reader | Merged for all OSes, shipped in v2.8.0 but **off by default** behind `-tags accessibility`; v2.9 timing unknown [src 1, 2]. Open macOS AX bugs (card content missing, tray item unlabeled) [src 4] | Comes from the browser engine's accessibility tree (HTML + ARIA) [inf]. WebKitGTK GTK3 a11y works, the GTK4 bridge is new and "improvements still needed" [src 10]. Open Wails macOS keyboard-focus a11y bug [src 7] | Same as Wails (same engines) [inf] | Semantic tree exists (`io/semantic`) but only the Android backend wires it to a screen reader; Windows backend has none [src 11, 12] | Qt's mature accessibility stack [inf, not checked in depth] |
| Runs as a cobra subcommand in one binary | Yes: `app.New()` / `Run()` are plain Go calls; must own the main goroutine, which cobra's `Execute` already runs on [inf, src 5] | Yes in principle (`application.New` + `Run` are Go calls) [src 6]; the normal flow uses the `wails3` CLI and a frontend build (Node) [src 6] | Yes, a library call [src 9] | Yes | Yes, but links Qt |
| cgo | Required on all desktop OSes (C compiler) [src 3] | Required on Linux and macOS (gcc / Xcode CLT) [src 6]; Windows not stated | Required (C/C++ library) [src 9, inf] | Not on Windows; yes on Linux and macOS [unverified] | Required (C++) |
| Linux runtime deps | libGL, X11 or Wayland libs, libxkbcommon: present on any desktop [src 3] | WebKitGTK 6.0 + GTK4 by default (Ubuntu 24.04+, Debian 13+); `-tags gtk3` for WebKit2GTK 4.1, removed in v3.1 [src 6] | GTK3 + WebKitGTK 4.1 or GTK4 + WebKitGTK 6.0 [src 9] | X11/Wayland, EGL/Vulkan | Qt 5/6 shared libs |
| Coexists with fyne.io/systray | Fyne depends on `fyne.io/systray` itself and drives it through its tray API [src 2] | Ships its own tray API [src 8]; running two tray libraries in one process is untested [unverified] | Takes the main loop; use `systray.RunWithExternalLoop` [src 13] | Same as webview_go | Same |
| Binary size / startup | Not measured [unverified] | Not measured [unverified]; webviews start a separate web process [inf] | Not measured | Not measured | Not measured |
| Maturity, Oct 2026 | Stable; v2.8.2 on 2026-10-12, regular releases [src 1] | v2 stable (v2.17.0, 2026-10-18); v3 still beta (beta.32, 2026-10-18) [src 14] | Last push 2024-08-31; upstream `webview` at 0.12.0 [src 9, 15] | Active, v0.10.x | miqt active (pushed 2026-10-09), no tagged releases [src 16] |

## Recommendation

**Fyne, as a `proz gui` subcommand behind a `gui` build tag**, ideally merged with the existing `tray` tag into one desktop build.

1. It is the only option that already shares proz's tray library: Fyne depends on `fyne.io/systray` [src 2], so tray plus window live in one Fyne main loop instead of two competing ones.
2. Linux runtime needs are only the graphics libraries every desktop has [src 3]. Webview options pull in WebKitGTK, and Wails v3 needs WebKitGTK 6.0, which rules out Ubuntu 22.04 and Debian 12 without a legacy tag that disappears in v3.1 [src 6].
3. No Node or frontend build step, so the release stays `go build` plus a tag, like the tray today.
4. Stable release line with steady cadence [src 1]; Wails v3 is still beta [src 14] and webview_go has been quiet since 2024 [src 15].

Choose **Wails v3** instead only if the map decides the desktop GUI and the v2 PWA should share one HTML UI (an open item on #49). The webview gives the most mature screen reader path and UI reuse, at the cost of WebKitGTK, a beta framework and a second tray stack.

Rule out Gio (no desktop screen reader support) and webview_go (stale bindings). Qt via miqt is viable but heavy to ship and build cross-platform.

## Open risks

- **Fyne screen reader support is new and opt-in.** It needs `-tags accessibility` in v2.8 [src 1, 2], and open macOS AX bugs exist [src 4]. Before relying on it, test with Orca, NVDA and VoiceOver.
- **Tray rewiring.** `tray/run.go` calls `systray.Run`, which owns the main loop. With Fyne in the same process, the tray must go through Fyne's desktop app tray API, or `RunWithExternalLoop` [src 13]. Fyne pins a newer systray pseudo-version than proz's v1.12.2 [src 2], so Go will upgrade it.
- **Threading.** Fyne's new threading model is opt-in in v2.8 and becomes the default in v2.9 [src 5]. Background updates from proz's watchers have to follow it.
- **Binary size and startup were not measured.** A throwaway spike (`proz gui` with one list window) should record the stripped binary delta and cold-start time per OS.
- **cgo everywhere.** Cross-compiling release binaries in `./build.sh` needs per-OS toolchains (or fyne-cross) for the GUI build. The non-GUI build stays pure Go.

## Sources

1. Fyne releases (v2.8.0 notes: "Adding accessibility support (currently off by default, use -tags accessibility to test)"; v2.8.2 2026-10-12): https://github.com/fyne-io/fyne/releases , https://github.com/fyne-io/fyne/releases/tag/v2.8.0
2. Fyne issue #1285, closed 2026-04-14: "merged accessibility in terms of screen reader for all OS ... in v2.8.0, but disabled by default": https://github.com/fyne-io/fyne/issues/1285 . Fyne `go.mod` (requires `fyne.io/systray v1.12.3-0.20260810170012-af4e8e793ec4`, `go-gl/glfw/v3.4`): https://github.com/fyne-io/fyne/blob/master/go.mod
3. Fyne quick start prerequisites (Go, C compiler, graphics driver; per-distro packages): https://github.com/fyne-io/docs.fyne.io/blob/master/content/docs/started/quick.md
4. Fyne open issues #6491 and #6492 (macOS AX tree gaps): https://github.com/fyne-io/fyne/issues/6491 , https://github.com/fyne-io/fyne/issues/6492
5. Fyne goroutines doc ("This threading model will be default from v2.9.0"): https://github.com/fyne-io/docs.fyne.io/blob/master/content/docs/started/goroutines.md
6. Wails v3 installation doc (Go 1.25+, WebKitGTK 6.0 default, `-tags gtk3` legacy until v3.1, Xcode CLT, WebView2): https://github.com/wailsapp/wails/blob/master/docs/mpress/content/quick-start/installation.md
7. Wails issue #6249 (macOS keyboard focus order a11y bug): https://github.com/wailsapp/wails/issues/6249
8. Wails v3 system tray doc: https://github.com/wailsapp/wails/blob/master/docs/mpress/content/features/menus/systray.md
9. webview (C/C++ library) README, platform engines and Linux packages, version 0.12.0: https://github.com/webview/webview . webview_go README: https://github.com/webview/webview_go
10. WebKitGTK 2.44 notes ("The existing accesibility support for GTK3 continues to work as before"; GTK4 web content a11y tree newly exposed): https://webkitgtk.org/2024/03/27/webkigit-2.44.html
11. Gio `io/semantic` package: https://pkg.go.dev/gioui.org/io/semantic
12. Gio platform backends: `app/os_android.go` maps semantics to `AccessibilityNodeInfo`; `app/os_windows.go` has no accessibility code: https://github.com/gioui/gio/tree/main/app
13. fyne.io/systray README (requires cgo; DBus StatusNotifierItem on Linux, no GTK; `RunWithExternalLoop` for other toolkits): https://github.com/fyne-io/systray
14. Wails releases (v2.17.0 and v3.0.0-beta.32, both 2026-10-18): https://github.com/wailsapp/wails/releases
15. GitHub API, webview/webview_go `pushed_at` 2024-08-31: https://api.github.com/repos/webview/webview_go
16. GitHub API, mappu/miqt (pushed 2026-10-09, no releases): https://github.com/mappu/miqt
