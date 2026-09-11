package x402

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/blake2b"
)

// Vectors produced with @mysten/sui 2.30.0 on 2026-09-11 (a throwaway key made for this test).
const (
	vecPrivKey   = "suiprivkey1qpz3pcuu9tpgncalqugs39wutldhrtgp8d8thrdzg8rzfu2z2guksdfuxe7"
	vecSeedHex   = "4510e39c2ac289e3bf07110895dc5fdb71ad013b4ebb8da241c624f142523968"
	vecPubHex    = "2f7a0a47933227e0cdc70261d51a07bc89f74d9979ae72fbe337fae30864580f"
	vecAddress   = "0x5732b0622e59e1d8b19ad18af43da5a4a044537dabb701bedc07cbdfa6f1b3b4"
	vecRawSigHex = "0ad266c6d527c3b27d639ac9028860c5973611eae870e9625b0c68eb9a6ff94669d5a8f4af355f0255d343aef0ab114c4d663f78ea9dfd90b13c09f869730802"
	vecPayTo     = "0x1eb7c57e3f2bd0fc6cb9dcffd143ea957e4d98f805c358733f76dee0667fe0b1"
	vecGasID     = "0xabababababababababababababababababababababababababababababababab"
	vecGasDigest = "D3vRb6kAnkAWvhP5pCnCGuTefBeWiEt4eNHpHNunpKtx"
	vecTxB64     = "AAACAAiAlpgAAAAAAAAgHrfFfj8r0Pxsudz/0UPqlX5NmPgFw1hzP3be4GZ/4LECAgABAQAAAQEDAAAAAAEBAFcysGIuWeHYsZrRivQ9paSgRFN9q7cBvtwHy9+m8bO0Aaurq6urq6urq6urq6urq6urq6urq6urq6urq6urq6urOTAAAAAAAAAgswvPtY7z0JK7yKuzhxZsSgzvQttCEld89q/inMeFPDVXMrBiLlnh2LGa0Yr0PaWkoERTfau3Ab7cB8vfpvGztOgDAAAAAAAAwMYtAAAAAAAA"
	vecTxSigB64  = "ABJWEMMUQUI1gcTZE52t0VOxWcu2hOLH45z1ByDYqWS3rRsj+XHf7BjDGaC9YY2IVNRnkcoFSKPVio78o8UbmAYvegpHkzIn4M3HAmHVGge8ifdNmXmucvvjN/rjCGRYDw=="
	vecTxDigest  = "HD2ERHqFxb71NcjG75SGCLNFDsHA5guhkjDnUjKZJpWE"
)

func vecTx() PaymentTx {
	return PaymentTx{
		Sender: vecAddress, PayTo: vecPayTo, AmountMist: 10000000,
		Gas:      ObjectRef{ObjectID: vecGasID, Version: 12345, Digest: vecGasDigest},
		GasPrice: 1000, GasBudget: 3000000,
	}
}

func TestWallet_MatchesSDK(t *testing.T) {
	w, err := NewWalletFromSuiPrivateKey(vecPrivKey)
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(w.PublicKey()); got != vecPubHex {
		t.Fatalf("pubkey %s, want %s", got, vecPubHex)
	}
	if w.Address() != vecAddress {
		t.Fatalf("address %s, want %s", w.Address(), vecAddress)
	}
	if got := hex.EncodeToString(w.SignRaw([]byte("labs x402 vector"))); got != vecRawSigHex {
		t.Fatalf("raw signature differs from the SDK's")
	}
	seed, _ := hex.DecodeString(vecSeedHex)
	if NewWalletFromSeed(seed).Address() != vecAddress {
		t.Fatal("seed path gives a different address")
	}
}

func TestPaymentTx_MatchesSDKBytes(t *testing.T) {
	got, err := vecTx().Encode()
	if err != nil {
		t.Fatal(err)
	}
	want, _ := base64.StdEncoding.DecodeString(vecTxB64)
	if !bytes.Equal(got, want) {
		t.Fatalf("BCS differs from @mysten/sui Transaction.build():\n got %x\nwant %x", got, want)
	}
	if Digest(got) != vecTxDigest {
		t.Fatalf("digest %s, want %s", Digest(got), vecTxDigest)
	}
	w, _ := NewWalletFromSuiPrivateKey(vecPrivKey)
	if sig := w.SignTransaction(got); sig != vecTxSigB64 {
		t.Fatalf("transaction signature differs from keypair.signTransaction():\n got %s\nwant %s", sig, vecTxSigB64)
	}
}

