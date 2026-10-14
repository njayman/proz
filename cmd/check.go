package cmd

import (
	"fmt"

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
		"and show one OS notification per project per quiet stretch. Safe to run\n" +
		"often, from cron or a scheduler; it never repeats a notification.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var notified []core.Status
		if err := withStore(func(s *core.Store) error {
			on, err := s.ModuleOn(core.ModuleNotifications)
			if err != nil || !on {
				return err
			}
			return s.Check(func(due []core.Status) error {
				if err := send(notify.Render(due)); err != nil {
					return err
				}
				notified = due
				return nil
			})
		}); err != nil {
			return err
		}
		if checkJSON {
			out := []nudgeJSON{}
			for _, n := range notified {
				out = append(out, toNudgeJSON(n))
			}
			return printJSON(out)
		}
		for _, n := range notified {
			fmt.Printf("notified: %s\n", n.Name)
		}
		return nil
	},
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
		if err := send([]notify.Message{{
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
	testCmd.AddCommand(testNotificationCmd)
	rootCmd.AddCommand(checkCmd, testCmd)
}
