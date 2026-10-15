package cmd

import (
	"errors"
	"fmt"

	"github.com/njayman/proz/contract"
	"github.com/njayman/proz/core"
	"github.com/njayman/proz/notify"
	"github.com/spf13/cobra"
)

// send shows notifications; tests replace it so none is ever shown for real.
var send = notify.Send

var checkJSON bool

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Run one check: notify about projects that just went quiet",
	Long: "Run one iteration of the resurfacing loop: find projects that went quiet\n" +
		"and show one OS notification per project per quiet stretch, and send the\n" +
		"weekly email digest when it is due. Safe to run often, from cron or a\n" +
		"scheduler; it never repeats a notification or a digest.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		notified, err := runCheck()
		if err != nil {
			return err
		}
		if checkJSON {
			return printJSON(contract.Nudges(notified))
		}
		for _, n := range notified {
			fmt.Printf("notified: %s\n", n.Name)
		}
		return nil
	},
}

// runCheck notifies about projects that just went quiet, if notifications
// are on, and returns those it notified; then sends the weekly digest if
// email is on and it is due. The tray calls it on its interval.
func runCheck() (notified []core.Status, err error) {
	err = withStore(func(s *core.Store) error {
		on, err := s.ModuleOn(core.ModuleNotifications)
		if err == nil && on {
			err = s.Check(func(due []core.Status) (int, error) {
				shown, err := send(notify.Render(due))
				covered := notify.Covered(due, shown)
				notified = due[:covered]
				return covered, err
			})
		}
		return errors.Join(err, digest(s)) // one channel failing never blocks the other
	})
	return notified, err
}

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Send a sample through a delivery channel to check it works",
}

var testNotificationCmd = &cobra.Command{
	Use:   "notification",
	Short: "Show one sample OS notification",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := send([]notify.Message{{
			Title: "proz notifications work",
			Body:  "You will see one of these when a project goes quiet.",
		}}); err != nil {
			return err
		}
		fmt.Println("Sent a sample notification.")
		return nil
	},
}

func init() {
	checkCmd.Flags().BoolVar(&checkJSON, "json", false, "print the projects notified as JSON")
	testCmd.AddCommand(testNotificationCmd, testEmailCmd)
	rootCmd.AddCommand(checkCmd, testCmd)
}
