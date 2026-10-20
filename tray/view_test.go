package tray

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image/png"
	"reflect"
	"strings"
	"testing"

	"github.com/njayman/proz/core"
)

func nudge(id int64, name, note string) core.Status {
	return core.Status{Project: core.Project{ID: id, Name: name}, Note: core.Note{Lines: []string{note}}}
}

func TestNoNudgesNoDot(t *testing.T) {
	v := Render(nil)
	if v.Dot || v.Tooltip != "proz: nothing has gone quiet" || len(v.Items) != 0 {
		t.Fatalf("got %+v", v)
	}
}

func TestEachNudgeIsAMenuItemAndTheDotShows(t *testing.T) {
	v := Render([]core.Status{nudge(1, "web", "Quiet for 8 days"), nudge(2, "api", "Quiet for 9 days")})
	want := View{
		Dot:     true,
		Tooltip: "proz: 2 projects went quiet",
		Items:   []Item{{ID: 1, Label: "web: Quiet for 8 days"}, {ID: 2, Label: "api: Quiet for 9 days"}},
	}
	if !reflect.DeepEqual(v, want) {
		t.Fatalf("got %+v", v)
	}
	if Render([]core.Status{nudge(1, "web", "x")}).Tooltip != "proz: web went quiet" {
		t.Fatal("singular tooltip")
	}
}

func TestLongLabelsAreTruncated(t *testing.T) {
	v := Render([]core.Status{nudge(1, "web", strings.Repeat("x", 200))})
	if n := len([]rune(v.Items[0].Label)); n != maxLabel || !strings.HasSuffix(v.Items[0].Label, "...") {
		t.Fatalf("got %d runes", n)
	}
}

func TestIconsArePNGAndTheDotChangesThem(t *testing.T) {
	plain, dot := Icon(false, "linux"), Icon(true, "linux")
	for _, b := range [][]byte{plain, dot} {
		if _, err := png.Decode(bytes.NewReader(b)); err != nil {
			t.Fatalf("not a png: %v", err)
		}
	}
	if bytes.Equal(plain, dot) {
		t.Fatal("the dot must change the icon")
	}
	ico := Icon(true, "windows")
	var hdr struct{ Reserved, Type, Count uint16 }
	binary.Read(bytes.NewReader(ico), binary.LittleEndian, &hdr)
	if hdr.Type != 1 || hdr.Count != 1 || !bytes.HasSuffix(ico, dot) {
		t.Fatalf("windows needs an ICO wrapping the png: %+v", hdr)
	}
}

func TestSnapshotReadsNudgesAndReportsErrorsWithoutLosingTheIcon(t *testing.T) {
	v, err := snapshot(Deps{Nudges: func() ([]core.Status, error) { return []core.Status{nudge(1, "web", "q")}, nil }})
	if err != nil || !v.Dot || len(v.Items) != 1 {
		t.Fatalf("got %+v, %v", v, err)
	}
	boom := errors.New("database locked")
	v, err = snapshot(Deps{Nudges: func() ([]core.Status, error) { return nil, boom }})
	if !errors.Is(err, boom) || v.Dot || !strings.Contains(v.Tooltip, "database locked") {
		t.Fatalf("an error still renders a view that says so: got %+v, %v", v, err)
	}
}

func TestSnoozeOffersFiveLengthsShortestFirst(t *testing.T) {
	want := []SnoozeChoice{{"Snooze until tomorrow", 1}, {"Snooze 3 days", 3}, {"Snooze 1 week", 7}, {"Snooze 2 weeks", 14}, {"Snooze 1 month", 30}}
	if !reflect.DeepEqual(SnoozeChoices, want) {
		t.Fatalf("got %+v", SnoozeChoices)
	}
}

func TestPickingASnoozeLengthSnoozesThatManyDaysAndRefreshesTheMenu(t *testing.T) {
	snoozed := map[int64]int{}
	d := Deps{
		Snooze: func(id int64, days int) error { snoozed[id] = days; return nil },
		Nudges: func() ([]core.Status, error) {
			var ns []core.Status
			for _, n := range []core.Status{nudge(1, "web", "Quiet for 8 days"), nudge(2, "api", "Quiet for 9 days")} {
				if _, ok := snoozed[n.ID]; !ok {
					ns = append(ns, n)
				}
			}
			return ns, nil
		},
		Log: func(err error) { t.Error(err) },
	}
	views := make(chan View, 1)
	pick := snoozeAction(d, 1, SnoozeChoices[3].Days, func() { v, _ := snapshot(d); views <- v })
	pick()
	v := <-views
	if snoozed[1] != 14 || len(v.Items) != 1 || v.Items[0].ID != 2 {
		t.Fatalf("snoozed %v, menu %+v", snoozed, v.Items)
	}
}
