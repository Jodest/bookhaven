package mailer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
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
)

// defaultSMTPTimeout bounds the whole SMTP conversation (dial + commands +
// DATA). The effective deadline is the earlier of this and ctx's deadline.
const defaultSMTPTimeout = 30 * time.Second

// implicitTLSPort is the SMTPS port: connect with TLS from the first byte.
// Any other port connects in plain text and upgrades via STARTTLS when the
// server advertises it.
const implicitTLSPort = 465

// SMTPOptions configures an SMTPSender.
type SMTPOptions struct {
	Host     string
	Port     int
	User     string // empty -> no AUTH (e.g. Mailpit)
	Password string
	From     string // RFC 5322 address, e.g. "BookHaven <noreply@example.com>"
}

// SMTPSender delivers mail through an SMTP submission server using the
// stdlib net/smtp client.
type SMTPSender struct {
	opts    SMTPOptions
	timeout time.Duration
}

// NewSMTPSender builds an SMTPSender.
func NewSMTPSender(opts SMTPOptions) *SMTPSender {
	return &SMTPSender{opts: opts, timeout: defaultSMTPTimeout}
}

// Send delivers msg. SMTP replies are returned wrapped (errors.As works for
// *textproto.Error) so the task handler can tell permanent 5xx rejections
// from transient failures. Errors never include the message body.
func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	if err := msg.Validate(); err != nil {
		return err
	}
	from, err := mail.ParseAddress(s.opts.From)
	if err != nil {
		return fmt.Errorf("mailer: invalid from address: %w", err)
	}
	to, err := mail.ParseAddress(msg.To)
	if err != nil {
		return ErrInvalidRecipient
	}

	deadline := time.Now().Add(s.timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	tlsConfig := &tls.Config{ServerName: s.opts.Host, MinVersion: tls.VersionTLS12}
	addr := net.JoinHostPort(s.opts.Host, strconv.Itoa(s.opts.Port))
	netDialer := &net.Dialer{}

	var conn net.Conn
	if s.opts.Port == implicitTLSPort {
		conn, err = (&tls.Dialer{NetDialer: netDialer, Config: tlsConfig}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = netDialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("mailer: dial %s: %w", addr, err)
	}
	// Bound every subsequent read/write, and abort promptly if ctx is
	// cancelled (worker shutdown / task timeout).
	_ = conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	client, err := smtp.NewClient(conn, s.opts.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("mailer: smtp greeting: %w", err)
	}
	defer client.Close()

	if s.opts.Port != implicitTLSPort {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsConfig); err != nil {
				return fmt.Errorf("mailer: starttls: %w", err)
			}
		}
	}

	if s.opts.User != "" {
		// smtp.PlainAuth refuses to send credentials over an unencrypted
		// connection to a non-localhost server - intended.
		auth := smtp.PlainAuth("", s.opts.User, s.opts.Password, s.opts.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("mailer: auth: %w", err)
		}
	}

	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("mailer: mail from: %w", err)
	}
	if err := client.Rcpt(to.Address); err != nil {
		return fmt.Errorf("mailer: rcpt to: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("mailer: data: %w", err)
	}
	if _, err := w.Write(buildMessage(from.String(), msg, time.Now(), newMessageID(from.Address))); err != nil {
		_ = w.Close()
		return fmt.Errorf("mailer: write message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mailer: end data: %w", err)
	}
	// The message has been accepted at this point; a failing QUIT must not
	// trigger a retry (which would send a duplicate).
	_ = client.Quit()
	return nil
}

// newMessageID returns a unique Message-ID value ("<random@domain>") using
// the sender address's domain.
func newMessageID(fromAddress string) string {
	domain := "bookhaven.local"
	if i := strings.LastIndex(fromAddress, "@"); i >= 0 && i < len(fromAddress)-1 {
		domain = fromAddress[i+1:]
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failing is effectively impossible; fall back to time.
		return fmt.Sprintf("<%d@%s>", time.Now().UnixNano(), domain)
	}
	return "<" + hex.EncodeToString(buf) + "@" + domain + ">"
}

// buildMessage renders msg as an RFC 5322 / MIME message with CRLF line
// endings. from is an already-formatted From header value. The body is
// text/plain when HTMLBody is empty, otherwise multipart/alternative with
// the text part first and the HTML part second. Both parts are UTF-8,
// quoted-printable; the Subject is RFC 2047 Q-encoded when non-ASCII.
func buildMessage(from string, msg Message, now time.Time, messageID string) []byte {
	to := msg.To
	if addr, err := mail.ParseAddress(msg.To); err == nil {
		to = addr.String()
	}

	var buf bytes.Buffer
	writeHeader := func(key, value string) {
		buf.WriteString(key)
		buf.WriteString(": ")
		buf.WriteString(value)
		buf.WriteString("\r\n")
	}

	writeHeader("From", from)
	writeHeader("To", to)
	writeHeader("Subject", mime.QEncoding.Encode("UTF-8", msg.Subject))
	writeHeader("Date", now.Format(time.RFC1123Z))
	writeHeader("Message-ID", messageID)
	writeHeader("MIME-Version", "1.0")

	if msg.HTMLBody == "" {
		writeHeader("Content-Type", "text/plain; charset=UTF-8")
		writeHeader("Content-Transfer-Encoding", "quoted-printable")
		buf.WriteString("\r\n")
		writeQuotedPrintable(&buf, msg.TextBody)
		return buf.Bytes()
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	writeHeader("Content-Type", "multipart/alternative; boundary="+mw.Boundary())
	buf.WriteString("\r\n")

	for _, part := range []struct {
		contentType string
		content     string
	}{
		{"text/plain; charset=UTF-8", msg.TextBody},
		{"text/html; charset=UTF-8", msg.HTMLBody},
	} {
		pw, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.contentType},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			// Writing to a bytes.Buffer cannot fail.
			panic(err)
		}
		writeQuotedPrintable(pw, part.content)
	}
	_ = mw.Close()

	buf.Write(body.Bytes())
	return buf.Bytes()
}

// writeQuotedPrintable writes s quoted-printable encoded to w; line breaks
// in s become CRLF.
func writeQuotedPrintable(w io.Writer, s string) {
	qp := quotedprintable.NewWriter(w)
	_, _ = qp.Write([]byte(s))
	_ = qp.Close()
}
