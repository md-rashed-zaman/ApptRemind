package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type Sender interface {
	Send(ctx context.Context, to string, body string) error
	ProviderID() string
}

type WebhookSender struct {
	url   string
	token string
	http  *http.Client
}

func NewWebhookSender(url string, token string) *WebhookSender {
	return &WebhookSender{
		url:   strings.TrimSpace(url),
		token: strings.TrimSpace(token),
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (s *WebhookSender) ProviderID() string {
	return "sms-webhook"
}

func (s *WebhookSender) Send(ctx context.Context, to string, body string) error {
	if s.url == "" {
		return errors.New("sms webhook url not configured")
	}
	payload := map[string]string{
		"to":   to,
		"body": body,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("sms webhook returned non-2xx")
	}
	return nil
}

type NoopSender struct{}

func NewNoopSender() *NoopSender {
	return &NoopSender{}
}

func (s *NoopSender) ProviderID() string {
	return "sms-noop"
}

func (s *NoopSender) Send(_ context.Context, _ string, _ string) error {
	return nil
}
