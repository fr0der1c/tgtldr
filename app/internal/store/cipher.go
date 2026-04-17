package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

type Cipher struct {
	gcm cipher.AEAD
}

func NewCipher(key []byte) (Cipher, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return Cipher{}, fmt.Errorf("new aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return Cipher{}, fmt.Errorf("new gcm: %w", err)
	}
	return Cipher{gcm: gcm}, nil
}

func (c Cipher) EncryptString(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("read nonce: %w", err)
	}
	payload := c.gcm.Seal(nonce, nonce, []byte(value), nil)
	return base64.StdEncoding.EncodeToString(payload), nil
}

func (c Cipher) DecryptString(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	payload, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("decode secret: %w", err)
	}
	size := c.gcm.NonceSize()
	if len(payload) < size {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce := payload[:size]
	body := payload[size:]
	plain, err := c.gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plain), nil
}

func (c Cipher) EncryptBytes(value []byte) (string, error) {
	return c.EncryptString(string(value))
}

func (c Cipher) DecryptBytes(value string) ([]byte, error) {
	plain, err := c.DecryptString(value)
	if err != nil {
		return nil, err
	}
	return []byte(plain), nil
}
