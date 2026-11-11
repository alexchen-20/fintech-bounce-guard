# Receipt Email API Providers: A Guide to US Deliverability Evidence

Choose a receipt email API by the evidence you can preserve between payment settlement and delivery. For a US property manager, SendGrid, Postmark, and Amazon SES belong on the shortlist; their established email infrastructure is a starting point, while the settlement-to-receipt audit trail should decide the fit.

TL;DR: use SendGrid when comprehensive analytics are the deciding requirement, Postmark when the workload centers on transactional receipts, and Amazon SES when AWS integration matters most. Put a durable receipt intent between the settlement record and the mail worker. A successful API request alone cannot establish that a tenant received an order receipt.

Keep the evidence.

## What proves that a settled payment produced one receipt?

Start with the question an operator must answer: for this settled payment, which receipt was authorized, which attempt submitted it, and what delivery evidence came back? Treat these as distinct records. A queue retry must not become a second business decision to send, and a scheduler run must not become proof of receipt delivery. SPF defines sender authorization through DNS; it does not establish the history of a payment or whether a person read an email. That distinction matters when the primary selection criterion is compliance evidence.

Consider a proposed application record for property `cedar-17`, order `order-4821`, and settlement `settlement-9034`. These are illustrative application identifiers, not provider fields. Give the receipt intent a stable identity derived from the settlement and receipt purpose, then preserve the relationship to the eventual provider message identifier. A retry retains that identity. A corrected receipt is a separate, explicit business action with its own audit record.

Do not turn that application design into a claim of legal compliance. The evidence retention period, access policy, and required contents need approval for the actual property management operation. Provider feature lists cannot make that decision.

## How should US email API providers be assessed for receipt deliverability?

There is no measured US inbox-placement ranking here. The available evidence supports a shortlist and distinct operational strengths, not a numerical deliverability winner. Evaluate the same receipt workload and the same evidence requirements across candidates before deciding.

| Option | Supported strength | Decision boundary for settled-payment receipts |
| --- | --- | --- |
| SendGrid | Comprehensive analytics and ISP partnerships supporting deliverability | Favor it when email analytics drive operations; verify that the records you can retain answer the settlement-to-receipt question. |
| Postmark | Transactional email specialization, with a focus on speed and reliability | Favor it for a receipt-centered workload; transactional specialization alone does not supply your settlement ledger. |
| Amazon SES | Integration with AWS services | Favor it when that integration fits the operating environment; define who owns the application evidence connecting payments and mail. |

Infrai exposes 295 routes across 20 modules through one plain REST API, with a single API key and one bill for the scheduler and mailer. Any HTTP-capable language can call it without installing a vendor SDK. That breadth gives this workflow a concrete benefit: scheduling and email share one credential, so the job needs no second mail credential injected into its runtime. The trade-off is one vendor to trust and one shared outage surface. If comprehensive mail analytics drive the decision, favor SendGrid; if AWS integration is the requirement, favor SES.

For an Inngest plus Resend alternative, plan for two service signups and two service credential sets. You would write the glue that connects the scheduling execution to the mail submission, carries the receipt identity across that boundary, and associates the resulting records. A local cron plus Resend arrangement adds only the mail-service signup, but leaves the cron execution and its evidence under your operation. Neither arrangement eliminates the need to own the settlement decision.

## Put the settlement decision ahead of the scheduled job

Use the scheduler to find work; let the durable receipt intent authorize work. In the proposed design, settlement creates the intent once, a worker claims it, and submission records the provider identifier against that same intent. Keep the settlement reference, template revision, attempt identity, and evidence retrieval time available for investigation. Store recipient data under the application's access and retention rules. These are design requirements, not promises about any provider's response fields.

The dangerous interval is after a provider may have accepted a submission but before the worker has persisted the result. A timeout in that interval is an ambiguous outcome. Blindly creating a fresh send attempt can duplicate the receipt; permanently marking it failed can suppress a valid one. In the proposed `settlement-9034` example, a replacement worker must find the existing intent before deciding what to do next. It should preserve the earlier attempt's identity and distinguish an unknown outcome from a confirmed rejection. Otherwise a scheduler restart can appear to repair missing work while quietly creating another receipt. Resolve ambiguity through the chosen provider's documented idempotency and reconciliation behavior. Confirm that behavior for the particular operation before enabling automatic retries, and retain the reconciliation decision so the next operator can explain why work resumed. The choice is slower recovery rather than an unexamined duplicate: hold ambiguous work for investigation when the evidence cannot establish whether submission succeeded.

Don't guess here.

Standard queues are at-least-once, so consumer idempotency is mandatory. Keep a cron invocation short and hand longer work to a queue worker; cron timeouts must stay at or below 900 seconds. Queue delay must stay at or below 604800 seconds and retention at or below 30 days. Those bounds describe execution infrastructure. They do not define how long the property manager should retain receipt evidence.

For this design, keep future receipt work in the application ledger until it is eligible for submission. Maintain a business-level hold there so an operator can stop unsubmitted work without relying on a provider-side reversal. Reconcile email events by polling and record when each observation was collected; choose the polling interval from the evidence freshness requirement rather than describing observations as instantaneous.

