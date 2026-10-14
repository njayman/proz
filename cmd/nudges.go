package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/njayman/proz/contract"
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
			return printJSON(contract.Nudges(nudges))
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
