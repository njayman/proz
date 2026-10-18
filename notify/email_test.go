package notify

import (
	"bufio"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/njayman/proz/core"
)

func TestDigestListsEveryProjectWithFullNoteAndCommands(t *testing.T) {
	a := nudge("web", "Quiet for 8 days", "Milestone: shipped login")
	a.Path = "/home/me/web"
	m := Digest([]core.Status{a, nudge("habit tracker", "Quiet for 9 days")})
	subject, body := m.Subject, m.Text
	if subject != "proz: 2 projects went quiet" {
		t.Errorf("subject: %q", subject)
	}
	for _, want := range []string{
		"web (/home/me/web)\n  Quiet for 8 days\n  Milestone: shipped login\n",
		"  Resume: proz resume web\n  Snooze: proz snooze web\n  Drop:   proz drop web\n",
		"habit tracker\n  Quiet for 9 days\n",
		"proz resume 'habit tracker'",
		"proz config email off",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q:\n%s", want, body)
		}
	}
	if s := Digest([]core.Status{a}).Subject; s != "proz: web went quiet" {
		t.Errorf("one project subject: %q", s)
	}
}

func TestDigestHTMLListsTheSameProjectsEscapedAndLocal(t *testing.T) {
	a := nudge("<b>web</b> & co", "Quiet for 8 days", "Milestone: <script>alert(1)</script>")
	a.Path = "/home/me/<web>"
	got := Digest([]core.Status{a, nudge("habit tracker", "Quiet for 9 days")}).HTML
	for _, want := range []string{
		">" + html.EscapeString("<b>web</b> & co") + "</h2>",
		html.EscapeString("/home/me/<web>"),
		html.EscapeString("Milestone: <script>alert(1)</script>"),
		"<code>proz resume " + html.EscapeString(shellQuote(a.Name)) + "</code>",
		"<code>proz snooze " + html.EscapeString(shellQuote(a.Name)) + "</code>",
		"<code>proz drop " + html.EscapeString(shellQuote(a.Name)) + "</code>",
		">habit tracker</h2>",
		"Quiet for 9 days",
		"<code>proz config email off</code>",
		"max-width:600px",
		"font-family:system-ui",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("html missing %q:\n%s", want, got)
		}
	}
	for _, remote := range []string{"<script", "<img", "<link", "<style", "src=", "href=", "url(", "http"} {
		if strings.Contains(got, remote) {
			t.Errorf("html has %q:\n%s", remote, got)
		}
	}
}

func TestSampleEmailUsesTheDigestFormat(t *testing.T) {
	m := Sample()
	if m.Subject != "proz email works" {
		t.Errorf("subject %q", m.Subject)
	}
	for _, part := range []string{m.Text, m.HTML} {
		for _, want := range []string{"This is a sample.", "example", "Milestone: shipped login; next: signup", "proz resume example"} {
			if !strings.Contains(part, want) {
				t.Errorf("sample missing %q:\n%s", want, part)
			}
		}
	}
	if !strings.Contains(m.HTML, ">example</h2>") {
		t.Errorf("sample html is not the digest layout:\n%s", m.HTML)
	}
}

// fakeSMTP is a local SMTP server that accepts one message and reports what
// it received: the AUTH line, the envelope and the data.
func fakeSMTP(t *testing.T) (port int, got chan string) { return fakeSMTPOn(t, "127.0.0.1") }

func fakeSMTPOn(t *testing.T, host string) (port int, got chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", host+":0")
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { ln.Close() })
	got = make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		r := bufio.NewReader(c)
		say := func(s string) { io.WriteString(c, s+"\r\n") }
		var log strings.Builder
		say("220 fake")
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			switch cmd := strings.ToUpper(strings.Fields(line + " x")[0]); cmd {
			case "EHLO", "HELO":
				say("250-fake")
				say("250 AUTH PLAIN")
			case "AUTH":
				log.WriteString(line + "\n")
				say("235 ok")
			case "MAIL", "RCPT":
				log.WriteString(line + "\n")
				say("250 ok")
			case "DATA":
				say("354 go")
				for {
					l, err := r.ReadString('\n')
					if err != nil || l == ".\r\n" {
						break
					}
					log.WriteString(l)
				}
				say("250 queued")
			case "QUIT":
				say("221 bye")
				got <- log.String()
				return
			default:
				say("250 ok")
			}
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, got
}

