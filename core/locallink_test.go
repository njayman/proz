package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// real is path with symlinks resolved, as proz stores a local link.
func real(t *testing.T, path string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestALocalLinkIsNamedFromWhatIsOnDisk(t *testing.T) {
	s := open(t, t.TempDir())
	s.AddIdea("web")
	web := projectNamed(t, s, "web")
	root := t.TempDir()
	vault, graph, plain := filepath.Join(root, "vault"), filepath.Join(root, "graph"), filepath.Join(root, "notes")
	mkdirs(t, filepath.Join(vault, ".obsidian"), filepath.Join(vault, "projects"), filepath.Join(graph, "logseq"), filepath.Join(graph, "pages"), plain)
	touch(t, vault, filepath.Join("projects", "web.md"), t0)

	for _, c := range []struct{ ref, provider string }{
		{filepath.Join(vault, "projects", "web.md"), "obsidian"},
		{vault, "obsidian"},
		{filepath.Join(graph, "pages"), "logseq"},
		{plain + string(filepath.Separator), "folder"},
	} {
		l, err := s.AddLink(web.ID, c.ref, "", "")
		if want := (Link{Kind: LinkBrain, Provider: c.provider, Ref: real(t, c.ref)}); err != nil || l != want {
			t.Errorf("%s: got %+v, %v; want %+v", c.ref, l, err, want)
		}
	}
	if _, err := s.AddLink(web.ID, plain, "", LinkTracker); err == nil {
		t.Fatal("a local link is always a brain")
	}
	if _, err := s.AddLink(web.ID, plain, "github", ""); err == nil {
		t.Fatal("a local link's provider comes from disk")
	}
	if got := links(t, s, "web"); len(got) != 4 {
		t.Fatalf("stored: %+v", got)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(plain, alias); err != nil {
		t.Skip(err) // Windows without the symlink privilege
	}
	if l, err := s.AddLink(web.ID, alias, "", ""); err != nil || l.Ref != real(t, plain) {
		t.Fatalf("a symlink is stored as where it points: %+v, %v", l, err)
	}
	if err := s.RemoveLink(web.ID, alias); err != nil {
		t.Fatalf("unlink a folder by another name: %v", err)
	}
}

func TestEditingLinkedNotesIsActivity(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	s.AddIdea("web")
	web := projectNamed(t, s, "web")
	notes := t.TempDir()
	touch(t, notes, "plan.md", t0.Add(-day))
	s.AddLink(web.ID, notes, "", "")
	s.AddLink(web.ID, "https://github.com/acme/web", "", "")

	c.t = t0.Add(10 * day)
	touch(t, notes, filepath.Join("deep", "todo.md"), t0.Add(3*day))
	touch(t, notes, "later.md", t0.Add(20*day)) // from the future: never counts
	st, err := s.Status(web.ID)
	if err != nil || !st.LastActivity.Equal(t0.Add(3*day)) {
		t.Fatalf("last activity %v, %v", st.LastActivity, err)
	}
}

func TestProzsOwnFilesInALinkedFolderAreNeverActivity(t *testing.T) {
	notes := t.TempDir()
	c := &clock{t0.Add(-30 * day)} // registered before the store file's real modification time
	s, err := Open(filepath.Join(notes, "proz"), c.now)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.AddIdea("web")
	web := projectNamed(t, s, "web")
	for _, ref := range []string{notes, filepath.Join(notes, "proz")} {
		if _, err := s.AddLink(web.ID, ref, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	c.t = t0.Add(400 * day)
	if st, _ := s.Status(web.ID); !st.LastActivity.Equal(t0.Add(-30 * day)) {
		t.Fatalf("proz's database counted: %v", st.LastActivity)
	}
}

func TestABrainRootLinkIsNeverActivity(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	vault, graph, brain := t.TempDir(), t.TempDir(), t.TempDir()
	mkdirs(t, filepath.Join(vault, ".obsidian"), filepath.Join(graph, "logseq"))
	if err := s.SetBrainPath(brain); err != nil {
		t.Fatal(err)
	}
	c.t = t0.Add(10 * day)
	for i, root := range []string{vault, graph, brain} {
		name := []string{"a", "b", "c"}[i]
		s.AddIdea(name)
		p := projectNamed(t, s, name)
		if _, err := s.AddLink(p.ID, root, "", ""); err != nil {
			t.Fatal(err)
		}
		touch(t, root, "note.md", t0.Add(15*day))
		c.t = t0.Add(20 * day)
		if st, _ := s.Status(p.ID); !st.LastActivity.Equal(t0.Add(10 * day)) {
			t.Errorf("%s: a whole vault, graph or brain counted: %v", root, st.LastActivity)
		}
		c.t = t0.Add(10 * day)
	}
	// a folder inside a vault is the project's own notes
	sub := filepath.Join(vault, "projects", "d")
	touch(t, sub, "note.md", t0.Add(12*day))
	c.t = t0.Add(20 * day)
	s.AddIdea("d")
	d := projectNamed(t, s, "d")
	s.AddLink(d.ID, sub, "", "")
	c.t = t0.Add(30 * day)
	if st, _ := s.Status(d.ID); !st.LastActivity.Equal(t0.Add(20 * day)) {
		t.Fatalf("idea registered at day 20, notes from day 12: %v", st.LastActivity)
	}
	touch(t, sub, "note.md", t0.Add(25*day))
	if st, _ := s.Status(d.ID); !st.LastActivity.Equal(t0.Add(25 * day)) {
		t.Fatalf("a folder inside a vault counts: %v", st.LastActivity)
	}
}

func TestAMissingLocalLinkIsKeptFlaggedAndNoted(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	s.AddIdea("web")
	web := projectNamed(t, s, "web")
	notes := filepath.Join(t.TempDir(), "notes")
	mkdirs(t, notes)
	s.AddLink(web.ID, notes, "", "")
	ref := real(t, notes)
	if err := os.RemoveAll(notes); err != nil {
		t.Fatal(err)
	}

	got := links(t, s, "web")
	if len(got) != 1 || !got[0].Missing || got[0].Ref != ref {
		t.Fatalf("kept and flagged: %+v", got)
	}
	c.t = t0.Add(day)
	st, err := s.Status(web.ID)
	if err != nil || !st.Links[0].Missing || !strings.Contains(st.Note.Full(), "Link missing: "+ref) {
		t.Fatalf("status: %+v, %v", st, err)
	}
}
