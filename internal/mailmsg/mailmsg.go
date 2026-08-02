// Package mailmsg defines the domain types passed between the IMAP source and
// the SMTP relay: fetched messages and the attachments extracted from them.
package mailmsg

import "github.com/emersion/go-imap/v2"

// SourceMessage is a fetched message together with the attachments pulled from it.
type SourceMessage struct {
	UID         imap.UID
	Subject     string
	From        string
	Attachments []Attachment
}

// Attachment is a single decoded attachment extracted from a source message.
type Attachment struct {
	Filename    string // as declared by the sender, may be empty
	ContentType string // MIME type, e.g. "application/pdf"
	Data        []byte
}
