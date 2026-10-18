package tray

import "time"

// offPoll is how often a running tray re-reads its switch, so 'proz config
// tray off' takes effect without waiting for the next check.
const offPoll = 30 * time.Second

// watchOff calls quit once enabled reports false, checking every poll.
func watchOff(enabled func() bool, poll time.Duration, quit func()) {
	for range time.Tick(poll) {
		if !enabled() {
			quit()
			return
		}
	}
}
