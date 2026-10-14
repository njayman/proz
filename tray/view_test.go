package tray

import (
	"bytes"
	"encoding/binary"
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
