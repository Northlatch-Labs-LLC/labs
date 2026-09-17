package x402

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/btcsuite/btcutil/bech32"
	"golang.org/x/crypto/blake2b"
)

// Wallet is one ed25519 Sui key. Signing is ed25519 over blake2b-256 of the
// intent-prefixed BCS bytes; the address is blake2b-256 of flag||pubkey. Both
// primitives are in Go's own crypto tree; nothing here is invented.
type Wallet struct {
	priv    ed25519.PrivateKey
	pub     ed25519.PublicKey
	address string
}

const (
	flagEd25519 byte = 0x00
	bech32HRP        = "suiprivkey"
)

// intentTransactionData is the Sui intent for signing transaction bytes:
// scope TransactionData (0), version V0 (0), app id Sui (0).
var intentTransactionData = []byte{0, 0, 0}

// NewWalletFromSuiPrivateKey parses the bech32 "suiprivkey1..." form the Sui
// tooling exports (flag byte followed by the 32-byte seed).
func NewWalletFromSuiPrivateKey(s string) (*Wallet, error) {
	hrp, data, err := bech32.Decode(strings.TrimSpace(s))
	if err != nil {
		return nil, fmt.Errorf("x402: bech32 decode: %w", err)
	}
	if hrp != bech32HRP {
		return nil, fmt.Errorf("x402: key prefix %q, want %q", hrp, bech32HRP)
	}
	raw, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		return nil, fmt.Errorf("x402: bech32 bit conversion: %w", err)
	}
	if len(raw) != 33 || raw[0] != flagEd25519 {
		return nil, fmt.Errorf("x402: key is not a 32-byte ed25519 seed with flag 0x00 (len %d, flag %#x)", len(raw), raw[0])
	}
	return NewWalletFromSeed(raw[1:]), nil
}

// NewWalletFromSeed builds a wallet from a 32-byte ed25519 seed.
func NewWalletFromSeed(seed []byte) *Wallet {
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	h := blake2b.Sum256(append([]byte{flagEd25519}, pub...))
	return &Wallet{priv: priv, pub: pub, address: "0x" + hex.EncodeToString(h[:])}
}

// Address returns the 0x-prefixed 32-byte Sui address.
func (w *Wallet) Address() string { return w.address }

// PublicKey returns the raw 32-byte ed25519 public key.
func (w *Wallet) PublicKey() []byte { return append([]byte(nil), w.pub...) }

// SignTransaction signs BCS TransactionData bytes the way Sui expects and
// returns the base64 serialized signature (flag || 64-byte sig || 32-byte pubkey).
func (w *Wallet) SignTransaction(txBytes []byte) string {
	msg := make([]byte, 0, len(intentTransactionData)+len(txBytes))
	msg = append(msg, intentTransactionData...)
	msg = append(msg, txBytes...)
	digest := blake2b.Sum256(msg)
	sig := ed25519.Sign(w.priv, digest[:])
	out := make([]byte, 0, 1+64+32)
	out = append(out, flagEd25519)
	out = append(out, sig...)
	out = append(out, w.pub...)
	return base64.StdEncoding.EncodeToString(out)
}

// signRaw signs bytes directly (no intent, no hashing); for use only inside
// package tests against the SDK's keypair.sign vector. Unexported (L-04) to
// prevent callers from signing arbitrary bytes without an intent prefix.
func (w *Wallet) signRaw(msg []byte) []byte { return ed25519.Sign(w.priv, msg) }

