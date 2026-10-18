// PROTOTYPE, throwaway: answers "how should the launcher look and behave?"
// for the map ticket "Launcher layout and behavior" (#60). Three structurally
// different main windows on your real projects, plus the setup and settings
// windows they share. Nothing launches and nothing is saved: every action
// only reports what the real launcher would do, in the bar at the bottom.
//
// adw.go holds the pick, B restyled: D like Adwaita, E like Material 3.
//
// Run from this folder:  GOTOOLCHAIN=auto go run .
// Switch variants:       Ctrl+Left / Ctrl+Right, or the arrows in the bar.
// Start on a variant:    PROTO_VARIANT=B GOTOOLCHAIN=auto go run .
// Start on a page:       PROTO_PAGE=settings (or setup)
package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/njayman/proz/core"
)

// ---------- data (read only) ----------

type project struct {
	core.Status
	folders []string // main first, then worktrees found from git
}

func (p project) idea() bool { return p.Path == "" }

var (
	store    *core.Store
	projects []project
	agents   []string
)

func load() {
	dir, _ := os.UserConfigDir()
	var err error
	store, err = core.Open(filepath.Join(dir, "proz"), time.Now)
	if err != nil {
		panic(err)
	}
	sts, err := store.Statuses()
	if err != nil {
		panic(err)
	}
	for _, st := range sts {
		p := project{Status: st}
		if st.Path != "" {
			p.folders = append([]string{st.Path}, worktrees(st.Path)...)
		}
		projects = append(projects, p)
	}
	// quiet ones first, then ideas, then active, archived last
	rank := map[core.State]int{core.StateStale: 0, core.StateSnoozed: 1, core.StateActive: 3, core.StateArchived: 4}
	r := func(p project) int {
		if p.idea() && p.State != core.StateArchived {
			return 2
		}
		return rank[p.State]
	}
	sort.SliceStable(projects, func(i, j int) bool {
		if r(projects[i]) != r(projects[j]) {
			return r(projects[i]) < r(projects[j])
		}
		return projects[i].Quiet > projects[j].Quiet
	})
	for _, a := range []string{"claude", "codex", "gemini", "opencode", "aider"} {
		if _, err := exec.LookPath(a); err == nil {
			agents = append(agents, a)
		}
	}
}

func worktrees(dir string) (out []string) {
	b, err := exec.Command("git", "-C", dir, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil
	}
	for _, l := range strings.Split(string(b), "\n") {
		if wt, ok := strings.CutPrefix(l, "worktree "); ok && wt != dir {
			out = append(out, wt)
		}
	}
	return out
}

// targets are the ways to open a project in a folder: its programs, a
// terminal, then each agent found on PATH.
func targets(p project) (out []string) {
	for _, prog := range p.Programs {
		out = append(out, "Open in "+filepath.Base(prog))
	}
	out = append(out, "Terminal here")
	for _, a := range agents {
		out = append(out, "Start "+a+" here")
	}
	return out
}

func filter(q string) (out []project) {
	q = strings.ToLower(q)
	for _, p := range projects {
		if q == "" || strings.Contains(strings.ToLower(p.Name), q) {
			out = append(out, p)
		}
	}
	return out
}

func subtitle(p project) string {
	if p.idea() {
		return "Idea, no folder yet"
	}
	parts := []string{}
	for _, l := range p.Note.Lines {
		if strings.HasPrefix(l, "Milestone: ") || strings.HasPrefix(l, "Quiet") || strings.HasPrefix(l, "Last commit: ") {
			parts = append(parts, l)
		}
		if len(parts) == 2 {
			break
		}
	}
	if len(parts) == 0 && len(p.Note.Lines) > 0 {
		parts = p.Note.Lines[:1]
	}
	return strings.Join(parts, "  ·  ")
}

func stateWord(p project) string {
	switch {
	case p.State == core.StateArchived:
		return "archived"
	case p.idea():
		return "idea"
	case p.State == core.StateStale:
		return "gone quiet"
	}
	return string(p.State)
}

// ---------- shell: window, variant switcher, action bar ----------

var (
	a       fyne.App
	w       fyne.Window
	current int
	said    = widget.NewLabel("Nothing yet. Every action reports here instead of running.")
	vlabel  = widget.NewLabel("")
)

type variant struct {
	key, name string
	size      fyne.Size
	build     func() (fyne.CanvasObject, fyne.Focusable)
}

var variants []variant

func say(format string, args ...any) { said.SetText(fmt.Sprintf(format, args...)) }

func launch(p project, folder, target string) {
	say("Would run %q in %s for %s, then close the window.", target, folder, p.Name)
}

func show(i int) {
	current = (i + len(variants)) % len(variants)
	v := variants[current]
	vlabel.SetText(fmt.Sprintf("%s  (%s)", v.key, v.name))
	openSettings, openSetup = nil, nil
	w.Canvas().SetOnTypedKey(nil)
	content, focus := v.build()
	bar := container.NewBorder(nil, nil,
		container.NewHBox(widget.NewButton("◀", func() { show(current - 1) }), vlabel, widget.NewButton("▶", func() { show(current + 1) })),
		nil, said)
	w.SetContent(container.NewBorder(nil, container.NewVBox(widget.NewSeparator(), bar), nil, nil, content))
	if focus != nil {
		w.Canvas().Focus(focus)
	}
}

// globalShortcut handles shortcuts every variant shares, whatever has focus.
func globalShortcut(s fyne.Shortcut) bool {
	cs, ok := s.(*desktop.CustomShortcut)
	if !ok || cs.Modifier != fyne.KeyModifierControl {
		return false
	}
	switch cs.KeyName {
	case fyne.KeyLeft:
		show(current - 1)
	case fyne.KeyRight:
		show(current + 1)
	case fyne.KeyN:
		setupWindow(nil)
	case fyne.KeyComma:
		if openSettings != nil {
			openSettings()
		} else {
			settingsWindow()
		}
	default:
		return false
	}
	return true
}

// keyEntry is a search box that lets the window steer the list while typing.
type keyEntry struct {
	widget.Entry
	onKey func(*fyne.KeyEvent) bool
}

func newKeyEntry(placeholder string) *keyEntry {
	e := &keyEntry{}
	e.ExtendBaseWidget(e)
	e.SetPlaceHolder(placeholder)
	return e
}

func (e *keyEntry) TypedKey(k *fyne.KeyEvent) {
	if e.onKey != nil && e.onKey(k) {
		return
	}
	e.Entry.TypedKey(k)
}

func (e *keyEntry) TypedShortcut(s fyne.Shortcut) {
	if !globalShortcut(s) {
		e.Entry.TypedShortcut(s)
	}
}

func (e *keyEntry) AcceptsTab() bool { return true }

func main() {
	load()
	a = app.NewWithID("dev.proz.prototype.launcher")
	useTheme(adwaitaTheme)
	w = a.NewWindow("proz")
	for _, k := range []fyne.KeyName{fyne.KeyLeft, fyne.KeyRight, fyne.KeyN, fyne.KeyComma} {
		sc := &desktop.CustomShortcut{KeyName: k, Modifier: fyne.KeyModifierControl}
		w.Canvas().AddShortcut(sc, func(s fyne.Shortcut) { globalShortcut(s) })
	}
	variants = []variant{
		{"A", "Spotlight: one box, keyboard first", fyne.NewSize(600, 460), spotlight},
		{"B", "Two panes: list and detail", fyne.NewSize(980, 600), twoPane},
		{"C", "Grouped rows with buttons", fyne.NewSize(860, 640), grouped},
		{"D", "B, Adwaita and minimal", fyne.NewSize(1000, 640), adwaita},
		{"E", "B, Material 3 and minimal", fyne.NewSize(1000, 640), material},
	}
	start := 1 // B, the pick
	for i, v := range variants {
		if v.key == os.Getenv("PROTO_VARIANT") {
			start = i
		}
	}
	show(start)
	switch os.Getenv("PROTO_PAGE") { // start on a page, for screenshots
	case "settings":
		globalShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyComma, Modifier: fyne.KeyModifierControl})
	case "setup":
		setupWindow(nil)
	}
	w.Resize(fyne.NewSize(1000, 640)) // once: no view change resizes the window
	w.ShowAndRun()
}

