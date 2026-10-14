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
	systray.SetTitle("")
	var mu sync.Mutex
	done := make(chan struct{}) // closed when the menu is rebuilt, to stop old click listeners
	var refresh func()
	refresh = func() {
		mu.Lock()
		defer mu.Unlock()
		if err := d.Check(); err != nil {
			d.Log(err)
		}
		nudges, err := d.Nudges()
		if err != nil {
			d.Log(err)
			return
		}
		v := Render(nudges)
		systray.SetIcon(Icon(v.Dot, runtime.GOOS))
		systray.SetTooltip(v.Tooltip)

		close(done)
		done = make(chan struct{})
		systray.ResetMenu()
		for _, it := range v.Items {
			m := systray.AddMenuItem(it.Label, "")
			on(m.AddSubMenuItem("Resume", "Open it with its program"), done, func() { act(d, d.Resume, it.ID, refresh) })
			on(m.AddSubMenuItem("Snooze 7 days", "Remind me in a week if still quiet"), done, func() { act(d, d.Snooze, it.ID, refresh) })
			on(m.AddSubMenuItem("Drop", "Archive until there is new activity"), done, func() { act(d, d.Drop, it.ID, refresh) })
		}
		if len(v.Items) > 0 {
			systray.AddSeparator()
		}
		on(systray.AddMenuItem("Check now", ""), done, func() { go refresh() })
		on(systray.AddMenuItem("Quit proz tray", ""), done, systray.Quit)
	}
	refresh()
	go func() {
		for range time.Tick(d.Interval) {
			refresh()
		}
	}()
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

func act(d Deps, fn func(int64) error, id int64, refresh func()) {
	if err := fn(id); err != nil {
		d.Log(err)
	}
	go refresh()
}
