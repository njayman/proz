package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/njayman/proz/core"
	"github.com/njayman/proz/notify"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// sendEmail sends one email; tests replace it so none is ever sent for real.
var sendEmail = notify.Email

// digest sends the weekly email digest if email is on and it is due.
func digest(s *core.Store) error {
	on, err := s.ModuleOn(core.ModuleEmail)
	if err != nil || !on {
		return err
	}
	c, ok, err := s.SMTP()
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("email is on but SMTP is not configured: proz config smtp")
	}
	return s.Digest(func(stale []core.Status) error {
		subject, body := notify.Digest(stale)
		return sendEmail(c, subject, body, now())
	})
}

var (
	smtpFlags         core.SMTP
	smtpPasswordStdin bool
)

var smtpCmd = &cobra.Command{
	Use:   "smtp",
	Short: "Show or set the SMTP server for the weekly email digest",
	Long: "Show or set the SMTP server for the weekly email digest. Setting it switches\n" +
		"email on the first time. Flags not given keep their saved value; a new host\n" +
		"or username needs the password again. The password is read from\n" +
		"stdin with --password-stdin, never from a flag, and is saved in smtp.json in\n" +
		"the config folder, readable only by you. Port 465 uses TLS; other ports must\n" +
		"offer STARTTLS (plaintext is allowed only to localhost).",
	Example: "  proz config smtp --host smtp.gmail.com --username me@gmail.com \\\n" +
		"    --from me@gmail.com --to me@gmail.com --password-stdin",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return withStore(func(s *core.Store) error {
			c, ok, err := s.SMTP()
			if err != nil {
				return err
			}
			set := false
			cmd.Flags().VisitAll(func(f *pflag.Flag) { set = set || f.Changed })
			if !set {
				if !ok {
					fmt.Println("not configured")
					return nil
				}
				pw := "not set"
				if c.Password != "" {
					pw = "set"
				}
				fmt.Printf("server: %s:%d\nusername: %s\nfrom: %s\nto: %s\npassword: %s\n", c.Host, c.Port, c.Username, c.From, c.To, pw)
				return nil
			}
			if !ok {
				c.Port = 587
			}
			f := cmd.Flags()
			for name, dst := range map[string]*string{"host": &c.Host, "username": &c.Username, "from": &c.From, "to": &c.To} {
				if f.Changed(name) {
					v, _ := f.GetString(name)
					*dst = v
				}
			}
			if f.Changed("port") {
				c.Port = smtpFlags.Port
			}
			if ok && c.Password != "" && !smtpPasswordStdin && (f.Changed("host") || f.Changed("username")) {
				return errors.New("a new host or username needs its password again: add --password-stdin")
			}
			if smtpPasswordStdin {
				if c.Password, err = readPassword(); err != nil {
					return err
				}
			}
			if err := s.SetSMTP(c); err != nil {
				return err
			}
			if ok { // an edit keeps email as it was
				fmt.Println("Saved.")
				return nil
			}
			if err := s.SetModule(core.ModuleEmail, true); err != nil {
				return err
			}
			fmt.Println("Saved. Email is on; send a sample with: proz test email")
			return nil
		})
	},
}

// readPassword reads one line from stdin, without echo on a terminal.
func readPassword() (string, error) {
	if term.IsTerminal(os.Stdin.Fd()) {
		fmt.Fprint(os.Stderr, "SMTP password: ")
		b, err := term.ReadPassword(os.Stdin.Fd())
		fmt.Fprintln(os.Stderr)
		return string(b), err
	}
	line, err := stdin.ReadString('\n')
	if err != nil && line == "" {
		return "", errors.New("--password-stdin: no password on stdin")
	}
	return strings.TrimRight(line, "\r\n"), nil
}

var digestDayCmd = &cobra.Command{
	Use:   "digest-day [weekday]",
	Short: "Show or set the weekday the email digest goes out (default monday)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withStore(func(s *core.Store) error {
			if len(args) == 1 {
				for d := time.Sunday; d <= time.Saturday; d++ {
					name := strings.ToLower(d.String())
					if a := strings.ToLower(args[0]); len(a) >= 3 && strings.HasPrefix(name, a) {
						return s.SetDigestDay(d)
					}
				}
				return fmt.Errorf("not a weekday: %q", args[0])
			}
			d, err := s.DigestDay()
			if err != nil {
				return err
			}
			fmt.Println(strings.ToLower(d.String()))
			return nil
		})
	},
}

var testEmailCmd = &cobra.Command{
	Use:   "email",
	Short: "Send one sample email digest through the configured SMTP server",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return withStore(func(s *core.Store) error {
			c, ok, err := s.SMTP()
			if err != nil {
				return err
			}
			if !ok {
				return errors.New("SMTP is not configured: proz config smtp")
			}
			_, body := notify.Digest([]core.Status{{
				Project: core.Project{Name: "example"},
				Note:    core.Note{Lines: []string{"Quiet for 8 days", "Milestone: shipped login; next: signup"}},
			}})
			if err := sendEmail(c, "proz email works", "This is a sample. The real digest looks like this:\n\n"+body, now()); err != nil {
				return err
			}
			fmt.Println("Sent a sample email to " + c.To + ".")
			return nil
		})
	},
}

func init() {
	f := smtpCmd.Flags()
	f.String("host", "", "SMTP server host")
	f.IntVar(&smtpFlags.Port, "port", 587, "SMTP server port (465 for TLS)")
	f.String("username", "", "SMTP username (empty: no login)")
	f.String("from", "", "sender address")
	f.String("to", "", "your address, where the digest goes")
	f.BoolVar(&smtpPasswordStdin, "password-stdin", false, "read the SMTP password from stdin")
	configCmd.AddCommand(smtpCmd, digestDayCmd)
}
