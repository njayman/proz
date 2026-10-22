//go:build tray

package tray

import (
	"runtime"
	"sync"
	"time"

	"fyne.io/systray"
)

// Available reports whether this binary was built with the tray.
const Available = true

// Run hosts the tray icon until the user quits. It checks on start, then
// every d.Interval, and refreshes after every action.
func Run(d Deps) error {
	systray.Run(func() { onReady(d) }, nil)
	return nil
}

func onReady(d Deps) {
	systray.SetIcon(Icon(false, runtime.GOOS))
	systray.SetTooltip("proz")
	var mu sync.Mutex           // guards rebuilding the menu, never held while checking
	done := make(chan struct{}) // closed when the menu is rebuilt, to stop old click listeners
	var show func()
	var render func(View)
	show = func() {
		mu.Lock()
		defer mu.Unlock()
		if !d.Enabled() {
			systray.Quit()
			return
		}
		v, err := snapshot(d)
		if err != nil {
			d.Log(err)
		}
		render(v)
	}
	// render replaces the menu with v; the caller holds mu.
	render = func(v View) {
		systray.SetIcon(Icon(v.Dot, runtime.GOOS))
		systray.SetTooltip(v.Tooltip)

		close(done)
		done = make(chan struct{})
		systray.ResetMenu()
		if v.OfferLauncher {
			on(systray.AddMenuItem("Open proz", "Open the launcher"), done, func() {
				if err := d.OpenLauncher(); err != nil {
					d.Log(err)
				}
			})
			systray.AddSeparator()
		}
		for _, it := range v.Items {
			m := systray.AddMenuItem(it.Label, "")
			on(m.AddSubMenuItem("Resume", "Open it with its program"), done, func() { act(d, d.Resume, it.ID, show) })
			snooze := m.AddSubMenuItem("Snooze", "Remind me later if still quiet")
			for _, c := range SnoozeChoices {
				on(snooze.AddSubMenuItem(c.Label, ""), done, snoozeAction(d, it.ID, c.Days, show))
			}
			on(m.AddSubMenuItem("Drop", "Archive until there is new activity"), done, func() { act(d, d.Drop, it.ID, show) })
		}
		if v.Loading {
			systray.AddMenuItem("Checking projects…", "").Disable()
		}
		if len(v.Items) > 0 || v.Loading {
			systray.AddSeparator()
		}
		on(systray.AddMenuItem("Check now", ""), done, func() { go check(d, show) })
		on(systray.AddMenuItem("Quit proz tray", ""), done, systray.Quit)
	}
	// the menu at once, its projects once their git status is in
	mu.Lock()
	render(View{Loading: true, Tooltip: "proz: checking projects", OfferLauncher: d.OpenLauncher != nil})
	mu.Unlock()
	go watchOff(d.Enabled, offPoll, systray.Quit)
	go func() {
		show() // projects first, then the slower check, one read at a time
		check(d, show)
		for range time.Tick(d.Interval) {
			check(d, show)
		}
	}()
}

// check notifies what just went quiet, then refreshes the menu.
func check(d Deps, show func()) {
	if err := d.Check(); err != nil {
		d.Log(err)
	}
	show()
}

// on runs fn for each click on m until done closes.
func on(m *systray.MenuItem, done chan struct{}, fn func()) {
	go func() {
		for {
			select {
			case <-m.ClickedCh:
				fn()
			case <-done:
				return
			}
		}
	}()
}
