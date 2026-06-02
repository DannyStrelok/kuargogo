package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/zalando/go-keyring"
	"golang.org/x/crypto/pbkdf2"
)

const (
	keyringService  = "kuargogo-cloud-sync"
	keyringUser     = "master-key"
	keySize         = 32 // AES-256
	pbkdfIterations = 600000
)

var (
	masterKeyCache     string
	masterKeyCacheErr  error
	masterKeyCacheDone bool
	masterKeyMutex     sync.RWMutex

	derivedKeyCache      map[string][]byte = make(map[string][]byte)
	derivedKeyCacheMutex sync.RWMutex
)

// StoreMasterKey saves the master passphrase in the OS keychain.
func StoreMasterKey(passphrase string) error {
	masterKeyMutex.Lock()
	masterKeyCache = passphrase
	masterKeyCacheErr = nil
	masterKeyCacheDone = true
	// Clear derived key cache as passphrase has changed
	derivedKeyCacheMutex.Lock()
	derivedKeyCache = make(map[string][]byte)
	derivedKeyCacheMutex.Unlock()
	masterKeyMutex.Unlock()

	return keyring.Set(keyringService, keyringUser, passphrase)
}

// GetMasterKey retrieves the master passphrase from the OS keychain or environment.
func GetMasterKey() (string, error) {
	// 1. Check environment variable first (for headless/distributed support)
	if envPass := os.Getenv("KGG_MASTER_PASSPHRASE"); envPass != "" {
		return envPass, nil
	}

	masterKeyMutex.RLock()
	if masterKeyCacheDone {
		val, err := masterKeyCache, masterKeyCacheErr
		masterKeyMutex.RUnlock()
		return val, err
	}
	masterKeyMutex.RUnlock()

	masterKeyMutex.Lock()
	defer masterKeyMutex.Unlock()

	// Double-check under write lock
	if masterKeyCacheDone {
		return masterKeyCache, masterKeyCacheErr
	}

	// 2. Fallback to OS Keychain
	val, err := keyring.Get(keyringService, keyringUser)
	if err == nil {
		masterKeyCache = val
		masterKeyCacheErr = nil
		masterKeyCacheDone = true
		return val, nil
	}

	// 3. Backward compatibility fallback: check legacy keyring services used under rk-cli
	legacyServices := []string{
		"rk-cloud-sync",
		"rkcli-cloud-sync",
		"rk-cli-cloud-sync",
	}
	for _, legacySvc := range legacyServices {
		if val, err = keyring.Get(legacySvc, keyringUser); err == nil {
			// Migrate the legacy key to the new keyringService immediately
			masterKeyCache = val
			masterKeyCacheErr = nil
			masterKeyCacheDone = true
			_ = keyring.Set(keyringService, keyringUser, val)
			return val, nil
		}
	}

	// Cache the error to prevent repeating failed keyring queries on startup
	masterKeyCache = ""
	masterKeyCacheErr = err
	masterKeyCacheDone = true
	return "", err
}

// ClearMasterKey removes the master passphrase from the OS keychain.
func ClearMasterKey() error {
	masterKeyMutex.Lock()
	masterKeyCache = ""
	masterKeyCacheErr = nil
	masterKeyCacheDone = true
	// Clear derived key cache
	derivedKeyCacheMutex.Lock()
	derivedKeyCache = make(map[string][]byte)
	derivedKeyCacheMutex.Unlock()
	masterKeyMutex.Unlock()

	return keyring.Delete(keyringService, keyringUser)
}

// deriveKey derives a 32-byte key from a passphrase and salt using PBKDF2-SHA256.
func deriveKey(passphrase string, salt []byte) []byte {
	saltStr := base64.StdEncoding.EncodeToString(salt)
	cacheKey := passphrase + "|" + saltStr

	derivedKeyCacheMutex.RLock()
	if cachedKey, ok := derivedKeyCache[cacheKey]; ok {
		derivedKeyCacheMutex.RUnlock()
		keyCopy := make([]byte, len(cachedKey))
		copy(keyCopy, cachedKey)
		return keyCopy
	}
	derivedKeyCacheMutex.RUnlock()

	derivedKeyCacheMutex.Lock()
	defer derivedKeyCacheMutex.Unlock()

	// Double-check under write lock
	if cachedKey, ok := derivedKeyCache[cacheKey]; ok {
		keyCopy := make([]byte, len(cachedKey))
		copy(keyCopy, cachedKey)
		return keyCopy
	}

	key := pbkdf2.Key([]byte(passphrase), salt, pbkdfIterations, keySize, sha256.New)
	derivedKeyCache[cacheKey] = key

	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)
	return keyCopy
}

// Encrypt encrypts plain text using AES-256-GCM with a passphrase-derived key.
func Encrypt(plainText []byte, passphrase string, salt []byte) ([]byte, error) {
	key := deriveKey(passphrase, salt)
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

	// Seal returns the encrypted data prefixed with the nonce
	return gcm.Seal(nonce, nonce, plainText, nil), nil
}

// Decrypt decrypts cipher text using AES-256-GCM with a passphrase-derived key.
func Decrypt(cipherText []byte, passphrase string, salt []byte) ([]byte, error) {
	key := deriveKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(cipherText) < nonceSize {
		return nil, fmt.Errorf("cipher text too short")
	}

	nonce, encryptedData := cipherText[:nonceSize], cipherText[nonceSize:]
	return gcm.Open(nil, nonce, encryptedData, nil)
}

// GenerateSalt creates a random 16-byte salt.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	return salt, nil
}

// GenerateRandomString creates a secure random string of a given length using base64 URL-safe encoding.
func GenerateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return "", err
	}
	// Use RawURLEncoding to avoid padding and special chars that might break CLI/YAML easily
	return base64.RawURLEncoding.EncodeToString(bytes)[:length], nil
}
