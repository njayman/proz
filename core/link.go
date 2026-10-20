package core

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Link is a project's pointer to a tracker or brain it uses (see
// GLOSSARY.md). proz never reads through it; agents do.
type Link struct {
	Kind     string `json:"kind"`     // LinkTracker or LinkBrain
	Provider string `json:"provider"` // such as github or notion
	Ref      string `json:"ref"`      // the canonical web URL
	Missing  bool   `json:"missing"`  // computed on every read, never stored
}

const (
	LinkTracker = "tracker"
	LinkBrain   = "brain"
)

// presetKind is each recognized provider's default kind.
var presetKind = map[string]string{
	"github": LinkTracker, "linear": LinkTracker, "jira": LinkTracker, "trello": LinkTracker,
	"asana": LinkTracker, "todoist": LinkTracker, "notion": LinkBrain,
}

// presetHosts are the hosts proz recognizes; any *.atlassian.net is jira.
var presetHosts = map[string]string{
	"github.com": "github", "linear.app": "linear", "trello.com": "trello",
	"app.asana.com": "asana", "app.todoist.com": "todoist",
	"notion.so": "notion", "www.notion.so": "notion",
}

func presetProvider(host string) string {
	if strings.HasSuffix(host, ".atlassian.net") {
		return "jira"
	}
	return presetHosts[host]
}

// canonicalURL is ref with its scheme and host lowercased and any trailing
// slash dropped, so one link pasted two ways is stored once.
func canonicalURL(ref string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(ref))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%q is not a web address (https://...)", ref)
	}
	u.Host = strings.ToLower(u.Host) // url.Parse lowercases the scheme
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = strings.TrimRight(u.RawPath, "/")
	return u, nil
}

// AddLink links project id to ref. An empty kind takes the provider's
// default; linking a ref the project already has updates its kind.
func (s *Store) AddLink(id int64, ref, kind string) (Link, error) {
	u, err := canonicalURL(ref)
	if err != nil {
		return Link{}, err
	}
	l := Link{Provider: presetProvider(u.Host), Ref: u.String(), Kind: kind}
	if l.Provider == "" {
		return Link{}, fmt.Errorf("proz does not recognize %s", u.Host)
	}
	if l.Kind == "" {
		l.Kind = presetKind[l.Provider]
	}
	if l.Kind != LinkTracker && l.Kind != LinkBrain {
		return Link{}, fmt.Errorf("kind %q: want %s or %s", l.Kind, LinkTracker, LinkBrain)
	}
	res, err := s.db.Exec(`INSERT INTO links (workspace_id, project_id, kind, provider, ref, added_at)
		SELECT workspace_id, id, ?, ?, ?, ? FROM projects WHERE id = ? AND workspace_id = ?
		ON CONFLICT (workspace_id, project_id, ref) DO UPDATE SET kind = excluded.kind, provider = excluded.provider`,
		l.Kind, l.Provider, l.Ref, s.now().Unix(), id, DefaultWorkspace)
	return l, foundOrErr(res, err)
}

// links returns every project's links by project id, in the order added.
func (s *Store) links() (map[int64][]Link, error) {
	rows, err := s.db.Query(`SELECT project_id, kind, provider, ref FROM links WHERE workspace_id = ? ORDER BY id`, DefaultWorkspace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]Link{}
	for rows.Next() {
		var id int64
		var l Link
		if err := rows.Scan(&id, &l.Kind, &l.Provider, &l.Ref); err != nil {
			return nil, err
		}
		out[id] = append(out[id], l)
	}
	return out, rows.Err()
}

// RemoveLink removes ref from project id's links; other projects keep theirs.
func (s *Store) RemoveLink(id int64, ref string) error {
	u, err := canonicalURL(ref)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(`DELETE FROM links WHERE project_id = ? AND workspace_id = ? AND ref = ?`, id, DefaultWorkspace, u.String())
	if err = foundOrErr(res, err); errors.Is(err, ErrNotFound) {
		return fmt.Errorf("%s is not a link of this project", u)
	}
	return err
}
