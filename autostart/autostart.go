// Package autostart starts 'proz tray' at login: an XDG autostart entry on
// Linux and the BSDs, a LaunchAgent on macOS, a registry Run value on Windows.
package autostart

import (
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
)

// entry is a login item that is a file.
type entry struct {
	path   string
	render func(exe string) string
}

func entryFor(goos, home, configDir string) entry {
	if goos == "darwin" {
		return entry{filepath.Join(home, "Library", "LaunchAgents", "dev.proz.tray.plist"), plist}
	}
	return entry{filepath.Join(configDir, "autostart", "proz.desktop"), desktop}
}

func (e entry) install(exe string) error {
	if err := os.MkdirAll(filepath.Dir(e.path), 0755); err != nil {
		return err
	}
	return os.WriteFile(e.path, []byte(e.render(exe)), 0644)
}

func (e entry) remove() error {
	if err := os.Remove(e.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (e entry) installed() bool {
	_, err := os.Stat(e.path)
	return err == nil
}

// desktop is an XDG autostart entry; Exec quoting follows the desktop entry spec.
func desktop(exe string) string {
	q := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", "$", `\$`).Replace(exe)
	return fmt.Sprintf("[Desktop Entry]\nType=Application\nName=proz\nComment=Nudges you about projects that went quiet\nExec=\"%s\" tray\nX-GNOME-Autostart-enabled=true\n", q)
}

func plist(exe string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>dev.proz.tray</string>
	<key>ProgramArguments</key>
	<array>
		<string>` + html.EscapeString(exe) + `</string>
		<string>tray</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`
}
