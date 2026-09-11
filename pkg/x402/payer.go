package x402

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// GasSource supplies what a payment transaction needs from the chain: a gas
// coin owned by the sender, the reference gas price, and a budget.
//
// Pass one ships StaticGas for tests. The chain-backed source over Sui gRPC
// (coin selection and reference gas price) is the unfinished step; see README.
type GasSource interface {
	GasFor(ctx context.Context, sender string) (gas ObjectRef, price, budget uint64, err error)
}

// StaticGas returns fixed values. For tests, and for callers that already
// know their gas coin.
type StaticGas struct {
	Gas    ObjectRef
	Price  uint64
	Budget uint64
}

func (s StaticGas) GasFor(context.Context, string) (ObjectRef, uint64, uint64, error) {
	return s.Gas, s.Price, s.Budget, nil
}

// Payer is an http.RoundTripper that pays a 402 on Sui and retries with proof.
// Swap it into any http.Client and payment exists; nothing upstream needs to
// know. A 402 above the ceiling is returned to the caller with HeaderRefused
// set, never paid.
type Payer struct {
	Next    http.RoundTripper
	Wallet  *Wallet
	Gas     GasSource
	Ledger  *Ledger
	Network string // CAIP-2, e.g. "sui:mainnet" or "sui:testnet"; only this network is paid

	Ceiling uint64 // per call, MIST; 0 refuses everything
	PerDay  uint64 // rolling 24h total, MIST; 0 means no daily limit

	// MaxBody bounds how much request body is buffered for the retry (default 4 MiB).
	MaxBody int64
	now     func() time.Time
}

const defaultMaxBody = 4 << 20

func (p *Payer) clock() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}

func (p *Payer) RoundTrip(r *http.Request) (*http.Response, error) {
	next := p.Next
	if next == nil {
		next = http.DefaultTransport
	}
	// A 402 arrives after the body is consumed. Make it replayable before the
	// first trip, or the retry sends nothing.
	if r.Body != nil && r.Body != http.NoBody && r.GetBody == nil {
		limit := p.MaxBody
		if limit <= 0 {
			limit = defaultMaxBody
		}
		buf, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
		r.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("x402: buffering request body: %w", err)
		}
		if int64(len(buf)) > limit {
			return nil, fmt.Errorf("x402: request body over %d bytes cannot be replayed", limit)
		}
		r.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(buf)), nil }
		r.Body, _ = r.GetBody()
		r.ContentLength = int64(len(buf))
	}

	resp, err := next.RoundTrip(r)
	if err != nil || resp.StatusCode != http.StatusPaymentRequired {
		return resp, err
	}
	if r.Header.Get(HeaderPaymentSignature) != "" {
		// We already paid once for this request; a second 402 is the server's answer, not a loop.
		return resp, nil
	}

	pr, err := ReadPaymentRequired(resp)
	if err != nil {
		return resp, nil // not an x402 402; hand it back untouched
	}
	req, reason := p.choose(pr)
	if reason != "" {
		return p.refuse(resp, pr, req, reason)
	}
	mist, _ := strconv.ParseUint(req.Amount, 10, 64)

	// Pay: build, sign, retry with proof.
	drain(resp)
	gas, price, budget, err := p.Gas.GasFor(r.Context(), p.Wallet.Address())
	if err != nil {
		return nil, fmt.Errorf("x402: gas: %w", err)
	}
	txBytes, err := PaymentTx{
		Sender: p.Wallet.Address(), PayTo: req.PayTo, AmountMist: mist,
		Gas: gas, GasPrice: price, GasBudget: budget,
	}.Encode()
	if err != nil {
		return nil, fmt.Errorf("x402: building payment: %w", err)
	}
	payload := PaymentPayload{
		X402Version: Version,
		Resource:    &pr.Resource,
		Accepted:    *req,
		Payload: SuiPayload{
			Signature:   p.Wallet.SignTransaction(txBytes),
			Transaction: b64(txBytes),
		},
		Extensions: pr.Extensions,
	}
	h, err := encodeHeader(payload)
	if err != nil {
		return nil, err
	}
	retry := r.Clone(r.Context())
	if r.GetBody != nil {
		retry.Body, err = r.GetBody()
		if err != nil {
			return nil, fmt.Errorf("x402: replaying body: %w", err)
		}
	}
	retry.Header.Set(HeaderPaymentSignature, h)

	entry := Entry{
		At: p.clock(), Outcome: "paid", Resource: pr.Resource.URL, PayTo: req.PayTo,
		Network: req.Network, Asset: req.Asset, Mist: mist, Digest: Digest(txBytes),
	}
	resp2, err := next.RoundTrip(retry)
	if err != nil {
		entry.Reason = "retry failed: " + err.Error()
		_ = p.Ledger.Append(entry)
		return nil, err
	}
	if s, ok := ReadSettlement(resp2); ok {
		entry.Settled = s.Transaction
		if !s.Success {
			entry.Outcome = "refused-by-server"
			entry.Reason = s.ErrorReason
		}
	}
	if err := p.Ledger.Append(entry); err != nil {
		return resp2, err
	}
	return resp2, nil
}

// choose picks the requirement labs can meet, or the reason it cannot.
func (p *Payer) choose(pr *PaymentRequired) (*PaymentRequirements, string) {
	var candidate *PaymentRequirements
	for i := range pr.Accepts {
		a := &pr.Accepts[i]
		if a.Scheme == SchemeExact && a.Network == p.Network && a.Asset == AssetSUI {
			candidate = a
			break
		}
	}
	if candidate == nil {
		return nil, fmt.Sprintf("no accepted option is exact/%s/%s", p.Network, AssetSUI)
	}
	mist, err := strconv.ParseUint(candidate.Amount, 10, 64)
	if err != nil {
		return candidate, fmt.Sprintf("amount %q is not an integer of MIST", candidate.Amount)
	}
	if mist > p.Ceiling {
		return candidate, fmt.Sprintf("amount %d MIST is above the per-call ceiling %d", mist, p.Ceiling)
	}
	if p.PerDay > 0 {
		spent, err := p.Ledger.PaidSince(p.clock().Add(-24 * time.Hour))
		if err != nil {
			return candidate, "ledger unreadable: " + err.Error()
		}
		if spent+mist > p.PerDay {
			return candidate, fmt.Sprintf("amount %d MIST would take the last 24h to %d, above the daily limit %d", mist, spent+mist, p.PerDay)
		}
	}
	return candidate, ""
}

func (p *Payer) refuse(resp *http.Response, pr *PaymentRequired, req *PaymentRequirements, reason string) (*http.Response, error) {
	e := Entry{At: p.clock(), Outcome: "refused", Reason: reason, Resource: pr.Resource.URL}
	if req != nil {
		e.PayTo, e.Network, e.Asset = req.PayTo, req.Network, req.Asset
		e.Mist, _ = strconv.ParseUint(req.Amount, 10, 64)
	}
	_ = p.Ledger.Append(e)
	resp.Header.Set(HeaderRefused, reason)
	return resp, nil
}

func drain(resp *http.Response) {
	if resp.Body != nil {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
	}
}
