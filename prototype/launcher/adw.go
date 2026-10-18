// PROTOTYPE, throwaway: the winner B (two panes), kept minimal, in two looks:
// D like libadwaita, E like Material 3. Settings is a page inside the window
// with a back button, not a second window. Text truncates or wraps so long
// names, notes and paths never resize the panes.
package main

import (
	"fmt"
	"image/color"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/njayman/proz/core"
)

// ---------- themes ----------

const (
	colorHeader fyne.ThemeColorName = "pHeader"
	colorCard   fyne.ThemeColorName = "pCard"
	colorSide   fyne.ThemeColorName = "pSidebar"
	colorDim    fyne.ThemeColorName = "pDim"
	colorTonal  fyne.ThemeColorName = "pTonal" // Material secondary container
)

type palette map[fyne.ThemeColorName][2]string // light, dark as #rrggbbaa

type styledTheme struct {
	colors   palette
	radius   float32 // buttons, entries, selection
	material bool
	plain    bool // keep Fyne's sizes
}

var adwaitaTheme = styledTheme{radius: 8, colors: palette{
	theme.ColorNameBackground:          {"#fafafbff", "#222226ff"},
	colorHeader:                        {"#ebebedff", "#2e2e32ff"},
	colorSide:                          {"#ebebedff", "#2e2e32ff"},
	colorCard:                          {"#ffffffff", "#36363aff"},
	colorTonal:                         {"#ffffffff", "#36363aff"},
	theme.ColorNameForeground:          {"#000006cc", "#ffffffff"},
	colorDim:                           {"#0000068c", "#ffffff8c"},
	theme.ColorNameDisabled:            {"#0000068c", "#ffffff8c"},
	theme.ColorNamePlaceHolder:         {"#0000068c", "#ffffff8c"},
	theme.ColorNamePrimary:             {"#3584e4ff", "#3584e4ff"},
	theme.ColorNameFocus:               {"#3584e480", "#3584e480"},
	theme.ColorNameForegroundOnPrimary: {"#ffffffff", "#ffffffff"},
	theme.ColorNameButton:              {"#00000614", "#ffffff1a"},
	theme.ColorNameInputBackground:     {"#00000614", "#ffffff1a"},
	theme.ColorNameHover:               {"#0000060c", "#ffffff12"},
	theme.ColorNamePressed:             {"#00000628", "#ffffff2e"},
	theme.ColorNameSelection:           {"#3584e42e", "#3584e446"},
	theme.ColorNameSeparator:           {"#0000061a", "#ffffff1a"},
	theme.ColorNameInputBorder:         {"#0000061a", "#ffffff1a"},
	theme.ColorNameMenuBackground:      {"#ffffffff", "#36363aff"},
	theme.ColorNameOverlayBackground:   {"#ffffffff", "#36363aff"},
	theme.ColorNameShadow:              {"#00000000", "#00000000"},
}}

// Material 3 baseline (purple) tokens.
var materialTheme = styledTheme{radius: 20, material: true, colors: palette{
	theme.ColorNameBackground:          {"#fef7ffff", "#141218ff"},
	colorHeader:                        {"#fef7ffff", "#141218ff"},
	colorSide:                          {"#f3edf7ff", "#1d1b20ff"},
	colorCard:                          {"#f3edf7ff", "#211f26ff"},
	colorTonal:                         {"#e8def8ff", "#4a4458ff"},
	theme.ColorNameForeground:          {"#1d1b20ff", "#e6e0e9ff"},
	colorDim:                           {"#49454fff", "#cac4d0ff"},
	theme.ColorNameDisabled:            {"#49454fff", "#cac4d0ff"},
	theme.ColorNamePlaceHolder:         {"#49454fff", "#cac4d0ff"},
	theme.ColorNamePrimary:             {"#6750a4ff", "#d0bcffff"},
	theme.ColorNameFocus:               {"#6750a480", "#d0bcff80"},
	theme.ColorNameForegroundOnPrimary: {"#ffffffff", "#381e72ff"},
	theme.ColorNameButton:              {"#e8def8ff", "#4a4458ff"},
	theme.ColorNameInputBackground:     {"#ece6f0ff", "#2b2930ff"},
	theme.ColorNameHover:               {"#1d1b2014", "#e6e0e914"},
	theme.ColorNamePressed:             {"#1d1b201f", "#e6e0e91f"},
	theme.ColorNameSelection:           {"#e8def8ff", "#4a4458ff"},
	theme.ColorNameSeparator:           {"#cac4d0ff", "#49454fff"},
	theme.ColorNameInputBorder:         {"#79747eff", "#938f99ff"},
	theme.ColorNameMenuBackground:      {"#f3edf7ff", "#211f26ff"},
	theme.ColorNameOverlayBackground:   {"#f3edf7ff", "#211f26ff"},
	theme.ColorNameShadow:              {"#00000033", "#00000066"},
}}

