// Package secure 保存需要回读但不应明文落库的凭据。
package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

const prefix = "enc:v1:"

var associatedData = []byte("folio-storage-secret-v1")

func key(secret string) [32]byte {
	return sha256.Sum256([]byte(strings.TrimSpace(secret)))
}

// Encrypt 使用站点密钥派生出的 AES-GCM 密钥加密凭据。
func Encrypt(secret, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	derived := key(secret)
	block, err := aes.NewCipher(derived[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), associatedData)
	return prefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt 解密后台保存的凭据；没有前缀的值视为历史明文，原样返回。
func Decrypt(secret, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, prefix) {
		return value, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return "", errors.New("凭据格式损坏")
	}
	derived := key(secret)
	block, err := aes.NewCipher(derived[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("凭据长度不正确")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, associatedData)
	if err != nil {
		return "", errors.New("凭据无法解密，请重新填写密钥")
	}
	return string(plain), nil
}