// verifySuiSignature is what a facilitator does: flag, sig, pubkey; intent-prefixed blake2b.
func verifySuiSignature(t *testing.T, sigB64, txB64 string) (pub []byte) {
	t.Helper()
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil || len(sig) != 97 || sig[0] != 0 {
		t.Fatalf("serialized signature shape: len %d err %v", len(sig), err)
	}
	tx, _ := base64.StdEncoding.DecodeString(txB64)
	digest := blake2b.Sum256(append([]byte{0, 0, 0}, tx...))
	if !ed25519.Verify(ed25519.PublicKey(sig[65:]), digest[:], sig[1:65]) {
		t.Fatal("signature does not verify over intent||tx")
	}
	return sig[65:]
}

func newPayer(t *testing.T, next http.RoundTripper, ceiling, perDay uint64) (*Payer, *Ledger) {
	t.Helper()
	w, err := NewWalletFromSuiPrivateKey(vecPrivKey)
	if err != nil {
		t.Fatal(err)
	}
	l := NewLedger(filepath.Join(t.TempDir(), "x402.jsonl"))
	return &Payer{
		Next: next, Wallet: w, Ledger: l, Network: "sui:testnet",
		Gas:     StaticGas{Gas: ObjectRef{ObjectID: vecGasID, Version: 12345, Digest: vecGasDigest}, Price: 1000, Budget: 3000000},
		Ceiling: ceiling, PerDay: perDay,
	}, l
}

func paymentRequired(t *testing.T, amount string) string {
	t.Helper()
	h, err := encodeHeader(PaymentRequired{
		X402Version: 2, Error: "PAYMENT-SIGNATURE header is required",
		Resource: ResourceInfo{URL: "https://api.example.test/premium", MimeType: "application/json"},
		Accepts: []PaymentRequirements{{
			Scheme: "exact", Network: "sui:testnet", Amount: amount, Asset: AssetSUI,
			PayTo: vecPayTo, MaxTimeoutSeconds: 60,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestPayer_PaysA402AndReplaysTheBody(t *testing.T) {
	var sawBody []string
	var sawPayload PaymentPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		sawBody = append(sawBody, string(body))
		sig := r.Header.Get(HeaderPaymentSignature)
		if sig == "" {
			w.Header().Set(HeaderPaymentRequired, paymentRequired(t, "10000000"))
			w.WriteHeader(http.StatusPaymentRequired)
			io.WriteString(w, "{}")
			return
		}
		if err := decodeHeader(sig, &sawPayload); err != nil {
			t.Errorf("payload header: %v", err)
		}
		pub := verifySuiSignature(t, sawPayload.Payload.Signature, sawPayload.Payload.Transaction)
		if hex.EncodeToString(pub) != vecPubHex {
			t.Errorf("payer pubkey %x", pub)
		}
		settle, _ := encodeHeader(SettlementResponse{Success: true, Transaction: "0xsettled", Network: "sui:testnet", Payer: vecAddress})
		w.Header().Set(HeaderPaymentResponse, settle)
		io.WriteString(w, `{"data":"premium"}`)
	}))
	defer srv.Close()

	p, l := newPayer(t, srv.Client().Transport, 20000000, 0)
	client := &http.Client{Transport: p}
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/premium", bytes.NewBufferString(`{"query":"latest"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || string(out) != `{"data":"premium"}` {
		t.Fatalf("status %d body %s", resp.StatusCode, out)
	}
	if len(sawBody) != 2 || sawBody[0] != sawBody[1] || sawBody[1] != `{"query":"latest"}` {
		t.Fatalf("body not replayed on retry: %q", sawBody)
	}
	if sawPayload.X402Version != 2 || sawPayload.Accepted.Amount != "10000000" || sawPayload.Accepted.Asset != AssetSUI {
		t.Fatalf("payload: %+v", sawPayload)
	}
	// The transaction sends exactly the amount to payTo from this wallet: same bytes as the vector.
	if sawPayload.Payload.Transaction != vecTxB64 {
		t.Fatalf("transaction bytes differ from the SDK vector")
	}
	paid, _ := l.PaidSince(time.Now().Add(-time.Hour))
	if paid != 10000000 {
		t.Fatalf("ledger paid %d", paid)
	}
	raw, _ := os.ReadFile(l.path)
	var e Entry
	if err := json.Unmarshal(bytes.TrimSpace(raw), &e); err != nil {
		t.Fatal(err)
	}
	if e.Outcome != "paid" || e.Digest != vecTxDigest || e.Settled != "0xsettled" || e.Resource != "https://api.example.test/premium" {
		t.Fatalf("ledger entry %+v", e)
	}
}

func TestPayer_RefusesAboveCeiling(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set(HeaderPaymentRequired, paymentRequired(t, "50000000"))
		w.WriteHeader(http.StatusPaymentRequired)
	}))
	defer srv.Close()
	p, l := newPayer(t, srv.Client().Transport, 20000000, 0)
	resp, err := (&http.Client{Transport: p}).Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPaymentRequired || calls != 1 {
		t.Fatalf("status %d calls %d: a refusal must not retry", resp.StatusCode, calls)
	}
	if reason := resp.Header.Get(HeaderRefused); reason != "amount 50000000 MIST is above the per-call ceiling 20000000" {
		t.Fatalf("refusal reason %q", reason)
	}
	paid, _ := l.PaidSince(time.Time{})
	if paid != 0 {
		t.Fatalf("refused payment counted as paid: %d", paid)
	}
	raw, _ := os.ReadFile(l.path)
	if !bytes.Contains(raw, []byte(`"outcome":"refused"`)) {
		t.Fatalf("refusal not in ledger: %s", raw)
	}
}

