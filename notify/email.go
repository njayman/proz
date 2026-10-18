package notify

import (
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"html/template"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/njayman/proz/core"
)

// Mail is one email, as plain text and as HTML with the same content.
type Mail struct{ Subject, Text, HTML string }

// Sample is the email sent by proz test email: the digest for one example
// project, marked as a sample.
func Sample() Mail {
	m := digest("This is a sample. The real digest looks like this:", []core.Status{{
		Project: core.Project{Name: "example"},
		Note:    core.Note{Lines: []string{"Quiet for 8 days", "Milestone: shipped login; next: signup"}},
	}})
	m.Subject = "proz email works"
	return m
}

// Digest renders the weekly email: every stale project with its full note
// and the exact commands that answer it.
func Digest(stale []core.Status) Mail { return digest("", stale) }

const (
	intro  = "These projects went quiet. Each asks one decision: resume, snooze or drop."
	footer = "Sent by proz once a week. To stop: "
)

func digest(lead string, stale []core.Status) Mail {
	subject := fmt.Sprintf("proz: %d projects went quiet", len(stale))
	if len(stale) == 1 {
		subject = "proz: " + stale[0].Name + " went quiet"
	}
	var b strings.Builder
	if lead != "" {
		b.WriteString(lead + "\n\n")
	}
	b.WriteString(intro + "\n")
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
	b.WriteString("\n" + footer + "proz config email off\n")
	var h strings.Builder
	if err := page.Execute(&h, map[string]any{"Lead": lead, "Intro": intro, "Footer": footer, "Stale": stale}); err != nil {
		panic(err) // only a broken template fails here, and the tests render it
	}
	return Mail{Subject: subject, Text: b.String(), HTML: h.String()}
}

// page is the HTML part: one plain column that any mail client renders, with
// inline styles and nothing loaded from the network. html/template escapes
// every name, path and note.
var page = template.Must(template.New("digest").Funcs(template.FuncMap{"q": shellQuote}).Parse(`<!doctype html>
<html><body style="margin:0;padding:16px">
<div style="max-width:600px;margin:0 auto;font-family:system-ui,-apple-system,'Segoe UI',Roboto,sans-serif;font-size:15px;line-height:1.5">
{{if .Lead}}<p>{{.Lead}}</p>
{{end}}<p>{{.Intro}}</p>
{{range .Stale}}<h2 style="font-size:18px;margin:28px 0 4px">{{.Name}}</h2>
{{if .Path}}<p style="margin:0 0 8px">{{.Path}}</p>
{{end}}{{range .Note.Lines}}<p style="margin:0">{{.}}</p>
{{end}}{{$n := q .Name}}<p style="margin:8px 0 0;font-family:ui-monospace,Menlo,Consolas,monospace;font-size:13px">Resume: <code>proz resume {{$n}}</code><br>Snooze: <code>proz snooze {{$n}}</code><br>Drop: <code>proz drop {{$n}}</code></p>
{{end}}<p style="margin:32px 0 0;font-size:13px">{{.Footer}}<code>proz config email off</code></p>
</div>
</body></html>
`))

const smtpTimeout = 30 * time.Second

// Email sends one message as text and HTML alternatives, always encrypted except to localhost.
// Port 465 speaks TLS from the start; any other port must offer STARTTLS.
// Every step is bounded, so a dead server never hangs the tray.
func Email(c core.SMTP, m Mail, now time.Time) error {
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
	mw := multipart.NewWriter(w)
	fmt.Fprintf(w, "From: %s\nTo: %s\nSubject: %s\nDate: %s\nMessage-ID: %s\nMIME-Version: 1.0\nContent-Type: multipart/alternative; boundary=%s\n\n",
		from, to, mime.QEncoding.Encode("utf-8", m.Subject), now.Format(time.RFC1123Z), messageID(from.Address), mw.Boundary())
	for _, part := range []struct{ typ, body string }{{"text/plain", m.Text}, {"text/html", m.HTML}} { // plain first: clients show the last part they can
		if part.body == "" {
			continue // an empty alternative would show as a blank email
		}
		pw, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.typ + "; charset=UTF-8"},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return fmt.Errorf("sending email: %w", err)
		}
		qp := quotedprintable.NewWriter(pw)
		qp.Write([]byte(part.body))
		qp.Close()
	}
	mw.Close() // ponytail: write errors stick in the connection's buffer and surface at w.Close
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
