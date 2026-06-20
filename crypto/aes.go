package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
)

const (
	PurposeAuth    = "shroud-auth-v2"
	PurposeEncrypt = "shroud-encrypt-v2"
)

func DeriveKey(secret []byte, purpose string) []byte {
	if len(secret) == 0 {
		return nil
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(purpose))
	return mac.Sum(nil)
}

func genNonce(nonceSize int) ([]byte, error) {
	nonce := make([]byte, nonceSize)
	_, err := io.ReadFull(rand.Reader, nonce)
	return nonce, err
}

func AESDecrypt(cryptedData, key []byte) ([]byte, error) {
	if key == nil {
		return cryptedData, nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(cryptedData) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, cryptedData := cryptedData[:nonceSize], cryptedData[nonceSize:]
	origData, err := gcm.Open(nil, nonce, cryptedData, nil)
	if err != nil {
		return nil, err
	}
	return origData, nil
}

func AESEncrypt(origData, key []byte) ([]byte, error) {
	if key == nil {
		return origData, nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce, err := genNonce(gcm.NonceSize())
	if err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, origData, nil), nil
}
