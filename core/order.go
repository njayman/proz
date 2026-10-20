package core

import (
	"cmp"
	"slices"
)

// RemindOrder sorts statuses the way reminders surface them: quiet first
// (ideas too, since they are what proz resurfaces), then snoozed, then ideas,
// active and archived; within a group, the longest quiet first.
func RemindOrder(sts []Status) {
	group := func(st Status) int {
		switch {
		case st.State == StateStale:
			return 0
		case st.State == StateSnoozed:
			return 1
		case st.State == StateArchived:
			return 4
		case st.Path == "":
			return 2
		}
		return 3
	}
	slices.SortStableFunc(sts, func(a, b Status) int {
		return cmp.Or(cmp.Compare(group(a), group(b)), cmp.Compare(b.Quiet, a.Quiet))
	})
}
