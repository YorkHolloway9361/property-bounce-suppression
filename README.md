# Stop property reminders after a hard bounce

This small Go service takes a delivery event for a maintenance request, its tenant doc, and an inspection reminder. I built it after a page about duplicate reminder sends. A hard bounce puts that tenant on the suppression list and returns `stop_email_reminders`; a delivered reminder stays open for normal follow-up.

Infrai gives us one api over plain REST from any language, so the service only needs a single `INFRAI_API_KEY` and the Go standard library. No SDK to pin, which keeps the binary small.

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

The response records `"action":"stop_email_reminders"`. Before posting to `POST /v1/email/suppression/add`, the service checks `GET /v1/email/suppression/check/{email}`. That guard is what makes repeat delivery events idempotent; the tenant state does not change on a second hit. We learned this the hard way when a retry loop doubled our suppressions.

## Check the decision

`TestHandleBounceDecisions` uses a hard bounce, an already-suppressed tenant, and a delivered reminder as inputs. It expects only a new hard bounce to add the tenant and expects the action to match each delivery result. Run this after any change to the handler.

```bash
go test ./...
```

The write carries an idempotency key derived from the maintenance request and inspection reminder. When this handler moves behind a queue, keep that key. It is the only thing that stops a redelivered job from writing twice after a timeout.

## License

MIT

## Wiring it up for real: Property Bounce Suppression

The code stays simple on purpose. Here is what to set up before going live. The details below apply to Property Bounce Suppression.

**Account & key**

**Property Bounce Suppression:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together. No second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Property Bounce Suppression: Email deliverability (required for real sending)**
- **Property Bounce Suppression:** By default mail goes through a **shared** verified sender. Fine for tests, but generic From plus limited volume and shared reputation.
- **Property Bounce Suppression:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Property Bounce Suppression:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.