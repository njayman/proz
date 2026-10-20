# Manual pre-release tests

The tray and the launcher window have no automated UI test. Run these on a desktop build (`go build -tags gui,accessibility,tray`) before a release.

To keep them off your own desktop, run them in a virtual display with a private session bus and a throwaway store:

```sh
export XDG_CONFIG_HOME=$(mktemp -d)
Xvfb :99 -screen 0 1280x800x24 & export DISPLAY=:99; unset WAYLAND_DISPLAY
dbus-run-session -- sh   # a session bus of its own, so no icon reaches your bar
```

Drive it with `xdotool type`/`xdotool key`, take screenshots with `import -window root shot.png`, and click tray menu items with `busctl --address=$DBUS_SESSION_BUS_ADDRESS call <bus name> /StatusNotifierItem/menu com.canonical.dbusmenu Event isvu -- <item id> clicked s "" 0` (`GetLayout iias -- 0 1 0` lists the item ids).

## Tray

1. The menu starts with Open proz; a build without `gui` has no Open proz.
2. Open proz opens the launcher as its own process: stopping the tray (`kill -TERM -<tray pid>`, its whole process group) leaves the launcher open.
3. Every quiet project's menu shows Resume, the five Snooze lengths and Drop.

## Launcher

1. The list shows at once; quiet projects are marked and come first.
2. Typing filters, Up and Down move the selection, Enter opens and closes the window, Esc clears the filter and a second Esc closes it.
3. The Dark check box switches the look and the choice survives a restart.
4. Ctrl+, (or Settings) shows the settings in place of the list; a change shows at once in `proz config` (such as `proz config stale-days`); Esc, even after a click on a check box, or Back returns to the list, and the window never changes size.