// plainTheme is Fyne's own look, with only the colors our pages name.
var plainTheme = styledTheme{plain: true, colors: palette{
	colorHeader: {"#00000000", "#00000000"},
	colorSide:   {"#00000000", "#00000000"},
	colorCard:   {"#0000000d", "#ffffff0d"},
	colorTonal:  {"#0000000d", "#ffffff0d"},
	colorDim:    {"#00000099", "#ffffff99"},
}}

var activeTheme = adwaitaTheme

func hex(s string) color.Color {
	var r, g, b, a uint8
	fmt.Sscanf(s, "#%02x%02x%02x%02x", &r, &g, &b, &a)
	return color.NRGBA{r, g, b, a}
}

func (t styledTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if c, ok := t.colors[n]; ok {
		if v == theme.VariantDark {
			return hex(c[1])
		}
		return hex(c[0])
	}
	return theme.DefaultTheme().Color(n, v)
}

func (t styledTheme) Size(n fyne.ThemeSizeName) float32 {
	if t.plain {
		return theme.DefaultTheme().Size(n)
	}
	switch n {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return t.radius
	case theme.SizeNameInputBorder:
		return 0
	case theme.SizeNameText:
		return 14
	case theme.SizeNameHeadingText:
		return 24
	}
	return theme.DefaultTheme().Size(n)
}

func (styledTheme) Font(s fyne.TextStyle) fyne.Resource     { return theme.DefaultTheme().Font(s) }
func (styledTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }

func useTheme(t styledTheme) {
	activeTheme = t
	fyne.CurrentApp().Settings().SetTheme(t)
}

func themeColor(n fyne.ThemeColorName) color.Color {
	return activeTheme.Color(n, fyne.CurrentApp().Settings().ThemeVariant())
}

// ---------- small building blocks ----------

// fixed is a short label that always shows in full: titles, field names.
// Only use it for text we write, never for names, notes or paths.
func fixed(s string, size fyne.ThemeSizeName, bold bool, c fyne.ThemeColorName) *widget.Label {
	l := widget.NewLabel(s)
	l.SizeName, l.TextStyle.Bold = size, bold
	l.Importance = importance(c)
	return l
}

// line is text of any length that truncates with an ellipsis. It asks for
// almost no width, so only put it where it can stretch (a Border's center).
func line(s string, size fyne.ThemeSizeName, bold bool, c fyne.ThemeColorName) *widget.Label {
	l := fixed(s, size, bold, c)
	l.Truncation = fyne.TextTruncateEllipsis
	return l
}

func importance(c fyne.ThemeColorName) widget.Importance {
	switch c {
	case colorDim:
		return widget.LowImportance // the themes map Disabled to the dim color
	case theme.ColorNamePrimary:
		return widget.HighImportance
	}
	return widget.MediumImportance
}

func setLine(l *widget.Label, s string, c fyne.ThemeColorName) {
	l.Text, l.Importance = s, importance(c)
	l.Refresh()
}

func wrapped(s string) *widget.Label {
	l := widget.NewLabel(s)
	l.Wrapping = fyne.TextWrapWord
	return l
}

func bg(c fyne.ThemeColorName, radius float32, o fyne.CanvasObject) fyne.CanvasObject {
	r := canvas.NewRectangle(themeColor(c))
	r.CornerRadius = radius
	return container.NewStack(r, o)
}

// boxed is a group of rows on a rounded card: Adwaita's boxed list with
// separators, or a Material filled card without them.
func boxed(rows ...fyne.CanvasObject) fyne.CanvasObject {
	box := container.NewVBox()
	for i, r := range rows {
		if i > 0 && !activeTheme.material {
			box.Add(widget.NewSeparator())
		}
		box.Add(r) // labels and controls carry their own padding
	}
	radius := float32(12)
	if activeTheme.material {
		radius = 16
	}
	return bg(colorCard, radius, container.NewPadded(box))
}

