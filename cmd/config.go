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

var staleProject string

var staleDaysCmd = &cobra.Command{
	Use:   "stale-days [days]",
	Short: "Show or set how many quiet days make a project stale (default 7)",
	Long: "Show or set how many quiet days make a project stale (default 7).\n" +
		"With --project, show or set an override for one project; 0 removes it.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var p Project
		if staleProject != "" {
			var err error
			if p, err = findProject(staleProject); err != nil {
				return err
			}
		}
		return withStore(func(s *core.Store) error {
			if len(args) == 1 {
				days, err := strconv.Atoi(args[0])
				if staleProject != "" && err == nil && days == 0 {
					return s.SetProjectStaleThreshold(p.ID, 0)
				}
				if err != nil || days < 1 || days > 36500 {
					return fmt.Errorf("days must be a whole number from 1 to 36500, got %q", args[0])
				}
				d := time.Duration(days) * 24 * time.Hour
				if staleProject != "" {
					return s.SetProjectStaleThreshold(p.ID, d)
				}
				return s.SetStaleThreshold(d)
			}
			d, err := s.StaleThreshold()
			if staleProject != "" {
				d, err = s.ProjectStaleThreshold(p.ID)
			}
			if err != nil {
				return err
			}
			fmt.Println(int(d / (24 * time.Hour)))
			return nil
		})
	},
}

func init() {
	staleDaysCmd.Flags().StringVar(&staleProject, "project", "", "project name to override")
	configCmd.AddCommand(staleDaysCmd)
	rootCmd.AddCommand(configCmd)
}
