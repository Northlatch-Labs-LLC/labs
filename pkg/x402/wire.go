// Package x402 pays for what labs fetches: an http.RoundTripper that answers a
// 402 by paying on Sui and retrying with proof, within a ceiling it enforces
// itself.
//
// Wire format verified against the x402 v2 specification and its HTTP
// transport (github.com/coinbase/x402, specs/x402-specification-v2.md and
// specs/transports-v2/http.md, read 2026-09-11) and the Sui exact scheme
// (specs/schemes/exact/scheme_exact_sui.md):
//
//	server -> client   402 + PAYMENT-REQUIRED:  base64(JSON PaymentRequired)
//	client -> server   PAYMENT-SIGNATURE:       base64(JSON PaymentPayload)
//	server -> client   PAYMENT-RESPONSE:        base64(JSON SettlementResponse)
//
// On Sui the payload is a fully signed transaction transferring exactly
// `amount` of `asset` to `payTo`; the facilitator broadcasts it.
package x402

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	HeaderPaymentRequired  = "PAYMENT-REQUIRED"
	HeaderPaymentSignature = "PAYMENT-SIGNATURE"
	HeaderPaymentResponse  = "PAYMENT-RESPONSE"

	// HeaderRefused is labs' own: set on a 402 that labs chose not to pay, with the reason.
	HeaderRefused = "Labs-X402-Refused"

	Version     = 2
	SchemeExact = "exact"

	// AssetSUI is the only asset labs pays in. Never USDC.
	AssetSUI = "0x2::sui::SUI"
)

type ResourceInfo struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

type PaymentRequirements struct {
	Scheme            string          `json:"scheme"`
	Network           string          `json:"network"`
	Amount            string          `json:"amount"`
	Asset             string          `json:"asset"`
	PayTo             string          `json:"payTo"`
	MaxTimeoutSeconds int             `json:"maxTimeoutSeconds"`
	Extra             json.RawMessage `json:"extra,omitempty"`
}

type PaymentRequired struct {
	X402Version int                   `json:"x402Version"`
	Error       string                `json:"error,omitempty"`
	Resource    ResourceInfo          `json:"resource"`
	Accepts     []PaymentRequirements `json:"accepts"`
	Extensions  json.RawMessage       `json:"extensions,omitempty"`
}

// SuiPayload is the scheme-specific payload for exact on Sui.
type SuiPayload struct {
	Signature   string `json:"signature"`   // base64 Sui serialized signature: flag || sig || pubkey
	Transaction string `json:"transaction"` // base64 BCS TransactionData
}

type PaymentPayload struct {
	X402Version int                 `json:"x402Version"`
	Resource    *ResourceInfo       `json:"resource,omitempty"`
	Accepted    PaymentRequirements `json:"accepted"`
	Payload     SuiPayload          `json:"payload"`
	Extensions  json.RawMessage     `json:"extensions,omitempty"`
}

type SettlementResponse struct {
	Success     bool   `json:"success"`
	ErrorReason string `json:"errorReason,omitempty"`
	Payer       string `json:"payer,omitempty"`
	Transaction string `json:"transaction"`
	Network     string `json:"network"`
	Amount      string `json:"amount,omitempty"`
}

func encodeHeader(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func decodeHeader(s string, v any) error {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return fmt.Errorf("x402: header is not base64: %w", err)
	}
	return json.Unmarshal(b, v)
}

// ReadPaymentRequired reads the PaymentRequired object from a 402 response's header.
func ReadPaymentRequired(resp *http.Response) (*PaymentRequired, error) {
	h := resp.Header.Get(HeaderPaymentRequired)
	if h == "" {
		return nil, fmt.Errorf("x402: 402 without %s header", HeaderPaymentRequired)
	}
	var pr PaymentRequired
	if err := decodeHeader(h, &pr); err != nil {
		return nil, err
	}
	if pr.X402Version != Version {
		return nil, fmt.Errorf("x402: version %d, labs speaks %d", pr.X402Version, Version)
	}
	return &pr, nil
}

// ReadSettlement reads the PAYMENT-RESPONSE header if the server set one.
func ReadSettlement(resp *http.Response) (*SettlementResponse, bool) {
	h := resp.Header.Get(HeaderPaymentResponse)
	if h == "" {
		return nil, false
	}
	var s SettlementResponse
	if err := decodeHeader(h, &s); err != nil {
		return nil, false
	}
	return &s, true
}

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }
