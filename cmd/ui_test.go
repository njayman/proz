package cmd

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func key(s string) tea.KeyMsg {
	switch s {
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "/":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func TestBinaryPickerMultiSelectVerify(t *testing.T) {
	m := newBinaryPickerModel([]string{"echo", "true", "false", "cat"})
	m.showAll = true
	m.applyFilter()

	var model tea.Model = m
	// toggle "echo" (index 0 after sort alphabetically: cat, echo, false, true -> cursor starts at 0 = cat)
	// move down to echo
	model, _ = model.Update(key("down"))
	bm := model.(binaryPickerModel)
	if bm.filtered[bm.cursor] != "echo" {
		t.Fatalf("expected cursor on echo, got %s (list=%v)", bm.filtered[bm.cursor], bm.filtered)
	}
	model, _ = model.Update(key(" "))
	bm = model.(binaryPickerModel)
	if !bm.isToggled("echo") {
		t.Fatalf("expected echo toggled, toggled=%v", bm.toggled)
	}
	if bm.filtered[0] != "echo" {
		t.Fatalf("expected echo pinned to top after toggle, got %v", bm.filtered)
	}

	// move down to true and toggle it too
	// after pin, list = echo, cat, false, true (recent-less, alpha for rest)
	for bm.filtered[bm.cursor] != "true" {
		model, _ = model.Update(key("down"))
		bm = model.(binaryPickerModel)
	}
	model, _ = model.Update(key(" "))
	bm = model.(binaryPickerModel)
	if !bm.isToggled("true") {
		t.Fatalf("expected true toggled, toggled=%v", bm.toggled)
	}
	if len(bm.toggled) != 2 || bm.toggled[0] != "echo" || bm.toggled[1] != "true" {
		t.Fatalf("expected toggled=[echo true], got %v", bm.toggled)
	}

	// toggle echo off again
	for bm.filtered[bm.cursor] != "echo" {
		model, _ = model.Update(key("up"))
		bm = model.(binaryPickerModel)
	}
	model, _ = model.Update(key(" "))
	bm = model.(binaryPickerModel)
	if bm.isToggled("echo") {
		t.Fatalf("expected echo un-toggled, toggled=%v", bm.toggled)
	}
	if len(bm.toggled) != 1 || bm.toggled[0] != "true" {
		t.Fatalf("expected toggled=[true], got %v", bm.toggled)
	}

	// confirm with enter -> selected should be the toggled set, not the cursor item
	model, cmd := model.Update(key("enter"))
	bm = model.(binaryPickerModel)
	if cmd == nil {
		t.Fatalf("expected tea.Quit cmd on enter")
	}
	if len(bm.selected) != 1 || bm.selected[0] != "true" {
		t.Fatalf("expected selected=[true], got %v", bm.selected)
	}
}

func TestBinaryPickerEnterNoToggleSelectsHighlighted(t *testing.T) {
	m := newBinaryPickerModel([]string{"echo", "true"})
	m.showAll = true
	m.applyFilter()
	var model tea.Model = m
	model, _ = model.Update(key("enter"))
	bm := model.(binaryPickerModel)
	if len(bm.selected) != 1 || bm.selected[0] != bm.filtered[0] {
		t.Fatalf("expected single highlighted selection, got %v", bm.selected)
	}
}

func TestBinaryPickerSpaceInSearchModeTypesInsteadOfToggling(t *testing.T) {
	m := newBinaryPickerModel([]string{"echo", "true"})
	m.showAll = true
	var model tea.Model = m
	model, _ = model.Update(key("/"))
	model, _ = model.Update(key(" "))
	bm := model.(binaryPickerModel)
	if bm.filter != " " {
		t.Fatalf("expected filter to contain literal space, got %q", bm.filter)
	}
	if len(bm.toggled) != 0 {
		t.Fatalf("expected no toggles from space while searching, got %v", bm.toggled)
	}
}

func TestProjectPickerAddRowVerify(t *testing.T) {
	projects := []Project{{Name: "a", Path: "/a"}, {Name: "b", Path: "/b"}}
	m := newProjectPickerModel(projects)
	var model tea.Model = m
	model, _ = model.Update(key("down"))
	model, _ = model.Update(key("down"))
	pm := model.(projectPickerModel)
	if pm.cursor != 2 {
		t.Fatalf("expected cursor at add row (2), got %d", pm.cursor)
	}
	model, cmd := model.Update(key("enter"))
	pm = model.(projectPickerModel)
	if cmd == nil || !pm.addRequested {
		t.Fatalf("expected addRequested true on enter at add row")
	}
}

func TestSelectPickerVerify(t *testing.T) {
	m := selectModel{title: "Choose a program to launch with", items: []string{"code", "nvim", "claude"}}
	var model tea.Model = m
	model, _ = model.Update(key("down"))
	model, cmd := model.Update(key("enter"))
	sm := model.(selectModel)
	if cmd == nil || !sm.done || sm.items[sm.cursor] != "nvim" {
		t.Fatalf("expected nvim picked, got %+v", sm)
	}
}
