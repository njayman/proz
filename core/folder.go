package core

import (
	"errors"
	"os"
	"syscall"
	"time"
)

// Folder is one folder behind a project. Exactly one is main whenever the
// project has any; a project with none is an idea.
type Folder struct {
	Path    string `json:"path"`
	Main    bool   `json:"main"`
	Missing bool   `json:"missing"` // computed on every read, never stored
}

// mainPath selects the main folder of the project aliased p, or "" for an idea.
const mainPath = `coalesce((SELECT f.path FROM folders f WHERE f.project_id = p.id AND f.workspace_id = p.workspace_id AND f.main = 1), '')`

// folders returns every project's folders by project id, main first.
func (s *Store) folders() (map[int64][]Folder, error) {
	rows, err := s.db.Query(`SELECT project_id, path, main FROM folders WHERE workspace_id = ? ORDER BY project_id, main DESC, id`, DefaultWorkspace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]Folder{}
	for rows.Next() {
		var id int64
		var f Folder
		if err := rows.Scan(&id, &f.Path, &f.Main); err != nil {
			return nil, err
		}
		f.Missing = missing(f.Path)
		out[id] = append(out[id], f)
	}
	return out, rows.Err()
}

// setMainFolder makes path the main folder of project id, replacing the old
// main one; an empty path leaves it with no main folder. ponytail: replaces
// only the main folder; the folder commands (#78) own adding the others.
func setMainFolder(db execer, id int64, path string, now time.Time) error {
	if _, err := db.Exec(`DELETE FROM folders WHERE project_id = ? AND workspace_id = ? AND main = 1`, id, DefaultWorkspace); err != nil || path == "" {
		return err
	}
	_, err := db.Exec(`INSERT INTO folders (workspace_id, project_id, path, main, added_at) VALUES (?, ?, ?, 1, ?)`, DefaultWorkspace, id, path, now.Unix())
	return err
}

// missing reports whether the folder at path is gone or a parent was
// replaced by a file. Permission denied is not missing.
func missing(path string) bool {
	_, err := os.Stat(path)
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}