func TestEmailIsSentThroughSMTPWithAuth(t *testing.T) {
	port, got := fakeSMTP(t)
	c := core.SMTP{Host: "127.0.0.1", Port: port, Username: "me", Password: "pw", From: "proz <me@example.com>", To: "me@example.com"}
	m := Mail{Subject: "proz: web went quiet", Text: "Quiet for 8 days: café\n", HTML: "<p>Quiet for 8 days: café</p>\n"}
	if err := Email(c, m, time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}
	var log string
	select {
	case log = <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("no message received")
	}
	for _, want := range []string{"AUTH PLAIN ", "MAIL FROM:<me@example.com>", "RCPT TO:<me@example.com>"} {
		if !strings.Contains(log, want) {
			t.Errorf("missing %q in\n%s", want, log)
		}
	}
	data := log[strings.Index(log, "RCPT"):]
	data = data[strings.Index(data, "\n")+1:]
	msg, err := mail.ReadMessage(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if s := msg.Header.Get("Subject"); s != "proz: web went quiet" {
		t.Errorf("subject %q", s)
	}
	if msg.Header.Get("Date") == "" {
		t.Error("no Date header")
	}
	mt, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || mt != "multipart/alternative" {
		t.Fatalf("content type %q: %v", msg.Header.Get("Content-Type"), err)
	}
	r := multipart.NewReader(msg.Body, params["boundary"])
	for _, want := range []struct{ typ, body string }{{"text/plain", m.Text}, {"text/html", m.HTML}} {
		p, err := r.NextPart() // decodes quoted-printable
		if err != nil {
			t.Fatalf("want a %s part: %v", want.typ, err)
		}
		if typ, ps, _ := mime.ParseMediaType(p.Header.Get("Content-Type")); typ != want.typ || ps["charset"] != "UTF-8" {
			t.Errorf("part type %q", p.Header.Get("Content-Type"))
		}
		if b, _ := io.ReadAll(p); strings.ReplaceAll(string(b), "\r\n", "\n") != want.body { // SMTP sends CRLF
			t.Errorf("%s part %q, want %q", want.typ, b, want.body)
		}
	}
	if _, err := r.NextPart(); err != io.EOF {
		t.Errorf("want exactly two parts, got %v", err)
	}
}

func TestEmailFailsFastWhenTheServerIsDown(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	c := core.SMTP{Host: "127.0.0.1", Port: port, From: "me@example.com", To: "me@example.com"}
	if err := Email(c, Mail{Subject: "s", Text: "b"}, time.Now()); err == nil {
		t.Fatal("want an error")
	}
}

func TestEmailRefusesPlaintextToARemoteServer(t *testing.T) {
	port, got := fakeSMTPOn(t, "127.0.0.2") // loopback, but not "localhost": stands in for a remote server
	c := core.SMTP{Host: "127.0.0.2", Port: port, From: "me@example.com", To: "me@example.com"}
	if err := Email(c, Mail{Subject: "s", Text: "secret notes", HTML: "secret notes"}, time.Now()); err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("want a STARTTLS error, got %v", err)
	}
	select {
	case log := <-got:
		if strings.Contains(log, "secret notes") {
			t.Fatal("body sent in plaintext")
		}
	case <-time.After(time.Second):
	}
}

func TestEveryEmailHasAUniqueMessageID(t *testing.T) {
	ids := map[string]bool{}
	for _, tc := range []struct{ from, want string }{
		{"proz <me@example.com>", `^<[A-Z2-7]{26}@example\.com>$`},
		{"me@example.com", `^<[A-Z2-7]{26}@example\.com>$`},
		{"me@bücher.example", `^<[A-Z2-7]{26}@proz\.invalid>$`}, // a msg-id must be ASCII
	} {
		port, got := fakeSMTP(t)
		c := core.SMTP{Host: "127.0.0.1", Port: port, From: tc.from, To: "me@elsewhere.test"}
		if err := Email(c, Mail{Subject: "s", Text: "b"}, time.Now()); err != nil {
			t.Fatal(err)
		}
		log := <-got
		i := strings.Index(log, "RCPT")
		if i < 0 {
			t.Fatalf("no envelope in\n%s", log)
		}
		data := log[i:]
		msg, err := mail.ReadMessage(strings.NewReader(data[strings.Index(data, "\n")+1:]))
		if err != nil {
			t.Fatal(err)
		}
		id := msg.Header.Get("Message-ID")
		if !regexp.MustCompile(tc.want).MatchString(id) {
			t.Fatalf("from %q: Message-ID %q, want %s", tc.from, id, tc.want)
		}
		ids[id] = true
	}
	if len(ids) != 3 {
		t.Fatal("Message-ID repeats")
	}
}

func TestEmailReportsAConnectionDroppedMidMessage(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		r := bufio.NewReader(c)
		io.WriteString(c, "220 fake\r\n")
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if strings.HasPrefix(line, "DATA") {
				io.WriteString(c, "354 go\r\n")
				c.Close() // the server dies before the message is in
				return
			}
			io.WriteString(c, "250 ok\r\n")
		}
	}()
	c := core.SMTP{Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port, From: "me@example.com", To: "me@example.com"}
	big := strings.Repeat("a long note line\n", 20000) // overflows every buffer, so a write fails
	if err := Email(c, Mail{Subject: "s", Text: big, HTML: big}, time.Now()); err == nil {
		t.Fatal("want an error")
	}
}

func TestEmailWithoutHTMLSendsOnlyText(t *testing.T) {
	port, got := fakeSMTP(t)
	c := core.SMTP{Host: "127.0.0.1", Port: port, From: "me@example.com", To: "me@example.com"}
	if err := Email(c, Mail{Subject: "s", Text: "just text"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	data := <-got
	data = data[strings.Index(data, "RCPT"):]
	msg, err := mail.ReadMessage(strings.NewReader(data[strings.Index(data, "\n")+1:]))
	if err != nil {
		t.Fatal(err)
	}
	_, params, _ := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	r := multipart.NewReader(msg.Body, params["boundary"])
	if p, err := r.NextPart(); err != nil || !strings.HasPrefix(p.Header.Get("Content-Type"), "text/plain") {
		t.Fatalf("want a text part: %v", err)
	}
	if _, err := r.NextPart(); err != io.EOF {
		t.Fatalf("an empty HTML part makes clients show a blank email: %v", err)
	}
}
