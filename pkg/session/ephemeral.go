package session

import (
	"sync"

	"github.com/Northlatch-Labs-LLC/labs/pkg/providers"
)

// EphemeralStore is the only session store labs has. It lives in memory for
// the life of one process and is gone when that process exits.
//
// A labs citizen wakes knowing nothing but its files and the chain. Upstream
// PicoClaw keyed sessions by channel and replayed the whole transcript into
// every request (measured: ~43,000 tokens per request, 85% of spend, one
// 243 KB file shared by every waking for hours). This store makes that
// impossible rather than merely configured away: there is no file to grow.
type EphemeralStore struct {
	mu        sync.RWMutex
	histories map[string][]providers.Message
	summaries map[string]string
}

// NewEphemeralStore returns an empty in-memory store.
func NewEphemeralStore() *EphemeralStore {
	return &EphemeralStore{
		histories: make(map[string][]providers.Message),
		summaries: make(map[string]string),
	}
}

func (s *EphemeralStore) AddMessage(sessionKey, role, content string) {
	s.AddFullMessage(sessionKey, providers.Message{Role: role, Content: content})
}

func (s *EphemeralStore) AddFullMessage(sessionKey string, msg providers.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.histories[sessionKey] = append(s.histories[sessionKey], msg)
}

func (s *EphemeralStore) GetHistory(key string) []providers.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h := s.histories[key]
	out := make([]providers.Message, len(h))
	copy(out, h)
	return out
}

func (s *EphemeralStore) GetSummary(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.summaries[key]
}

func (s *EphemeralStore) SetSummary(key, summary string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.summaries[key] = summary
}

func (s *EphemeralStore) SetHistory(key string, history []providers.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h := make([]providers.Message, len(history))
	copy(h, history)
	s.histories[key] = h
}

func (s *EphemeralStore) TruncateHistory(key string, keepLast int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h := s.histories[key]
	if keepLast <= 0 {
		s.histories[key] = nil
		return
	}
	if len(h) > keepLast {
		s.histories[key] = h[len(h)-keepLast:]
	}
}

// Save is a no-op: nothing is ever written to disk.
func (s *EphemeralStore) Save(string) error { return nil }

func (s *EphemeralStore) ListSessions() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.histories))
	for k := range s.histories {
		keys = append(keys, k)
	}
	return keys
}

// Close drops everything. The process is ending; so is the memory of this waking.
func (s *EphemeralStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.histories = make(map[string][]providers.Message)
	s.summaries = make(map[string]string)
	return nil
}
