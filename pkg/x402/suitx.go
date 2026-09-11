package x402

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/blake2b"
)

// ObjectRef names one owned object on Sui: id, version, digest (base58).
type ObjectRef struct {
	ObjectID string
	Version  uint64
	Digest   string
}

// PaymentTx is the simplest transaction Sui has: SplitCoins off the gas coin,
// TransferObjects the piece to the payee. Encoded by hand as BCS
// TransactionData::V1 { ProgrammableTransaction, sender, GasData, expiration None }.
// Verified byte-for-byte against @mysten/sui's Transaction.build() in the tests.
type PaymentTx struct {
	Sender     string
	PayTo      string
	AmountMist uint64
	Gas        ObjectRef
	GasPrice   uint64
	GasBudget  uint64
}

// Encode returns the BCS bytes of the transaction.
func (t PaymentTx) Encode() ([]byte, error) {
	sender, err := parseAddress(t.Sender)
	if err != nil {
		return nil, fmt.Errorf("sender: %w", err)
	}
	payTo, err := parseAddress(t.PayTo)
	if err != nil {
		return nil, fmt.Errorf("payTo: %w", err)
	}
	gasID, err := parseAddress(t.Gas.ObjectID)
	if err != nil {
		return nil, fmt.Errorf("gas object: %w", err)
	}
	gasDigest, err := base58Decode(t.Gas.Digest)
	if err != nil || len(gasDigest) != 32 {
		return nil, fmt.Errorf("gas digest %q is not a 32-byte base58 digest", t.Gas.Digest)
	}

	var b bcs
	b.u8(0) // TransactionData::V1
	b.u8(0) // TransactionKind::ProgrammableTransaction
	// inputs: [Pure(u64 amount), Pure(address payTo)]
	b.uleb(2)
	amount := make([]byte, 8)
	binary.LittleEndian.PutUint64(amount, t.AmountMist)
	b.u8(0) // CallArg::Pure
	b.bytes(amount)
	b.u8(0) // CallArg::Pure
	b.bytes(payTo)
	// commands: [SplitCoins(GasCoin, [Input(0)]), TransferObjects([NestedResult(0,0)], Input(1))]
	b.uleb(2)
	b.u8(2) // Command::SplitCoins
	b.u8(0) // Argument::GasCoin
	b.uleb(1)
	b.u8(1) // Argument::Input
	b.u16(0)
	b.u8(1) // Command::TransferObjects
	b.uleb(1)
	b.u8(3) // Argument::NestedResult(0, 0): the first coin SplitCoins produced, as the SDK emits it
	b.u16(0)
	b.u16(0)
	b.u8(1) // Argument::Input
	b.u16(1)
	// sender
	b.raw(sender)
	// GasData { payment: [ObjectRef], owner, price, budget }
	b.uleb(1)
	b.raw(gasID)
	b.u64(t.Gas.Version)
	b.bytes(gasDigest)
	b.raw(sender)
	b.u64(t.GasPrice)
	b.u64(t.GasBudget)
	// TransactionExpiration::None
	b.u8(0)
	return b.buf, nil
}

// Digest returns the Sui transaction digest of BCS bytes, base58, as the
// explorer shows it: blake2b-256("TransactionData::" || bytes).
func Digest(txBytes []byte) string {
	h := blake2b.Sum256(append([]byte("TransactionData::"), txBytes...))
	return base58Encode(h[:])
}

type bcs struct{ buf []byte }

func (b *bcs) u8(v byte)      { b.buf = append(b.buf, v) }
func (b *bcs) raw(v []byte)   { b.buf = append(b.buf, v...) }
func (b *bcs) bytes(v []byte) { b.uleb(uint64(len(v))); b.raw(v) }
func (b *bcs) u16(v uint16) {
	b.buf = binary.LittleEndian.AppendUint16(b.buf, v)
}
func (b *bcs) u64(v uint64) {
	b.buf = binary.LittleEndian.AppendUint64(b.buf, v)
}
func (b *bcs) uleb(v uint64) {
	for v >= 0x80 {
		b.buf = append(b.buf, byte(v)|0x80)
		v >>= 7
	}
	b.buf = append(b.buf, byte(v))
}

func parseAddress(s string) ([]byte, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "0x")
	if len(s) > 64 {
		return nil, fmt.Errorf("address longer than 32 bytes")
	}
	s = strings.Repeat("0", 64-len(s)) + s
	return hex.DecodeString(s)
}

// --- base58 (bitcoin alphabet), as Sui digests use ---

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

func base58Decode(s string) ([]byte, error) {
	n := new(big.Int)
	radix := big.NewInt(58)
	for _, c := range s {
		idx := strings.IndexRune(base58Alphabet, c)
		if idx < 0 {
			return nil, fmt.Errorf("base58 character %q", c)
		}
		n.Mul(n, radix)
		n.Add(n, big.NewInt(int64(idx)))
	}
	out := n.Bytes()
	zeros := 0
	for zeros < len(s) && s[zeros] == '1' {
		zeros++
	}
	return append(make([]byte, zeros), out...), nil
}

func base58Encode(b []byte) string {
	n := new(big.Int).SetBytes(b)
	radix := big.NewInt(58)
	mod := new(big.Int)
	var out []byte
	for n.Sign() > 0 {
		n.DivMod(n, radix, mod)
		out = append(out, base58Alphabet[mod.Int64()])
	}
	for _, c := range b {
		if c != 0 {
			break
		}
		out = append(out, '1')
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}