// ---------- A: spotlight ----------
// rofi-like. Type to filter, Enter opens the main folder with the first
// target, Tab shows every target and folder for the selected project.

func spotlight() (fyne.CanvasObject, fyne.Focusable) {
	shown := filter("")
	sel := 0
	var actions []string // non-nil while showing one project's actions
	folder := ""
	hint := widget.NewLabel("")
	entry := newKeyEntry("Open a project…")
	const newRow = "+ New project or idea…"

	rows := func() int {
		if actions != nil {
			return len(actions)
		}
		return len(shown) + 1
	}
	var list *widget.List
	list = widget.NewList(rows,
		func() fyne.CanvasObject {
			return container.NewVBox(widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), widget.NewLabel(""))
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			box := o.(*fyne.Container)
			title, sub := box.Objects[0].(*widget.Label), box.Objects[1].(*widget.Label)
			switch {
			case actions != nil:
				title.SetText(actions[id])
				sub.SetText("")
				sub.Hide()
			case id == len(shown):
				title.SetText(newRow)
				sub.Hide()
			default:
				p := shown[id]
				title.SetText(fmt.Sprintf("%s   [%s]", p.Name, stateWord(p)))
				sub.SetText(subtitle(p))
				sub.Show()
			}
		})
	refreshHint := func() {
		if actions != nil {
			hint.SetText(fmt.Sprintf("%s in %s   ·   Enter: run   Esc: back", shown[sel].Name, folder))
		} else {
			hint.SetText("Enter: open   Tab: more ways to open   Ctrl+N: new   Ctrl+,: settings   Esc: close")
		}
	}
	move := func(d int) {
		sel = max(0, min(rows()-1, sel+d))
		list.Select(sel)
	}
	list.OnSelected = func(id widget.ListItemID) { sel = id }
	showActions := func(p project) {
		if folder == "" && len(p.folders) > 0 {
			folder = p.folders[0]
		}
		actions = targets(p)
		for _, f := range p.folders {
			if f != folder {
				actions = append(actions, "Switch folder: "+f)
			}
		}
		actions = append(actions, "Set up…")
		sel = 0
		list.Refresh()
		list.Select(0)
		refreshHint()
	}
	back := func() {
		actions, folder = nil, ""
		list.Refresh()
		refreshHint()
	}
	var projectSel int
	enter := func() {
		if actions != nil {
			p := shown[projectSel]
			act := actions[sel]
			switch {
			case strings.HasPrefix(act, "Switch folder: "):
				folder = strings.TrimPrefix(act, "Switch folder: ")
				showActions(p)
			case act == "Set up…":
				setupWindow(&p)
			default:
				launch(p, folder, act)
			}
			return
		}
		if sel == len(shown) {
			setupWindow(nil)
			return
		}
		p := shown[sel]
		if p.idea() {
			say("%s is an idea: opening setup to add a folder.", p.Name)
			setupWindow(&p)
			return
		}
		launch(p, p.folders[0], targets(p)[0])
	}
	entry.OnChanged = func(q string) {
		if actions != nil {
			back()
		}
		shown, sel = filter(q), 0
		list.Refresh()
		list.Select(0)
	}
	entry.onKey = func(k *fyne.KeyEvent) bool {
		switch k.Name {
		case fyne.KeyDown:
			move(1)
		case fyne.KeyUp:
			move(-1)
		case fyne.KeyReturn, fyne.KeyEnter:
			enter()
		case fyne.KeyTab:
			if actions == nil && sel < len(shown) && !shown[sel].idea() {
				projectSel = sel
				showActions(shown[sel])
			}
		case fyne.KeyEscape:
			switch {
			case actions != nil:
				back()
				sel = projectSel
				list.Select(sel)
			case entry.Text != "":
				entry.SetText("")
			default:
				say("Esc: the window would close.")
			}
		default:
			return false
		}
		return true
	}
	list.Select(0)
	refreshHint()
	return container.NewBorder(container.NewVBox(entry, hint), nil, nil, nil, list), entry
}

// ---------- B: two panes ----------
// List on the left, everything about the selected project on the right:
// left-off note, folder choice, links, and the ways to open it. Settings and
// setup replace the window's view; long text truncates or wraps, so nothing
// resizes the panes.

