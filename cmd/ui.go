package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)


type projectPickerModel struct {
	projects   []Project
	filtered   []Project
	cursor     int
	filter     string
	searchMode bool
	selected   *Project
	cancelled  bool
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
		case "enter":
			if len(m.filtered) > 0 {
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
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
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
			exec := p.Executable
			if exec == "" {
				exec = "cd"
			}
			b.WriteString(fmt.Sprintf("%s%s (%s → %s)\n", prefix, p.Name, exec, p.Path))
		}
		if len(m.filtered) > maxVisible {
			b.WriteString(fmt.Sprintf("\nShowing %d/%d\n", maxVisible, len(m.filtered)))
		}
	}

	if m.searchMode {
		b.WriteString("\n↑↓/jk navigate · enter select · esc/backspace exit search")
	} else {
		b.WriteString("\n↑↓/jk navigate · / search · enter select · esc cancel")
	}
	return b.String()
}

func runProjectPicker(projects []Project) (*Project, error) {
	m := newProjectPickerModel(projects)
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	model := final.(projectPickerModel)
	if model.cancelled || model.selected == nil {
		return nil, nil
	}
	return model.selected, nil
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
	selected     string
	cancelled    bool
}

func newBinaryPickerModel(binaries []string) binaryPickerModel {
	apps := parseDesktopFiles()
	return binaryPickerModel{
		desktopApps:  apps,
		filteredApps: apps,
		allBinaries:  binaries,
		filtered:     binaries,
	}
}

func (m binaryPickerModel) Init() tea.Cmd { return nil }

func (m binaryPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			if m.showAll {
				if len(m.filtered) > 0 {
					m.selected = m.filtered[m.cursor]
				}
			} else {
				if len(m.filteredApps) > 0 {
					m.selected = m.filteredApps[m.cursor].Exec
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
				prefix := "  "
				if i == m.cursor {
					prefix = "▸ "
				}
				b.WriteString(fmt.Sprintf("%s%s\n", prefix, m.filtered[i]))
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
				prefix := "  "
				if i == m.cursor {
					prefix = "▸ "
				}
				a := m.filteredApps[i]
				b.WriteString(fmt.Sprintf("%s%s (%s)\n", prefix, a.Name, a.Exec))
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
		b.WriteString(" · enter select · esc cancel")
	}
	return b.String()
}

func runBinaryPicker(binaries []string) (string, error) {
	m := newBinaryPickerModel(binaries)
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	final, err := p.Run()
	if err != nil {
		return "", err
	}
	model := final.(binaryPickerModel)
	if model.cancelled {
		return "", nil
	}
	return model.selected, nil
}

type editFormModel struct {
	project Project
	inputs  []textinput.Model
	labels  []string
	focused int
	done    bool
}

func newEditFormModel(project Project) editFormModel {
	labels := []string{"Name", "Executable", "Arguments", "Tags"}
	inputs := make([]textinput.Model, 4)

	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].Prompt = ""
	}

	inputs[0].SetValue(project.Name)
	inputs[0].Focus()

	inputs[1].SetValue(project.Executable)
	inputs[1].Placeholder = "empty for cd, or type a path"

	inputs[2].SetValue(strings.Join(project.Arguments, " "))

	inputs[3].SetValue(strings.Join(project.Tags, ","))

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
			m.project.Executable = m.inputs[1].Value()
			argsStr := strings.TrimSpace(m.inputs[2].Value())
			if argsStr != "" {
				m.project.Arguments = strings.Fields(argsStr)
			} else {
				m.project.Arguments = nil
			}
			tagsStr := strings.TrimSpace(m.inputs[3].Value())
			if tagsStr != "" {
				var tags []string
				for _, t := range strings.Split(tagsStr, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						tags = append(tags, t)
					}
				}
				m.project.Tags = tags
			} else {
				m.project.Tags = nil
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
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	model := final.(editFormModel)
	if !model.done {
		return nil, nil
	}
	return &model.project, nil
}
