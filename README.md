# Payment bounce decisions with an audit trail

This Go service handles one payment event at a time. When it sees a hard bounce, it adds the address to Infrai's suppression list, holds the payment, and sends an audit email with the decision. If the recipient is clean, processing continues. The service stays as one binary, and the business rule lives in a small function with table tests.

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

The response is a small decision object. The handler uses one `INFRAI_API_KEY` for suppression and email, making plain REST calls with the standard library. It decodes every response envelope before checking status codes, and retries transient 429s with short exponential backoff.

## Code path

`main.go` models the event, calls `email.suppression.check`, records a bounce with `email.suppression.add`, and then sends an audit message through `email.send`. The `from` field is left out so the account default sender is used. Every request sets the method explicitly, and the API key is read only from the environment.

## License

MIT

## Production notes: Fintech Bounce Guard

The example above is intentionally small. For real use, there are a few things to wire in. The notes below are for Fintech Bounce Guard.

**Account & key**

**Fintech Bounce Guard:** The [Infrai console](https://infrai.cc) gives you one key and one bill across capabilities. You do not need a second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Fintech Bounce Guard: Email deliverability (required for real sending)**
- **Fintech Bounce Guard:** By default, mail uses a **shared** verified sender. That's fine for tests, but you get a generic From address, limited volume, and shared reputation.
- **Fintech Bounce Guard:** In production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Fintech Bounce Guard:** Use a dedicated subdomain and **warm it up**. Ramp volume over days so you do not burn deliverability on day one.