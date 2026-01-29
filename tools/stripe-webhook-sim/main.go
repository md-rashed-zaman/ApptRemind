package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v79/webhook"
)

func main() {
	var (
		baseURL   = flag.String("base-url", getenv("BASE_URL", "http://localhost:8080"), "gateway base url")
		evtType   = flag.String("type", getenv("STRIPE_EVENT_TYPE", "checkout.session.completed"), "stripe event type")
		business  = flag.String("business-id", getenv("BUSINESS_ID", ""), "business_id metadata")
		tier      = flag.String("tier", getenv("TIER", "starter"), "tier metadata")
		secret    = flag.String("secret", getenv("STRIPE_WEBHOOK_SECRET", ""), "stripe webhook signing secret (whsec_...)")
	)
	flag.Parse()

	if strings.TrimSpace(*secret) == "" {
		fatal("STRIPE_WEBHOOK_SECRET is required")
	}
	if strings.TrimSpace(*business) == "" {
		fatal("BUSINESS_ID is required")
	}

	now := time.Now().UTC()
	eventID := fmt.Sprintf("evt_test_%d", now.UnixNano())

	payload, err := buildEventJSON(eventID, *evtType, now, *business, *tier)
	if err != nil {
		fatal(err.Error())
	}

	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   payload,
		Secret:    *secret,
		Timestamp: now,
		Scheme:    "v1",
	})

	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(*baseURL, "/")+"/api/v1/billing/webhooks/stripe", bytes.NewReader(payload))
	if err != nil {
		fatal(err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", signed.Header)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()

	fmt.Printf("status=%d\n", resp.StatusCode)
}

func buildEventJSON(eventID, eventType string, t time.Time, businessID, tier string) ([]byte, error) {
	created := t.Unix()
	switch eventType {
	case "checkout.session.completed":
		return json.Marshal(map[string]any{
			"id":         eventID,
			"object":     "event",
			"created":    created,
			"type":       eventType,
			"api_version": "2020-08-27",
			"data": map[string]any{
				"object": map[string]any{
					"id":     "cs_test_123",
					"object": "checkout.session",
					"metadata": map[string]any{
						"business_id": businessID,
						"tier":        tier,
					},
				},
			},
		})
	case "customer.subscription.updated", "customer.subscription.created", "customer.subscription.deleted":
		// Only metadata is used by our MVP handler.
		return json.Marshal(map[string]any{
			"id":         eventID,
			"object":     "event",
			"created":    created,
			"type":       eventType,
			"api_version": "2020-08-27",
			"data": map[string]any{
				"object": map[string]any{
					"id":     "sub_test_123",
					"object": "subscription",
					"status": "active",
					"metadata": map[string]any{
						"business_id": businessID,
						"tier":        tier,
					},
				},
			},
		})
	default:
		return nil, fmt.Errorf("unsupported event type: %s", eventType)
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(2)
}

