package mailer

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)

func parseBuilt(t *testing.T, raw []byte) *mail.Message {
	t.Helper()
	if bytes.Contains(bytes.ReplaceAll(raw, []byte("\r\n"), nil), []byte("\n")) {
		t.Fatal("message contains bare LF line endings")
	}
	m, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("mail.ReadMessage: %v", err)
	}
	return m
}

func TestBuildMessage_Headers(t *testing.T) {
	msg := validMessage()
	raw := buildMessage("BookHaven <noreply@example.com>", msg, testNow, "<id-1@example.com>")
	m := parseBuilt(t, raw)

	from, err := m.Header.AddressList("From")
	if err != nil || len(from) != 1 || from[0].Address != "noreply@example.com" {
		t.Errorf("From = %v, err=%v", from, err)
	}
	to, err := m.Header.AddressList("To")
	if err != nil || len(to) != 1 || to[0].Address != "reader@example.com" {
		t.Errorf("To = %v, err=%v", to, err)
	}
	if got := m.Header.Get("Message-ID"); got != "<id-1@example.com>" {
		t.Errorf("Message-ID = %q", got)
	}
	if got := m.Header.Get("MIME-Version"); got != "1.0" {
		t.Errorf("MIME-Version = %q", got)
	}
	date, err := m.Header.Date()
	if err != nil || !date.Equal(testNow) {
		t.Errorf("Date = %v, err=%v", date, err)
	}

	rawSubject := m.Header.Get("Subject")
	if !strings.HasPrefix(rawSubject, "=?UTF-8?") {
		t.Errorf("Cyrillic subject not RFC 2047 encoded: %q", rawSubject)
	}
	decoded, err := new(mime.WordDecoder).DecodeHeader(rawSubject)
	if err != nil || decoded != msg.Subject {
		t.Errorf("decoded subject = %q, err=%v, want %q", decoded, err, msg.Subject)
	}
}

func TestBuildMessage_TextOnly(t *testing.T) {
	msg := validMessage()
	m := parseBuilt(t, buildMessage("noreply@example.com", msg, testNow, "<id@example.com>"))

	mediaType, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if err != nil || mediaType != "text/plain" || !strings.EqualFold(params["charset"], "utf-8") {
		t.Fatalf("Content-Type = %q (%v)", m.Header.Get("Content-Type"), err)
	}
	if got := m.Header.Get("Content-Transfer-Encoding"); got != "quoted-printable" {
		t.Errorf("Content-Transfer-Encoding = %q", got)
	}
	body, err := io.ReadAll(quotedprintable.NewReader(m.Body))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != msg.TextBody {
		t.Errorf("body = %q, want %q", body, msg.TextBody)
	}
}

func TestBuildMessage_Multipart(t *testing.T) {
	msg := validMessage()
	msg.HTMLBody = `<p>Привет! <a href="https://bookhaven.example.com/verify?token=abc">Подтвердить</a></p>`
	m := parseBuilt(t, buildMessage("noreply@example.com", msg, testNow, "<id@example.com>"))

	mediaType, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/alternative" || params["boundary"] == "" {
		t.Fatalf("Content-Type = %q (%v)", m.Header.Get("Content-Type"), err)
	}

	mr := multipart.NewReader(m.Body, params["boundary"])
	want := []struct {
		mediaType string
		body      string
	}{
		{"text/plain", msg.TextBody},
		{"text/html", msg.HTMLBody},
	}
	for i, w := range want {
		part, err := mr.NextPart()
		if err != nil {
			t.Fatalf("part %d: %v", i, err)
		}
		pt, pparams, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil || pt != w.mediaType || !strings.EqualFold(pparams["charset"], "utf-8") {
			t.Errorf("part %d Content-Type = %q", i, part.Header.Get("Content-Type"))
		}
		// multipart.Reader transparently decodes quoted-printable parts.
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != w.body {
			t.Errorf("part %d body = %q, want %q", i, body, w.body)
		}
	}
	if _, err := mr.NextPart(); err != io.EOF {
		t.Errorf("expected exactly two parts, got extra (err=%v)", err)
	}
}

