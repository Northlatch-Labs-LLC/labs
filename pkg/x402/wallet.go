package x402

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

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
	hrp, data, err := bech32Decode(strings.TrimSpace(s))
	if err != nil {
		return nil, err
	}
	if hrp != bech32HRP {
		return nil, fmt.Errorf("x402: key prefix %q, want %q", hrp, bech32HRP)
	}
	raw, err := convertBits(data, 5, 8, false)
	if err != nil {
		return nil, err
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

// SignRaw signs bytes directly (no intent, no hashing); used only by tests
// against the SDK's keypair.sign vector.
func (w *Wallet) SignRaw(msg []byte) []byte { return ed25519.Sign(w.priv, msg) }

// --- bech32 (BIP-173), decode only ---

const bech32Charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l" // 32 symbols, BIP-173; checked against sipa/bech32 ref

func bech32Polymod(values []byte) uint32 {
	gen := []uint32{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	chk := uint32(1)
	for _, v := range values {
		b := chk >> 25
		chk = (chk&0x1ffffff)<<5 ^ uint32(v)
		for i := 0; i < 5; i++ {
			if (b>>uint(i))&1 == 1 {
				chk ^= gen[i]
			}
		}
	}
	return chk
}

func bech32HRPExpand(hrp string) []byte {
	out := make([]byte, 0, len(hrp)*2+1)
	for i := 0; i < len(hrp); i++ {
		out = append(out, hrp[i]>>5)
	}
	out = append(out, 0)
	for i := 0; i < len(hrp); i++ {
		out = append(out, hrp[i]&31)
	}
	return out
}

func bech32Decode(s string) (string, []byte, error) {
	if strings.ToLower(s) != s && strings.ToUpper(s) != s {
		return "", nil, fmt.Errorf("x402: bech32 mixed case")
	}
	s = strings.ToLower(s)
	pos := strings.LastIndex(s, "1")
	if pos < 1 || pos+7 > len(s) {
		return "", nil, fmt.Errorf("x402: bech32 separator")
	}
	hrp := s[:pos]
	data := make([]byte, 0, len(s)-pos-1)
	for _, c := range s[pos+1:] {
		idx := strings.IndexRune(bech32Charset, c)
		if idx < 0 {
			return "", nil, fmt.Errorf("x402: bech32 character %q", c)
		}
		data = append(data, byte(idx))
	}
	if bech32Polymod(append(bech32HRPExpand(hrp), data...)) != 1 {
		return "", nil, fmt.Errorf("x402: bech32 checksum")
	}
	return hrp, data[:len(data)-6], nil
}

func convertBits(data []byte, from, to uint, pad bool) ([]byte, error) {
	acc, bits := uint32(0), uint(0)
	out := make([]byte, 0, len(data)*int(from)/int(to)+1)
	maxv := uint32(1)<<to - 1
	for _, v := range data {
		if uint32(v)>>from != 0 {
			return nil, fmt.Errorf("x402: convertBits value out of range")
		}
		acc = acc<<from | uint32(v)
		bits += from
		for bits >= to {
			bits -= to
			out = append(out, byte(acc>>bits&maxv))
		}
	}
	if pad {
		if bits > 0 {
			out = append(out, byte(acc<<(to-bits)&maxv))
		}
	} else if bits >= from || acc<<(to-bits)&maxv != 0 {
		return nil, fmt.Errorf("x402: convertBits padding")
	}
	return out, nil
}
