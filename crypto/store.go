package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	storeVersion byte = 2
	argonTime         = 3
	argonMemory       = 256 * 1024 // KiB = 256 MiB
	argonThreads      = 4
	argonKeyLen       = 32
	saltLen           = 16

	v1ArgonMemory uint32 = 64 * 1024 // KiB = 64 MiB (RFC default)
)

// EncryptStore encrypts data with a passphrase via Argon2id + AES-256-GCM.
// Format: [1B version][16B salt][nonce+ciphertext+tag].
func EncryptStore(data, passphrase []byte) ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	key := argon2.IDKey(passphrase, salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	out := make([]byte, 0, 1+saltLen+len(ciphertext))
	out = append(out, storeVersion)
	out = append(out, salt...)
	out = append(out, ciphertext...)
	return out, nil
}

// DecryptStore decrypts data produced by EncryptStore.
func DecryptStore(encrypted, passphrase []byte) ([]byte, error) {
	if len(encrypted) < 1+saltLen+12+16 {
		return nil, errors.New("encrypted data too short")
	}
	ver := encrypted[0]
	var mem uint32
	switch ver {
	case 1:
		mem = v1ArgonMemory
	case 2:
		mem = argonMemory
	default:
		return nil, errors.New("unsupported store encryption version")
	}
	salt := encrypted[1 : 1+saltLen]
	payload := encrypted[1+saltLen:]

	key := argon2.IDKey(passphrase, salt, argonTime, mem, argonThreads, argonKeyLen)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(payload) < nonceSize {
		return nil, errors.New("encrypted payload too short")
	}
	return gcm.Open(nil, payload[:nonceSize], payload[nonceSize:], nil)
}
