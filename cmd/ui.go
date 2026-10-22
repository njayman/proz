package cmd

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
)

type projectPickerModel struct {
	projects     []Project
	filtered     []Project
	cursor       int
	filter       string
	searchMode   bool
	selected     *Project
	addRequested bool
	cancelled    bool
}

func newProjectPickerModel(projects []Project) projectPickerModel {
	return projectPickerModel{
		projects: projects,
		filtered: projects,
	}
}

func (m projectPickerModel) Init() tea.Cmd { return nil }

func (m projectPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		case "q":
			if m.searchMode {
				m.filter += "q"
				m.applyFilter()
			} else {
				m.cancelled = true
				return m, tea.Quit
			}
		case "enter":
			if m.cursor == len(m.filtered) {
				m.addRequested = true
			} else if len(m.filtered) > 0 {
				m.selected = &m.filtered[m.cursor]
			}
			return m, tea.Quit
		case "esc":
			if m.searchMode {
				m.searchMode = false
				m.filter = ""
				m.applyFilter()
			} else {
				m.cancelled = true
				return m, tea.Quit
			}
		case "/":
			m.searchMode = true
			m.filter = ""
			m.applyFilter()
		case "down", "j":
			if m.cursor < len(m.filtered) {
				m.cursor++
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "backspace":
			if m.searchMode {
				if len(m.filter) > 0 {
					m.filter = m.filter[:len(m.filter)-1]
					m.applyFilter()
				} else {
					m.searchMode = false
				}
			}
		case "ctrl+u":
			if m.searchMode {
				m.filter = ""
				m.applyFilter()
			}
		default:
			if m.searchMode && len(msg.String()) == 1 {
				m.filter += msg.String()
				m.applyFilter()
			}
		}
	}
	return m, nil
}

