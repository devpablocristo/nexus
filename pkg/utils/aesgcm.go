package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

type AESGCM struct {
	key []byte
}

func NewAESGCM(masterKey string) (*AESGCM, error) {
	key, err := base64.StdEncoding.DecodeString(masterKey)
	if err != nil {
		return nil, fmt.Errorf("master key must be base64: %w", err)
	}
	if len(key) != 32 {
		return nil, errors.New("master key must decode to 32 bytes")
	}
	return &AESGCM{key: key}, nil
}

func (a *AESGCM) Encrypt(plaintext []byte) (ciphertext []byte, nonce []byte, err error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

func (a *AESGCM) Decrypt(ciphertext []byte, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}
