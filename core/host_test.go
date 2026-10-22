package core

import (
	"errors"
	"strings"
	"testing"
)

func TestAnUnknownHostNeedsAProviderThenIsRemembered(t *testing.T) {
	s := open(t, t.TempDir())
	s.AddIdea("web")
	s.AddIdea("api")
	web, api := projectNamed(t, s, "web"), projectNamed(t, s, "api")

	_, err := s.AddLink(web.ID, "https://Jira.Acme.internal/browse/WEB", "", "")
	var unknown *UnknownHostError
	if !errors.As(err, &unknown) || unknown.Host != "jira.acme.internal" {
		t.Fatalf("want an UnknownHostError, got %v", err)
	}
	for _, choice := range append(Providers(), "tracker", "brain") {
		if !strings.Contains(err.Error(), choice) {
			t.Errorf("error %q does not name %s", err, choice)
		}
	}
	if l, err := s.AddLink(web.ID, "https://jira.acme.internal/browse/WEB", "jira", ""); err != nil || l.Provider != "jira" || l.Kind != LinkTracker {
		t.Fatalf("a preset provider brings its kind: %+v, %v", l, err)
	}
	if l, err := s.AddLink(api.ID, "https://JIRA.acme.internal/browse/API", "", ""); err != nil || l.Provider != "jira" || l.Kind != LinkTracker {
		t.Fatalf("the host is remembered for the next link: %+v, %v", l, err)
	}
	if l, err := s.AddLink(api.ID, "https://jira.acme.internal/wiki/API", "", LinkBrain); err != nil || l.Provider != "jira" || l.Kind != LinkBrain {
		t.Fatalf("a kind given wins over the remembered one: %+v, %v", l, err)
	}
}

func TestOtherIsStoredAsWebAndNeedsAKind(t *testing.T) {
	s := open(t, t.TempDir())
	s.AddIdea("web")
	web := projectNamed(t, s, "web")
	for _, provider := range []string{"other", "brainmd"} {
		if _, err := s.AddLink(web.ID, "https://tasks.example.com/web", provider, ""); err == nil || !strings.Contains(err.Error(), "kind") {
			t.Errorf("%s with no kind: want an error naming the kind, got %v", provider, err)
		}
	}
	if _, err := s.AddLink(web.ID, "https://tasks.example.com/web", "gitlab", LinkTracker); err == nil {
		t.Fatal("a provider proz does not know must fail")
	}
	l, err := s.AddLink(web.ID, "https://tasks.example.com/web", "other", LinkTracker)
	if want := (Link{Kind: LinkTracker, Provider: "web", Ref: "https://tasks.example.com/web"}); err != nil || l != want {
		t.Fatalf("got %+v, %v", l, err)
	}
	if l, err := s.AddLink(web.ID, "https://tasks.example.com/other", "", ""); err != nil || l.Provider != "web" || l.Kind != LinkTracker {
		t.Fatalf("remembered as web, still a tracker: %+v, %v", l, err)
	}
	if l, err := s.AddLink(web.ID, "https://notes.example.com/web", "brainmd", LinkBrain); err != nil || l.Provider != "brainmd" {
		t.Fatalf("brainmd: %+v, %v", l, err)
	}
}

func TestAnExplicitProviderWinsButAPresetHostKeepsItsPreset(t *testing.T) {
	s := open(t, t.TempDir())
	s.AddIdea("web")
	web := projectNamed(t, s, "web")
	if l, err := s.AddLink(web.ID, "https://github.com/acme/wiki", "other", LinkBrain); err != nil || l.Provider != "web" {
		t.Fatalf("an explicit provider wins for that link: %+v, %v", l, err)
	}
	if l, _ := s.AddLink(web.ID, "https://github.com/acme/web", "", ""); l.Provider != "github" {
		t.Fatalf("github.com stays github: %+v", l)
	}
}

func TestShortFormsExpandOnlyWhenUnambiguous(t *testing.T) {
	s := open(t, t.TempDir())
	s.AddIdea("web")
	web := projectNamed(t, s, "web")
	for ref, want := range map[string]string{
		"acme/web":    "https://github.com/acme/web",
		"acme/web#12": "https://github.com/acme/web/issues/12",
	} {
		if l, err := s.AddLink(web.ID, ref, "", ""); err != nil || l.Ref != want || l.Provider != "github" {
			t.Errorf("%s: got %+v, %v", ref, l, err)
		}
	}
	if _, err := s.AddLink(web.ID, "ENG-123", "", ""); err == nil {
		t.Fatal("an issue key with no earlier Linear or Jira link must fail")
	}
	s.AddLink(web.ID, "https://linear.app/acme/project/web-1a2b", "", "")
	if l, err := s.AddLink(web.ID, "ENG-123", "", ""); err != nil || l.Ref != "https://linear.app/acme/issue/ENG-123" || l.Provider != "linear" {
		t.Fatalf("one Linear workspace known: %+v, %v", l, err)
	}
	s.AddLink(web.ID, "https://acme.atlassian.net/browse/WEB", "", "")
	if _, err := s.AddLink(web.ID, "WEB-7", "", ""); err == nil {
		t.Fatal("Linear and Jira both known: the key is ambiguous")
	}
	if err := s.RemoveLink(web.ID, "acme/web#12"); err != nil {
		t.Fatalf("unlink takes the short form too: %v", err)
	}
}
