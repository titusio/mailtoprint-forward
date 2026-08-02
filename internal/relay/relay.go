// Package relay builds forwarded messages and delivers them to the mail-to-print
// destination over SMTP.
package relay

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/smtp"
	"time"

	"github.com/emersion/go-message/mail"

	"github.com/titus/mailtoprint-forward/internal/config"
	"github.com/titus/mailtoprint-forward/internal/mailmsg"
)

// Build assembles an RFC 5322 message carrying a single attachment, addressed to
// the mail-to-print destination.
func Build(cfg *config.Config, src mailmsg.SourceMessage, att mailmsg.Attachment) ([]byte, error) {
	var buf bytes.Buffer

	var h mail.Header
	h.SetDate(time.Now())
	h.SetAddressList("From", []*mail.Address{{Address: cfg.From}})
	h.SetAddressList("To", []*mail.Address{{Address: cfg.PrintAddr}})

	// Use the fixed configured Betreff for every forwarded message; fall back to
	// the attachment filename when none is set.
	subject := cfg.PrintSubject
	if subject == "" {
		subject = att.Filename
	}
	h.SetSubject(subject)

	mw, err := mail.CreateWriter(&buf, h)
	if err != nil {
		return nil, err
	}

	// Short text body describing the origin of the forwarded attachment.
	tw, err := mw.CreateInline()
	if err != nil {
		return nil, err
	}
	var th mail.InlineHeader
	th.SetContentType("text/plain", map[string]string{"charset": "utf-8"})
	pw, err := tw.CreatePart(th)
	if err != nil {
		return nil, err
	}
	body := fmt.Sprintf("Forwarded attachment %q from message %q (%s).\n",
		att.Filename, src.Subject, src.From)
	if _, err := io.WriteString(pw, body); err != nil {
		return nil, err
	}
	pw.Close()
	tw.Close()

	// The attachment itself.
	var ah mail.AttachmentHeader
	ah.SetContentType(att.ContentType, nil)
	ah.SetFilename(att.Filename)
	aw, err := mw.CreateAttachment(ah)
	if err != nil {
		return nil, err
	}
	if _, err := aw.Write(att.Data); err != nil {
		return nil, err
	}
	aw.Close()

	if err := mw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Send delivers a pre-built message via the configured SMTP relay, handling both
// STARTTLS (typically port 587) and implicit TLS (port 465).
func Send(cfg *config.Config, msg []byte) error {
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)

	if cfg.SMTPPort == 465 {
		return sendImplicitTLS(cfg, addr, auth, msg)
	}
	return smtp.SendMail(addr, auth, cfg.From, []string{cfg.PrintAddr}, msg)
}

func sendImplicitTLS(cfg *config.Config, addr string, auth smtp.Auth, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: cfg.SMTPHost})
	if err != nil {
		return fmt.Errorf("tls dial %s: %w", addr, err)
	}

	c, err := smtp.NewClient(conn, cfg.SMTPHost)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := c.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := c.Mail(cfg.From); err != nil {
		return err
	}
	if err := c.Rcpt(cfg.PrintAddr); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