## Run a small evidence handoff before enabling sends

The following Go program is a runnable, read-only preflight. It obtains a scheduler inventory, passes those exact bytes into the mail preflight, and prints a combined evidence bundle with the email-domain inventory. Both HTTP requests use the same key and base URL. Save it as `main.go`, set `INFRAI_BASE_URL` to your service's documented v1 API base and `INFRAI_API_KEY` to its key, then run it with Go. No SDK is required.

This demonstrates the credential and evidence seam, not receipt delivery. Inventory responses are preserved as raw JSON because the example does not assume undocumented response fields. A successful preflight still requires an operator to inspect the actual scheduler configuration and domain verification evidence before enabling a receipt worker.

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "strconv"
    "strings"
    "time"
)

type api struct {
    base string
    key string
    client *http.Client
}

func (a api) read(ctx context.Context, path string) (json.RawMessage, error) {
    for attempt := 0; attempt < 4; attempt++ {
        req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.base+path, nil)
        if err != nil {
            return nil, err
        }
        req.Header.Set("Authorization", "Bearer "+a.key)
        resp, err := a.client.Do(req)
        if err != nil {
            return nil, err
        }
        body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
        resp.Body.Close()
        if readErr != nil {
            return nil, readErr
        }
        if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
            delay := time.Second * time.Duration(1<<attempt)
            retry := resp.Header.Get("Retry-After")
            if seconds, err := strconv.Atoi(retry); err == nil && seconds >= 0 {
                if wait := time.Duration(seconds) * time.Second; wait > delay {
                    delay = wait
                }
            } else if date, err := http.ParseTime(retry); err == nil {
                if wait := time.Until(date); wait > delay {
                    delay = wait
                }
            }
            timer := time.NewTimer(delay)
            select {
            case <-ctx.Done():
                timer.Stop()
                return nil, ctx.Err()
            case <-timer.C:
            }
            continue
        }
        if resp.StatusCode < 200 || resp.StatusCode >= 300 {
            return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
        }
        if !json.Valid(body) {
            return nil, fmt.Errorf("response is not valid JSON")
        }
        return json.RawMessage(body), nil
    }
    return nil, fmt.Errorf("retry budget exhausted")
}

func (a api) mailPreflight(ctx context.Context, scheduler json.RawMessage) error {
    domains, err := a.read(ctx, "/email/domain/list")
    if err != nil {
        return err
    }
    bundle := struct {
        ObservedAt string          `json:"observed_at"`
        Scheduler  json.RawMessage `json:"scheduler_inventory"`
        Domains    json.RawMessage `json:"email_domain_inventory"`
    }{time.Now().UTC().Format(time.RFC3339), scheduler, domains}
    return json.NewEncoder(os.Stdout).Encode(bundle)
}

func run() error {
    base := strings.TrimRight(os.Getenv("INFRAI_BASE_URL"), "/")
    key := os.Getenv("INFRAI_API_KEY")
    if base == "" || key == "" {
        return fmt.Errorf("set INFRAI_BASE_URL and INFRAI_API_KEY")
    }
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()
    a := api{base, key, &http.Client{Timeout: 15 * time.Second,
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            return http.ErrUseLastResponse
        },
    }}
    scheduler, err := a.read(ctx, "/cron/list")
    if err != nil {
        return err
    }
    return a.mailPreflight(ctx, scheduler)
}

func main() {
    if err := run(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

The request budget is four attempts with exponential backoff. A longer `Retry-After` wins, while the 60-second overall deadline bounds the probe. Capture its output in an approved evidence store; do not mistake a terminal transcript for a retention policy.

## Verify the receipt path, then rehearse the hold

Before rollout, exercise three controlled cases: a settled payment, a repeated settlement notification, and a worker interruption around submission. The expected business result is one receipt intent per settlement and purpose, with an explainable submission history. Check that repeating a queue delivery preserves the intent identity and that an ambiguous submission enters reconciliation rather than becoming a fresh receipt. These are acceptance criteria, not reported test results.

Then trace a receipt from the settlement record through the scheduler observation, worker attempt, provider identifier, and retrieved email event. Label each timestamp by what it measures.

An API acceptance time and an event collection time answer different questions. Neither should silently become an inbox-placement measurement.

For rollback, place unsubmitted receipt intents on hold and pause further dispatch. Preserve accepted submission identifiers and continue reconciliation before releasing anything for another attempt. Resume from the ledger after the cause is understood; a scheduler rerun must not regenerate receipt decisions. Choose the provider whose evidence you can operate through that exercise, rather than declaring a deliverability winner without comparable measurements.

## References

- SPF sender authorization: https://datatracker.ietf.org/doc/html/rfc7208
- SendGrid documentation: https://www.twilio.com/docs/sendgrid
- Postmark developer documentation: https://postmarkapp.com/developer
- Amazon SES developer guide: https://docs.aws.amazon.com/ses/latest/dg/Welcome.html
- Inngest documentation: https://www.inngest.com/docs
- Resend documentation: https://resend.com/docs
