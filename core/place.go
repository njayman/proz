package core

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"time"
)

// place is somewhere a project's work happens: one of its present folders,
// or the same place in a worktree of one.
type place struct {
	path, real string // as written, and resolved to compare
	main       bool
	worktree   bool
}

// placesOf returns every present folder, each followed by its worktrees,
// and the folders that are gone.
func placesOf(ctx context.Context, folders []Folder) (places []place, gone []string) {
	add := func(p place) {
		if p.real = resolve(p.path); !slices.ContainsFunc(places, func(q place) bool { return q.real == p.real }) {
			places = append(places, p)
		}
	}
	for _, f := range folders {
		if f.Missing {
			gone = append(gone, f.Path)
			continue
		}
		add(place{path: f.Path, main: f.Main})
		for _, wt := range otherCheckouts(ctx, f.Path) {
			if !missing(wt) {
				add(place{path: wt, worktree: true})
			}
		}
	}
	return places, gone
}

// placeSignals is what one place shows of the user's work.
type placeSignals struct {
	g      gitState
	hasGit bool
	work   workState
	cl     claudeState
	hasCl  bool
	active time.Time // newest of the above
}

// readPlace reads pl's signals. Another of places nested in pl counts its
// own files, so they are not counted twice.
func readPlace(ctx context.Context, pl place, places []place, now time.Time, ignore []string) (x placeSignals) {
	skip := slices.Clone(ignore)
	for _, other := range places {
		if other.real != pl.real && within(other.real, pl.real) {
			skip = append(skip, other.real)
		}
	}
	x.g, x.hasGit = readGit(ctx, pl.path)
	x.work = readWork(ctx, pl.path, now, skip)
	x.cl, x.hasCl = readClaude(pl.path)
	x.active = x.work.when
	if x.hasGit && x.g.when.After(x.active) {
		x.active = x.g.when
	}
	if x.hasCl && x.cl.when.After(x.active) {
		x.active = x.cl.when
	}
	return x
}

// ignored is what never counts as the user's activity: proz's own folder,
// the brain, and the notes proz writes there, where they really live (a
// note may be a symlink into a folder or link that does count).
func (s *Store) ignored() ([]string, error) {
	ignore := []string{resolve(s.dir)}
	brain, err := s.BrainPath()
	if err != nil || brain == "" {
		return ignore, err
	}
	ignore = append(ignore, resolve(brain))
	rows, err := s.db.Query(`SELECT brain_note FROM projects WHERE workspace_id = ? AND brain_note IS NOT NULL`, DefaultWorkspace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		ignore = append(ignore, resolve(filepath.Join(brain, name)))
	}
	return ignore, rows.Err()
}

// Checkout is a place a project can be opened in: one of its folders, or
// the same place in a worktree of one (see GLOSSARY.md).
type Checkout struct {
	Path       string
	Main       bool
	Worktree   bool
	LastActive time.Time
}

// Checkouts returns where project p can be opened, most recently active
// first; the main folder wins a tie.
func (s *Store) Checkouts(p Project) ([]Checkout, error) {
	p, err := s.Project(p.ID)
	if err != nil {
		return nil, err
	}
	ignore, err := s.ignored()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitLimit)
	defer cancel()
	places, _ := placesOf(ctx, p.Folders)
	out := make([]Checkout, 0, len(places))
	for _, pl := range places {
		seen := readPlace(ctx, pl, places, s.now(), ignore)
		out = append(out, Checkout{Path: pl.path, Main: pl.main, Worktree: pl.worktree, LastActive: seen.active})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LastActive.After(out[j].LastActive) })
	return out, nil
}

// openIn returns the folder to open project p in: folder when it is one of
// p's folders or worktrees, its main folder when folder is empty.
func (s *Store) openIn(p Project, folder string) (string, error) {
	p, err := s.Project(p.ID)
	if err != nil || folder == "" {
		return p.Path, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitLimit)
	defer cancel()
	places, _ := placesOf(ctx, p.Folders)
	want := resolve(folder)
	for _, pl := range places {
		if pl.real == want {
			return pl.path, nil
		}
	}
	if ctx.Err() != nil {
		return "", fmt.Errorf("%s is not a folder of %s, and git was too slow to check its worktrees", folder, p.Name)
	}
	return "", fmt.Errorf("%s is not a folder or worktree of %s", folder, p.Name)
}