// row puts a title on the left and a control on the right; the title takes
// the leftover width and truncates.
func row(title string, right fyne.CanvasObject) fyne.CanvasObject {
	t := line(title, theme.SizeNameText, false, theme.ColorNameForeground)
	if right == nil {
		return t
	}
	return container.NewBorder(nil, nil, nil, container.NewCenter(right), t)
}

func heading(s string) fyne.CanvasObject {
	return fixed(s, theme.SizeNameCaptionText, true, colorDim)
}

func fixedWidth(width float32, o fyne.CanvasObject) fyne.CanvasObject {
	return container.NewGridWrap(fyne.NewSize(width, o.MinSize().Height), o)
}

func flat(icon fyne.Resource, label string, tap func()) *widget.Button {
	b := widget.NewButtonWithIcon(label, icon, tap)
	b.Importance = widget.LowImportance
	return b
}

func gap() fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(1, 12))
	return r
}

func headerBar(left, center, right fyne.CanvasObject) fyne.CanvasObject {
	if center == nil {
		center = layout.NewSpacer()
	}
	return bg(colorHeader, 0, container.NewPadded(container.NewBorder(nil, nil, left, right, center)))
}

// avatar is a Material list item's leading circle with the project's initial.
func avatar(name string) fyne.CanvasObject {
	c := canvas.NewCircle(themeColor(colorTonal))
	initial := "?"
	if name != "" {
		initial = strings.ToUpper(string([]rune(name)[:1]))
	}
	t := canvas.NewText(initial, themeColor(theme.ColorNameForeground))
	t.TextStyle.Bold = true
	return container.NewGridWrap(fyne.NewSize(36, 36), container.NewStack(c, container.NewCenter(t)))
}

func setAvatar(o fyne.CanvasObject, name string) {
	t := o.(*fyne.Container).Objects[0].(*fyne.Container).Objects[0].(*fyne.Container).Objects[1].(*fyne.Container).Objects[0].(*canvas.Text)
	t.Text = "?"
	if name != "" {
		t.Text = strings.ToUpper(string([]rune(name)[:1]))
	}
	t.Refresh()
}

func quietFor(d time.Duration) string {
	switch days := int(d.Hours() / 24); {
	case days < 1:
		return "today"
	case days < 14:
		return fmt.Sprintf("%d days", days)
	default:
		return fmt.Sprintf("%d weeks", days/7)
	}
}

func status(p project) string {
	switch {
	case p.State == core.StateArchived:
		return "Archived"
	case p.idea():
		return "Idea"
	case p.State == core.StateStale:
		return "Quiet for " + quietFor(p.Quiet)
	case p.State == core.StateSnoozed:
		return "Snoozed"
	}
	return "Active " + quietFor(p.Quiet) + " ago"
}

// ---------- D and E: two panes, minimal ----------

var openSettings func() // set by D and E so Ctrl+, opens their page

func adwaita() (fyne.CanvasObject, fyne.Focusable) {
	useTheme(adwaitaTheme)
	return styledPanes()
}

func material() (fyne.CanvasObject, fyne.Focusable) {
	useTheme(materialTheme)
	return styledPanes()
}

