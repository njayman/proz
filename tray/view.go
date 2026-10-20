// Package tray hosts the tray icon: a dot whenever a project has gone quiet,
// and a menu to resume, snooze or drop each one. The view and icons here are
// plain functions; the tray library lives behind the "tray" build tag.
package tray

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"

	"github.com/njayman/proz/core"
)

// View is everything the tray shows.
type View struct {
	Dot     bool
	Tooltip string
	Items   []Item
	Loading bool // projects are still being read: the menu says so
}

// Item is one quiet project in the menu.
type Item struct {
	ID    int64
	Label string
}

const maxLabel = 60

// SnoozeChoice is one Snooze length in a project's menu.
type SnoozeChoice struct {
	Label string
	Days  int
}

// SnoozeChoices are the Snooze lengths, shortest first. ponytail: fixed;
// 'proz snooze <project> <days>' covers any other length.
var SnoozeChoices = []SnoozeChoice{{"Snooze until tomorrow", 1}, {"Snooze 3 days", 3}, {"Snooze 1 week", 7}, {"Snooze 2 weeks", 14}, {"Snooze 1 month", 30}}

// Render turns the current nudges into what the tray shows.
func Render(nudges []core.Status) View {
	v := View{Dot: len(nudges) > 0}
	switch len(nudges) {
	case 0:
		v.Tooltip = "proz: nothing has gone quiet"
	case 1:
		v.Tooltip = "proz: " + nudges[0].Name + " went quiet"
	default:
		v.Tooltip = fmt.Sprintf("proz: %d projects went quiet", len(nudges))
	}
	for _, n := range nudges {
		label := []rune(n.Name + ": " + n.Note.Short())
		if len(label) > maxLabel {
			label = append(label[:maxLabel-3], []rune("...")...)
		}
		v.Items = append(v.Items, Item{ID: n.ID, Label: string(label)})
	}
	return v
}

// act runs one menu action on a project, then refreshes the menu.
func act(d Deps, fn func(int64) error, id int64, show func()) {
	if err := fn(id); err != nil {
		d.Log(err)
	}
	go show()
}

// snoozeAction is what picking a Snooze length does.
func snoozeAction(d Deps, id int64, days int, show func()) func() {
	return func() { act(d, func(id int64) error { return d.Snooze(id, days) }, id, show) }
}

// snapshot is what the tray should show now. On error it still returns a
// view, one that says what went wrong, so the tray keeps its icon and menu.
func snapshot(d Deps) (View, error) {
	nudges, err := d.Nudges()
	if err != nil {
		return View{Tooltip: "proz: " + err.Error()}, err
	}
	return Render(nudges), nil
}

// Icon draws the tray icon: a ring, plus a red dot when something went
// quiet. Windows wants an ICO, which can wrap a PNG.
func Icon(dot bool, goos string) []byte {
	const size = 32
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	ring := color.NRGBA{0x8a, 0x8f, 0x98, 0xff}
	red := color.NRGBA{0xe5, 0x48, 0x4d, 0xff}
	for y := range size {
		for x := range size {
			dx, dy := float64(x)-15.5, float64(y)-15.5
			if d := dx*dx + dy*dy; d <= 13*13 && d >= 9*9 {
				img.Set(x, y, ring)
			}
			if ex, ey := float64(x)-24.5, float64(y)-7.5; dot && ex*ex+ey*ey <= 7*7 {
				img.Set(x, y, red)
			}
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	if goos != "windows" {
		return b.Bytes()
	}
	var ico bytes.Buffer
	binary.Write(&ico, binary.LittleEndian, struct {
		Reserved, Type, Count   uint16
		W, H, Colors, Reserved2 uint8
		Planes, BitCount        uint16
		Size, Offset            uint32
	}{0, 1, 1, size, size, 0, 0, 1, 32, uint32(b.Len()), 22})
	ico.Write(b.Bytes())
	return ico.Bytes()
}