func TestPayer_RefusesWhenDailyLimitWouldBeExceeded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(HeaderPaymentRequired, paymentRequired(t, "6000000"))
		w.WriteHeader(http.StatusPaymentRequired)
	}))
	defer srv.Close()
	p, l := newPayer(t, srv.Client().Transport, 10000000, 10000000)
	_ = l.Append(Entry{At: time.Now().Add(-2 * time.Hour), Outcome: "paid", Mist: 5000000})
	resp, err := (&http.Client{Transport: p}).Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 402 || resp.Header.Get(HeaderRefused) == "" {
		t.Fatalf("expected a refusal, got %d %q", resp.StatusCode, resp.Header.Get(HeaderRefused))
	}
	// A payment older than 24h no longer counts.
	l2 := NewLedger(filepath.Join(t.TempDir(), "x402.jsonl"))
	_ = l2.Append(Entry{At: time.Now().Add(-25 * time.Hour), Outcome: "paid", Mist: 5000000})
	spent, _ := l2.PaidSince(time.Now().Add(-24 * time.Hour))
	if spent != 0 {
		t.Fatalf("stale entry counted: %d", spent)
	}
}

func TestPayer_LeavesOtherNetworksAndAssetsAlone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, _ := encodeHeader(PaymentRequired{X402Version: 2, Resource: ResourceInfo{URL: "x"}, Accepts: []PaymentRequirements{
			{Scheme: "exact", Network: "eip155:84532", Amount: "1", Asset: "0x036CbD53842c5426634e7929541eC2318f3dCF7e", PayTo: "0x0", MaxTimeoutSeconds: 60},
			{Scheme: "exact", Network: "sui:testnet", Amount: "1", Asset: "0xdead::usdc::USDC", PayTo: vecPayTo, MaxTimeoutSeconds: 60},
		}})
		w.Header().Set(HeaderPaymentRequired, h)
		w.WriteHeader(http.StatusPaymentRequired)
	}))
	defer srv.Close()
	p, _ := newPayer(t, srv.Client().Transport, 1000, 0)
	resp, err := (&http.Client{Transport: p}).Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 402 || resp.Header.Get(HeaderRefused) != "no accepted option is exact/sui:testnet/0x2::sui::SUI" {
		t.Fatalf("%d %q", resp.StatusCode, resp.Header.Get(HeaderRefused))
	}
}

func TestPayer_PassesNon402Through(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "free") }))
	defer srv.Close()
	p, _ := newPayer(t, srv.Client().Transport, 1, 0)
	resp, err := (&http.Client{Transport: p}).Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "free" {
		t.Fatal(string(b))
	}
}
