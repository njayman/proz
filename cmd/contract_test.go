package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/njayman/proz/core"
	"github.com/njayman/proz/notify"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// execute runs proz with args from a clean flag state: cobra keeps flag
// values between Execute calls in one process.
func execute(args ...string) error {
	var reset func(*cobra.Command)
	reset = func(c *cobra.Command) {
		c.Flags().VisitAll(func(f *pflag.Flag) {
			if v, ok := f.Value.(pflag.SliceValue); ok { // Set appends to a slice
				v.Replace(nil)
			} else {
				f.Value.Set(f.DefValue)
			}
			f.Changed = false
		})
		for _, sub := range c.Commands() {
			reset(sub)
		}
	}
	reset(rootCmd)
	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

// run executes proz with args and returns what it printed to stdout.
func run(t *testing.T, args ...string) string {
	t.Helper()
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w
	err := execute(args...)
	os.Stdout = stdout
	w.Close()
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// Agents parse `proz list --json`; field names and shape are a contract.
func TestListJSONContract(t *testing.T) {
	dir := configDir(t)
	os.MkdirAll(filepath.Join(dir, "proz"), 0775)
	os.WriteFile(filepath.Join(dir, "proz", "store.json"), []byte(`[
  {"name": "web", "path": "/web", "programs": ["code", "claude"], "arguments": ["--new-window"]},
  {"name": "notes", "path": "/notes", "arguments": []},
  {"name": "legacy", "path": "/old", "executable": "nvim"}
]`), 0644)

	out := run(t, "list", "--json")

	want := `[
  {
    "name": "web",
    "path": "/web",
    "folders": [
      {
        "path": "/web",
        "main": true,
        "missing": true
      }
    ],
    "links": [],
    "programs": [
      "code",
      "claude"
    ],
    "arguments": [
      "--new-window"
    ]
  },
  {
    "name": "notes",
    "path": "/notes",
    "folders": [
      {
        "path": "/notes",
        "main": true,
        "missing": true
      }
    ],
    "links": [],
    "arguments": []
  },
  {
    "name": "legacy",
    "path": "/old",
    "folders": [
      {
        "path": "/old",
        "main": true,
        "missing": true
      }
    ],
    "links": [],
    "programs": [
      "nvim"
    ],
    "arguments": null
  }
]
`
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

// Agents parse `proz nudges --json`; field names and shape are a contract.
func TestNudgesJSONContract(t *testing.T) {
	dir := configDir(t)
	os.MkdirAll(filepath.Join(dir, "proz"), 0775)
	os.WriteFile(filepath.Join(dir, "proz", "store.json"), []byte(`[{"name": "web", "path": "/web"}]`), 0644)
	t0 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	t.Cleanup(func() { now = time.Now })

	now = func() time.Time { return t0 }
	if out := run(t, "nudges", "--json"); out != "[]\n" {
		t.Fatalf("fresh project: got %q", out)
	}

	now = func() time.Time { return t0.Add(10*24*time.Hour + time.Hour) }
	want := `[
  {
    "name": "web",
    "path": "/web",
    "folders": [
      {
        "path": "/web",
        "main": true,
        "missing": true
      }
    ],
    "links": [],
    "last_activity": "2026-10-01T09:00:00Z",
    "quiet_days": 10,
    "quiet_seconds": 867600,
    "note": "Quiet for 10 days\nFolder missing: /web",
    "short_note": "Quiet for 10 days; Folder missing: /web"
  }
]
`
	if out := run(t, "nudges", "--json"); out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestConfigStaleDays(t *testing.T) {
	configDir(t)
	if out := run(t, "config", "stale-days"); out != "7\n" {
		t.Fatalf("default: got %q", out)
	}
	if execute("config", "stale-days", "300000") == nil {
		t.Fatal("a threshold that overflows a duration should be rejected")
	}
	run(t, "config", "stale-days", "14")
	if out := run(t, "config", "stale-days"); out != "14\n" {
		t.Fatalf("after set: got %q", out)
	}
}

func TestConfigProjectStaleDays(t *testing.T) {
	dir := configDir(t)
	os.MkdirAll(filepath.Join(dir, "proz"), 0775)
	os.WriteFile(filepath.Join(dir, "proz", "store.json"), []byte(`[{"name": "web", "path": "/web"}, {"name": "api", "path": "/api"}]`), 0644)

	run(t, "config", "stale-days", "30", "--project", "web")
	if out := run(t, "config", "stale-days", "--project", "web"); out != "30\n" {
		t.Fatalf("override: got %q", out)
	}
	if out := run(t, "config", "stale-days", "--project", "api"); out != "7\n" {
		t.Fatalf("no override shows global: got %q", out)
	}
	run(t, "config", "stale-days", "0", "--project", "web")
	if out := run(t, "config", "stale-days", "--project", "web"); out != "7\n" {
		t.Fatalf("cleared: got %q", out)
	}
	for _, name := range []string{"nope", ""} {
		if execute("config", "stale-days", "5", "--project", name) == nil {
			t.Fatalf("project %q should fail", name)
		}
	}
	if out := run(t, "config", "stale-days", "--project", "api"); out != "7\n" {
		t.Fatalf("global must be untouched by failed sets: got %q", out)
	}
}

func TestStatusResolvesProjectFromWorkingDirectory(t *testing.T) {
	dir := configDir(t)
	project := filepath.Join(dir, "web")
	os.MkdirAll(filepath.Join(project, "src", "deep"), 0775)
	os.MkdirAll(filepath.Join(dir, "proz"), 0775)
	os.WriteFile(filepath.Join(dir, "proz", "store.json"), []byte(`[{"name": "web", "path": `+jsonString(project)+`}]`), 0644)
	t0 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	now = func() time.Time { return t0 }
	run(t, "list", "--json") // import at t0
	now = func() time.Time { return t0.Add(3 * 24 * time.Hour) }
	t.Cleanup(func() { now = time.Now })

	if out := run(t, "status", "web"); out != "Quiet for 3 days\n" {
		t.Fatalf("by name: got %q", out)
	}
	t.Chdir(filepath.Join(project, "src", "deep"))
	if out := run(t, "status"); out != "Quiet for 3 days\n" {
		t.Fatalf("from subfolder: got %q", out)
	}
	want := `{
  "name": "web",
  "path": ` + jsonString(project) + `,
  "folders": [
    {
      "path": ` + jsonString(project) + `,
      "main": true,
      "missing": false
    }
  ],
  "links": [],
  "last_activity": "2026-10-01T09:00:00Z",
  "quiet_days": 3,
  "quiet_seconds": 259200,
  "note": "Quiet for 3 days",
  "short_note": "Quiet for 3 days",
  "stale": false,
  "state": "active",
  "milestones": []
}
`
	if out := run(t, "status", "--json"); out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
	t.Chdir(dir)
	if execute("status") == nil {
		t.Fatal("outside any project with no name should fail")
	}
}

func TestMilestoneByNameAndFromWorkingDirectory(t *testing.T) {
	dir := configDir(t)
	project := filepath.Join(dir, "web")
	os.MkdirAll(filepath.Join(project, "src"), 0775)
	os.MkdirAll(filepath.Join(dir, "proz"), 0775)
	os.WriteFile(filepath.Join(dir, "proz", "store.json"), []byte(`[{"name": "web", "path": `+jsonString(project)+`}]`), 0644)
	t0 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	t.Cleanup(func() { now = time.Now })

	now = func() time.Time { return t0 }
	run(t, "milestone", "web", "spike done")
	now = func() time.Time { return t0.Add(24 * time.Hour) }
	t.Chdir(filepath.Join(project, "src"))
	run(t, "milestone", "api wired; next: tests")
	now = func() time.Time { return t0.Add(3 * 24 * time.Hour) }

	if out, want := run(t, "status"), "Quiet for 2 days\nMilestone: api wired; next: tests\n\nMilestones:\n2026-10-02  api wired; next: tests\n2026-10-01  spike done\n"; out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
	want := `{
  "name": "web",
  "path": ` + jsonString(project) + `,
  "folders": [
    {
      "path": ` + jsonString(project) + `,
      "main": true,
      "missing": false
    }
  ],
  "links": [],
  "last_activity": "2026-10-02T09:00:00Z",
  "quiet_days": 2,
  "quiet_seconds": 172800,
  "note": "Quiet for 2 days\nMilestone: api wired; next: tests",
  "short_note": "Quiet for 2 days; Milestone: api wired; next: tests",
  "stale": false,
  "state": "active",
  "milestones": [
    {
      "text": "api wired; next: tests",
      "at": "2026-10-02T09:00:00Z"
    },
    {
      "text": "spike done",
      "at": "2026-10-01T09:00:00Z"
    }
  ]
}
`
	if out := run(t, "status", "--json"); out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
	if execute("milestone", "web", "  ") == nil {
		t.Fatal("blank milestone should fail")
	}
	if execute("milestone", "shipped login", "next: signup") == nil {
		t.Fatal("two args means project then text; an unknown project must fail, not guess")
	}
}

func TestIdeaCaptureResurfaces(t *testing.T) {
	configDir(t)
	t0 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	t.Cleanup(func() { now = time.Now })
	now = func() time.Time { return t0 }

	run(t, "idea", "habit", "tracker", "app")
	now = func() time.Time { return t0.Add(8 * 24 * time.Hour) }
	want := `[
  {
    "name": "habit tracker app",
    "path": "",
    "folders": [],
    "links": [],
    "last_activity": "2026-10-01T09:00:00Z",
    "quiet_days": 8,
    "quiet_seconds": 691200,
    "note": "Quiet for 8 days",
    "short_note": "Quiet for 8 days"
  }
]
`
	if out := run(t, "nudges", "--json"); out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
	if execute("idea") == nil {
		t.Fatal("idea with no text should fail")
	}
}

func TestAddOffersToAttachAFolderToAMatchingIdea(t *testing.T) {
	dir := configDir(t)
	t.Cleanup(func() { stdin = bufio.NewReader(os.Stdin) })
	folder := func(name string) string {
		p := filepath.Join(dir, name, "habit-tracker")
		os.MkdirAll(p, 0775)
		return p
	}
	type entry struct{ Name, Path string }
	list := func() (out []entry) {
		json.Unmarshal([]byte(run(t, "list", "--json")), &out)
		return out
	}

	run(t, "idea", "Habit Tracker")
	run(t, "milestone", "Habit Tracker", "sketched data model")

	stdin = bufio.NewReader(strings.NewReader("n\n\n")) // declined: a separate project
	declined := folder("a")
	run(t, declined)
	if got := list(); len(got) != 2 || got[0].Path != "" || got[1].Path != declined {
		t.Fatalf("decline: got %+v", got)
	}

	stdin = bufio.NewReader(strings.NewReader("")) // no answer, as for an agent: attach
	attached := folder("b")
	run(t, attached)
	got := list()
	if len(got) != 2 || got[0] != (entry{"Habit Tracker", attached}) {
		t.Fatalf("attach: got %+v", got)
	}
	if out := run(t, "status", "Habit Tracker"); !strings.Contains(out, "sketched data model") {
		t.Fatalf("attached idea must keep its milestones: %q", out)
	}
}

func TestAddAsksBeforeTrackingAnotherProjectsFolderOrWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("needs git")
	}
	configDir(t)
	t.Cleanup(func() { stdin = bufio.NewReader(os.Stdin) })
	web := filepath.Join(t.TempDir(), "web")
	os.MkdirAll(web, 0775)
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "start"}} {
		if out, err := exec.Command("git", append([]string{"-C", web}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	wt := filepath.Join(filepath.Dir(web), "web-fix")
	if out, err := exec.Command("git", "-C", web, "worktree", "add", "-q", wt).CombinedOutput(); err != nil {
		t.Fatalf("worktree: %s", out)
	}
	stdin = bufio.NewReader(strings.NewReader("\n"))
	run(t, web)
	count := func() int {
		var out []struct{ Name string }
		json.Unmarshal([]byte(run(t, "list", "--json")), &out)
		return len(out)
	}

	for _, dir := range []string{web, wt} {
		for _, answer := range []string{"", "n\n"} { // no answer (an agent) or no: nothing added
			stdin = bufio.NewReader(strings.NewReader(answer))
			if out := run(t, dir); !strings.Contains(out, "web") || !strings.Contains(out, "Not added") || count() != 1 {
				t.Fatalf("%s, answer %q: %q, %d projects", dir, answer, out, count())
			}
		}
	}
	stdin = bufio.NewReader(strings.NewReader("y\n\n"))
	run(t, wt)
	if count() != 2 {
		t.Fatalf("confirmed: want a separate project, got %d", count())
	}
}

func TestRespondingToNudges(t *testing.T) {
	dir := configDir(t)
	os.MkdirAll(filepath.Join(dir, "proz"), 0775)
	web := t.TempDir() // its own folder: the config dir's database file would count as activity
	os.WriteFile(filepath.Join(dir, "proz", "store.json"), []byte(`[
  {"name": "web", "path": `+jsonString(web)+`, "programs": [`+jsonString(noop(t))+`]},
  {"name": "api", "path": "/api"}
]`), 0644)
	t0 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	t.Cleanup(func() { now = time.Now })
	at := func(days int) { now = func() time.Time { return t0.Add(time.Duration(days) * 24 * time.Hour) } }
	state := func(name string) (out struct {
		State        string `json:"state"`
		SnoozedUntil string `json:"snoozed_until"`
	}) {
		json.Unmarshal([]byte(run(t, "status", name, "--json")), &out)
		return out
	}
	names := func(args ...string) (out []string) {
		var entries []struct{ Name string }
		json.Unmarshal([]byte(run(t, args...)), &entries)
		for _, e := range entries {
			out = append(out, e.Name)
		}
		return out
	}

	at(0)
	run(t, "list", "--json") // import at t0
	at(10)
	if got := names("nudges", "--json"); len(got) != 2 {
		t.Fatalf("both stale: got %v", got)
	}

	run(t, "snooze", "web", "3")
	if got := state("web"); got.State != "snoozed" || got.SnoozedUntil != "2026-10-14T09:00:00Z" {
		t.Fatalf("snooze: got %+v", got)
	}
	run(t, "drop", "api")
	if got := names("archived", "--json"); len(got) != 1 || got[0] != "api" {
		t.Fatalf("archived: got %v", got)
	}
	if got := names("nudges", "--json"); len(got) != 0 {
		t.Fatalf("snoozed and dropped: want none, got %v", got)
	}

	run(t, "restore", "api")
	if got := state("api"); got.State != "stale" {
		t.Fatalf("restore: got %+v", got)
	}

	at(20)
	if out := run(t, "resume", "web"); !strings.HasPrefix(out, "Quiet for 20 days\n") {
		t.Fatalf("resume prints the note first: %q", out)
	}
	if got := state("web"); got.State != "active" {
		t.Fatalf("resume is a launch, so activity: got %+v", got)
	}
	if execute("snooze", "web", "0") == nil {
		t.Fatal("zero-day snooze should fail")
	}
}

func TestSnoozeArguments(t *testing.T) {
	dir := configDir(t)
	project := filepath.Join(dir, "web")
	os.MkdirAll(project, 0775)
	os.MkdirAll(filepath.Join(dir, "proz"), 0775)
	os.WriteFile(filepath.Join(dir, "proz", "store.json"), []byte(`[{"name": "web", "path": `+jsonString(project)+`}, {"name": "2024", "path": "/old"}]`), 0644)
	t0 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	t.Cleanup(func() { now = time.Now })
	now = func() time.Time { return t0 }
	until := func(name string) string {
		var out struct {
			SnoozedUntil string `json:"snoozed_until"`
		}
		json.Unmarshal([]byte(run(t, "status", name, "--json")), &out)
		return out.SnoozedUntil
	}
	run(t, "list", "--json")
	now = func() time.Time { return t0.Add(10 * 24 * time.Hour) }

	run(t, "snooze", "2024") // a project named like a number is the project
	if got := until("2024"); got != "2026-10-18T09:00:00Z" {
		t.Fatalf("named 2024, default 7 days: got %q", got)
	}
	t.Chdir(project)
	run(t, "snooze", "3") // no such project: days, for the project here
	if got := until("web"); got != "2026-10-14T09:00:00Z" {
		t.Fatalf("days for cwd project: got %q", got)
	}
	run(t, "snooze", "2024", "1")
	if got := until("2024"); got != "2026-10-12T09:00:00Z" {
		t.Fatalf("project then days: got %q", got)
	}
}

func TestCheckDeliversOnceAndRespectsTheSwitch(t *testing.T) {
	dir := configDir(t)
	os.MkdirAll(filepath.Join(dir, "proz"), 0775)
	os.WriteFile(filepath.Join(dir, "proz", "store.json"), []byte(`[{"name": "web", "path": "/web"}]`), 0644)
	t0 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	var shown []notify.Message
	t.Cleanup(func() { now = time.Now; send = notify.Send })
	send = func(m []notify.Message) (int, error) { shown = append(shown, m...); return len(m), nil }
	now = func() time.Time { return t0 }
	run(t, "list", "--json")
	now = func() time.Time { return t0.Add(8 * 24 * time.Hour) }

	run(t, "config", "notifications", "off")
	if out := run(t, "config", "notifications"); out != "off\n" {
		t.Fatalf("switch: got %q", out)
	}
	if out := run(t, "check", "--json"); out != "[]\n" || len(shown) != 0 {
		t.Fatalf("notifications off: got %q, shown %+v", out, shown)
	}

	run(t, "config", "notifications", "on")
	var notified []struct{ Name string }
	json.Unmarshal([]byte(run(t, "check", "--json")), &notified)
	if len(notified) != 1 || notified[0].Name != "web" || len(shown) != 1 || shown[0].Title != "web went quiet" {
		t.Fatalf("got %+v, shown %+v", notified, shown)
	}
	if out := run(t, "check", "--json"); out != "[]\n" || len(shown) != 1 {
		t.Fatalf("second check must not repeat: got %q, shown %d", out, len(shown))
	}

	run(t, "test", "notification")
	if len(shown) != 2 || !strings.Contains(shown[1].Title, "proz") {
		t.Fatalf("test notification: shown %+v", shown)
	}
	if execute("config", "notifications", "maybe") == nil {
		t.Fatal("only on or off")
	}
}

func TestEmailDigestIsConfiguredThenSentWeeklyByCheck(t *testing.T) {
	dir := configDir(t)
	os.MkdirAll(filepath.Join(dir, "proz"), 0775)
	os.WriteFile(filepath.Join(dir, "proz", "store.json"), []byte(`[{"name": "web", "path": "/web"}]`), 0644)
	t0 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC) // a Thursday
	var sent []string                                  // subjects
	t.Cleanup(func() {
		now = time.Now
		send = notify.Send
		sendEmail = notify.Email
		stdin = bufio.NewReader(os.Stdin)
	})
	send = func(m []notify.Message) (int, error) { return len(m), nil }
	sendEmail = func(c core.SMTP, m notify.Mail, _ time.Time) error {
		if c.Password != "s3cret" || !strings.Contains(m.Text, "proz snooze ") || !strings.Contains(m.HTML, "proz snooze ") {
			t.Errorf("smtp %+v, mail %+v", c, m)
		}
		sent = append(sent, m.Subject)
		return nil
	}
	now = func() time.Time { return t0 }
	run(t, "list", "--json")

	if execute("config", "email", "on") == nil {
		t.Fatal("email must stay off until SMTP is configured")
	}
	if execute("test", "email") == nil {
		t.Fatal("test email needs SMTP")
	}
	stdin = bufio.NewReader(strings.NewReader("s3cret\n"))
	run(t, "config", "smtp", "--host", "smtp.example.com", "--username", "me", "--from", "me@example.com", "--to", "me@example.com", "--password-stdin")
	if out := run(t, "config", "email"); out != "on\n" {
		t.Fatalf("configuring SMTP switches email on: got %q", out)
	}
	if out := run(t, "config", "smtp"); !strings.Contains(out, "smtp.example.com:587") || strings.Contains(out, "s3cret") {
		t.Fatalf("show smtp without the password: got %q", out)
	}

	now = func() time.Time { return t0.Add(11 * 24 * time.Hour) } // Monday, web stale
	run(t, "check")
	run(t, "check")
	if len(sent) != 1 || sent[0] != "proz: web went quiet" {
		t.Fatalf("one digest a week: %v", sent)
	}
	run(t, "test", "email")
	if len(sent) != 2 {
		t.Fatalf("test email: %v", sent)
	}

	if execute("config", "smtp", "--host", "evil.example.com") == nil {
		t.Fatal("a new host must not get the saved password")
	}
	run(t, "config", "email", "off")
	run(t, "config", "smtp", "--to", "other@example.com")
	if out := run(t, "config", "email"); out != "off\n" {
		t.Fatalf("editing SMTP keeps email off: got %q", out)
	}
	run(t, "config", "email", "on")

	// email failing must not hide the notifications already shown
	now = func() time.Time { return t0.Add(14 * 24 * time.Hour) }
	run(t, "milestone", "web", "back at it") // a new stale period to notify about
	now = func() time.Time { return t0.Add(25 * 24 * time.Hour) }
	sendEmail = func(core.SMTP, notify.Mail, time.Time) error { return errors.New("smtp down") }
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w
	err := execute("check", "--json")
	os.Stdout = stdout
	w.Close()
	out, _ := io.ReadAll(r)
	if err == nil || !strings.Contains(string(out), `"name": "web"`) {
		t.Fatalf("check --json with email down: err %v, out %q", err, out)
	}

	if out := run(t, "config", "digest-day"); out != "monday\n" {
		t.Fatalf("default digest day: got %q", out)
	}
	run(t, "config", "digest-day", "Friday")
	if out := run(t, "config", "digest-day"); out != "friday\n" {
		t.Fatalf("digest day: got %q", out)
	}
	if execute("config", "digest-day", "someday") == nil {
		t.Fatal("not a weekday")
	}
}

func TestBrainIsConfiguredReadAndSyncedByCheck(t *testing.T) {
	dir := configDir(t)
	os.MkdirAll(filepath.Join(dir, "proz"), 0775)
	os.WriteFile(filepath.Join(dir, "proz", "store.json"), []byte(`[{"name": "web", "path": "/web"}]`), 0644)
	t.Cleanup(func() { send = notify.Send })
	send = func(m []notify.Message) (int, error) { return len(m), nil }
	brain := t.TempDir()

	if execute("config", "brain", "on") == nil {
		t.Fatal("brain stays off until a folder is set")
	}
	if execute("brain", "web") == nil {
		t.Fatal("reading with no brain must fail")
	}
	if out := run(t, "config", "brain-path"); out != "not set\n" {
		t.Fatalf("unset: %q", out)
	}
	run(t, "config", "brain-path", brain)
	if out := run(t, "config", "brain-path"); out != brain+"\n" {
		t.Fatalf("brain-path: %q", out)
	}
	if out := run(t, "config", "brain"); out != "on\n" {
		t.Fatalf("setting a folder switches the brain on: %q", out)
	}

	run(t, "check")
	if _, err := os.Stat(filepath.Join(brain, "web.md")); err != nil {
		t.Fatalf("check mirrors notes: %v", err)
	}
	var note struct{ Path, Content string }
	json.Unmarshal([]byte(run(t, "brain", "web", "--json")), &note)
	if note.Path != filepath.Join(brain, "web.md") || !strings.Contains(note.Content, "<!-- proz:start -->") {
		t.Fatalf("brain --json: %+v", note)
	}
	if out := run(t, "brain", "web"); out != note.Content {
		t.Fatalf("brain prints the note: %q", out)
	}
}

func TestTrayStaysOnUnlessTheSettingSaysOff(t *testing.T) {
	broken := filepath.Join(t.TempDir(), "file")
	os.WriteFile(broken, nil, 0o644)
	useConfigDir(t, broken) // the store cannot open
	if !trayEnabled() {
		t.Fatal("an unreadable setting must keep the tray running")
	}
	configDir(t)
	if err := withStore(func(s *core.Store) error { return s.SetModule(core.ModuleTray, false) }); err != nil {
		t.Fatal(err)
	}
	if trayEnabled() {
		t.Fatal("tray off must read as off")
	}
}

func TestTraySnoozeHidesTheProjectForTheChosenDays(t *testing.T) {
	configDir(t)
	var p core.Project
	if err := withStore(func(s *core.Store) (err error) {
		if err = s.AddProject(core.Project{Name: "web", Path: t.TempDir()}); err != nil {
			return err
		}
		p, err = s.ProjectNamed("web")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := traySnooze(p.ID, 14); err != nil {
		t.Fatal(err)
	}
	var st core.Status
	withStore(func(s *core.Store) (err error) { st, err = s.Status(p.ID); return err })
	if d := st.SnoozedUntil.Sub(now()); d < 14*24*time.Hour-time.Minute || d > 14*24*time.Hour {
		t.Fatalf("snoozed for %v", d)
	}
}

func TestSnoozeUntilADate(t *testing.T) {
	dir := configDir(t)
	os.MkdirAll(filepath.Join(dir, "proz"), 0775)
	os.WriteFile(filepath.Join(dir, "proz", "store.json"), []byte(`[{"name": "web", "path": `+jsonString(t.TempDir())+`}]`), 0644)
	t0 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	t.Cleanup(func() { now = time.Now })
	now = func() time.Time { return t0 }
	run(t, "list", "--json")
	now = func() time.Time { return t0.Add(10 * 24 * time.Hour) } // Oct 11, stale
	until := func() string {
		var out struct {
			SnoozedUntil string `json:"snoozed_until"`
		}
		json.Unmarshal([]byte(run(t, "status", "web", "--json")), &out)
		return out.SnoozedUntil
	}
	for _, bad := range [][]string{
		{"snooze", "web", "--until", "2026-10-11"}, // today
		{"snooze", "web", "--until", "2026-10-01"}, // past
		{"snooze", "web", "--until", "2200-01-01"}, // over 100 years
		{"snooze", "web", "--until", "next week"},
		{"snooze", "web", "3", "--until", "2026-11-09"},
		{"snooze", "web", "--until", ""}, // an empty $DATE must not fall back to 7 days
	} {
		if execute(bad...) == nil {
			t.Errorf("%v: want an error", bad)
		}
		if got := until(); got != "" {
			t.Fatalf("%v changed the snooze to %q", bad, got)
		}
	}
	if out := run(t, "snooze", "web", "--until", "2026-11-09"); !strings.Contains(out, "until Mon 9 Nov 2026") {
		t.Fatalf("output %q", out)
	}
	if got := until(); got != "2026-11-09T00:00:00Z" {
		t.Fatalf("snoozed until %q, want the start of that day", got)
	}
}

func TestSnoozeUntilNamesAMissingProject(t *testing.T) {
	configDir(t)
	if err := execute("snooze", "3", "--until", "2026-11-09"); err == nil || !strings.Contains(err.Error(), `"3" not found`) {
		t.Fatalf("got %v", err)
	}
}

func TestStartOfDaySurvivesAMissingMidnight(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo") // clocks jumped from 00:00 to 01:00 on 2018-11-04
	if err != nil {
		t.Skip(err)
	}
	got, err := startOfDay("2018-11-04", loc)
	if err != nil || got.Format(time.DateOnly) != "2018-11-04" {
		t.Fatalf("got %v, %v", got, err)
	}
}

// jsonString is s as a JSON string, so Windows paths survive in store.json.
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// noop puts a program that does nothing and succeeds on PATH and returns
// its name; Windows has no true.
func noop(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	name, body := "proz-noop", "#!/bin/sh\n"
	if runtime.GOOS == "windows" {
		name, body = "proz-noop.cmd", "@exit /b 0\r\n"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return name
}

// Agents fix a person's terminal with `proz config terminal --json`.
func TestConfigTerminalJSONContract(t *testing.T) {
	configDir(t)
	var got map[string]string
	if err := json.Unmarshal([]byte(run(t, "config", "terminal", "--json")), &got); err != nil || got["terminal"] != "" {
		t.Fatalf("unset: want empty terminal, got %v, %v", got, err)
	}
	if err := json.Unmarshal([]byte(run(t, "config", "terminal", "wezterm start --", "--json")), &got); err != nil || got["terminal"] != "wezterm start --" {
		t.Fatalf("set: got %v, %v", got, err)
	}
	if out := strings.TrimSpace(run(t, "config", "terminal")); out != "wezterm start --" {
		t.Fatalf("plain: got %q", out)
	}
	run(t, "config", "terminal", "")
	if out := strings.TrimSpace(run(t, "config", "terminal")); out != "detect" {
		t.Fatalf("cleared: got %q", out)
	}
}

// The launcher's light or dark look, also set with `proz config theme`.
func TestConfigThemeJSONContract(t *testing.T) {
	configDir(t)
	var got map[string]string
	if err := json.Unmarshal([]byte(run(t, "config", "theme", "--json")), &got); err != nil || got["theme"] != "system" {
		t.Fatalf("unset: want system, got %v, %v", got, err)
	}
	if err := json.Unmarshal([]byte(run(t, "config", "theme", "dark", "--json")), &got); err != nil || got["theme"] != "dark" {
		t.Fatalf("set: got %v, %v", got, err)
	}
	if out := strings.TrimSpace(run(t, "config", "theme")); out != "dark" {
		t.Fatalf("plain: got %q", out)
	}
	if err := execute("config", "theme", "blue"); err == nil || !strings.Contains(err.Error(), "system, light or dark") {
		t.Fatalf("want a refusal naming the choices, got %v", err)
	}
	run(t, "config", "theme", "system")
	if out := strings.TrimSpace(run(t, "config", "theme")); out != "system" {
		t.Fatalf("back to system: got %q", out)
	}
}

// Agents fix a wrong guess with `proz config needs-terminal --json`.
func TestConfigNeedsTerminalJSONContract(t *testing.T) {
	configDir(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	var got map[string]any
	check := func(out string, want map[string]any) {
		t.Helper()
		got = nil
		if err := json.Unmarshal([]byte(out), &got); err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v, %v; want %v", got, err, want)
		}
	}
	check(run(t, "config", "needs-terminal", "nvim", "--json"), map[string]any{"program": "nvim", "needs_terminal": true, "set": false})
	check(run(t, "config", "needs-terminal", "nvim", "off", "--json"), map[string]any{"program": "nvim", "needs_terminal": false, "set": true})
	if out := strings.TrimSpace(run(t, "config", "needs-terminal", "nvim")); out != "off (set)" {
		t.Fatalf("plain: got %q", out)
	}
	check(run(t, "config", "needs-terminal", "nvim", "auto", "--json"), map[string]any{"program": "nvim", "needs_terminal": true, "set": false})
	if err := execute("config", "needs-terminal", "nvim", "maybe"); err == nil {
		t.Fatal("want an error for maybe")
	}
}

// The launcher's "Start Claude Code here" from the CLI: the agent starts in
// the project's folder, in the user's terminal, without the project's arguments.
func TestResumeAgentStartsTheAgentCLIInTheProjectFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake programs are shell scripts")
	}
	dir := configDir(t)
	project := filepath.Join(dir, "web")
	os.MkdirAll(project, 0775)
	os.MkdirAll(filepath.Join(dir, "proz"), 0775)
	os.WriteFile(filepath.Join(dir, "proz", "store.json"), []byte(`[{"name": "web", "path": `+jsonString(project)+`, "programs": ["code"], "arguments": ["--new-window"]}]`), 0644)
	bin, record := t.TempDir(), filepath.Join(t.TempDir(), "ran")
	os.WriteFile(filepath.Join(bin, "fake-term"), []byte("#!/bin/sh\nshift\nexec \"$@\"\n"), 0755) // drops its "--"
	os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\necho \"$(pwd) $*\" > "+record+"\n"), 0755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	run(t, "config", "terminal", "fake-term --")

	run(t, "resume", "web", "--agent", "claude")
	var ran []byte
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline) && len(ran) == 0; time.Sleep(20 * time.Millisecond) {
		ran, _ = os.ReadFile(record)
	}
	if got := strings.TrimSpace(string(ran)); got != project+" " && got != project {
		t.Fatalf("want claude run in %s with no arguments, got %q", project, got)
	}
	if err := execute("resume", "web", "--agent", "claude", "--terminal"); err == nil {
		t.Fatal("--agent and --terminal together should fail")
	}
}

// Agents rename a project and set its programs without a terminal, as the
// launcher's setup does.
func TestEditWithFlagsSetsNameAndProgramsWithoutPrompting(t *testing.T) {
	configDir(t)
	run(t, "idea", "habit tracker")
	run(t, "edit", "habit tracker", "--name", "habits", "--program", "code", "--program", "nvim")
	var got []Project
	if err := json.Unmarshal([]byte(run(t, "list", "--json")), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "habits" || !reflect.DeepEqual(got[0].Programs, []string{"code", "nvim"}) {
		t.Fatalf("got %+v", got)
	}
	run(t, "edit", "habits", "--program", "")
	got = nil
	if err := json.Unmarshal([]byte(run(t, "list", "--json")), &got); err != nil {
		t.Fatal(err)
	}
	if got[0].Name != "habits" || len(got[0].Programs) != 0 {
		t.Fatalf("--program \"\" clears them and keeps the name: %+v", got)
	}
	if execute("edit", "nope", "--name", "x") == nil {
		t.Fatal("a missing project must fail")
	}
	if execute("edit", "habits", "--name", " ") == nil {
		t.Fatal("an empty name must fail")
	}
}
