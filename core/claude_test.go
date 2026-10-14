package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// claudeLog writes a Claude Code session log for cwd, as Claude Code lays
// them out: <config>/projects/<cwd with non-alphanumerics as '-'>/<id>.jsonl.
func claudeLog(t *testing.T, config, cwd, id string, at time.Time, records ...map[string]any) {
	t.Helper()
	dir := filepath.Join(config, "projects", regexp.MustCompile(`[^A-Za-z0-9]`).ReplaceAllString(cwd, "-"))
	os.MkdirAll(dir, 0755)
	var b strings.Builder
	for _, r := range records {
		r["cwd"] = cwd
		if _, ok := r["timestamp"]; !ok {
			r["timestamp"] = at.Format(time.RFC3339Nano)
		}
		line, _ := json.Marshal(r)
		b.Write(append(line, '\n'))
	}
	path := filepath.Join(dir, id+".jsonl")
	os.WriteFile(path, []byte(b.String()), 0644)
	os.Chtimes(path, at, at)
}

func prompt(text string) map[string]any {
	return map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": text}}
}

func TestClaudeSessionInTheProjectIsActivityAndLastPromptJoinsTheNote(t *testing.T) {
	config := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", config)
	c := &clock{t0}
	s := openAt(t, c)
	project := filepath.Join(t.TempDir(), "web")
	os.MkdirAll(filepath.Join(project, "api"), 0755)
	s.AddProject(Project{Name: "web", Path: project})

	claudeLog(t, config, filepath.Join(project, "api"), "s1", t0.Add(5*day),
		prompt("wire the signup form to the api"),
		map[string]any{"type": "user", "isMeta": true, "message": map[string]any{"content": "caveat"}},
		map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "content": "ok"}}}},
		prompt("<command-name>/compact</command-name>"),
		map[string]any{"type": "assistant", "message": map[string]any{"content": "done"}},
	)

	c.t = t0.Add(10 * day)
	st := status(t, s)
	if !st.LastActivity.Equal(t0.Add(5 * day)) {
		t.Fatalf("session should be the last activity, got %v", st.LastActivity)
	}
	if got := st.Note.Lines[len(st.Note.Lines)-1]; got != "Last prompt: wire the signup form to the api" {
		t.Fatalf("got %q in %q", got, st.Note.Lines)
	}
}

func TestOnlyTheLatestSessionCountsAndLongPromptsAreTruncated(t *testing.T) {
	config := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", config)
	c := &clock{t0}
	s := openAt(t, c)
	project := filepath.Join(t.TempDir(), "web")
	os.MkdirAll(project, 0755)
	s.AddProject(Project{Name: "web", Path: project})

	claudeLog(t, config, project, "old", t0.Add(day), prompt("old question"))
	long := "refactor " + strings.Repeat("the billing module ", 20) + "\n\nand more"
	claudeLog(t, config, project, "new", t0.Add(2*day), prompt("first"), prompt(long))

	got := status(t, s).Note.Lines
	last := got[len(got)-1]
	if !strings.HasPrefix(last, "Last prompt: refactor the billing module the billing") || len([]rune(last)) != len("Last prompt: ")+maxPromptLen || strings.Contains(last, "\n") {
		t.Fatalf("got %q", last)
	}
}

func TestSessionsInOtherFoldersDoNotCount(t *testing.T) {
	config := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", config)
	c := &clock{t0}
	s := openAt(t, c)
	root := t.TempDir()
	project := filepath.Join(root, "web")
	os.MkdirAll(project, 0755)
	s.AddProject(Project{Name: "web", Path: project})

	claudeLog(t, config, filepath.Join(root, "web2"), "a", t0.Add(5*day), prompt("not this one")) // shares the encoded prefix
	claudeLog(t, config, root, "b", t0.Add(5*day), prompt("nor the parent"))

	c.t = t0.Add(10 * day)
	st := status(t, s)
	if !st.LastActivity.Equal(t0) || len(st.Note.Lines) != 1 {
		t.Fatalf("got %v %q", st.LastActivity, st.Note.Lines)
	}
}

func TestNoClaudeDirectoryIsFine(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(t.TempDir(), "missing"))
	s := open(t, t.TempDir())
	s.AddProject(Project{Name: "web", Path: t.TempDir()})
	if st := status(t, s); len(st.Note.Lines) != 1 {
		t.Fatalf("got %q", st.Note.Lines)
	}
}
