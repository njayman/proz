package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

func fixedClock() time.Time { return t0 }

func open(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(dir, fixedClock)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func names(t *testing.T, s *Store) []string {
	t.Helper()
	ps, err := s.Projects()
	if err != nil {
		t.Fatalf("projects: %v", err)
	}
	var out []string
	for _, p := range ps {
		out = append(out, p.Name)
	}
	return out
}

func TestProjectSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	want := Project{Name: "rt", Path: "/tmp", Programs: []string{"echo"}, Arguments: []string{"hello"}}
	if err := s.AddProject(want); err != nil {
		t.Fatal(err)
	}
	s.Close()

	ps, err := open(t, dir).Projects()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 {
		t.Fatalf("want 1 project, got %+v", ps)
	}
	got := ps[0]
	got.ID = 0
	want.Folders = []Folder{{Path: "/tmp", Main: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestEmptyStoreHasNoProjects(t *testing.T) {
	if got := names(t, open(t, t.TempDir())); len(got) != 0 {
		t.Fatalf("want none, got %v", got)
	}
}

func TestDuplicateNamesAreKeptAndTargetedIndividually(t *testing.T) {
	s := open(t, t.TempDir())
	s.AddProject(Project{Name: "app", Path: "/one"})
	s.AddProject(Project{Name: "app", Path: "/two"})
	s.AddProject(Project{Name: "web", Path: "/web"})

	ps, _ := s.Projects()
	second := ps[1]
	second.Name = "app2"
	second.Programs = []string{"nvim"}
	if err := s.UpdateProject(second); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveProject(ps[0].ID); err != nil {
		t.Fatal(err)
	}

	ps, _ = s.Projects()
	if len(ps) != 2 || ps[0].Name != "app2" || ps[0].Path != "/two" || ps[0].Programs[0] != "nvim" || ps[1].Name != "web" {
		t.Fatalf("got %+v", ps)
	}
}

func TestUpdateOrRemoveMissingProjectFails(t *testing.T) {
	s := open(t, t.TempDir())
	if err := s.UpdateProject(Project{ID: 42, Name: "x"}); err == nil {
		t.Fatal("update of missing project should fail")
	}
	if err := s.RemoveProject(42); err == nil {
		t.Fatal("remove of missing project should fail")
	}
}

const legacyJSON = `[
  {"name": "legacy", "path": "/tmp", "executable": "nvim", "arguments": []},
  {"name": "web", "path": "/web", "programs": ["code", "claude"], "arguments": ["--new-window"]}
]`

func writeLegacy(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "store.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestJSONStoreImportedOnceAndLeftUntouched(t *testing.T) {
	dir := t.TempDir()
	path := writeLegacy(t, dir, legacyJSON)

	s := open(t, dir)
	ps, _ := s.Projects()
	s.Close()
	open(t, dir) // second open must not import again

	if len(ps) != 2 {
		t.Fatalf("want 2 imported, got %+v", ps)
	}
	if ps[0].Programs[0] != "nvim" {
		t.Fatalf("legacy executable not carried into programs: %+v", ps[0])
	}
	if ps[1].Name != "web" || ps[1].Programs[1] != "claude" || ps[1].Arguments[0] != "--new-window" {
		t.Fatalf("got %+v", ps[1])
	}
	if got := names(t, open(t, dir)); len(got) != 2 {
		t.Fatalf("reopen duplicated projects: %v", got)
	}
	if b, _ := os.ReadFile(path); string(b) != legacyJSON {
		t.Fatal("store.json was modified")
	}
}

func TestImportDoesNotResurrectRemovedProjects(t *testing.T) {
	dir := t.TempDir()
	writeLegacy(t, dir, legacyJSON)
	s := open(t, dir)
	ps, _ := s.Projects()
	s.RemoveProject(ps[0].ID)
	s.Close()
	if got := names(t, open(t, dir)); len(got) != 1 {
		t.Fatalf("want 1 after remove and reopen, got %v", got)
	}
}

func TestCorruptJSONStoreFailsOpenUntilFixed(t *testing.T) {
	dir := t.TempDir()
	writeLegacy(t, dir, "not json}")
	if _, err := Open(dir, fixedClock); err == nil {
		t.Fatal("want error for corrupt store.json")
	}
	writeLegacy(t, dir, legacyJSON)
	if got := names(t, open(t, dir)); len(got) != 2 {
		t.Fatalf("want import after fix, got %v", got)
	}
}

func recent(t *testing.T, s *Store) []string {
	t.Helper()
	r, err := s.RecentPrograms()
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRecentProgramsNewestFirstDedupedMaxFour(t *testing.T) {
	s := open(t, t.TempDir())
	if got := recent(t, s); len(got) != 0 {
		t.Fatalf("want none, got %v", got)
	}
	for _, p := range []string{"vim", "code", "nvim", "code", "a", "b"} {
		if err := s.PushRecentProgram(p); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"b", "a", "code", "nvim"}
	if got := recent(t, s); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestRecentJSONImportedOnce(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "recent.json"), []byte(`["nvim", "code"]`), 0644)
	s := open(t, dir)
	s.PushRecentProgram("zed")
	s.Close()
	want := []string{"zed", "nvim", "code"}
	if got := recent(t, open(t, dir)); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestCorruptRecentJSONIsSkipped(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "recent.json"), []byte("not json}"), 0644)
	if got := recent(t, open(t, dir)); len(got) != 0 {
		t.Fatalf("want none, got %v", got)
	}
}

func TestLaunchRecordsProgramAndBuildsCommand(t *testing.T) {
	s := open(t, t.TempDir())
	s.AddProject(Project{Name: "web", Path: "/web", Arguments: []string{"--new-window"}})
	ps, _ := s.Projects()
	p := ps[0]

	if cmd, err := s.Launch(p, "", true); err != nil || cmd != nil {
		t.Fatalf("no program: want nil command, got %v, %v", cmd, err)
	}
	if got := recent(t, s); len(got) != 0 {
		t.Fatalf("no program must not be recorded, got %v", got)
	}

	cmd, err := s.Launch(p, "code", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Launch(Project{ID: 42}, "", true); err == nil {
		t.Fatal("launching an unknown project should fail")
	}
	if want := []string{"code", "--new-window", "/web"}; !reflect.DeepEqual(cmd.Args, want) || cmd.Dir != "/web" {
		t.Fatalf("got args %v dir %q", cmd.Args, cmd.Dir)
	}
	if got := recent(t, s); !reflect.DeepEqual(got, []string{"code"}) {
		t.Fatalf("got %v", got)
	}
}
