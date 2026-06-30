//go:build !linux && !windows && !darwin

package cmd

func parseDesktopFiles() []desktopApp {
	return nil
}