func twoPane() (fyne.CanvasObject, fyne.Focusable) {
	useTheme(plainTheme)
	shown := filter("")
	sel := 0
	entry := newKeyEntry("Filter")
	detail := container.NewVBox()
	page := container.NewStack()
	list := widget.NewList(func() int { return len(shown) },
		func() fyne.CanvasObject {
			l := widget.NewLabel("")
			l.Truncation = fyne.TextTruncateEllipsis
			return l
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			p := shown[id]
			mark := "   "
			if p.State == core.StateStale {
				mark = "● "
			}
			o.(*widget.Label).SetText(mark + p.Name)
		})
	list.HideSeparators = true
	folderChoice := map[int64]string{}
	trunc := func(s string, style fyne.TextStyle) *widget.Label {
		l := widget.NewLabelWithStyle(s, fyne.TextAlignLeading, style)
		l.Truncation = fyne.TextTruncateEllipsis
		return l
	}
	render := func() {
		detail.RemoveAll()
		if len(shown) == 0 {
			detail.Add(widget.NewLabel("No match. Ctrl+N sets up a new project."))
			return
		}
		p := shown[sel]
		detail.Add(trunc(p.Name, fyne.TextStyle{Bold: true}))
		detail.Add(widget.NewLabel(strings.ToUpper(stateWord(p))))
		detail.Add(widget.NewSeparator())
		detail.Add(widget.NewLabelWithStyle("Where you left off", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}))
		for _, l := range p.Note.Lines {
			detail.Add(wrapped(l))
		}
		detail.Add(widget.NewSeparator())
		if p.idea() {
			detail.Add(widget.NewLabel("An idea: add a folder to open it."))
			detail.Add(container.NewHBox(widget.NewButton("Set up…", func() { setupWindow(&p) })))
			return
		}
		if folderChoice[p.ID] == "" {
			folderChoice[p.ID] = p.folders[0]
		}
		if len(p.folders) > 1 {
			// short names, so long paths can't widen the pane
			byName, names := map[string]string{}, []string{}
			for i, f := range p.folders {
				n := filepath.Base(f)
				if i == 0 {
					n += " (main)"
				}
				byName[n] = f
				names = append(names, n)
			}
			detail.Add(widget.NewLabelWithStyle("Folder", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}))
			rg := widget.NewRadioGroup(names, func(n string) { folderChoice[p.ID] = byName[n] })
			for n, f := range byName {
				if f == folderChoice[p.ID] {
					rg.SetSelected(n)
				}
			}
			detail.Add(rg)
		} else {
			detail.Add(trunc("Folder: "+p.folders[0], fyne.TextStyle{}))
		}
		detail.Add(wrapped("Links: none, so agents keep a checklist in the proz brain"))
		// one main button and a menu for the rest, so the row never overflows
		ts := targets(p)
		open := widget.NewButton(ts[0], func() { launch(p, folderChoice[p.ID], ts[0]) })
		open.Importance = widget.HighImportance
		btns := container.NewHBox(open)
		if len(ts) > 1 {
			more := widget.NewSelect(ts[1:], nil)
			more.PlaceHolder = "More ways to open"
			more.OnChanged = func(t string) {
				if t != "" {
					launch(p, folderChoice[p.ID], t)
					more.ClearSelected()
				}
			}
			btns.Add(more)
		}
		detail.Add(btns)
		detail.Add(container.NewHBox(
			widget.NewButton("Set up…", func() { setupWindow(&p) }),
			widget.NewButton("Snooze a week", func() { say("Would snooze %s for 7 days.", p.Name) }),
			widget.NewButton("Drop", func() { say("Would drop %s (archived until something happens).", p.Name) }),
		))
	}
	list.OnSelected = func(id widget.ListItemID) { sel = id; render() }
	entry.OnChanged = func(q string) {
		shown, sel = filter(q), 0
		list.Refresh()
		if len(shown) > 0 {
			list.Select(0)
		}
		render()
	}

	var home fyne.CanvasObject
	goHome := func() {
		page.Objects = []fyne.CanvasObject{home}
		page.Refresh()
		w.Canvas().Focus(entry)
		w.Canvas().SetOnTypedKey(nil)
	}
	showPage := func(o fyne.CanvasObject) {
		page.Objects = []fyne.CanvasObject{o}
		page.Refresh()
		w.Canvas().Unfocus()
		w.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
			if k.Name == fyne.KeyEscape {
				goHome()
			}
		})
	}
	openSettings = func() { showPage(settingsPage(goHome)) }
	openSetup = func(p *project) {
		title := "New project or idea"
		if p != nil {
			title = "Set up " + p.Name
		}
		showPage(subPage(title, goHome, column(setupForm(p, w, goHome))))
	}

	entry.onKey = func(k *fyne.KeyEvent) bool {
		switch k.Name {
		case fyne.KeyDown:
			if sel < len(shown)-1 {
				list.Select(sel + 1)
			}
		case fyne.KeyUp:
			if sel > 0 {
				list.Select(sel - 1)
			}
		case fyne.KeyReturn, fyne.KeyEnter:
			if len(shown) > 0 {
				p := shown[sel]
				if p.idea() {
					setupWindow(&p)
				} else {
					launch(p, folderChoice[p.ID], targets(p)[0])
				}
			}
		case fyne.KeyEscape:
			say("Esc: the window would close.")
		default:
			return false
		}
		return true
	}
	hint := widget.NewLabel("↑↓ choose   Enter: open with the first button   Ctrl+N   Ctrl+,")
	hint.Truncation = fyne.TextTruncateEllipsis
	toolbar := container.NewBorder(nil, nil,
		container.NewHBox(widget.NewButton("New…", func() { setupWindow(nil) }), widget.NewButton("Settings", func() { openSettings() })),
		nil, hint)
	left := container.NewBorder(entry, nil, nil, nil, list)
	split := container.NewHSplit(left, container.NewVScroll(container.NewPadded(detail)))
	split.Offset = 0.3
	home = container.NewBorder(toolbar, nil, nil, nil, split)
	page.Objects = []fyne.CanvasObject{home}
	if len(shown) > 0 {
		list.Select(0)
	}
	render()
	return page, entry
}

