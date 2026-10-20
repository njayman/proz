package cmd

import "os/exec"

// ownSession does nothing on Windows: a console program already gets its
// own console there, and a GUI program is not stopped with proz.
func ownSession(*exec.Cmd) {}
