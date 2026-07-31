// Command mailtoprint-forward reads mail from an IMAP mailbox and forwards each
// attachment as its own message to a mail-to-print address.
package main

import (
	"log"
	"os"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("mailtoprint-forward: ")

	loadDotEnv()

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	if err := run(cfg); err != nil {
		log.Fatalf("%v", err)
	}
}

func run(cfg *Config) error {
	ic, err := dialIMAP(cfg)
	if err != nil {
		return err
	}
	defer ic.Close()

	messages, err := ic.FetchUnseen()
	if err != nil {
		return err
	}

	if len(messages) == 0 {
		log.Printf("no unseen messages in %q", cfg.Mailbox)
		return nil
	}

	var sent, failed int
	for _, msg := range messages {
		if len(msg.Attachments) == 0 {
			log.Printf("uid %d %q: no matching attachments", msg.UID, msg.Subject)
			if cfg.MarkSeen && !cfg.DryRun {
				if err := ic.MarkSeen(msg.UID); err != nil {
					log.Printf("uid %d: mark seen failed: %v", msg.UID, err)
				}
			}
			continue
		}

		allOK := true
		for _, att := range msg.Attachments {
			if cfg.DryRun {
				log.Printf("[dry-run] would send %q (%d bytes) -> %s",
					att.Filename, len(att.Data), cfg.PrintAddr)
				sent++
				continue
			}

			raw, err := buildMessage(cfg, msg, att)
			if err != nil {
				log.Printf("uid %d: build %q failed: %v", msg.UID, att.Filename, err)
				failed++
				allOK = false
				continue
			}
			if err := sendMessage(cfg, raw); err != nil {
				log.Printf("uid %d: send %q failed: %v", msg.UID, att.Filename, err)
				failed++
				allOK = false
				continue
			}
			log.Printf("sent %q (%d bytes) -> %s", att.Filename, len(att.Data), cfg.PrintAddr)
			sent++
		}

		// Only mark the source message seen once every attachment was forwarded,
		// so a partial failure is retried on the next run.
		if allOK && cfg.MarkSeen && !cfg.DryRun {
			if err := ic.MarkSeen(msg.UID); err != nil {
				log.Printf("uid %d: mark seen failed: %v", msg.UID, err)
			}
		}
	}

	log.Printf("done: %d attachment(s) sent, %d failed", sent, failed)
	if failed > 0 {
		os.Exit(1)
	}
	return nil
}
