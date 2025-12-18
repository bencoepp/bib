package breakglass

import (
	"context"
	"encoding/json"
	"fmt"
)

// WebhookNotifier sends notifications via webhook.
// This is currently a stub implementation.
type WebhookNotifier struct {
	webhookURL string
}

// NewWebhookNotifier creates a new webhook notifier.
func NewWebhookNotifier(webhookURL string) *WebhookNotifier {
	return &WebhookNotifier{
		webhookURL: webhookURL,
	}
}

// SendNotification sends a notification to the configured webhook.
// TODO: Implement actual HTTP POST to webhook URL.
func (w *WebhookNotifier) SendNotification(ctx context.Context, notification *Notification) error {
	if w.webhookURL == "" {
		return nil // No webhook configured, skip silently
	}

	// Stub: Log what would be sent
	payload, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// TODO: Implement actual webhook POST
	// For now, just validate the payload can be serialized
	_ = payload

	return nil
}

// EmailNotifier sends notifications via email.
// This is currently a stub implementation.
type EmailNotifier struct {
	emailAddress string
}

// NewEmailNotifier creates a new email notifier.
func NewEmailNotifier(emailAddress string) *EmailNotifier {
	return &EmailNotifier{
		emailAddress: emailAddress,
	}
}

// SendNotification sends a notification to the configured email address.
// TODO: Implement actual email sending.
func (e *EmailNotifier) SendNotification(ctx context.Context, notification *Notification) error {
	if e.emailAddress == "" {
		return nil // No email configured, skip silently
	}

	// Stub: Log what would be sent
	// TODO: Implement actual email sending via SMTP or external service

	return nil
}

// CompositeNotifier sends notifications to multiple backends.
type CompositeNotifier struct {
	notifiers []NotifyCallback
}

// NewCompositeNotifier creates a notifier that sends to multiple backends.
func NewCompositeNotifier(notifiers ...NotifyCallback) *CompositeNotifier {
	return &CompositeNotifier{
		notifiers: notifiers,
	}
}

// SendNotification sends a notification to all configured backends.
func (c *CompositeNotifier) SendNotification(ctx context.Context, notification *Notification) error {
	var lastErr error
	for _, n := range c.notifiers {
		if err := n.SendNotification(ctx, notification); err != nil {
			lastErr = err
			// Continue to try other notifiers
		}
	}
	return lastErr
}
