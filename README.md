# mailtoprint-forward

A small Go tool that reads unseen mail from an IMAP mailbox and forwards **each
attachment as its own message** to a mail-to-print address (e.g. HP ePrint,
Canon PIXMA Cloud, or any printer that accepts email).

Each run:

1. Connects to IMAP over TLS and selects the mailbox (default `INBOX`).
2. Finds all **unseen** messages, optionally only those matching a subject
   (Betreff) filter.
3. Extracts every attachment (optionally filtered by extension).
4. Sends each attachment in a separate email to `PRINT_ADDR` via SMTP, using a
   fixed subject (Betreff) when `PRINT_SUBJECT` is set.
5. Marks the source message `\Seen` once all its attachments were forwarded, so
   it isn't processed again (a partial failure is retried next run).

## Development

Go is provided by the Nix flake:

```sh
nix develop            # drops you into a shell with go, gopls, etc.
go build ./...
```

With [direnv](https://direnv.net/) + `nix-direnv`, `cd` into the repo and the
shell (plus a local `.env`) loads automatically.

## Configuration

All configuration is via environment variables. Copy `.env.example` to `.env`
and fill it in.

| Variable       | Required | Default              | Description                                        |
| -------------- | -------- | -------------------- | -------------------------------------------------- |
| `IMAP_HOST`    | yes      |                      | IMAP server hostname                               |
| `IMAP_PORT`    | no       | `993`                | IMAP TLS port                                      |
| `IMAP_USER`    | yes      |                      | IMAP username                                      |
| `IMAP_PASS`    | yes      |                      | IMAP password / app password                       |
| `IMAP_MAILBOX` | no       | `INBOX`              | Mailbox to read                                    |
| `SMTP_HOST`    | no\*     | derived from IMAP    | SMTP server hostname (`imap.` → `smtp.`)           |
| `SMTP_PORT`    | no       | `587`                | `587` = STARTTLS, `465` = implicit TLS             |
| `SMTP_USER`    | no       | `IMAP_USER`          | SMTP username                                      |
| `SMTP_PASS`    | no       | `IMAP_PASS`          | SMTP password                                      |
| `SMTP_FROM`    | no       | `SMTP_USER`          | From address for outgoing mail                     |
| `PRINT_ADDR`   | yes      |                      | Mail-to-print destination address                  |
| `SUBJECT_FILTER` | no     | *(all)*              | Only process messages whose subject (Betreff) contains this string |
| `PRINT_SUBJECT` | no      | *(filename)*         | Fixed subject (Betreff) set on every forwarded message |
| `ALLOW_EXT`    | no       | *(all)*              | Comma-separated extensions, e.g. `pdf,jpg,png`     |
| `MARK_SEEN`    | no       | `true`               | Mark source messages `\Seen` after forwarding      |
| `DRY_RUN`      | no       | `false`              | Log actions without sending or marking anything    |

\* `SMTP_HOST` is only optional when `IMAP_HOST` starts with `imap.`.

> **Gmail / Outlook:** use an [app password](https://support.google.com/accounts/answer/185833),
> not your account password, and keep 2FA enabled.

## Run

```sh
# Preview what would be sent, changing nothing:
DRY_RUN=true go run .

# Forward for real:
go run .

# Or build a static binary:
go build -o mtpf .
./mtpf
```

Run it on a schedule (cron, systemd timer, launchd) to poll continuously — for
example every 5 minutes:

```cron
*/5 * * * * cd /path/to/mailtoprint-forward && /path/to/mtpf >> /var/log/mtpf.log 2>&1
```

The command exits non-zero if any attachment failed to send.
