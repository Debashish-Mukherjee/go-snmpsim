package v3

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type goldenVectors struct {
	HMAC map[string]string `json:"hmac"`
	CFB  map[string]string `json:"cfb"`
}

func loadGolden(t *testing.T) goldenVectors {
	t.Helper()
	path := filepath.Join("testdata", "crypto_golden.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}
	var g goldenVectors
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatalf("parse golden file: %v", err)
	}
	return g
}

func TestHMACGoldenVectors(t *testing.T) {
	g := loadGolden(t)
	key := []byte("auth-key-123456")
	msg := []byte("snmpsim-golden-payload")

	tests := []struct {
		name  string
		proto AuthProtocol
	}{
		{"MD5", AuthMD5},
		{"SHA1", AuthSHA1},
		{"SHA224", AuthSHA224},
		{"SHA256", AuthSHA256},
		{"SHA384", AuthSHA384},
		{"SHA512", AuthSHA512},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			digest, err := HMACDigest(tc.proto, key, msg)
			if err != nil {
				t.Fatalf("HMACDigest error: %v", err)
			}
			got := hex.EncodeToString(digest)
			if got != g.HMAC[tc.name] {
				t.Fatalf("digest mismatch for %s\nexpected: %s\nactual:   %s", tc.name, g.HMAC[tc.name], got)
			}
			ok, err := VerifyHMAC(tc.proto, key, msg, digest)
			if err != nil || !ok {
				t.Fatalf("VerifyHMAC failed for %s: ok=%v err=%v", tc.name, ok, err)
			}
		})
	}
}

func TestPrivacyGoldenVectors(t *testing.T) {
	g := loadGolden(t)
	iv := []byte("0123456789abcdef0123456789abcdef")
	plaintext := []byte("snmpsim-cfb-plaintext-16bytes!!")

	tests := []struct {
		name  string
		proto PrivProtocol
		key   []byte
	}{
		{"DES", PrivDES, []byte("des-key1")},
		{"3DES", Priv3DES, []byte("0123456789abcdef01234567")},
		{"AES128", PrivAES128, []byte("0123456789abcdef")},
		{"AES192", PrivAES192, []byte("0123456789abcdef01234567")},
		{"AES256", PrivAES256, []byte("0123456789abcdef0123456789abcdef")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ciphertext, err := EncryptCFB(tc.proto, tc.key, iv, plaintext)
			if err != nil {
				t.Fatalf("EncryptCFB error: %v", err)
			}
			got := hex.EncodeToString(ciphertext)
			if got != g.CFB[tc.name] {
				t.Fatalf("cipher mismatch for %s\nexpected: %s\nactual:   %s", tc.name, g.CFB[tc.name], got)
			}
			decrypted, err := DecryptCFB(tc.proto, tc.key, iv, ciphertext)
			if err != nil {
				t.Fatalf("DecryptCFB error: %v", err)
			}
			if string(decrypted) != string(plaintext) {
				t.Fatalf("roundtrip mismatch for %s", tc.name)
			}
		})
	}
}

func TestLocalizeKey(t *testing.T) {
	engineID := []byte{0x80, 0x00, 0x1f, 0x88, 0x01, 0x02, 0x03, 0x04}
	key1, err := LocalizeKey(AuthSHA256, []byte("auth-pass"), engineID)
	if err != nil {
		t.Fatalf("LocalizeKey error: %v", err)
	}
	key2, err := LocalizeKey(AuthSHA256, []byte("auth-pass"), engineID)
	if err != nil {
		t.Fatalf("LocalizeKey error: %v", err)
	}
	if hex.EncodeToString(key1) != hex.EncodeToString(key2) {
		t.Fatalf("localized keys are not deterministic")
	}
}
