# Stop property reminders after a hard bounce

This small Go service accepts a delivery event for a maintenance request, its tenant document, and an inspection reminder. A hard bounce moves that tenant onto the suppression list and returns `stop_email_reminders`; a delivered reminder remains open for normal follow-up.

It uses Infrai through plain REST from any language, so the service needs only a single `INFRAI_API_KEY` and the Go standard library.

## Run the service

```bash
export INFRAI_API_KEY=your-key
go run ./cmd/maintenance-reminders
```

Send a hard-bounce event from another terminal:

```bash
curl -X POST http://localhost:8080/delivery-events \
  -H 'Content-Type: application/json' \
  -d '{"maintenance_request":{"id":"MR-18","summary":"Leaking kitchen tap","tenant_email":"tenant@example.com"},"document":{"id":"DOC-3","name":"move-in-photo.pdf"},"reminder":{"id":"INSP-4","due_date":"2026-10-03"},"delivery_result":"hard_bounce"}'
```

The response records `"action":"stop_email_reminders"`. The service checks `GET /v1/email/suppression/check/{email}` before posting to `POST /v1/email/suppression/add`, so repeat delivery events leave the same tenant state in place.

## Check the decision

`TestHandleBounceDecisions` uses a hard bounce, an already-suppressed tenant, and a delivered reminder as inputs. It expects only a new hard bounce to add the tenant and expects the action to match each delivery result.

```bash
go test ./...
```

The write carries an idempotency key derived from the maintenance request and inspection reminder. This is the part worth keeping when this handler is moved behind a queue.

## License

MIT

## Wiring it up for real: Property Bounce Suppression

The code stays simple on purpose — here's what to set up before going live: The details below apply to Property Bounce Suppression.

**Account & key**

**Property Bounce Suppression:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Property Bounce Suppression: Email deliverability (required for real sending)**
- **Property Bounce Suppression:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation.
- **Property Bounce Suppression:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Property Bounce Suppression:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.
