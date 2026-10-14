package cmd

import (
	"fmt"
	"strconv"
	"time"

	"github.com/njayman/proz/core"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show or change proz settings",
}

var staleDaysCmd = &cobra.Command{
	Use:   "stale-days [days]",
	Short: "Show or set how many quiet days make a project stale (default 7)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withStore(func(s *core.Store) error {
			if len(args) == 1 {
				days, err := strconv.Atoi(args[0])
				if err != nil || days < 1 || days > 36500 {
					return fmt.Errorf("days must be a whole number from 1 to 36500, got %q", args[0])
				}
				return s.SetStaleThreshold(time.Duration(days) * 24 * time.Hour)
			}
			d, err := s.StaleThreshold()
			if err != nil {
				return err
			}
			fmt.Println(int(d / (24 * time.Hour)))
			return nil
		})
	},
}

func init() {
	configCmd.AddCommand(staleDaysCmd)
	rootCmd.AddCommand(configCmd)
}
