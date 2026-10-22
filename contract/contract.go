// Package contract holds the JSON shapes agents read, from the CLI's --json
// output and from the MCP server alike, so the two cannot drift apart.
// Adding fields is fine; renaming or removing one breaks agents.
package contract

import (
	"time"

	"github.com/njayman/proz/core"
)

// Nudge is one project in nudges, archived and check output.
type Nudge struct {
	Name         string        `json:"name"`
	Path         string        `json:"path"`
	Folders      []core.Folder `json:"folders"`
	LastActivity string        `json:"last_activity"`
	QuietDays    int           `json:"quiet_days"`
	QuietSeconds int64         `json:"quiet_seconds"`
	Note         string        `json:"note"`
	ShortNote    string        `json:"short_note"`
}

func NewNudge(s core.Status) Nudge {
	return Nudge{
		Name:         s.Name,
		Path:         s.Path,
		Folders:      append([]core.Folder{}, s.Folders...),
		LastActivity: s.LastActivity.UTC().Format(time.RFC3339),
		QuietDays:    int(s.Quiet / (24 * time.Hour)),
		QuietSeconds: int64(s.Quiet / time.Second),
		Note:         s.Note.Full(),
		ShortNote:    s.Note.Short(),
	}
}

func Nudges(ss []core.Status) []Nudge {
	out := []Nudge{}
	for _, s := range ss {
		out = append(out, NewNudge(s))
	}
	return out
}

type Milestone struct {
	Text string `json:"text"`
	At   string `json:"at"`
}

// Status is one project in full, as 'proz status --json' prints it.
type Status struct {
	Nudge
	Stale        bool        `json:"stale"`
	State        core.State  `json:"state"`
	SnoozedUntil string      `json:"snoozed_until,omitempty"`
	Milestones   []Milestone `json:"milestones"`
}

func NewStatus(st core.Status, milestones []core.Milestone) Status {
	out := Status{Nudge: NewNudge(st), Stale: st.Stale, State: st.State, Milestones: []Milestone{}}
	if st.State == core.StateSnoozed {
		out.SnoozedUntil = st.SnoozedUntil.UTC().Format(time.RFC3339)
	}
	for _, m := range milestones {
		out.Milestones = append(out.Milestones, Milestone{m.Text, m.At.UTC().Format(time.RFC3339)})
	}
	return out
}

// ThemeSetting is the launcher's look, as 'proz config theme --json'
// prints it: system, light or dark.
type ThemeSetting struct {
	Theme string `json:"theme"`
}

// TerminalSetting is the terminal proz opens terminal programs in, as
// 'proz config terminal --json' prints it. Empty means detect.
type TerminalSetting struct {
	Terminal string `json:"terminal"`
}

// NeedsTerminal is whether proz opens a program in a terminal, as
// 'proz config needs-terminal --json' prints it. Set is true when it is the
// user's setting rather than proz's guess.
type NeedsTerminal struct {
	Program       string `json:"program"`
	NeedsTerminal bool   `json:"needs_terminal"`
	Set           bool   `json:"set"`
}

// BrainNote is a project's note in the brain, as 'proz brain --json' and the
// read_brain_note tool return it: the whole file, the user's text included.
type BrainNote struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}
