package credential

import "sync"

// SecureStore holds a passphrase in memory as a mutable []byte so Clear()
// can zero the bytes before releasing the allocation (L-01).
//
// Uses sync.Mutex instead of atomic.Pointer[string] because zeroing requires
// holding the allocation while writing; an atomic swap to nil only clears the
// pointer, leaving the old bytes live in the heap until GC collects them.
type SecureStore struct {
	mu  sync.Mutex
	val []byte
}

// NewSecureStore creates an empty SecureStore.
func NewSecureStore() *SecureStore {
	return &SecureStore{}
}

// SetString stores the passphrase. An empty string clears the store.
func (s *SecureStore) SetString(passphrase string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.val {
		s.val[i] = 0
	}
	if passphrase == "" {
		s.val = nil
		return
	}
	s.val = []byte(passphrase)
}

// Get returns the stored passphrase, or "" if not set.
func (s *SecureStore) Get() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.val == nil {
		return ""
	}
	return string(s.val)
}

// IsSet reports whether a passphrase is currently stored.
func (s *SecureStore) IsSet() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.val != nil
}

// Clear zeros the stored bytes before releasing the allocation.
func (s *SecureStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.val {
		s.val[i] = 0
	}
	s.val = nil
}
