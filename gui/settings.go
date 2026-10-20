package gui

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/njayman/proz/autostart"
	"github.com/njayman/proz/core"
)

const day = 24 * time.Hour

// removeAutostart stops the tray starting at login; tests swap it.
var removeAutostart = autostart.Remove

// settings is the settings view over s. Every change applies at once and
// any problem shows at the bottom; back, or Esc, leaves it.
func settings(s *core.Store, back func()) fyne.CanvasObject {
	problem := widget.NewLabel("")
	problem.Wrapping = fyne.TextWrapWord
	problem.Importance = widget.DangerImportance
	problem.Hide()
	// applied shows err, or clears the last problem.
	applied := func(err error) {
		if err == nil {
			problem.Hide()
			return
		}
		problem.SetText(err.Error())
		problem.Show()
	}
	group := func(title string) *widget.Label {
		return widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}

	quiet := newEscEntry("7", back)
	if d, err := s.StaleThreshold(); err == nil {
		quiet.SetText(strconv.Itoa(int(d / day)))
	} else {
		applied(err)
	}
	quiet.OnChanged = func(v string) {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 36500 {
			applied(errQuietDays)
			return
		}
		applied(s.SetStaleThreshold(time.Duration(n) * day))
	}

	features := container.NewVBox()
	checks := map[core.Module]*escCheck{}
	weekly := newEscSelect(append([]string{"Off"}, weekdays...), back)
	// sync shows the modules and the weekly email as saved, without
	// applying them again: one can switch the other, or fail to switch.
	sync := func() {
		for m, c := range checks {
			v, err := s.ModuleOn(m)
			if err != nil {
				applied(err)
			}
			c.Checked = v
			c.Refresh()
		}
		weekly.Selected = "Off"
		if checks[core.ModuleEmail].Checked {
			d, err := s.DigestDay()
			if err != nil {
				applied(err)
			}
			weekly.Selected = d.String()
		}
		weekly.Refresh()
	}
	for _, f := range []struct {
		m     core.Module
		label string
	}{
		{core.ModuleNotifications, "Notifications"},
		{core.ModuleTray, "Tray icon"},
		{core.ModuleEmail, "Email digest"},
		{core.ModuleBrain, "Brain notes"},
		{core.ModulePrompts, "Last agent prompt in notes"},
	} {
		c := newEscCheck(f.label, back)
		c.OnChanged = func(v bool) {
			var err error
			if f.m == core.ModuleTray && !v { // off means off: no tray at the next login either, as proz config tray off
				err = removeAutostart()
			}
			if err == nil {
				err = s.SetModule(f.m, v)
			}
			applied(err)
			sync()
		}
		checks[f.m] = c
		features.Add(c)
	}
	weekly.OnChanged = func(v string) {
		err := s.SetModule(core.ModuleEmail, v != "Off")
		if i := slices.Index(weekdays, v); err == nil && i >= 0 {
			err = s.SetDigestDay(time.Weekday((i + 1) % 7))
		}
		applied(err)
		sync()
	}
	sync()

	host, port, user := newEscEntry("smtp.example.com", back), newEscEntry("587", back), newEscEntry("Only if it needs one", back)
	pass, from, to := newEscEntry("Password", back), newEscEntry("Sender address", back), newEscEntry("Your address", back)
	pass.Password = true
	c, ok, err := s.SMTP()
	applied(err)
	if !ok {
		c.Port = 587
	}
	host.SetText(c.Host)
	port.SetText(strconv.Itoa(c.Port))
	user.SetText(c.Username)
	pass.SetText(c.Password)
	from.SetText(c.From)
	to.SetText(c.To)
	mail := func(string) {
		p, err := strconv.Atoi(port.Text)
		if err == nil {
			err = s.SetSMTP(core.SMTP{Host: host.Text, Port: p, Username: user.Text, Password: pass.Text, From: from.Text, To: to.Text})
		}
		if err != nil {
			err = fmt.Errorf("Mail server not saved: %w", err)
		}
		applied(err)
	}
	for _, e := range []*escEntry{host, port, user, pass, from, to} {
		e.OnChanged = mail
	}

	terminal := newEscEntry("Detect", back)
	if t, err := s.Terminal(); err == nil {
		terminal.SetText(t)
	} else {
		applied(err)
	}
	terminal.OnChanged = func(t string) { applied(s.SetTerminal(t)) }

	return page("Settings", back, problem, container.NewVBox(
		group("Reminders"),
		container.New(layout.NewFormLayout(),
			widget.NewLabel("Quiet after (days)"), quiet,
			widget.NewLabel("Weekly email"), weekly,
		),
		group("Features"),
		features,
		group("Email"),
		container.New(layout.NewFormLayout(),
			widget.NewLabel("Mail server"), host,
			widget.NewLabel("Port"), port,
			widget.NewLabel("Username"), user,
			widget.NewLabel("Password"), pass,
			widget.NewLabel("From"), from,
			widget.NewLabel("To"), to,
		),
		group("Launching"),
		container.New(layout.NewFormLayout(), widget.NewLabel("Terminal"), terminal),
	))
}

// weekdays are the days the weekly email can go out, Monday first.
var weekdays = []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}

var errQuietDays = errors.New("quiet after: whole days from 1 to 36500")

// page is a view that replaces home: a back arrow and title over body,
// which scrolls rather than grow the window, over bottom, which stays.
func page(title string, back func(), bottom, body fyne.CanvasObject) fyne.CanvasObject {
	top := container.NewBorder(nil, nil, widget.NewButtonWithIcon("Back", theme.NavigateBackIcon(), back), nil, truncated(title, fyne.TextStyle{Bold: true}))
	return container.NewBorder(top, container.NewPadded(bottom), nil, nil, container.NewVScroll(container.NewPadded(body)))
}

// escEntry is a text box that hands Esc to esc, which a plain one swallows.
type escEntry struct {
	widget.Entry
	esc func()
}

func newEscEntry(hint string, esc func()) *escEntry {
	e := &escEntry{esc: esc}
	e.ExtendBaseWidget(e)
	e.PlaceHolder = hint
	return e
}

func (e *escEntry) TypedKey(k *fyne.KeyEvent) {
	if k.Name == fyne.KeyEscape {
		e.esc()
		return
	}
	e.Entry.TypedKey(k)
}

// escCheck is a check box that hands Esc to esc.
type escCheck struct {
	widget.Check
	esc func()
}

func newEscCheck(text string, esc func()) *escCheck {
	c := &escCheck{esc: esc}
	c.ExtendBaseWidget(c)
	c.Text = text
	return c
}

func (c *escCheck) TypedKey(k *fyne.KeyEvent) {
	if k.Name == fyne.KeyEscape {
		c.esc()
	}
}

// escSelect is a menu that hands Esc to esc.
type escSelect struct {
	widget.Select
	esc func()
}

func newEscSelect(options []string, esc func()) *escSelect {
	s := &escSelect{esc: esc}
	s.ExtendBaseWidget(s)
	s.Options = options
	return s
}

func (s *escSelect) TypedKey(k *fyne.KeyEvent) {
	if k.Name == fyne.KeyEscape {
		s.esc()
		return
	}
	s.Select.TypedKey(k)
}