// ---------- C: grouped rows ----------
// Mouse first, for people who never use a keyboard shortcut: sections by
// state, each row with its own buttons. Typing filters the rows.

func grouped() (fyne.CanvasObject, fyne.Focusable) {
	entry := newKeyEntry("Search")
	body := container.NewVBox()
	build := func(q string) {
		body.RemoveAll()
		groups := []struct {
			title string
			match func(project) bool
		}{
			{"Gone quiet", func(p project) bool { return p.State == core.StateStale && !p.idea() }},
			{"Snoozed", func(p project) bool { return p.State == core.StateSnoozed && !p.idea() }},
			{"Ideas", func(p project) bool { return p.idea() && p.State != core.StateArchived }},
			{"Active", func(p project) bool { return p.State == core.StateActive && !p.idea() }},
			{"Archived", func(p project) bool { return p.State == core.StateArchived }},
		}
		acc := widget.NewAccordion()
		acc.MultiOpen = true
		for _, g := range groups {
			rows := container.NewVBox()
			n := 0
			for _, p := range filter(q) {
				if !g.match(p) {
					continue
				}
				n++
				rows.Add(groupedRow(p))
				rows.Add(widget.NewSeparator())
			}
			if n == 0 {
				continue
			}
			acc.Append(widget.NewAccordionItem(fmt.Sprintf("%s (%d)", g.title, n), rows))
			if g.title != "Archived" {
				acc.Open(len(acc.Items) - 1)
			}
		}
		body.Add(acc)
	}
	entry.OnChanged = build
	entry.onKey = func(k *fyne.KeyEvent) bool {
		if k.Name == fyne.KeyEscape {
			say("Esc: the window would close.")
			return true
		}
		return false
	}
	build("")
	top := container.NewBorder(nil, nil, nil,
		container.NewHBox(widget.NewButton("+ New project or idea", func() { setupWindow(nil) }), widget.NewButton("Settings", settingsWindow)),
		entry)
	return container.NewBorder(top, nil, nil, nil, container.NewVScroll(body)), entry
}