func styledPanes() (fyne.CanvasObject, fyne.Focusable) {
	m := activeTheme.material
	shown := filter("")
	sel := 0
	folderChoice := map[int64]string{}
	entry := newKeyEntry("Search projects")
	if m {
		entry.ActionItem = widget.NewIcon(theme.SearchIcon())
	}
	detail := container.NewVBox()
	page := container.NewStack()

	list := widget.NewList(func() int { return len(shown) },
		func() fyne.CanvasObject {
			// two lines pulled together; each label brings its own padding
			texts := container.New(layout.NewCustomPaddedVBoxLayout(-12),
				line("", theme.SizeNameText, false, theme.ColorNameForeground), line("", theme.SizeNameCaptionText, false, colorDim))
			if m {
				return container.NewBorder(nil, nil, container.NewCenter(avatar("")), nil, texts)
			}
			return texts
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			p := shown[id]
			texts := o.(*fyne.Container)
			if m {
				// Border keeps the center object first, then the leading one
				setAvatar(texts.Objects[1], p.Name)
				texts = texts.Objects[0].(*fyne.Container)
			}
			sub := colorDim
			if p.State == core.StateStale && !p.idea() {
				sub = theme.ColorNamePrimary
			}
			setLine(texts.Objects[0].(*widget.Label), p.Name, theme.ColorNameForeground)
			setLine(texts.Objects[1].(*widget.Label), status(p), sub)
		})

	render := func() {
		detail.RemoveAll()
		if len(shown) == 0 {
			detail.Add(container.NewCenter(fixed("No projects match", theme.SizeNameSubHeadingText, false, colorDim)))
			return
		}
		p := shown[sel]
		detail.Add(line(p.Name, theme.SizeNameHeadingText, true, theme.ColorNameForeground))
		detail.Add(line(status(p), theme.SizeNameText, false, colorDim))
		detail.Add(gap())

		if p.idea() {
			detail.Add(boxed(wrapped("An idea, no folder yet. Add one to open it.")))
			detail.Add(gap())
			b := widget.NewButtonWithIcon("Add a folder", theme.FolderNewIcon(), func() { setupWindow(&p) })
			b.Importance = widget.HighImportance
			detail.Add(container.NewHBox(b))
			return
		}
		if folderChoice[p.ID] == "" {
			folderChoice[p.ID] = p.folders[0]
		}

		// one primary action, the rest in a menu, so the row never overflows
		ts := targets(p)
		open := widget.NewButtonWithIcon(ts[0], theme.MediaPlayIcon(), func() { launch(p, folderChoice[p.ID], ts[0]) })
		open.Importance = widget.HighImportance
		acts := container.NewHBox(open)
		if len(ts) > 1 {
			more := widget.NewSelect(ts[1:], nil)
			more.PlaceHolder = "More ways to open"
			more.OnChanged = func(t string) {
				if t != "" {
					launch(p, folderChoice[p.ID], t)
					more.ClearSelected()
				}
			}
			acts.Add(more)
		}
		detail.Add(acts)
		detail.Add(gap())

		notes := []fyne.CanvasObject{}
		for _, l := range p.Note.Lines[1:] { // the first line repeats the status
			notes = append(notes, wrapped(l))
		}
		if len(notes) > 0 {
			detail.Add(heading("Where you left off"))
			detail.Add(boxed(notes...))
			detail.Add(gap())
		}

		var folderRow fyne.CanvasObject
		if len(p.folders) > 1 {
			// Select can't truncate, so offer short names and keep the paths behind them
			byName, names := map[string]string{}, []string{}
			for i, f := range p.folders {
				n := filepath.Base(f)
				if i == 0 {
					n += " (main)"
				}
				byName[n] = f
				names = append(names, n)
			}
			s := widget.NewSelect(names, func(n string) { folderChoice[p.ID] = byName[n] })
			for n, f := range byName {
				if f == folderChoice[p.ID] {
					s.SetSelected(n)
				}
			}
			folderRow = container.NewBorder(nil, nil, fixed("Folder", theme.SizeNameText, false, theme.ColorNameForeground), nil, s)
		} else {
			folderRow = container.NewBorder(nil, nil, fixed("Folder", theme.SizeNameText, false, theme.ColorNameForeground), nil,
				line(p.folders[0], theme.SizeNameText, false, colorDim))
		}
		detail.Add(heading("Project"))
		detail.Add(boxed(
			folderRow,
			container.NewBorder(nil, nil, fixed("Links", theme.SizeNameText, false, theme.ColorNameForeground), nil,
				line("None, agents keep a checklist", theme.SizeNameText, false, colorDim)),
			row("Edit folders, links and programs", flat(theme.NavigateNextIcon(), "", func() { setupWindow(&p) })),
		))
		detail.Add(gap())
		detail.Add(container.NewHBox(
			flat(theme.HistoryIcon(), "Snooze a week", func() { say("Would snooze %s for 7 days.", p.Name) }),
			flat(theme.DeleteIcon(), "Drop", func() { say("Would drop %s.", p.Name) }),
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
	openSetup = func(p *project) {
		title := "New project or idea"
		if p != nil {
			title = "Set up " + p.Name
		}
		page.Objects = []fyne.CanvasObject{subPage(title, goHome, column(setupForm(p, w, goHome)))}
		page.Refresh()
		w.Canvas().Unfocus()
		w.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
			if k.Name == fyne.KeyEscape {
				goHome()
			}
		})
	}
	openSettings = func() {
		page.Objects = []fyne.CanvasObject{settingsPage(goHome)}
		page.Refresh()
		w.Canvas().Unfocus()
		w.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
			if k.Name == fyne.KeyEscape {
				goHome()
			}
		})
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
			if entry.Text != "" {
				entry.SetText("")
			} else {
				say("Esc: the window would close.")
			}
		default:
			return false
		}
		return true
	}

	gear := flat(theme.SettingsIcon(), "", func() { openSettings() })
	var header fyne.CanvasObject
	if m {
		// top app bar: title left, search pill, actions right
		newBtn := widget.NewButtonWithIcon("New project", theme.ContentAddIcon(), func() { setupWindow(nil) })
		header = headerBar(fixed("proz", theme.SizeNameSubHeadingText, false, theme.ColorNameForeground),
			container.NewPadded(entry), container.NewHBox(newBtn, gear))
	} else {
		header = headerBar(flat(theme.ContentAddIcon(), "", func() { setupWindow(nil) }),
			container.NewCenter(fixedWidth(360, entry)), gear)
	}
	sidebar := bg(colorSide, 0, list)
	right := fyne.CanvasObject(container.NewVScroll(container.NewPadded(container.NewPadded(detail))))
	split := container.NewHSplit(sidebar, right)
	split.Offset = 0.3
	home = container.NewBorder(header, nil, nil, nil, split)
	page.Objects = []fyne.CanvasObject{home}
	if len(shown) > 0 {
		list.Select(0)
	}
	render()
	return page, entry
}

