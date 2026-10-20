package core

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// replaceMainFolder makes path the main folder of project id, removing the
// old main one, as editing a project's path always has; an empty path leaves
// it with no main folder. An unchanged main folder is left as it is.
func replaceMainFolder(db execer, id int64, path string, now time.Time) error {
	if _, err := db.Exec(`DELETE FROM folders WHERE project_id = ? AND workspace_id = ? AND main = 1 AND path != ?`, id, DefaultWorkspace, path); err != nil || path == "" {
		return err
	}
	if _, err := db.Exec(`UPDATE folders SET main = 1 WHERE project_id = ? AND workspace_id = ? AND path = ?`, id, DefaultWorkspace, path); err != nil {
		return err
	}
	_, err := db.Exec(`INSERT INTO folders (workspace_id, project_id, path, main, added_at)
		SELECT ?, ?, ?, 1, ? WHERE NOT EXISTS (SELECT 1 FROM folders WHERE project_id = ? AND workspace_id = ? AND main = 1)`,
		DefaultWorkspace, id, path, now.Unix(), id, DefaultWorkspace)
	return err
}

// syncFolders takes in a folder an older proz wrote to projects.path alone,
// so mixing versions on one store never hides a project's folder.
func (s *Store) syncFolders() error {
	rows, err := s.db.Query(`SELECT p.id, p.path FROM projects p WHERE p.workspace_id = ? AND p.path != `+mainPath, DefaultWorkspace)
	if err != nil {
		return err
	}
	type stale struct {
		id   int64
		path string
	}
	var found []stale
	for rows.Next() {
		var f stale
		if err := rows.Scan(&f.id, &f.path); err != nil {
			rows.Close()
			return err
		}
		found = append(found, f)
	}
	rows.Close() // release the single connection before writing
	if err := rows.Err(); err != nil {
		return err
	}
	for _, f := range found {
		if err := replaceMainFolder(s.db, f.id, f.path, s.now()); err != nil {
			return err
		}
	}
	return nil
}

// missing reports whether the folder at path is gone or a parent was
// replaced by a file. Permission denied is not missing.
func missing(path string) bool {
	_, err := os.Stat(path)
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

// AddFolder adds dir to project id. The first folder is the main one, so
// adding a folder to an idea makes it a project.
func (s *Store) AddFolder(id int64, dir string) error {
	// ponytail: checked before the write; two agents adding one folder at once could both win
	if owner, err := s.FolderOwner(dir); err == nil {
		return fmt.Errorf("%s is already a folder of %s", dir, owner.Name)
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}
	return s.changeFolders(id, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO folders (workspace_id, project_id, path, main, added_at)
			SELECT ?, ?, ?, NOT EXISTS (SELECT 1 FROM folders WHERE project_id = ? AND workspace_id = ? AND main = 1), ?`,
			DefaultWorkspace, id, filepath.Clean(dir), id, DefaultWorkspace, s.now().Unix())
		return err
	})
}

// changeFolders runs change on project id's folders in one transaction, then
// keeps projects.path in step with the main folder for an older proz.
func (s *Store) changeFolders(id int64, change func(*sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := change(tx); err != nil {
		return err
	}
	res, err := tx.Exec(`UPDATE projects AS p SET path = `+mainPath+` WHERE p.id = ? AND p.workspace_id = ?`, id, DefaultWorkspace)
	if err := foundOrErr(res, err); err != nil {
		return err
	}
	return tx.Commit()
}

// FolderOwner returns the project that has dir as one of its folders.
func (s *Store) FolderOwner(dir string) (Project, error) {
	dir = resolve(dir)
	projects, err := s.Projects()
	if err != nil {
		return Project{}, err
	}
	for _, p := range projects {
		for _, f := range p.Folders {
			if resolve(f.Path) == dir {
				return p, nil
			}
		}
	}
	return Project{}, ErrNotFound
}

// RemoveFolder removes dir from project id. Removing the main folder makes
// the next oldest the main one; removing the last makes the project an idea,
// its history kept.
func (s *Store) RemoveFolder(id int64, dir string) error {
	path, err := s.folderOf(id, dir)
	if err != nil {
		return err
	}
	return s.changeFolders(id, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM folders WHERE project_id = ? AND workspace_id = ? AND path = ?`, id, DefaultWorkspace, path); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE folders SET main = 1 WHERE id = (SELECT min(id) FROM folders WHERE project_id = ? AND workspace_id = ?)
			AND NOT EXISTS (SELECT 1 FROM folders WHERE project_id = ? AND workspace_id = ? AND main = 1)`, id, DefaultWorkspace, id, DefaultWorkspace)
		return err
	})
}

// folderOf returns the stored path of project id's folder dir.
func (s *Store) folderOf(id int64, dir string) (string, error) {
	projects, err := s.Projects()
	if err != nil {
		return "", err
	}
	for _, p := range projects {
		if p.ID != id {
			continue
		}
		for _, f := range p.Folders {
			if resolve(f.Path) == resolve(dir) {
				return f.Path, nil
			}
		}
		return "", fmt.Errorf("%s is not a folder of %s", dir, p.Name)
	}
	return "", ErrNotFound
}

// SetMainFolder makes dir, one of project id's folders, its main folder.
func (s *Store) SetMainFolder(id int64, dir string) error {
	path, err := s.folderOf(id, dir)
	if err != nil {
		return err
	}
	return s.changeFolders(id, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE folders SET main = 0 WHERE project_id = ? AND workspace_id = ?`, id, DefaultWorkspace); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE folders SET main = 1 WHERE project_id = ? AND workspace_id = ? AND path = ?`, id, DefaultWorkspace, path)
		return err
	})
}
