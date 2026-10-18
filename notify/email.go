package notify

import (
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/njayman/proz/core"
)

// Digest renders the weekly email: every stale project with its full note
// and the exact commands that answer it.
func Digest(stale []core.Status) (subject, body string) {
	subject = fmt.Sprintf("proz: %d projects went quiet", len(stale))
	if len(stale) == 1 {
		subject = "proz: " + stale[0].Name + " went quiet"
	}
	var b strings.Builder
	b.WriteString("These projects went quiet. Each asks one decision: resume, snooze or drop.\n")
	for _, st := range stale {
		b.WriteString("\n" + st.Name)
		if st.Path != "" {
			b.WriteString(" (" + st.Path + ")")
		}
		b.WriteString("\n")
		for _, l := range st.Note.Lines {
			b.WriteString("  " + l + "\n")
		}
		name := shellQuote(st.Name)
		fmt.Fprintf(&b, "  Resume: proz resume %s\n  Snooze: proz snooze %s\n  Drop:   proz drop %s\n", name, name, name)
	}
	b.WriteString("\nSent by proz once a week. To stop: proz config email off\n")
	return subject, b.String()
}

const smtpTimeout = 30 * time.Second

// Email sends one plain-text message, always encrypted except to localhost.
// Port 465 speaks TLS from the start; any other port must offer STARTTLS.
// Every step is bounded, so a dead server never hangs the tray.
func Email(c core.SMTP, subject, body string, now time.Time) error {
	from, err := mail.ParseAddress(c.From)
	if err != nil {
		return fmt.Errorf("from address: %w", err)
	}
	to, err := mail.ParseAddress(c.To)
	if err != nil {
		return fmt.Errorf("to address: %w", err)
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(c.Host, strconv.Itoa(c.Port)), smtpTimeout)
	if err != nil {
		return fmt.Errorf("sending email: %w", err)
	}
	conn.SetDeadline(time.Now().Add(smtpTimeout))
	if c.Port == 465 {
		conn = tls.Client(conn, &tls.Config{ServerName: c.Host})
	}
	cl, err := smtp.NewClient(conn, c.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("sending email: %w", err)
	}
	defer cl.Close()
	if c.Port != 465 {
		if ok, _ := cl.Extension("STARTTLS"); ok {
			if err := cl.StartTLS(&tls.Config{ServerName: c.Host}); err != nil {
				return fmt.Errorf("sending email: %w", err)
			}
		} else if !localhost[c.Host] {
			return fmt.Errorf("sending email: %s does not offer STARTTLS; use port 465 for TLS", c.Host)
		}
	}
	if c.Username != "" {
		if err := cl.Auth(smtp.PlainAuth("", c.Username, c.Password, c.Host)); err != nil {
			return fmt.Errorf("sending email: %w", err)
		}
	}
	if err := cl.Mail(from.Address); err != nil {
		return fmt.Errorf("sending email: %w", err)
	}
	if err := cl.Rcpt(to.Address); err != nil {
		return fmt.Errorf("sending email: %w", err)
	}
	w, err := cl.Data()
	if err != nil {
		return fmt.Errorf("sending email: %w", err)
	}
	fmt.Fprintf(w, "From: %s\nTo: %s\nSubject: %s\nDate: %s\nMessage-ID: %s\nMIME-Version: 1.0\nContent-Type: text/plain; charset=UTF-8\nContent-Transfer-Encoding: quoted-printable\n\n",
		from, to, mime.QEncoding.Encode("utf-8", subject), now.Format(time.RFC1123Z), messageID(from.Address))
	qp := quotedprintable.NewWriter(w)
	qp.Write([]byte(body))
	qp.Close()
	if err := w.Close(); err != nil {
		return fmt.Errorf("sending email: %w", err)
	}
	cl.Quit() // the server has the message; a failed goodbye must not resend it
	return nil
}

// messageID is unique per message; strict servers such as Gmail reject mail
// without one. A msg-id must be ASCII, so an international domain gets a
// reserved stand-in rather than an idna dependency.
func messageID(from string) string {
	domain := from[strings.LastIndex(from, "@")+1:]
	for _, r := range domain {
		if r > unicode.MaxASCII {
			domain = "proz.invalid"
			break
		}
	}
	return fmt.Sprintf("<%s@%s>", rand.Text(), domain)
}

// localhost is where a plaintext connection never leaves the machine, such
// as a local relay.
var localhost = map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true}