func groupedRow(p project) fyne.CanvasObject {
	text := container.NewVBox(
		widget.NewLabelWithStyle(p.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(subtitle(p)))
	var btns *fyne.Container
	if p.idea() {
		b := widget.NewButton("Set up", func() { setupWindow(&p) })
		b.Importance = widget.HighImportance
		btns = container.NewHBox(b)
	} else {
		ts := targets(p)
		open := widget.NewButton(ts[0], func() { launch(p, p.folders[0], ts[0]) })
		open.Importance = widget.HighImportance
		more := widget.NewSelect(append(ts[1:], "Set up…", "Snooze a week", "Drop"), nil)
		more.PlaceHolder = "More…"
		more.OnChanged = func(s string) {
			switch s {
			case "":
				return
			case "Set up…":
				setupWindow(&p)
			case "Snooze a week", "Drop":
				say("Would %s %s.", strings.ToLower(s), p.Name)
			default:
				launch(p, p.folders[0], s)
			}
			more.ClearSelected()
		}
		btns = container.NewHBox(open, more)
		if len(p.folders) > 1 {
			btns.Add(widget.NewLabel(fmt.Sprintf("+%d worktrees", len(p.folders)-1)))
		}
	}
	return container.NewBorder(nil, nil, nil, container.NewCenter(btns), text)
}

// ---------- shared: project setup window ----------

var presets = map[string][2]string{ // host -> provider, default kind
	"github.com": {"github", "tracker"}, "linear.app": {"linear", "tracker"}, "trello.com": {"trello", "tracker"},
	"app.asana.com": {"asana", "tracker"}, "app.todoist.com": {"todoist", "tracker"}, "notion.so": {"notion", "brain"},
	"www.notion.so": {"notion", "brain"},
}

var openSetup func(*project) // set by D and E so setup is a page, not a window

func setupWindow(p *project) {
	if openSetup != nil {
		openSetup(p)
		return
	}
	title := "New project or idea"
	if p != nil {
		title = "Set up " + p.Name
	}
	sw := a.NewWindow(title)
	sw.SetContent(setupForm(p, sw, sw.Close))
	sw.Resize(fyne.NewSize(620, 640))
	sw.Show()
}

// setupForm is the project setup view; parent hosts its file dialogs and
// done leaves it.
func setupForm(p *project, sw fyne.Window, done func()) fyne.CanvasObject {
	name := widget.NewEntry()
	name.SetPlaceHolder("Name")
	var folders []string
	var links []string
	if p != nil {
		name.SetText(p.Name)
		folders = append(folders, p.folders...)
	}
	folderBox := container.NewVBox()
	var redrawFolders func()
	redrawFolders = func() {
		folderBox.RemoveAll()
		if len(folders) == 0 {
			folderBox.Add(widget.NewLabel("No folders: this stays an idea."))
		}
		for i, f := range folders {
			label := f
			if i == 0 {
				label += "   (main)"
			}
			folderBox.Add(container.NewBorder(nil, nil, nil, widget.NewButton("Remove", func() {
				folders = append(folders[:i:i], folders[i+1:]...)
				redrawFolders()
			}), widget.NewLabel(label)))
		}
	}
	redrawFolders()
	addFolder := widget.NewButton("Add folder…", func() {
		dialog.ShowFolderOpen(func(u fyne.ListableURI, err error) {
			if u != nil {
				folders = append(folders, u.Path())
				redrawFolders()
			}
		}, sw)
	})

	linkBox := container.NewVBox(widget.NewLabel("No links. Agents keep a checklist in the proz brain."))
	addLink := func(desc string) {
		if len(links) == 0 {
			linkBox.RemoveAll()
		}
		links = append(links, desc)
		linkBox.Add(widget.NewLabel(desc))
	}
	paste := widget.NewEntry()
	paste.SetPlaceHolder("Paste a link to where this project's tasks or notes live")
	recognized := widget.NewLabel("")
	whatIs := widget.NewSelect([]string{"GitHub", "Linear", "Jira", "Trello", "Asana", "Todoist", "Notion", "brain.md", "Other"}, nil)
	whatIs.PlaceHolder = "What is this?"
	kind := widget.NewRadioGroup([]string{"tasks", "notes"}, nil)
	kind.Horizontal = true
	unknown := container.NewHBox(whatIs, kind)
	unknown.Hide()
	paste.OnChanged = func(s string) {
		u, err := url.Parse(strings.TrimSpace(s))
		if s == "" || err != nil || u.Host == "" {
			recognized.SetText("")
			unknown.Hide()
			return
		}
		if pr, ok := presets[u.Host]; ok {
			recognized.SetText(fmt.Sprintf("Recognized: %s, holds %s", pr[0], map[string]string{"tracker": "tasks", "brain": "notes"}[pr[1]]))
			unknown.Hide()
		} else if strings.HasSuffix(u.Host, ".atlassian.net") {
			recognized.SetText("Recognized: jira, holds tasks")
			unknown.Hide()
		} else {
			recognized.SetText(u.Host + " is new to proz:")
			unknown.Show()
		}
	}
	paste.OnSubmitted = func(s string) {
		if s == "" {
			return
		}
		desc := s + "   " + recognized.Text
		if unknown.Visible() {
			desc = fmt.Sprintf("%s   (%s, %s; remembered for this host)", s, whatIs.Selected, kind.Selected)
		}
		addLink(desc)
		paste.SetText("")
	}
	chooseLocal := widget.NewButton("Choose a file or folder…", func() {
		dialog.ShowFolderOpen(func(u fyne.ListableURI, err error) {
			if u == nil {
				return
			}
			prov := "folder"
			for d := u.Path(); d != "/" && d != "."; d = filepath.Dir(d) {
				if _, err := os.Stat(filepath.Join(d, ".obsidian")); err == nil {
					prov = "obsidian"
					break
				}
				if _, err := os.Stat(filepath.Join(d, "logseq")); err == nil {
					prov = "logseq"
					break
				}
			}
			addLink(fmt.Sprintf("%s   (%s, notes)", u.Path(), prov))
		}, sw)
	})

	programs := widget.NewEntry()
	programs.SetPlaceHolder("Programs to open it in, e.g. nvim, code (optional)")
	if p != nil {
		programs.SetText(strings.Join(p.Programs, ", "))
	}
	save := widget.NewButton("Save", func() {
		say("Would save %q with %d folders, %d links, programs %q.", name.Text, len(folders), len(links), programs.Text)
		done()
	})
	save.Importance = widget.HighImportance
	form := container.NewVBox(
		name,
		widget.NewLabelWithStyle("Folders", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), folderBox, addFolder,
		widget.NewLabelWithStyle("Links", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), linkBox,
		paste, recognized, unknown, chooseLocal,
		widget.NewLabelWithStyle("Open with", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), programs,
		container.NewHBox(save, widget.NewButton("Cancel", done)),
	)
	return container.NewVScroll(container.NewPadded(form))
}

// ---------- shared: settings window ----------

func settingsWindow() {
	sw := a.NewWindow("proz settings")
	items := []*widget.FormItem{}
	for _, m := range core.Modules {
		on, _ := store.ModuleOn(m)
		c := widget.NewCheck("on", func(v bool) { say("Would turn %s %v.", m, map[bool]string{true: "on", false: "off"}[v]) })
		c.SetChecked(on)
		items = append(items, widget.NewFormItem(string(m), c))
	}
	th, _ := store.StaleThreshold()
	stale := widget.NewEntry()
	stale.SetText(fmt.Sprint(int(th.Hours() / 24)))
	items = append(items, widget.NewFormItem("Quiet after (days)", stale))
	digest := widget.NewSelect([]string{"Off", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}, nil)
	digest.SetSelected("Monday")
	items = append(items, widget.NewFormItem("Weekly email on", digest))
	smtp := widget.NewEntry()
	smtp.SetPlaceHolder("smtp.example.com:587")
	items = append(items, widget.NewFormItem("Mail server", smtp))
	terminal := widget.NewEntry()
	terminal.SetPlaceHolder("Found automatically; set to override")
	items = append(items, widget.NewFormItem("Terminal", terminal))
	form := widget.NewForm(items...)
	form.SubmitText = "Save"
	form.OnSubmit = func() { say("Would save settings (quiet after %s days).", stale.Text); sw.Close() }
	form.OnCancel = sw.Close
	sw.SetContent(container.NewPadded(form))
	sw.Resize(fyne.NewSize(480, 420))
	sw.Show()
}