func (m *projectPickerModel) applyFilter() {
	if m.filter == "" {
		m.filtered = m.projects
	} else {
		lower := strings.ToLower(m.filter)
		m.filtered = nil
		for _, p := range m.projects {
			if strings.Contains(strings.ToLower(p.Name), lower) {
				m.filtered = append(m.filtered, p)
			}
		}
	}
	if m.cursor > len(m.filtered) {
		m.cursor = len(m.filtered)
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m projectPickerModel) View() string {
	var b strings.Builder
	if m.searchMode {
		b.WriteString("/")
		b.WriteString(m.filter)
		b.WriteString("_")
		b.WriteString("\n\n")
	} else {
		b.WriteString("(/ search)\n\n")
	}

	if len(m.filtered) == 0 {
		b.WriteString("No matching projects\n")
	} else {
		maxVisible := 15
		start := 0
		if m.cursor > maxVisible/2 {
			start = m.cursor - maxVisible/2
		}
		if start+maxVisible > len(m.filtered) {
			start = max(0, len(m.filtered)-maxVisible)
		}
		for i := start; i < start+maxVisible && i < len(m.filtered); i++ {
			p := m.filtered[i]
			prefix := "  "
			if i == m.cursor {
				prefix = "▸ "
			}
			progs := "cd"
			if len(p.Programs) > 0 {
				progs = strings.Join(p.Programs, "+")
			}
			b.WriteString(fmt.Sprintf("%s%s (%s → %s)\n", prefix, p.Name, progs, p.Path))
		}
		if len(m.filtered) > maxVisible {
			b.WriteString(fmt.Sprintf("\nShowing %d/%d\n", maxVisible, len(m.filtered)))
		}
	}

	addPrefix := "  "
	if m.cursor == len(m.filtered) {
		addPrefix = "▸ "
	}
	b.WriteString(fmt.Sprintf("%s+ Add project (current dir)\n", addPrefix))

	if m.searchMode {
		b.WriteString("\n↑↓/jk navigate · enter select · esc/backspace exit search")
	} else {
		b.WriteString("\n↑↓/jk navigate · / search · enter select · q/esc cancel")
	}
	return b.String()
}

// runTUI runs a picker drawn on stderr. With no terminal on stdin (an agent,
// a pipe) it fails at once: on Windows bubbletea would otherwise open the
// console and wait for keys nobody presses.
func runTUI(m tea.Model) (tea.Model, error) {
	if !term.IsTerminal(os.Stdin.Fd()) {
		return nil, errors.New("no terminal to show the picker in")
	}
	return tea.NewProgram(m, tea.WithOutput(os.Stderr)).Run()
}

func runProjectPicker(projects []Project) (*Project, bool, error) {
	m := newProjectPickerModel(projects)
	final, err := runTUI(m)
	if err != nil {
		return nil, false, err
	}
	model := final.(projectPickerModel)
	if model.cancelled {
		return nil, false, nil
	}
	if model.addRequested {
		return nil, true, nil
	}
	return model.selected, false, nil
}

type binaryPickerModel struct {
	desktopApps  []desktopApp
	filteredApps []desktopApp
	allBinaries  []string
	filtered     []string
	cursor       int
	filter       string
	searchMode   bool
	showAll      bool
	toggled      []string
	selected     []string
	cancelled    bool
	recentExecs  []string
}

func newBinaryPickerModel(binaries []string) binaryPickerModel {
	apps := parseDesktopFiles()
	recent := loadRecentPrograms()
	m := binaryPickerModel{
		desktopApps:  apps,
		filteredApps: apps,
		allBinaries:  binaries,
		filtered:     binaries,
		recentExecs:  recent,
	}
	m.sortRecent()
	return m
}

func (m binaryPickerModel) Init() tea.Cmd { return nil }

func (m binaryPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		case "q":
			if m.searchMode {
				m.filter += "q"
				m.applyFilter()
			} else {
				m.cancelled = true
				return m, tea.Quit
			}
		case " ":
			if m.searchMode {
				m.filter += " "
				m.applyFilter()
				break
			}
			if m.showAll {
				if len(m.filtered) > 0 {
					m.toggle(m.filtered[m.cursor])
				}
			} else {
				if len(m.filteredApps) > 0 {
					m.toggle(m.filteredApps[m.cursor].Exec)
				}
			}
			m.sortRecent()
		case "enter":
			if len(m.toggled) > 0 {
				m.selected = m.toggled
			} else if m.showAll {
				if len(m.filtered) > 0 {
					m.selected = []string{m.filtered[m.cursor]}
				}
			} else {
				if len(m.filteredApps) > 0 {
					m.selected = []string{m.filteredApps[m.cursor].Exec}
				}
			}
			return m, tea.Quit
		case "esc":
			if m.searchMode {
				m.searchMode = false
				m.filter = ""
				m.applyFilter()
			} else {
				m.cancelled = true
				return m, tea.Quit
			}
		case "/":
			m.searchMode = true
			m.filter = ""
			m.applyFilter()
		case "tab":
			m.showAll = !m.showAll
			m.filter = ""
			m.searchMode = false
			m.cursor = 0
			m.applyFilter()
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "backspace":
			if m.searchMode {
				if len(m.filter) > 0 {
					m.filter = m.filter[:len(m.filter)-1]
					m.applyFilter()
				} else {
					m.searchMode = false
				}
			}
		case "ctrl+u":
			if m.searchMode {
				m.filter = ""
				m.applyFilter()
			}
		default:
			if m.searchMode && len(msg.String()) == 1 {
				m.filter += msg.String()
				m.applyFilter()
			}
		}
	}
	return m, nil
}

