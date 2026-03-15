package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// EncryptFileInfo encrypts the bot token and file ID into a URL-safe string.
func EncryptFileInfo(botToken, fileID string) (string, error) {
	secretKey := []byte(getEncryptionKey())

	// Combine the values with a delimiter.
	plaintext := []byte(fmt.Sprintf("%s|%s", botToken, fileID))

	// Create the cipher block.
	block, err := aes.NewCipher(secretKey)
	if err != nil {
		return "", err
	}

	// Create a GCM cipher mode.
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Generate a nonce.
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Encrypt the plaintext.
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)

	// Encode as base64 for safe URL transport.
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// DecryptFileInfo decrypts the encoded string and returns bot token and file ID.
func DecryptFileInfo(encryptedData string) (botToken, fileID string, err error) {
	secretKey := []byte(getEncryptionKey())

	// Decode the base64 payload.
	ciphertext, err := base64.URLEncoding.DecodeString(encryptedData)
	if err != nil {
		return "", "", err
	}

	// Create the cipher block.
	block, err := aes.NewCipher(secretKey)
	if err != nil {
		return "", "", err
	}

	// Create a GCM cipher mode.
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}

	// Determine the nonce size.
	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", "", errors.New("ciphertext too short")
	}

	// Split nonce and ciphertext.
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt the payload.
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", "", err
	}

	// Extract bot token and file ID.
	parts := strings.Split(string(plaintext), "|")
	if len(parts) != 2 {
		return "", "", errors.New("invalid data format after decryption")
	}

	return parts[0], parts[1], nil
}

// getEncryptionKey loads the key from the environment or falls back to a default.
func getEncryptionKey() string {
	key := os.Getenv("ENCRYPTION_KEY")
	if key == "" {
		// Use a default 32-byte key for AES-256 in local development.
		// In production, always provide the key via environment variable.
		key = "telerealm-default-encryption-key-32b"
	}

	// Ensure the key has the correct length for AES-256.
	if len(key) < 32 {
		key = key + strings.Repeat("0", 32-len(key))
	}

	return key[:32]
}
