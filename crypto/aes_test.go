package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"testing"
)

// ---------------------------------------------------------------------------
// DeriveKey
// ---------------------------------------------------------------------------

func TestDeriveKey_Deterministic(t *testing.T) {
	secret := []byte("my-secret")
	k1 := DeriveKey(secret, PurposeAuth)
	k2 := DeriveKey(secret, PurposeAuth)
	if !bytes.Equal(k1, k2) {
		t.Fatal("DeriveKey must be deterministic for the same inputs")
	}
}

func TestDeriveKey_Length(t *testing.T) {
	key := DeriveKey([]byte("secret"), PurposeEncrypt)
	if len(key) != 32 {
		t.Fatalf("expected 32-byte key (SHA-256 HMAC), got %d", len(key))
	}
}

func TestDeriveKey_EmptySecret(t *testing.T) {
	key := DeriveKey([]byte{}, PurposeAuth)
	if key != nil {
		t.Fatalf("expected nil for empty secret, got %v", key)
	}
}

func TestDeriveKey_NilSecret(t *testing.T) {
	key := DeriveKey(nil, PurposeAuth)
	if key != nil {
		t.Fatalf("expected nil for nil secret, got %v", key)
	}
}

func TestDeriveKey_DifferentPurposeYieldsDifferentKey(t *testing.T) {
	secret := []byte("shared-secret")
	kAuth := DeriveKey(secret, PurposeAuth)
	kEnc := DeriveKey(secret, PurposeEncrypt)
	if bytes.Equal(kAuth, kEnc) {
		t.Fatal("different purposes must produce different keys")
	}
}

func TestDeriveKey_DifferentSecretYieldsDifferentKey(t *testing.T) {
	k1 := DeriveKey([]byte("secret-a"), PurposeAuth)
	k2 := DeriveKey([]byte("secret-b"), PurposeAuth)
	if bytes.Equal(k1, k2) {
		t.Fatal("different secrets must produce different keys")
	}
}

// ---------------------------------------------------------------------------
// genNonce (unexported, but same package)
// ---------------------------------------------------------------------------

func TestGenNonce_Length(t *testing.T) {
	for _, size := range []int{12, 16, 24} {
		nonce, err := genNonce(size)
		if err != nil {
			t.Fatalf("genNonce(%d) error: %v", size, err)
		}
		if len(nonce) != size {
			t.Fatalf("expected nonce length %d, got %d", size, len(nonce))
		}
	}
}

func TestGenNonce_Uniqueness(t *testing.T) {
	const runs = 100
	seen := make(map[string]struct{}, runs)
	for i := 0; i < runs; i++ {
		nonce, err := genNonce(12)
		if err != nil {
			t.Fatalf("genNonce error on iteration %d: %v", i, err)
		}
		s := string(nonce)
		if _, dup := seen[s]; dup {
			t.Fatalf("duplicate nonce on iteration %d", i)
		}
		seen[s] = struct{}{}
	}
}

// ---------------------------------------------------------------------------
// AESEncrypt
// ---------------------------------------------------------------------------

func TestAESEncrypt_NilKey_Rejected(t *testing.T) {
	_, err := AESEncrypt([]byte("data"), nil)
	if err == nil {
		t.Fatal("expected error for nil key")
	}
}

func TestAESEncrypt_CiphertextDiffersFromPlaintext(t *testing.T) {
	key := DeriveKey([]byte("test"), PurposeEncrypt)
	plain := []byte("hello world")
	ct, err := AESEncrypt(plain, key)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}
	if bytes.Equal(ct, plain) {
		t.Fatal("ciphertext must differ from plaintext")
	}
}

func TestAESEncrypt_CiphertextIncludesNonce(t *testing.T) {
	key := DeriveKey([]byte("test"), PurposeEncrypt)
	plain := []byte("data")
	ct, err := AESEncrypt(plain, key)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}
	// GCM nonce is 12 bytes; ciphertext must be longer than plaintext+nonce.
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	minLen := gcm.NonceSize() + len(plain) + gcm.Overhead()
	if len(ct) != minLen {
		t.Fatalf("expected ciphertext length %d, got %d", minLen, len(ct))
	}
}

func TestAESEncrypt_InvalidKeySize(t *testing.T) {
	badKey := []byte("short")
	_, err := AESEncrypt([]byte("data"), badKey)
	if err == nil {
		t.Fatal("expected error for invalid key size")
	}
}

// ---------------------------------------------------------------------------
// AESDecrypt
// ---------------------------------------------------------------------------

func TestAESDecrypt_NilKey_Rejected(t *testing.T) {
	_, err := AESDecrypt([]byte("data"), nil)
	if err == nil {
		t.Fatal("expected error for nil key")
	}
}

func TestAESDecrypt_InvalidKeySize(t *testing.T) {
	_, err := AESDecrypt(make([]byte, 64), []byte("bad"))
	if err == nil {
		t.Fatal("expected error for invalid key size")
	}
}

