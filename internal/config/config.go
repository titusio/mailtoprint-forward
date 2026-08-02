// Package config loads and validates all runtime settings from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime settings, loaded from environment variables.
type Config struct {
	// IMAP source mailbox.
	IMAPHost string // e.g. "imap.gmail.com"
	IMAPPort int    // e.g. 993
	IMAPUser string
	IMAPPass string
	Mailbox  string // mailbox to read, default "INBOX"

	// SMTP relay used to send attachments onward.
	SMTPHost string // e.g. "smtp.gmail.com"
	SMTPPort int    // e.g. 587
	SMTPUser string
	SMTPPass string
	From     string // envelope/from address for outgoing mail

	// PrintAddr is the mail-to-print destination that each attachment is sent to.
	PrintAddr string

	// SubjectFilter (Betreff), when non-empty, restricts processing to source
	// messages whose subject contains this string (case-insensitive).
	SubjectFilter string

	// PrintSubject, when non-empty, is used as a fixed subject (Betreff) on every
	// forwarded message. When empty, the attachment filename is used.
	PrintSubject string

	// AllowExt, when non-empty, restricts forwarding to these attachment
	// extensions (lower-case, without dot), e.g. "pdf,jpg,png".
	AllowExt []string

	// MarkSeen marks source messages as \Seen once processed, so they are not
	// picked up again on the next run.
	MarkSeen bool

	// DryRun processes and logs but does not send anything.
	DryRun bool
}

// Load reads configuration from the environment, applies defaults, and validates
// that all required settings are present.
func Load() (*Config, error) {
	c := &Config{
		IMAPHost:      os.Getenv("IMAP_HOST"),
		IMAPUser:      os.Getenv("IMAP_USER"),
		IMAPPass:      os.Getenv("IMAP_PASS"),
		Mailbox:       envDefault("IMAP_MAILBOX", "INBOX"),
		SMTPHost:      os.Getenv("SMTP_HOST"),
		SMTPUser:      os.Getenv("SMTP_USER"),
		SMTPPass:      os.Getenv("SMTP_PASS"),
		From:          os.Getenv("SMTP_FROM"),
		PrintAddr:     os.Getenv("PRINT_ADDR"),
		SubjectFilter: strings.TrimSpace(envFirst("SUBJECT_FILTER", "BETREFF")),
		PrintSubject:  os.Getenv("PRINT_SUBJECT"),
		MarkSeen:      envBool("MARK_SEEN", true),
		DryRun:        envBool("DRY_RUN", false),
	}

	c.IMAPPort = envInt("IMAP_PORT", 993)
	c.SMTPPort = envInt("SMTP_PORT", 587)

	if raw := strings.TrimSpace(os.Getenv("ALLOW_EXT")); raw != "" {
		for _, e := range strings.Split(raw, ",") {
			e = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(e, ".")))
			if e != "" {
				c.AllowExt = append(c.AllowExt, e)
			}
		}
	}

	// SMTP credentials default to the IMAP account when not set separately,
	// which is the common single-account case.
	if c.SMTPHost == "" {
		c.SMTPHost = deriveSMTPHost(c.IMAPHost)
	}
	if c.SMTPUser == "" {
		c.SMTPUser = c.IMAPUser
	}
	if c.SMTPPass == "" {
		c.SMTPPass = c.IMAPPass
	}
	if c.From == "" {
		c.From = c.SMTPUser
	}

	var missing []string
	for _, f := range []struct {
		name string
		val  string
	}{
		{"IMAP_HOST", c.IMAPHost},
		{"IMAP_USER", c.IMAPUser},
		{"IMAP_PASS", c.IMAPPass},
		{"SMTP_HOST", c.SMTPHost},
		{"PRINT_ADDR", c.PrintAddr},
		{"SMTP_FROM", c.From},
	} {
		if strings.TrimSpace(f.val) == "" {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}

	return c, nil
}

// ExtAllowed reports whether an attachment filename passes the AllowExt filter.
func (c *Config) ExtAllowed(filename string) bool {
	if len(c.AllowExt) == 0 {
		return true
	}
	lower := strings.ToLower(filename)
	for _, e := range c.AllowExt {
		if strings.HasSuffix(lower, "."+e) {
			return true
		}
	}
	return false
}

// SubjectMatches reports whether a message subject passes the Betreff filter.
func (c *Config) SubjectMatches(subject string) bool {
	if c.SubjectFilter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(subject), strings.ToLower(c.SubjectFilter))
}

// deriveSMTPHost turns an "imap.example.com" host into "smtp.example.com" as a
// best-effort default when SMTP_HOST is not provided.
func deriveSMTPHost(imapHost string) string {
	if imapHost == "" {
		return ""
	}
	if strings.HasPrefix(imapHost, "imap.") {
		return "smtp." + strings.TrimPrefix(imapHost, "imap.")
	}
	return imapHost
}

// envFirst returns the value of the first environment variable that is set,
// letting a setting be given under any of several names.
func envFirst(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