func (m *binaryPickerModel) applyFilter() {
	if m.showAll {
		source := m.allBinaries
		if m.filter == "" {
			m.filtered = source
		} else {
			lower := strings.ToLower(m.filter)
			m.filtered = nil
			for _, b := range source {
				if strings.Contains(strings.ToLower(b), lower) {
					m.filtered = append(m.filtered, b)
				}
			}
		}
		if m.cursor >= len(m.filtered) {
			m.cursor = max(0, len(m.filtered)-1)
		}
	} else {
		source := m.desktopApps
		if m.filter == "" {
			m.filteredApps = source
		} else {
			lower := strings.ToLower(m.filter)
			m.filteredApps = nil
			for _, a := range source {
				if strings.Contains(strings.ToLower(a.Name), lower) || strings.Contains(strings.ToLower(a.Exec), lower) {
					m.filteredApps = append(m.filteredApps, a)
				}
			}
		}
		if m.cursor >= len(m.filteredApps) {
			m.cursor = max(0, len(m.filteredApps)-1)
		}
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.sortRecent()
}

func (m *binaryPickerModel) isToggled(name string) bool {
	return slices.Contains(m.toggled, name)
}

func (m *binaryPickerModel) toggle(name string) {
	for i, t := range m.toggled {
		if t == name {
			m.toggled = append(m.toggled[:i], m.toggled[i+1:]...)
			return
		}
	}
	m.toggled = append(m.toggled, name)
}

func (m *binaryPickerModel) sortRecent() {
	toggledRank := make(map[string]int, len(m.toggled))
	for i, t := range m.toggled {
		toggledRank[t] = i
	}
	recent := make(map[string]int, len(m.recentExecs))
	for i, e := range m.recentExecs {
		recent[e] = i
	}
	if m.showAll {
		sort.SliceStable(m.filtered, func(i, j int) bool {
			ti, tokI := toggledRank[m.filtered[i]]
			tj, tokJ := toggledRank[m.filtered[j]]
			if tokI != tokJ {
				return tokI
			}
			if tokI {
				return ti < tj
			}
			ri, okI := recent[m.filtered[i]]
			rj, okJ := recent[m.filtered[j]]
			if okI != okJ {
				return okI
			}
			if okI {
				return ri < rj
			}
			return m.filtered[i] < m.filtered[j]
		})
	} else {
		sort.SliceStable(m.filteredApps, func(i, j int) bool {
			ti, tokI := toggledRank[m.filteredApps[i].Exec]
			tj, tokJ := toggledRank[m.filteredApps[j].Exec]
			if tokI != tokJ {
				return tokI
			}
			if tokI {
				return ti < tj
			}
			ri, okI := recent[m.filteredApps[i].Exec]
			rj, okJ := recent[m.filteredApps[j].Exec]
			if okI != okJ {
				return okI
			}
			if okI {
				return ri < rj
			}
			return strings.ToLower(m.filteredApps[i].Name) < strings.ToLower(m.filteredApps[j].Name)
		})
	}
}

func (m binaryPickerModel) View() string {
	var b strings.Builder
	if m.searchMode {
		b.WriteString("/")
		b.WriteString(m.filter)
		b.WriteString("_")
	} else {
		b.WriteString("(/ search)")
	}
	if m.showAll {
		b.WriteString(" [All]")
	} else {
		b.WriteString(" [Programs]")
	}
	b.WriteString("\n\n")

	if m.showAll {
		if len(m.filtered) == 0 {
			b.WriteString("No matching binaries\n")
		} else {
			maxVisible := 15
			start := 0
			if m.cursor > maxVisible/2 {
				start = m.cursor - maxVisible/2
			}
			if start+maxVisible > len(m.filtered) {
				start = max(0, len(m.filtered)-maxVisible)
			}
			for i := start; i < start+maxVisible && i < len(m.filtered); i++ {
				cursorMark := " "
				if i == m.cursor {
					cursorMark = "▸"
				}
				checkMark := " "
				if m.isToggled(m.filtered[i]) {
					checkMark = "✓"
				}
				b.WriteString(fmt.Sprintf("%s%s %s\n", cursorMark, checkMark, m.filtered[i]))
			}
			if len(m.filtered) > maxVisible {
				b.WriteString(fmt.Sprintf("\nShowing %d/%d\n", maxVisible, len(m.filtered)))
			}
		}
	} else {
		if len(m.filteredApps) == 0 {
			b.WriteString("No matching programs\n")
		} else {
			maxVisible := 15
			start := 0
			if m.cursor > maxVisible/2 {
				start = m.cursor - maxVisible/2
			}
			if start+maxVisible > len(m.filteredApps) {
				start = max(0, len(m.filteredApps)-maxVisible)
			}
			for i := start; i < start+maxVisible && i < len(m.filteredApps); i++ {
				cursorMark := " "
				if i == m.cursor {
					cursorMark = "▸"
				}
				a := m.filteredApps[i]
				checkMark := " "
				if m.isToggled(a.Exec) {
					checkMark = "✓"
				}
				b.WriteString(fmt.Sprintf("%s%s %s (%s)\n", cursorMark, checkMark, a.Name, a.Exec))
			}
			if len(m.filteredApps) > maxVisible {
				b.WriteString(fmt.Sprintf("\nShowing %d/%d\n", maxVisible, len(m.filteredApps)))
			}
		}
	}

	b.WriteString("\n↑↓/jk navigate")
	if m.searchMode {
		b.WriteString(" · enter select · esc/backspace exit search")
	} else {
		if m.showAll {
			b.WriteString(" · / search · tab: show programs")
		} else {
			b.WriteString(" · / search · tab: show all")
		}
		b.WriteString(" · space toggle · enter select · q/esc cancel")
	}
	return b.String()
}

func runBinaryPicker(binaries []string) ([]string, error) {
	m := newBinaryPickerModel(binaries)
	final, err := runTUI(m)
	if err != nil {
		return nil, err
	}
	model := final.(binaryPickerModel)
	if model.cancelled {
		return nil, nil
	}
	return model.selected, nil
}

// selectModel picks one of items, starting at the first.
type selectModel struct {
	title     string
	items     []string
	cursor    int
	cancelled bool
	done      bool
}

func (m selectModel) Init() tea.Cmd { return nil }

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			m.done = true
			return m, tea.Quit
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		}
	}
	return m, nil
}

