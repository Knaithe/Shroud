package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptStore(t *testing.T) {
	data := []byte(`{"key":"value","secret":"deadbeef"}`)
	pass := []byte("my-passphrase")

	enc, err := EncryptStore(data, pass)
	if err != nil {
		t.Fatalf("EncryptStore: %v", err)
	}
	if enc[0] != storeVersion {
		t.Fatalf("expected version %d, got %d", storeVersion, enc[0])
	}

	dec, err := DecryptStore(enc, pass)
	if err != nil {
		t.Fatalf("DecryptStore: %v", err)
	}
	if !bytes.Equal(dec, data) {
		t.Fatalf("decrypted data mismatch: got %q", dec)
	}
}

func TestDecryptStore_WrongPassphrase(t *testing.T) {
	data := []byte("secret data")
	enc, _ := EncryptStore(data, []byte("correct"))
	_, err := DecryptStore(enc, []byte("wrong"))
	if err == nil {
		t.Fatal("expected error with wrong passphrase")
	}
}

func TestDecryptStore_TooShort(t *testing.T) {
	_, err := DecryptStore([]byte{1, 2, 3}, []byte("pass"))
	if err == nil {
		t.Fatal("expected error for short data")
	}
}