func TestAESDecrypt_CiphertextTooShort(t *testing.T) {
	key := DeriveKey([]byte("test"), PurposeEncrypt)
	_, err := AESDecrypt([]byte{0x01, 0x02}, key)
	if err == nil {
		t.Fatal("expected 'ciphertext too short' error")
	}
}

func TestAESDecrypt_CorruptedCiphertext(t *testing.T) {
	key := DeriveKey([]byte("test"), PurposeEncrypt)
	ct, err := AESEncrypt([]byte("secret"), key)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}
	// Flip last byte to corrupt the GCM authentication tag.
	ct[len(ct)-1] ^= 0xff
	_, err = AESDecrypt(ct, key)
	if err == nil {
		t.Fatal("expected error when decrypting corrupted ciphertext")
	}
}

func TestAESDecrypt_WrongKey(t *testing.T) {
	keyA := DeriveKey([]byte("key-a"), PurposeEncrypt)
	keyB := DeriveKey([]byte("key-b"), PurposeEncrypt)
	ct, err := AESEncrypt([]byte("secret"), keyA)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}
	_, err = AESDecrypt(ct, keyB)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

// ---------------------------------------------------------------------------
// Roundtrip (encrypt then decrypt)
// ---------------------------------------------------------------------------

func TestRoundtrip_Basic(t *testing.T) {
	key := DeriveKey([]byte("roundtrip"), PurposeEncrypt)
	original := []byte("roundtrip test data")
	ct, err := AESEncrypt(original, key)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}
	got, err := AESDecrypt(ct, key)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("roundtrip mismatch: got %q, want %q", got, original)
	}
}

func TestRoundtrip_EmptyPlaintext(t *testing.T) {
	key := DeriveKey([]byte("empty"), PurposeEncrypt)
	original := []byte{}
	ct, err := AESEncrypt(original, key)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}
	got, err := AESDecrypt(ct, key)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty plaintext, got %d bytes", len(got))
	}
}

func TestRoundtrip_SmallData(t *testing.T) {
	key := DeriveKey([]byte("small"), PurposeEncrypt)
	original := []byte("x")
	ct, err := AESEncrypt(original, key)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}
	got, err := AESDecrypt(ct, key)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("roundtrip mismatch for single byte")
	}
}

func TestRoundtrip_LargeData(t *testing.T) {
	key := DeriveKey([]byte("large"), PurposeEncrypt)
	original := make([]byte, 1<<20) // 1 MB
	for i := range original {
		original[i] = byte(i % 251) // deterministic non-zero pattern
	}
	ct, err := AESEncrypt(original, key)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}
	got, err := AESDecrypt(ct, key)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Fatal("roundtrip mismatch for 1 MB payload")
	}
}

func TestRoundtrip_VariousSizes(t *testing.T) {
	key := DeriveKey([]byte("sizes"), PurposeEncrypt)
	sizes := []int{0, 1, 15, 16, 17, 31, 32, 33, 127, 256, 4096, 65535}
	for _, sz := range sizes {
		data := bytes.Repeat([]byte{0xAB}, sz)
		ct, err := AESEncrypt(data, key)
		if err != nil {
			t.Fatalf("encrypt error for size %d: %v", sz, err)
		}
		got, err := AESDecrypt(ct, key)
		if err != nil {
			t.Fatalf("decrypt error for size %d: %v", sz, err)
		}
		if !bytes.Equal(got, data) {
			t.Fatalf("roundtrip mismatch for size %d", sz)
		}
	}
}

func TestRoundtrip_BothPurposes(t *testing.T) {
	secret := []byte("multi-purpose")
	for _, purpose := range []string{PurposeAuth, PurposeEncrypt} {
		key := DeriveKey(secret, purpose)
		plain := []byte("test-" + purpose)
		ct, err := AESEncrypt(plain, key)
		if err != nil {
			t.Fatalf("encrypt error (%s): %v", purpose, err)
		}
		got, err := AESDecrypt(ct, key)
		if err != nil {
			t.Fatalf("decrypt error (%s): %v", purpose, err)
		}
		if !bytes.Equal(got, plain) {
			t.Fatalf("roundtrip mismatch for purpose %s", purpose)
		}
	}
}

func TestEncrypt_ProducesUniqueCiphertext(t *testing.T) {
	key := DeriveKey([]byte("unique"), PurposeEncrypt)
	plain := []byte("same input every time")
	ct1, err := AESEncrypt(plain, key)
	if err != nil {
		t.Fatalf("first encrypt: %v", err)
	}
	ct2, err := AESEncrypt(plain, key)
	if err != nil {
		t.Fatalf("second encrypt: %v", err)
	}
	if bytes.Equal(ct1, ct2) {
		t.Fatal("encrypting same plaintext twice should produce different ciphertexts (random nonce)")
	}
}
