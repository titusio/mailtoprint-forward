package main

// Attachment is a single decoded attachment extracted from a source message.
type Attachment struct {
	Filename    string // as declared by the sender, may be empty
	ContentType string // MIME type, e.g. "application/pdf"
	Data        []byte
}
