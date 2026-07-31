package main

import (
	"bytes"
	"fmt"
	"io"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"

	// Register additional charset decoders so non-UTF-8 messages parse.
	_ "github.com/emersion/go-message/charset"
)

// SourceMessage is a fetched message together with the attachments pulled from it.
type SourceMessage struct {
	UID         imap.UID
	Subject     string
	From        string
	Attachments []Attachment
}

// IMAPClient wraps an authenticated connection to the source mailbox.
type IMAPClient struct {
	c   *imapclient.Client
	cfg *Config
}

// dialIMAP connects, authenticates and selects the configured mailbox.
func dialIMAP(cfg *Config) (*IMAPClient, error) {
	addr := fmt.Sprintf("%s:%d", cfg.IMAPHost, cfg.IMAPPort)
	// A non-nil Options is required: DialTLS -> Options.dialer() dereferences the
	// receiver without a nil check (go-imap v2.0.0-beta.6).
	c, err := imapclient.DialTLS(addr, &imapclient.Options{})
	if err != nil {
		return nil, fmt.Errorf("imap dial %s: %w", addr, err)
	}

	if err := c.Login(cfg.IMAPUser, cfg.IMAPPass).Wait(); err != nil {
		c.Close()
		return nil, fmt.Errorf("imap login: %w", err)
	}

	if _, err := c.Select(cfg.Mailbox, nil).Wait(); err != nil {
		c.Close()
		return nil, fmt.Errorf("select mailbox %q: %w", cfg.Mailbox, err)
	}

	return &IMAPClient{c: c, cfg: cfg}, nil
}

func (ic *IMAPClient) Close() error {
	ic.c.Logout().Wait()
	return ic.c.Close()
}

// FetchUnseen returns all unseen messages in the mailbox with their attachments.
func (ic *IMAPClient) FetchUnseen() ([]SourceMessage, error) {
	criteria := &imap.SearchCriteria{
		NotFlag: []imap.Flag{imap.FlagSeen},
	}
	// Narrow the search server-side to the configured Betreff. IMAP SUBJECT
	// search is a case-insensitive substring match (RFC 3501).
	if ic.cfg.SubjectFilter != "" {
		criteria.Header = append(criteria.Header, imap.SearchCriteriaHeaderField{
			Key:   "Subject",
			Value: ic.cfg.SubjectFilter,
		})
	}

	searchData, err := ic.c.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("search unseen: %w", err)
	}

	uids := searchData.AllUIDs()
	if len(uids) == 0 {
		return nil, nil
	}

	seqSet := imap.UIDSetNum(uids...)
	fetchOpts := &imap.FetchOptions{
		Envelope:    true,
		BodySection: []*imap.FetchItemBodySection{{}}, // entire raw message
	}

	buffers, err := ic.c.Fetch(seqSet, fetchOpts).Collect()
	if err != nil {
		return nil, fmt.Errorf("fetch messages: %w", err)
	}

	var out []SourceMessage
	for _, buf := range buffers {
		raw := firstBodySection(buf)
		if raw == nil {
			continue
		}

		msg := SourceMessage{UID: buf.UID}
		if buf.Envelope != nil {
			msg.Subject = buf.Envelope.Subject
			if len(buf.Envelope.From) > 0 {
				msg.From = buf.Envelope.From[0].Addr()
			}
		}

		// Guard against servers whose SUBJECT search is looser than a plain
		// substring match: skip anything that doesn't actually match the Betreff.
		if !ic.cfg.subjectMatches(msg.Subject) {
			continue
		}

		atts, err := extractAttachments(raw, ic.cfg)
		if err != nil {
			return out, fmt.Errorf("parse message uid %d: %w", buf.UID, err)
		}
		msg.Attachments = atts

		out = append(out, msg)
	}

	return out, nil
}

// MarkSeen flags a message as read so it is skipped on subsequent runs.
func (ic *IMAPClient) MarkSeen(uid imap.UID) error {
	// STORE responses arrive as FETCH data; Close waits for completion and we
	// don't need the returned flags.
	return ic.c.Store(imap.UIDSetNum(uid), &imap.StoreFlags{
		Op:    imap.StoreFlagsAdd,
		Flags: []imap.Flag{imap.FlagSeen},
	}, nil).Close()
}

func firstBodySection(buf *imapclient.FetchMessageBuffer) []byte {
	for _, bs := range buf.BodySection {
		return bs.Bytes
	}
	return nil
}

// extractAttachments walks a raw RFC 5322 message and returns its attachment parts.
func extractAttachments(raw []byte, cfg *Config) ([]Attachment, error) {
	mr, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}

	var atts []Attachment
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return atts, err
		}

		ah, ok := part.Header.(*mail.AttachmentHeader)
		if !ok {
			continue
		}

		filename, _ := ah.Filename()
		if filename == "" {
			continue
		}
		if !cfg.extAllowed(filename) {
			continue
		}

		data, err := io.ReadAll(part.Body)
		if err != nil {
			return atts, fmt.Errorf("read attachment %q: %w", filename, err)
		}

		ct, _, _ := ah.ContentType()
		atts = append(atts, Attachment{
			Filename:    filename,
			ContentType: ct,
			Data:        data,
		})
	}

	return atts, nil
}
