package notify

import (
	"bufio"
	"io"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strings"
	"testing"
	"time"

	"github.com/njayman/proz/core"
)

func TestDigestListsEveryProjectWithFullNoteAndCommands(t *testing.T) {
	a := nudge("web", "Quiet for 8 days", "Milestone: shipped login")
	a.Path = "/home/me/web"
	subject, body := Digest([]core.Status{a, nudge("habit tracker", "Quiet for 9 days")})
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
	if s, _ := Digest([]core.Status{a}); s != "proz: web went quiet" {
		t.Errorf("one project subject: %q", s)
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
	if err := Email(c, "proz: web went quiet", "Quiet for 8 days: café\n", time.Unix(0, 0)); err != nil {
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
	body, _ := io.ReadAll(quotedprintable.NewReader(msg.Body))
	if !strings.Contains(string(body), "café") {
		t.Errorf("body %q", body)
	}
}

func TestEmailFailsFastWhenTheServerIsDown(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	c := core.SMTP{Host: "127.0.0.1", Port: port, From: "me@example.com", To: "me@example.com"}
	if err := Email(c, "s", "b", time.Now()); err == nil {
		t.Fatal("want an error")
	}
}

func TestEmailRefusesPlaintextToARemoteServer(t *testing.T) {
	port, got := fakeSMTPOn(t, "127.0.0.2") // loopback, but not "localhost": stands in for a remote server
	c := core.SMTP{Host: "127.0.0.2", Port: port, From: "me@example.com", To: "me@example.com"}
	if err := Email(c, "s", "secret notes", time.Now()); err == nil || !strings.Contains(err.Error(), "STARTTLS") {
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