// settingsPage is a preferences page: grouped cards, changes apply at once,
// no Save button. Esc or the back arrow returns.
func settingsPage(back func()) fyne.CanvasObject {
	th, _ := store.StaleThreshold()
	stale := widget.NewEntry()
	stale.SetText(fmt.Sprint(int(th.Hours() / 24)))
	stale.OnChanged = func(s string) { say("Would set quiet after %s days.", s) }
	digest := widget.NewSelect([]string{"Off", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"},
		func(s string) { say("Would send the weekly email on %s.", s) })
	digest.SetSelected("Monday")

	modules := []fyne.CanvasObject{}
	labels := map[core.Module]string{
		core.ModuleNotifications: "Desktop notifications", core.ModuleTray: "Tray icon",
		core.ModuleEmail: "Email reminders", core.ModuleBrain: "Notes in a brain folder",
		core.ModulePrompts: "Show the last agent prompt",
	}
	for _, mod := range core.Modules {
		on, _ := store.ModuleOn(mod)
		c := widget.NewCheck("", func(v bool) { say("Would turn %s %s.", labels[mod], map[bool]string{true: "on", false: "off"}[v]) })
		c.SetChecked(on)
		modules = append(modules, row(labels[mod], c))
	}
	smtp := widget.NewEntry()
	smtp.SetPlaceHolder("smtp.example.com:587")
	terminal := widget.NewEntry()
	terminal.SetPlaceHolder("Found automatically")

	body := container.NewVBox(
		heading("Reminders"),
		boxed(row("Quiet after (days)", fixedWidth(80, stale)), row("Weekly email", fixedWidth(160, digest))),
		gap(),
		heading("Features"),
		boxed(modules...),
		gap(),
		heading("Email"),
		boxed(row("Mail server", fixedWidth(240, smtp))),
		gap(),
		heading("Launching"),
		boxed(row("Terminal", fixedWidth(240, terminal))),
	)
	return subPage("Settings", back, column(container.NewVScroll(container.NewPadded(body))))
}

// subPage replaces the whole window with a titled view and a back arrow.
func subPage(name string, back func(), body fyne.CanvasObject) fyne.CanvasObject {
	title := fixed(name, theme.SizeNameSubHeadingText, true, theme.ColorNameForeground)
	var header fyne.CanvasObject
	if activeTheme.material {
		header = headerBar(container.NewHBox(flat(theme.NavigateBackIcon(), "", back), title), nil, nil)
	} else {
		header = headerBar(flat(theme.NavigateBackIcon(), "", back), container.NewCenter(title), gap())
	}
	return container.NewBorder(header, nil, nil, nil, body)
}

// column centers a view in a readable width and gives it the full height,
// asking for no more room than its content's minimum.
func column(o fyne.CanvasObject) fyne.CanvasObject {
	return container.New(columnLayout{600}, o)
}

type columnLayout struct{ width float32 }

func (c columnLayout) MinSize(objs []fyne.CanvasObject) fyne.Size { return objs[0].MinSize() }

func (c columnLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	w := min(size.Width, c.width)
	objs[0].Move(fyne.NewPos((size.Width-w)/2, 0))
	objs[0].Resize(fyne.NewSize(w, size.Height))
}
