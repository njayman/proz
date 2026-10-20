package core

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

// Link is a project's pointer to a tracker or brain it uses (see
// GLOSSARY.md). proz never reads through it; agents do.
type Link struct {
	Kind     string `json:"kind"`     // LinkTracker or LinkBrain
	Provider string `json:"provider"` // such as github or notion
	Ref      string `json:"ref"`      // the canonical web URL
	Missing  bool   `json:"missing"`  // a local link that is gone; a web link never is
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

// canonicalURL is ref with its scheme and host lowercased, any trailing
// slash and user name or password dropped, so one link pasted two ways is
// stored once.
func canonicalURL(ref string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(ref))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%q is not a web address (https://...), owner/repo or an issue key", ref)
	}
	u.Host = strings.ToLower(u.Host) // url.Parse lowercases the scheme
	u.User = nil                     // a pasted user:token never reaches the store
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = strings.TrimRight(u.RawPath, "/")
	return u, nil
}

// Providers are the choices for a link whose host proz does not recognize:
// the presets, brainmd, and other for anything else (stored as web).
func Providers() []string {
	return []string{"github", "linear", "jira", "trello", "asana", "todoist", "notion", "brainmd", "other"}
}

// UnknownHostError means a link's host is neither a preset nor remembered,
// so the caller has to say which tool it is.
type UnknownHostError struct{ Host string }

func (e *UnknownHostError) Error() string {
	return fmt.Sprintf("proz does not recognize %s; say which tool it is with its provider (one of %s) and its kind (%s or %s)",
		e.Host, strings.Join(Providers(), ", "), LinkTracker, LinkBrain)
}

// AddLink links project id to ref, a URL or a short form (see expand). An
// empty provider is recognized from the host: a preset, or one an earlier
// link named. A named provider wins for this link and is remembered for its
// host. An empty kind takes the provider's default; linking a ref the
// project already has updates it.
func (s *Store) AddLink(id int64, ref, provider, kind string) (Link, error) {
	u, err := s.expand(ref)
	if err != nil {
		return Link{}, err
	}
	l := Link{Ref: u.String(), Provider: provider, Kind: kind}
	if l.Provider != "" {
		if !slices.Contains(Providers(), l.Provider) && l.Provider != "web" { // web, as proz prints it, is accepted back
			return Link{}, fmt.Errorf("provider %q: want one of %s", provider, strings.Join(Providers(), ", "))
		}
		if l.Provider == "other" {
			l.Provider = "web"
		}
	} else if l.Provider = presetProvider(u.Host); l.Provider == "" {
		var known string
		err := s.db.QueryRow(`SELECT provider, kind FROM hosts WHERE workspace_id = ? AND host = ?`, DefaultWorkspace, u.Host).Scan(&l.Provider, &known)
		if errors.Is(err, sql.ErrNoRows) {
			return Link{}, &UnknownHostError{Host: u.Host}
		}
		if err != nil {
			return Link{}, err
		}
		if l.Kind == "" {
			l.Kind = known
		}
	}
	if l.Kind == "" {
		l.Kind = presetKind[l.Provider]
	}
	if l.Kind == "" {
		return Link{}, fmt.Errorf("provider %s needs a kind too: %s or %s", provider, LinkTracker, LinkBrain)
	}
	if l.Kind != LinkTracker && l.Kind != LinkBrain {
		return Link{}, fmt.Errorf("kind %q: want %s or %s", l.Kind, LinkTracker, LinkBrain)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Link{}, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO links (workspace_id, project_id, kind, provider, ref, added_at)
		SELECT workspace_id, id, ?, ?, ?, ? FROM projects WHERE id = ? AND workspace_id = ?
		ON CONFLICT (workspace_id, project_id, ref) DO UPDATE SET kind = excluded.kind, provider = excluded.provider`,
		l.Kind, l.Provider, l.Ref, s.now().Unix(), id, DefaultWorkspace)
	if err := foundOrErr(res, err); err != nil {
		return Link{}, err
	}
	if provider != "" { // presets are recognized before remembered hosts, so remembering one is harmless
		if _, err := tx.Exec(`INSERT OR REPLACE INTO hosts (workspace_id, host, provider, kind) VALUES (?, ?, ?, ?)`,
			DefaultWorkspace, u.Host, l.Provider, l.Kind); err != nil {
			return Link{}, err
		}
	}
	return l, tx.Commit()
}

var (
	repoForm = regexp.MustCompile(`^([A-Za-z0-9-]+)/([\w.-]+)(?:#(\d+))?$`) // GitHub's owner/repo or owner/repo#n
	keyForm  = regexp.MustCompile(`^[A-Z][A-Z0-9]*-\d+$`)                   // an issue key such as ENG-123
)

// expand turns ref into a canonical URL. Short forms are taken only where
// they mean one thing: owner/repo and owner/repo#n are GitHub's, and an issue
// key is Linear's or Jira's when exactly one workspace or site of them is
// known from the links already stored.
func (s *Store) expand(ref string) (*url.URL, error) {
	ref = strings.TrimSpace(ref)
	if m := repoForm.FindStringSubmatch(ref); m != nil {
		ref = "https://github.com/" + m[1] + "/" + m[2]
		if m[3] != "" {
			ref += "/issues/" + m[3]
		}
	} else if keyForm.MatchString(ref) {
		var err error
		if ref, err = s.expandKey(ref); err != nil {
			return nil, err
		}
	}
	return canonicalURL(ref)
}

func (s *Store) expandKey(key string) (string, error) {
	rows, err := s.db.Query(`SELECT provider, ref FROM links WHERE workspace_id = ? AND provider IN ('linear', 'jira')`, DefaultWorkspace)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var found []string
	for rows.Next() {
		var provider, ref string
		if err := rows.Scan(&provider, &ref); err != nil {
			return "", err
		}
		u, err := url.Parse(ref)
		if err != nil {
			continue
		}
		var full string
		if provider == "jira" {
			full = u.Scheme + "://" + u.Host + "/browse/" + key
		} else if workspace, _, _ := strings.Cut(strings.TrimPrefix(u.Path, "/"), "/"); workspace != "" {
			full = "https://linear.app/" + workspace + "/issue/" + key
		}
		if full != "" && !slices.Contains(found, full) {
			found = append(found, full)
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(found) != 1 {
		return "", fmt.Errorf("%s is an issue key, but proz cannot tell whose: paste its full URL", key)
	}
	return found[0], nil
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
	u, err := s.expand(ref)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(`DELETE FROM links WHERE project_id = ? AND workspace_id = ? AND ref = ?`, id, DefaultWorkspace, u.String())
	if err = foundOrErr(res, err); errors.Is(err, ErrNotFound) {
		return fmt.Errorf("%s is not a link of this project", u)
	}
	return err
}