// fakeSMTPServer is a minimal in-process SMTP server on 127.0.0.1 (no
// STARTTLS, no AUTH) recording the DATA it receives. rcptReply overrides
// the RCPT TO response (default "250 OK").
type fakeSMTPServer struct {
	ln        net.Listener
	rcptReply string
	data      chan string
}

func newFakeSMTPServer(t *testing.T, rcptReply string) *fakeSMTPServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &fakeSMTPServer{ln: ln, rcptReply: rcptReply, data: make(chan string, 1)}
	t.Cleanup(func() { _ = ln.Close() })
	go s.serve()
	return s
}

func (s *fakeSMTPServer) port() int { return s.ln.Addr().(*net.TCPAddr).Port }

func (s *fakeSMTPServer) serve() {
	conn, err := s.ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	tp := textproto.NewConn(conn)
	reply := func(line string) { _ = tp.PrintfLine("%s", line) }

	reply("220 fake ESMTP")
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.SplitN(line, " ", 2)[0])
		switch cmd {
		case "EHLO", "HELO":
			reply("250-fake")
			reply("250 8BITMIME")
		case "MAIL":
			reply("250 OK")
		case "RCPT":
			if s.rcptReply != "" {
				reply(s.rcptReply)
			} else {
				reply("250 OK")
			}
		case "DATA":
			reply("354 go ahead")
			lines, err := tp.ReadDotLines()
			if err != nil {
				return
			}
			s.data <- strings.Join(lines, "\n")
			reply("250 queued")
		case "QUIT":
			reply("221 bye")
			return
		default:
			reply("250 OK")
		}
	}
}

func TestSMTPSender_SendDeliversMessage(t *testing.T) {
	srv := newFakeSMTPServer(t, "")
	sender := NewSMTPSender(SMTPOptions{Host: "127.0.0.1", Port: srv.port(), From: "BookHaven <noreply@example.com>"})

	msg := validMessage()
	msg.HTMLBody = "<p>Привет</p>"
	if err := sender.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	select {
	case data := <-srv.data:
		m, err := mail.ReadMessage(bufio.NewReader(strings.NewReader(data)))
		if err != nil {
			t.Fatalf("server received unparseable message: %v", err)
		}
		if mt, _, _ := mime.ParseMediaType(m.Header.Get("Content-Type")); mt != "multipart/alternative" {
			t.Errorf("Content-Type = %q", m.Header.Get("Content-Type"))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not receive DATA")
	}
}

func TestSMTPSender_PermanentRejectionIsTextprotoError(t *testing.T) {
	srv := newFakeSMTPServer(t, "550 mailbox unavailable")
	sender := NewSMTPSender(SMTPOptions{Host: "127.0.0.1", Port: srv.port(), From: "noreply@example.com"})

	err := sender.Send(context.Background(), validMessage())
	var tpErr *textproto.Error
	if !errors.As(err, &tpErr) || tpErr.Code != 550 {
		t.Fatalf("expected wrapped *textproto.Error 550, got %v", err)
	}
	if !isPermanentSMTPError(err) {
		t.Error("550 must be classified as permanent")
	}
}

func TestIsInsecureAuth(t *testing.T) {
	cases := []struct {
		name     string
		opts     SMTPOptions
		startTLS bool
		want     bool
	}{
		{"no credentials", SMTPOptions{Host: "smtp.example.com", Port: 587}, false, false},
		{"implicit TLS", SMTPOptions{Host: "smtp.example.com", Port: 465, User: "u"}, false, false},
		{"STARTTLS negotiated", SMTPOptions{Host: "smtp.example.com", Port: 587, User: "u"}, true, false},
		{"localhost plain", SMTPOptions{Host: "localhost", Port: 1025, User: "u"}, false, false},
		{"loopback IP plain", SMTPOptions{Host: "127.0.0.1", Port: 1025, User: "u"}, false, false},
		{"remote plain with credentials", SMTPOptions{Host: "smtp.example.com", Port: 587, User: "u"}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isInsecureAuth(tc.opts, tc.startTLS); got != tc.want {
				t.Fatalf("isInsecureAuth = %v, want %v", got, tc.want)
			}
		})
	}
}
