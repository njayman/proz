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
	Long: "List projects with no activity (registration, launch or git commit) for\n" +
		"longer than the stale threshold, each with its left-off note.\n" +
		"Change the threshold with 'proz config stale-days'.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var nudges []core.Status
		if err := withStore(func(s *core.Store) (err error) {
			nudges, err = s.Nudges()
			return err
		}); err != nil {
			return err
		}
		if nudgesJSON {
			out := []nudgeJSON{}
			for _, n := range nudges {
				out = append(out, toNudgeJSON(n))
			}
			return printJSON(out)
		}
		if len(nudges) == 0 {
			fmt.Fprintln(os.Stderr, "nothing has gone quiet")
		}
		for _, n := range nudges {
			fmt.Printf("%s\t%s\t%s\n", n.Name, n.Note.Short(), n.Path)
		}
		return nil
	},
}

// nudgeJSON is the agent contract for one project in nudges and status output.
type nudgeJSON struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	LastActivity string `json:"last_activity"`
	QuietDays    int    `json:"quiet_days"`
	QuietSeconds int64  `json:"quiet_seconds"`
	Note         string `json:"note"`
	ShortNote    string `json:"short_note"`
}

func toNudgeJSON(s core.Status) nudgeJSON {
	return nudgeJSON{
		Name:         s.Name,
		Path:         s.Path,
		LastActivity: s.LastActivity.UTC().Format(time.RFC3339),
		QuietDays:    int(s.Quiet / (24 * time.Hour)),
		QuietSeconds: int64(s.Quiet / time.Second),
		Note:         s.Note.Full(),
		ShortNote:    s.Note.Short(),
	}
}

func printJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func init() {
	nudgesCmd.Flags().BoolVar(&nudgesJSON, "json", false, "print nudges as JSON")
	rootCmd.AddCommand(nudgesCmd)
}
