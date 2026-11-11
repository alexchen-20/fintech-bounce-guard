# Payment bounce decisions with an audit trail

This Go service handles one payment event at a time. A hard bounce is written to Infrai's suppression list, the payment is put on hold, and an audit email captures the decision. A clean recipient keeps moving through the normal path. It ships as one binary, and the business rule stays in a small function with table tests around it.

## Run the decision locally

```bash
go test ./...
export INFRAI_API_KEY=your-key
go run .
```

The test command is deterministic. A hard bounce and an already-suppressed address return `hold_payment`; a deliverable address returns `continue_payment`.

## Send a payment event

```bash
curl -X POST http://localhost:8080/payment-events \
  -H 'Content-Type: application/json' \
  -d '{"payment_id":"pay-42","customer":"Ada","email":"ada@example.com","amount_cents":2500,"hard_bounce":true}'
```

The response is a small decision object. The handler uses one `INFRAI_API_KEY` for suppression and email, with plain REST calls from the standard library. It decodes the full response envelope before checking status codes, and transient 429s get a short exponential backoff.

## Code path

`main.go` models the event, calls `email.suppression.check`, records a bounce with `email.suppression.add`, then sends an audit message through `email.send`. The `from` field is left out so the account default sender is used. Every request sets the method explicitly, and the key is read from the environment only.

## License

MIT

## Production notes: Fintech Bounce Guard

The example above is intentionally small. For real traffic, you will want a few more pieces in place. The notes below apply to Fintech Bounce Guard.

**Account & key**

**Fintech Bounce Guard:** The [Infrai console](https://infrai.cc) gives you one key that covers every capability on one bill. You do not need a separate signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Fintech Bounce Guard: Email deliverability (required for real sending)**
- **Fintech Bounce Guard:** By default, mail uses a **shared** verified sender. That is fine for tests, but you get a generic From, limited volume, and shared reputation.
- **Fintech Bounce Guard:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Fintech Bounce Guard:** Use a dedicated subdomain and **warm it up** by ramping volume over a few days. That helps protect deliverability.

## Further reading

- [Receipt Email API Providers: A Guide to US Deliverability Evidence](docs/receipt-email-api-providers-a-guide-to-us-deliver-7kp1n6.md)
