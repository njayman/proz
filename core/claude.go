package core

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// maxPromptLen caps the last prompt in the note, in runes.
const maxPromptLen = 100

type claudeState struct {
	when   time.Time // newest record of a session in the project
	prompt string    // the last thing the user asked in it, one line, truncated
}

// claudeDir is where Claude Code keeps session logs: $CLAUDE_CONFIG_DIR
// (Claude Code's own setting) or ~/.claude, then projects/.
func claudeDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "projects")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "projects")
}

// Claude Code names each log folder after the session's working directory
// with every non-alphanumeric as '-'. That is lossy, so the folder only
// narrows the search; each record's own cwd decides.
var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]`)

// readClaude finds the newest Claude Code session whose working directory is
// the project folder or inside it. ok is false when there is none.
func readClaude(project string) (claudeState, bool) {
	if project == "" {
		return claudeState{}, false
	}
	base := claudeDir()
	dirs, err := os.ReadDir(base)
	if err != nil {
		return claudeState{}, false
	}
	root := resolve(project)
	prefixes := []string{nonAlnum.ReplaceAllString(root, "-"), nonAlnum.ReplaceAllString(filepath.Clean(project), "-")}
	type logFile struct {
		path string
		mod  time.Time
	}
	var logs []logFile
	for _, d := range dirs {
		// the project's folder or a subfolder of it: the name, or the name then '-'
		match := func(p string) bool { return d.Name() == p || strings.HasPrefix(d.Name(), p+"-") }
		if !d.IsDir() || !slices.ContainsFunc(prefixes, match) {
			continue
		}
		files, _ := os.ReadDir(filepath.Join(base, d.Name()))
		for _, f := range files {
			if info, err := f.Info(); err == nil && strings.HasSuffix(f.Name(), ".jsonl") {
				logs = append(logs, logFile{filepath.Join(base, d.Name(), f.Name()), info.ModTime()})
			}
		}
	}
	// Newest first: the first log with a session in the project is the latest
	// activity. Its prompt leads, unless it only ran commands like /compact;
	// then the newest session that has a prompt supplies it.
	slices.SortFunc(logs, func(a, b logFile) int { return b.mod.Compare(a.mod) })
	var latest claudeState
	found := false
	for _, l := range logs {
		st, ok := scanSession(l.path, root)
		if !ok {
			continue
		}
		if !found {
			latest, found = st, true
		}
		if latest.prompt == "" {
			latest.prompt = st.prompt
		}
		if latest.prompt != "" {
			break
		}
	}
	return latest, found
}

func scanSession(path, root string) (st claudeState, ok bool) {
	f, err := os.Open(path)
	if err != nil {
		return st, false
	}
	defer f.Close()
	inside := map[string]bool{} // cwd -> within root, resolved once per log
	record := func(line []byte) {
		if !bytes.Contains(line, []byte(`"cwd"`)) { // skip big records without one, like API blobs
			return
		}
		var rec struct {
			Type        string
			Cwd         string
			Timestamp   string // parsed separately, so an odd one cannot drop the record
			IsMeta      bool
			IsSidechain bool
			Message     struct{ Content json.RawMessage }
		}
		if json.Unmarshal(line, &rec) != nil || rec.Cwd == "" {
			return
		}
		in, seen := inside[rec.Cwd]
		if !seen {
			in = within(resolve(rec.Cwd), root)
			inside[rec.Cwd] = in
		}
		if !in {
			return
		}
		ok = true
		if ts, err := time.Parse(time.RFC3339Nano, rec.Timestamp); err == nil && ts.After(st.when) {
			st.when = ts
		}
		if rec.Type == "user" && !rec.IsMeta && !rec.IsSidechain {
			if p := promptText(rec.Message.Content); p != "" {
				st.prompt = p
			}
		}
	}
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadBytes('\n') // ponytail: whole lines in memory; fine at Claude Code's record sizes
		record(line)
		if err != nil {
			return st, ok
		}
	}
}

// wrapper matches the blocks Claude Code records as user messages but the
// user did not type: slash commands, their output, shell mode, reminders.
var wrapper = regexp.MustCompile(`^<(command-|local-command-|system-reminder|bash-|task-notification|user-prompt-submit-hook)`)

// promptText is what the user typed, or "" for tool results and wrappers.
// A prompt that merely starts with markup, like "<div> is off", is kept.
func promptText(content json.RawMessage) string {
	var text string
	if json.Unmarshal(content, &text) != nil {
		var parts []struct{ Type, Text string }
		if json.Unmarshal(content, &parts) != nil {
			return ""
		}
		for _, p := range parts {
			if p.Type == "tool_result" {
				return ""
			}
			if p.Type == "text" && !wrapper.MatchString(strings.TrimSpace(p.Text)) {
				text += " " + p.Text
			}
		}
	}
	if wrapper.MatchString(strings.TrimSpace(text)) {
		return ""
	}
	r := []rune(strings.Join(strings.Fields(text), " "))
	if len(r) > maxPromptLen {
		r = append(r[:maxPromptLen-3], []rune("...")...)
	}
	return string(r)
}
