package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/njayman/proz/core"
	"github.com/spf13/cobra"
)

var nudgesJSON bool

var nudgesCmd = &cobra.Command{
	Use:   "nudges",
	Short: "List projects that have gone quiet",
	Long: "List projects with no activity (a git commit or registration) for longer\n" +
		"than the stale threshold. Change the threshold with 'proz config stale-days'.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var nudges []core.Nudge
		if err := withStore(func(s *core.Store) (err error) {
			nudges, err = s.Nudges()
			return err
		}); err != nil {
			return err
		}
		if nudgesJSON {
			type nudgeJSON struct {
				Name         string `json:"name"`
				Path         string `json:"path"`
				LastActivity string `json:"last_activity"`
				QuietDays    int    `json:"quiet_days"`
			}
			out := []nudgeJSON{}
			for _, n := range nudges {
				out = append(out, nudgeJSON{n.Name, n.Path, n.LastActivity.UTC().Format(time.RFC3339), quietDays(n)})
			}
			b, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(b))
			return nil
		}
		if len(nudges) == 0 {
			fmt.Fprintln(os.Stderr, "nothing has gone quiet")
		}
		for _, n := range nudges {
			fmt.Printf("%s\tquiet %d days\t%s\n", n.Name, quietDays(n), n.Path)
		}
		return nil
	},
}

func quietDays(n core.Nudge) int { return int(n.Quiet / (24 * time.Hour)) }

func init() {
	nudgesCmd.Flags().BoolVar(&nudgesJSON, "json", false, "print nudges as JSON")
	rootCmd.AddCommand(nudgesCmd)
}
