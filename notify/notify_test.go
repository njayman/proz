package notify

import (
	"reflect"
	"testing"

	"github.com/njayman/proz/core"
)

func nudge(name string, lines ...string) core.Status {
	return core.Status{Project: core.Project{Name: name}, Note: core.Note{Lines: lines}}
}

func TestOneMessagePerNudgeWithNameAndShortNote(t *testing.T) {
	got := Render([]core.Status{
		nudge("web", "Quiet for 8 days", "Milestone: shipped login; next: signup"),
		nudge("api", "Quiet for 9 days"),
	})
	want := []Message{
		{Title: "web went quiet", Body: "Quiet for 8 days; Milestone: shipped login; next: signup\nproz resume web"},
		{Title: "api went quiet", Body: "Quiet for 9 days\nproz resume api"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestManyNudgesCollapseIntoOneSummary(t *testing.T) {
	got := Render([]core.Status{nudge("a"), nudge("b"), nudge("c"), nudge("d")})
	want := []Message{{Title: "4 projects went quiet", Body: "a, b, c, d\nproz nudges"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestNamesWithSpacesAreQuotedInTheCommand(t *testing.T) {
	got := Render([]core.Status{nudge("habit tracker", "Quiet for 8 days")})
	if got[0].Body != "Quiet for 8 days\nproz resume 'habit tracker'" {
		t.Fatalf("got %q", got[0].Body)
	}
}

func TestCoveredMapsShownMessagesBackToNudges(t *testing.T) {
	two := []core.Status{nudge("a"), nudge("b")}
	five := []core.Status{nudge("a"), nudge("b"), nudge("c"), nudge("d"), nudge("e")}
	for _, tc := range []struct {
		due         []core.Status
		shown, want int
	}{
		{two, 1, 1}, // one message per nudge
		{two, 2, 2},
		{five, 1, 5}, // the summary covers them all
		{five, 0, 0},
	} {
		if got := Covered(tc.due, tc.shown); got != tc.want {
			t.Errorf("%d due, %d shown: got %d, want %d", len(tc.due), tc.shown, got, tc.want)
		}
	}
}

func TestMarkupIsEscapedForNotifySend(t *testing.T) {
	if got := escapeMarkup(`fix <b>bold</b> & "x"`); got != `fix &lt;b&gt;bold&lt;/b&gt; &amp; "x"` {
		t.Fatalf("got %q", got)
	}
}