func (m selectModel) View() string {
	var b strings.Builder
	b.WriteString(m.title + "\n\n")
	for i, item := range m.items {
		prefix := "  "
		if i == m.cursor {
			prefix = "▸ "
		}
		b.WriteString(fmt.Sprintf("%s%s\n", prefix, item))
	}
	b.WriteString("\n↑↓/jk navigate · enter select · q/esc cancel")
	return b.String()
}

// runSelectPicker returns the index of the item picked, or -1 if cancelled.
func runSelectPicker(title string, items []string) (int, error) {
	final, err := runTUI(selectModel{title: title, items: items})
	if err != nil {
		return -1, err
	}
	if model := final.(selectModel); model.done {
		return model.cursor, nil
	}
	return -1, nil
}

func runProgramSelectPicker(programs []string) (string, error) {
	i, err := runSelectPicker("Choose a program to launch with", programs)
	if err != nil || i < 0 {
		return "", err
	}
	return programs[i], nil
}

type editFormModel struct {
	project Project
	inputs  []textinput.Model
	labels  []string
	focused int
	done    bool
}

func newEditFormModel(project Project) editFormModel {
	labels := []string{"Name", "Programs", "Arguments"}
	inputs := make([]textinput.Model, 3)

	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].Prompt = ""
	}

	inputs[0].SetValue(project.Name)
	inputs[0].Focus()

	inputs[1].SetValue(strings.Join(project.Programs, " "))
	inputs[1].Placeholder = "empty for cd, or space-separated programs"

	inputs[2].SetValue(strings.Join(project.Arguments, " "))

	return editFormModel{
		project: project,
		inputs:  inputs,
		labels:  labels,
		focused: 0,
	}
}

func (m editFormModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m editFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			m.done = true
			m.project.Name = m.inputs[0].Value()
			progsStr := strings.TrimSpace(m.inputs[1].Value())
			if progsStr != "" {
				m.project.Programs = strings.Fields(progsStr)
			} else {
				m.project.Programs = nil
			}
			argsStr := strings.TrimSpace(m.inputs[2].Value())
			if argsStr != "" {
				m.project.Arguments = strings.Fields(argsStr)
			} else {
				m.project.Arguments = nil
			}
			return m, tea.Quit
		case "tab", "down":
			m.inputs[m.focused].Blur()
			m.focused = (m.focused + 1) % len(m.inputs)
			m.inputs[m.focused].Focus()
		case "shift+tab", "up":
			m.inputs[m.focused].Blur()
			m.focused = (m.focused - 1 + len(m.inputs)) % len(m.inputs)
			m.inputs[m.focused].Focus()
		}
	}

	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	return m, cmd
}

func (m editFormModel) View() string {
	var b strings.Builder
	b.WriteString("Edit project\n\n")
	for i, input := range m.inputs {
		prefix := "  "
		if i == m.focused {
			prefix = "▸ "
		}
		b.WriteString(fmt.Sprintf("%s%s: %s\n", prefix, m.labels[i], input.View()))
	}
	b.WriteString("\ntab/↑↓ navigate · enter save · esc cancel")
	return b.String()
}

func runEditForm(project Project) (*Project, error) {
	m := newEditFormModel(project)
	final, err := runTUI(m)
	if err != nil {
		return nil, err
	}
	model := final.(editFormModel)
	if !model.done {
		return nil, nil
	}
	return &model.project, nil
}
