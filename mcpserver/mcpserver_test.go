package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/njayman/proz/core"
)

var t0 = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

// connect starts the server on a temp store and returns a client session.
func connect(t *testing.T, now *time.Time) (*mcp.ClientSession, string) {
	t.Helper()
	dir := t.TempDir()
	open := func() (*core.Store, error) { return core.Open(dir, func() time.Time { return *now }) }
	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := New(open, "test").Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs, dir
}

// call runs a tool and decodes its structured result into out.
func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any, out any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	if out != nil && !res.IsError {
		b, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatal(err)
		}
	}
	return res
}

func keys(m map[string]any) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func TestToolNamesAreTheContract(t *testing.T) {
	now := t0
	cs, _ := connect(t, &now)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
		if tool.Description == "" {
			t.Errorf("%s has no description", tool.Name)
		}
	}
	slices.Sort(names)
	want := []string{"add_link", "capture_idea", "get_project", "list_nudges", "list_projects", "read_brain_note", "record_milestone"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %v", names)
	}
}

func TestToolsEndToEndWithPinnedFields(t *testing.T) {
	now := t0
	cs, dir := connect(t, &now)
	project := t.TempDir()
	os.MkdirAll(filepath.Join(project, "src"), 0755)
	s, _ := core.Open(dir, func() time.Time { return t0 }) // MCP has no register tool in v1
	s.AddProject(core.Project{Name: "web", Path: project})
	s.Close()

	var idea map[string]any
	call(t, cs, "capture_idea", map[string]any{"text": "  habit   tracker "}, &idea)
	if !reflect.DeepEqual(idea, map[string]any{"name": "habit tracker"}) {
		t.Fatalf("capture_idea: %v", idea)
	}

	var ms map[string]any
	call(t, cs, "record_milestone", map[string]any{"dir": filepath.Join(project, "src"), "text": "api wired"}, &ms)
	if !reflect.DeepEqual(ms, map[string]any{"name": "web"}) {
		t.Fatalf("record_milestone by dir: %v", ms)
	}

	statusFields := []string{"folders", "last_activity", "links", "milestones", "name", "note", "path", "quiet_days", "quiet_seconds", "short_note", "stale", "state"}
	var byDir map[string]any
	call(t, cs, "get_project", map[string]any{"dir": project}, &byDir)
	if byDir["name"] != "web" {
		t.Fatalf("get_project by the folder itself: %v", byDir)
	}
	var got map[string]any
	call(t, cs, "get_project", map[string]any{"project": "web"}, &got)
	if k := keys(got); !reflect.DeepEqual(k, statusFields) {
		t.Fatalf("get_project fields: %v", k)
	}
	if got["note"] != "Quiet for less than an hour\nMilestone: api wired" {
		t.Fatalf("note: %v", got["note"])
	}

	var list struct{ Projects []map[string]any }
	call(t, cs, "list_projects", nil, &list)
	if len(list.Projects) != 2 || !reflect.DeepEqual(keys(list.Projects[0]), statusFields) {
		t.Fatalf("list_projects: %v", list.Projects)
	}

	now = t0.Add(10 * 24 * time.Hour)
	var nudges struct{ Nudges []map[string]any }
	call(t, cs, "list_nudges", nil, &nudges)
	nudgeFields := []string{"folders", "last_activity", "links", "name", "note", "path", "quiet_days", "quiet_seconds", "short_note"}
	if len(nudges.Nudges) != 2 || !reflect.DeepEqual(keys(nudges.Nudges[0]), nudgeFields) {
		t.Fatalf("list_nudges: %v", nudges.Nudges)
	}
}

func TestProjectArgumentsAreValidated(t *testing.T) {
	now := t0
	cs, dir := connect(t, &now)
	project := t.TempDir()
	s, _ := core.Open(dir, func() time.Time { return t0 })
	s.AddProject(core.Project{Name: "web", Path: project})
	s.Close()
	t.Chdir(project) // so a relative dir would resolve, wrongly, to web

	for name, args := range map[string]map[string]any{
		"neither":      {},
		"unknown name": {"project": "nope"},
		"outside":      {"dir": t.TempDir()},
		"relative dir": {"dir": "."},                                     // relative to the server's cwd, not the agent's
		"both":         {"project": "web", "dir": filepath.Dir(project)}, // which one did you mean?
	} {
		if res := call(t, cs, "get_project", args, nil); !res.IsError {
			t.Errorf("%s: want a tool error", name)
		}
	}
	if res := call(t, cs, "capture_idea", map[string]any{"text": " "}, nil); !res.IsError {
		t.Error("blank idea: want a tool error")
	}
}

func TestAddLink(t *testing.T) {
	now := t0
	cs, dir := connect(t, &now)
	project := t.TempDir()
	s, _ := core.Open(dir, func() time.Time { return t0 })
	s.AddProject(core.Project{Name: "web", Path: project})
	s.Close()

	res := call(t, cs, "add_link", map[string]any{"dir": project, "ref": "https://tasks.example.com/web"}, nil)
	if !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "brainmd") {
		t.Fatalf("unknown host: want a tool error naming the choices, got %+v", res.Content)
	}
	var got, again map[string]any
	call(t, cs, "add_link", map[string]any{"project": "web", "ref": "https://tasks.example.com/web", "provider": "other", "kind": "tracker"}, &got)
	link := map[string]any{"kind": "tracker", "provider": "web", "ref": "https://tasks.example.com/web", "missing": false}
	if links, _ := got["links"].([]any); len(links) != 1 || !reflect.DeepEqual(links[0], link) {
		t.Fatalf("add_link returns the project with its links: %v", got)
	}
	call(t, cs, "get_project", map[string]any{"project": "web"}, &again)
	if !reflect.DeepEqual(got, again) {
		t.Fatalf("add_link returns what get_project does:\n%v\n%v", got, again)
	}
}

func TestReadBrainNote(t *testing.T) {
	now := t0
	cs, dir := connect(t, &now)
	brain := t.TempDir()
	s, _ := core.Open(dir, func() time.Time { return t0 })
	s.AddProject(core.Project{Name: "web", Path: t.TempDir()})
	s.Close()

	if res := call(t, cs, "read_brain_note", map[string]any{"project": "web"}, nil); !res.IsError {
		t.Fatal("no brain configured must be an error")
	}
	s, _ = core.Open(dir, func() time.Time { return t0 })
	s.SetBrainPath(brain)
	s.Close()
	os.WriteFile(filepath.Join(brain, "web.md"), []byte("mine\n"), 0o644)
	var got map[string]any
	call(t, cs, "read_brain_note", map[string]any{"project": "web"}, &got)
	if k := keys(got); !reflect.DeepEqual(k, []string{"content", "path"}) || got["path"] != filepath.Join(brain, "web.md") {
		t.Fatalf("read_brain_note: %v", got)
	}
	if c := got["content"].(string); !strings.HasPrefix(c, "mine\n") || !strings.Contains(c, "<!-- proz:start -->") {
		t.Fatalf("content: %q", c)
	}
}
