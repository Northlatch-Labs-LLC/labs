package x402

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Entry is one line of the ledger: what was paid or refused, to whom, for
// which resource, and the transaction digest.
type Entry struct {
	At       time.Time `json:"at"`
	Outcome  string    `json:"outcome"` // "paid" or "refused"
	Reason   string    `json:"reason,omitempty"`
	Resource string    `json:"resource"`
	PayTo    string    `json:"payTo"`
	Network  string    `json:"network"`
	Asset    string    `json:"asset"`
	Mist     uint64    `json:"mist"`
	Digest   string    `json:"digest,omitempty"`
	Settled  string    `json:"settled,omitempty"` // the facilitator's transaction id from PAYMENT-RESPONSE, if any
}

// Ledger is an append-only JSONL file. Nothing is ever rewritten.
type Ledger struct {
	mu   sync.Mutex
	path string
}

func NewLedger(path string) *Ledger { return &Ledger{path: path} }

func (l *Ledger) Append(e Entry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("x402 ledger: %w", err)
	}
	defer f.Close()
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

// PaidSince sums MIST paid (outcome "paid") at or after t.
func (l *Ledger) PaidSince(t time.Time) (uint64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.Open(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer f.Close()
	var total uint64
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			continue
		}
		if e.Outcome == "paid" && !e.At.Before(t) {
			total += e.Mist
		}
	}
	return total, sc.Err()
}
