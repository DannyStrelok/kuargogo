package config

import (
	"bytes"
	"os"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestMain(m *testing.M) {
	// CRITICAL SAFEGUARD: Always force the mock keyring during tests
	// to prevent writing or reading from the host OS keyring database.
	keyring.MockInit()
	os.Exit(m.Run())
}

func TestEncryptDecrypt(t *testing.T) {
	passphrase := "my-secret-passphrase"
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("failed to generate salt: %v", err)
	}

	plaintext := []byte("hello world")
	ciphertext, err := Encrypt(plaintext, passphrase, salt)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	decrypted, err := Decrypt(ciphertext, passphrase, salt)
	if err != nil {
		t.Fatalf("failed to decrypt: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("expected %s, got %s", plaintext, decrypted)
	}
}

func TestCryptoCaching(t *testing.T) {
	// Initialize in-memory mock keyring so we don't overwrite the host OS keyring
	keyring.MockInit()

	passphrase := "caching-test-passphrase"
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("failed to generate salt: %v", err)
	}

	// Invalidate caches before starting test to have clean state
	derivedKeyCacheMutex.Lock()
	derivedKeyCache = make(map[string][]byte)
	derivedKeyCacheMutex.Unlock()

	masterKeyMutex.Lock()
	masterKeyCacheDone = false
	masterKeyMutex.Unlock()

	// First derivation should populate the cache
	key1 := deriveKey(passphrase, salt)
	if len(key1) != keySize {
		t.Fatalf("expected key size %d, got %d", keySize, len(key1))
	}

	derivedKeyCacheMutex.RLock()
	cacheSize := len(derivedKeyCache)
	derivedKeyCacheMutex.RUnlock()

	if cacheSize == 0 {
		t.Error("expected derived key cache to contain entries after derivation")
	}

	// Second derivation should hit the cache
	key2 := deriveKey(passphrase, salt)
	if !bytes.Equal(key1, key2) {
		t.Error("expected keys to be equal")
	}

	// Clear cache using StoreMasterKey / ClearMasterKey to verify invalidation works
	_ = StoreMasterKey("new-passphrase")

	derivedKeyCacheMutex.RLock()
	cacheSizeAfterStore := len(derivedKeyCache)
	derivedKeyCacheMutex.RUnlock()

	if cacheSizeAfterStore != 0 {
		t.Errorf("expected derived key cache to be cleared after StoreMasterKey, but got size %d", cacheSizeAfterStore)
	}
}
