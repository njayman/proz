// Package notify renders nudges as OS notifications, shown with each
// platform's built-in tool, and as the weekly email digest, sent over SMTP.
// It holds no domain rules: core decides what is due; this only says it.
package notify

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/njayman/proz/core"
)

type Message struct {
	Title string
	Body  string
}

// maxSingle is how many nudges get their own notification; more collapse
// into one summary so a first run over many quiet projects is not a flood.
const maxSingle = 3

// Render turns due nudges into notification messages.
func Render(nudges []core.Status) []Message {
	if len(nudges) > maxSingle {
		names := make([]string, len(nudges))
		for i, n := range nudges {
			names[i] = n.Name
		}
		return []Message{{
			Title: fmt.Sprintf("%d projects went quiet", len(nudges)),
			Body:  strings.Join(names, ", ") + "\nproz nudges",
		}}
	}
	out := make([]Message, len(nudges))
	for i, n := range nudges {
		out[i] = Message{
			Title: n.Name + " went quiet",
			Body:  n.Note.Short() + "\nproz resume " + shellQuote(n.Name),
		}
	}
	return out
}

var shellSafe = regexp.MustCompile(`^[A-Za-z0-9._/@:+-]+$`)

func shellQuote(s string) string {
	if shellSafe.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Covered is how many of due the first shown messages from Render covered:
// one each, or all of them for the summary.
func Covered(due []core.Status, shown int) int {
	if len(due) > maxSingle {
		return min(shown, 1) * len(due)
	}
	return shown
}

// Send shows each message as a native notification and returns how many it
// showed. Text always travels as an argument or environment variable, never
// spliced into a script.
func Send(msgs []Message) (int, error) {
	for i, m := range msgs {
		if err := command(m).Run(); err != nil {
			return i, fmt.Errorf("showing notification: %w", err)
		}
	}
	return len(msgs), nil
}

func command(m Message) *exec.Cmd {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("osascript",
			"-e", "on run argv", "-e", "display notification (item 2 of argv) with title (item 1 of argv)", "-e", "end run",
			m.Title, m.Body)
	case "windows":
		// ponytail: toast through PowerShell's registered app id; a proz app id needs an installer.
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", windowsToast)
		cmd.Env = append(os.Environ(), "PROZ_TITLE="+m.Title, "PROZ_BODY="+m.Body)
		return cmd
	default: // linux, bsd: libnotify
		return exec.Command("notify-send", "--app-name=proz", "--", m.Title, escapeMarkup(m.Body))
	}
}

const windowsToast = `
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
$t = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
$x = $t.GetElementsByTagName('text')
$x.Item(0).AppendChild($t.CreateTextNode($env:PROZ_TITLE)) | Out-Null
$x.Item(1).AppendChild($t.CreateTextNode($env:PROZ_BODY)) | Out-Null
$id = '{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe'
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($id).Show([Windows.UI.Notifications.ToastNotification]::new($t))
`

// escapeMarkup keeps a commit message like "<b>x</b>" literal: notification
// daemons render a subset of HTML in the body.
var escapeMarkup = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace
