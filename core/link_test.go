package core

import (
	"errors"
	"testing"
)

func links(t *testing.T, s *Store, name string) []Link {
	t.Helper()
	return projectNamed(t, s, name).Links
}

func TestKnownHostsAreRecognizedWithTheirDefaultKind(t *testing.T) {
	s := open(t, t.TempDir())
	s.AddProject(Project{Name: "web", Path: t.TempDir()})
	web := projectNamed(t, s, "web")
	for _, c := range []struct{ ref, provider, kind string }{
		{"https://github.com/njayman/proz", "github", LinkTracker},
		{"https://linear.app/acme/project/web-1a2b", "linear", LinkTracker},
		{"https://acme.atlassian.net/jira/software/projects/WEB/boards/1", "jira", LinkTracker},
		{"https://trello.com/b/AbC123/web", "trello", LinkTracker},
		{"https://app.asana.com/0/123456/list", "asana", LinkTracker},
		{"https://app.todoist.com/app/project/web-6Jf8", "todoist", LinkTracker},
		{"https://notion.so/acme/Web-0123", "notion", LinkBrain},
		{"https://www.notion.so/acme/Web-4567", "notion", LinkBrain},
	} {
		l, err := s.AddLink(web.ID, c.ref, "", "")
		if want := (Link{Kind: c.kind, Provider: c.provider, Ref: c.ref}); err != nil || l != want {
			t.Errorf("%s: got %+v, %v; want %+v", c.ref, l, err, want)
		}
	}
	if got := links(t, s, "web"); len(got) != 8 || got[0].Provider != "github" || got[7].Ref != "https://www.notion.so/acme/Web-4567" {
		t.Fatalf("stored in the order added: %+v", got)
	}
}

func TestAProjectWithNoLinksHasAnEmptyList(t *testing.T) {
	s := open(t, t.TempDir())
	s.AddIdea("habit tracker")
	if got := links(t, s, "habit tracker"); got == nil || len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
}

func TestLinksAreStoredCanonicalAndOncePerProject(t *testing.T) {
	s := open(t, t.TempDir())
	s.AddIdea("web")
	web := projectNamed(t, s, "web")
	for _, ref := range []string{"HTTPS://GitHub.com/Njayman/Proz/", " https://github.com/Njayman/Proz "} {
		if l, err := s.AddLink(web.ID, ref, "", ""); err != nil || l.Ref != "https://github.com/Njayman/Proz" {
			t.Fatalf("%q: got %+v, %v", ref, l, err)
		}
	}
	if got := links(t, s, "web"); len(got) != 1 {
		t.Fatalf("one link pasted two ways is one link: %+v", got)
	}
}

func TestANotionLinkCanBeATracker(t *testing.T) {
	s := open(t, t.TempDir())
	s.AddIdea("web")
	web := projectNamed(t, s, "web")
	s.AddLink(web.ID, "https://www.notion.so/acme/Tasks-89ab", "", "")
	if l, err := s.AddLink(web.ID, "https://www.notion.so/acme/Tasks-89ab", "", LinkTracker); err != nil || l.Kind != LinkTracker {
		t.Fatalf("got %+v, %v", l, err)
	}
	if got := links(t, s, "web"); len(got) != 1 || got[0].Kind != LinkTracker || got[0].Provider != "notion" {
		t.Fatalf("relinking switches the kind: %+v", got)
	}
	if _, err := s.AddLink(web.ID, "https://github.com/a/b", "", "tasks"); err == nil {
		t.Fatal("a kind other than tracker or brain must fail")
	}
	if got := links(t, s, "web"); len(got) != 1 {
		t.Fatalf("a failed link is not saved: %+v", got)
	}
}

func TestOneLinkOnSeveralProjectsAndSeveralOnOne(t *testing.T) {
	s := open(t, t.TempDir())
	s.AddIdea("web")
	s.AddIdea("api")
	web, api := projectNamed(t, s, "web"), projectNamed(t, s, "api")
	repo, page := "https://github.com/acme/app", "https://notion.so/acme/App-12"
	for _, add := range []struct {
		id  int64
		ref string
	}{{web.ID, repo}, {web.ID, page}, {api.ID, repo}} {
		if _, err := s.AddLink(add.id, add.ref, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	if got := links(t, s, "web"); len(got) != 2 || got[0].Ref != repo || got[1].Ref != page {
		t.Fatalf("web: %+v", got)
	}
	if got := links(t, s, "api"); len(got) != 1 || got[0].Ref != repo {
		t.Fatalf("api: %+v", got)
	}

	if err := s.RemoveLink(web.ID, "https://GITHUB.com/acme/app/"); err != nil {
		t.Fatal(err)
	}
	if got := links(t, s, "web"); len(got) != 1 || got[0].Ref != page {
		t.Fatalf("removed from web only: %+v", got)
	}
	if got := links(t, s, "api"); len(got) != 1 {
		t.Fatalf("api keeps its link: %+v", got)
	}
	if err := s.RemoveLink(web.ID, repo); err == nil {
		t.Fatal("removing a link the project does not have must fail")
	}
	if _, err := s.AddLink(9999, repo, "", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("no such project: %v", err)
	}

	s.RemoveProject(api.ID)
	s.AddIdea("api")
	if got := links(t, s, "api"); len(got) != 0 {
		t.Fatalf("a removed project takes its links along: %+v", got)
	}
}

func TestALinkNeverKeepsCredentials(t *testing.T) {
	s := open(t, t.TempDir())
	s.AddIdea("web")
	web := projectNamed(t, s, "web")
	if l, err := s.AddLink(web.ID, "https://me:ghp_secret@github.com/acme/web", "", ""); err != nil || l.Ref != "https://github.com/acme/web" {
		t.Fatalf("got %+v, %v", l, err)
	}
	if got := links(t, s, "web"); len(got) != 1 || got[0].Ref != "https://github.com/acme/web" {
		t.Fatalf("stored %+v", got)
	}
}

func TestAnUnrecognizedHostFails(t *testing.T) {
	s := open(t, t.TempDir())
	s.AddIdea("web")
	web := projectNamed(t, s, "web")
	for _, ref := range []string{"https://jira.acme.internal/browse/WEB", "ftp://github.com/a/b", "github.com/a/b/c", ""} {
		if _, err := s.AddLink(web.ID, ref, "", ""); err == nil {
			t.Errorf("%q: want an error", ref)
		}
	}
}

func TestARemoteLinkIsNeverActivity(t *testing.T) {
	c := &clock{t0}
	s := openAt(t, c)
	s.AddIdea("web")
	web := projectNamed(t, s, "web")
	c.t = t0.Add(3 * day)
	s.AddLink(web.ID, "https://github.com/acme/web", "", "")
	st, err := s.Status(web.ID)
	if err != nil || !st.LastActivity.Equal(t0) || len(st.Links) != 1 {
		t.Fatalf("got %+v, %v", st, err)
	}
}
