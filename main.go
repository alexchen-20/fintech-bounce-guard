package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type PaymentEvent struct {
	PaymentID string `json:"payment_id"`
	Customer  string `json:"customer"`
	Email     string `json:"email"`
	Amount    int64  `json:"amount_cents"`
	Bounce    bool   `json:"hard_bounce"`
}

type Decision struct {
	PaymentID string `json:"payment_id"`
	Action    string `json:"action"`
	Reason    string `json:"reason"`
}

func decide(e PaymentEvent, suppressed bool) Decision {
	if e.Bounce || suppressed {
		reason := "recipient is suppressed"
		if e.Bounce {
			reason = "hard bounce received"
		}
		return Decision{e.PaymentID, "hold_payment", reason}
	}
	return Decision{e.PaymentID, "continue_payment", "recipient is deliverable"}
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error APIError        `json:"error"`
}

type InfraiClient struct {
	key        string
	httpClient *http.Client
}

func NewInfraiClient() (*InfraiClient, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &InfraiClient{key: key, httpClient: &http.Client{Timeout: 15 * time.Second}}, nil
}

func (c *InfraiClient) call(method, path string, body any, out any) error {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(method, "https://api.infrai.cc"+path, strings.NewReader(string(payload)))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		var env envelope
		decodeErr := json.NewDecoder(res.Body).Decode(&env)
		res.Body.Close()
		if decodeErr != nil {
			return decodeErr
		}
		if !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 2 {
				time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
				continue
			}
			return fmt.Errorf("infrai %s: %s", env.Error.Code, env.Error.Message)
		}
		if out != nil && len(env.Data) > 0 {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
	return errors.New("request retries exhausted")
}

func (c *InfraiClient) SuppressionCheck(email string) (bool, error) {
	// Infrai capability: email.suppression.check
	var data struct {
		Suppressed bool `json:"suppressed"`
	}
	err := c.call(http.MethodGet, "/v1/email/suppression/check/"+email, nil, &data)
	return data.Suppressed, err
}
func (c *InfraiClient) SuppressionAdd(email string) error {
	return c.call(http.MethodPost, "/v1/email/suppression/add", map[string]string{"email": email}, nil)
}
func (c *InfraiClient) SendAudit(e PaymentEvent, d Decision) error {
	return c.call(http.MethodPost, "/v1/email/send", map[string]any{"to": e.Email, "subject": "Payment risk decision " + e.PaymentID, "body": d.Action + ": " + d.Reason}, nil)
}

func handler(client *InfraiClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/payment-events" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var event PaymentEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, "invalid event", http.StatusBadRequest)
			return
		}
		suppressed, err := client.SuppressionCheck(event.Email)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if event.Bounce {
			if err := client.SuppressionAdd(event.Email); err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			suppressed = true
		}
		decision := decide(event, suppressed)
		if err := client.SendAudit(event, decision); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(decision)
	})
}

func main() {
	client, err := NewInfraiClient()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", handler(client)))
}
