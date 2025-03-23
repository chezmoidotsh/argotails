package tsutils

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

type (
	// WebhookEvent represents a Tailscale webhook event.
	WebhookEvent struct {
		Timestamp time.Time         `json:"timestamp"`
		Version   int               `json:"version"`
		Type      string            `json:"type"`
		Data      *WebhookEventData `json:"data,omitempty"`
	}

	// WebhookEventData represents the data of a Tailscale webhook event.
	WebhookEventData struct {
		DeviceName string `json:"deviceName"`
		ManagedBy  string `json:"managedBy"`
		Actor      string `json:"actor"`
		URL        string `json:"url"`
	}
)

// NOTE: These functions are mainly based on the Tailscale webhook signature verification example
// from https://github.com/tailscale/tailscale/blob/main/docs/webhooks/example.go

var ErrWebhookNotSigned = fmt.Errorf("webhook has no signature")
var ErrWebhookInvalidSignature = fmt.Errorf("webhook has invalid signature")
var ErrWebhookSignatureExpired = fmt.Errorf("webhook signature as expired: timestamp older than 5 minutes")
var ErrWebhookSignatureMismatch = fmt.Errorf("webhook signature does not match")

// VerifyWebhookSignature checks the request's "Tailscale-Webhook-Signature"
// header to verify that the events were signed by your webhook secret.
// If verification fails, an error is reported.
// If verification succeeds, the events are unmarshaled into the object.
func VerifyWebhookSignature[T any](ctx context.Context, req *http.Request, secret string, object *T) error {
	log := log.FromContext(ctx)

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Error(err, "failed to close request body")
		}
	}(req.Body)

	// Grab the signature sent on the request header.
	timestamp, signatures, err := parseSignatureHeader(req.Header.Get("Tailscale-Webhook-Signature"))
	if err != nil {
		return err
	}

	// Verify that the timestamp is recent.
	// Here, we use a threshold of 5 minutes.
	if timestamp.Before(time.Now().Add(-time.Minute * 5)) {
		return ErrWebhookSignatureExpired
	}

	// Form the expected signature.
	b, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprint(timestamp.Unix())))
	mac.Write([]byte("."))
	mac.Write(b)
	want := hex.EncodeToString(mac.Sum(nil))

	// Verify that the signatures match.
	var match bool
	for _, signature := range signatures["v1"] {
		if subtle.ConstantTimeCompare([]byte(signature), []byte(want)) == 1 {
			match = true
			break
		}
	}
	if !match {
		log.V(6).Info("signature does not match", "want", want, "got", signatures["v1"])
		return ErrWebhookSignatureMismatch
	}

	// If verified, return the events.
	return json.Unmarshal(b, object)
}

// parseSignatureHeader splits header into its timestamp and included signatures.
// The signatures are reported as a map of version (e.g. "v1") to a list of signatures
// found with that version.
func parseSignatureHeader(header string) (timestamp time.Time, signatures map[string][]string, err error) {
	if header == "" {
		return time.Time{}, nil, ErrWebhookNotSigned
	}

	signatures = make(map[string][]string)
	pairs := strings.Split(header, ",")
	for _, pair := range pairs {
		parts := strings.Split(pair, "=")
		if len(parts) != 2 {
			return time.Time{}, nil, ErrWebhookNotSigned
		}

		switch parts[0] {
		case "t":
			tsint, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return time.Time{}, nil, ErrWebhookInvalidSignature
			}
			timestamp = time.Unix(tsint, 0)
		case "v1":
			signatures[parts[0]] = append(signatures[parts[0]], parts[1])
		default:
			// Ignore unknown parts of the header.
			continue
		}
	}

	if len(signatures) == 0 {
		return time.Time{}, nil, ErrWebhookNotSigned
	}
	return
}
