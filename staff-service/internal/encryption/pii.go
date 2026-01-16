package encryption

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

var (
	ErrInvalidCiphertext = errors.New("invalid ciphertext format")
	ErrDecryptionFailed  = errors.New("decryption failed")
	ErrKeyNotFound       = errors.New("encryption key not found")
	ErrInvalidKeyLength  = errors.New("encryption key must be 32 bytes for AES-256")
)

// PIIEncryptor handles encryption/decryption of PII data for staff service
type PIIEncryptor struct {
	key           []byte
	keyVersion    string
	mu            sync.RWMutex
	gcpClient     *secretmanager.Client
	projectID     string
	secretName    string
	lastRotation  time.Time
	rotationCheck time.Duration
}

// Config holds configuration for the PII encryptor
type Config struct {
	// GCP Secret Manager configuration
	GCPProjectID string
	SecretName   string

	// Local key for development (not for production use)
	LocalKey string

	// Key rotation check interval
	RotationCheckInterval time.Duration
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		GCPProjectID:          os.Getenv("GCP_PROJECT_ID"),
		SecretName:            os.Getenv("STAFF_PII_ENCRYPTION_SECRET"),
		LocalKey:              os.Getenv("STAFF_PII_LOCAL_KEY"),
		RotationCheckInterval: 1 * time.Hour,
	}
}

// NewPIIEncryptor creates a new PII encryptor
func NewPIIEncryptor(ctx context.Context, cfg Config) (*PIIEncryptor, error) {
	encryptor := &PIIEncryptor{
		projectID:     cfg.GCPProjectID,
		secretName:    cfg.SecretName,
		rotationCheck: cfg.RotationCheckInterval,
	}

	// Try GCP Secret Manager first (production)
	if cfg.GCPProjectID != "" && cfg.SecretName != "" {
		client, err := secretmanager.NewClient(ctx)
		if err == nil {
			encryptor.gcpClient = client
			if err := encryptor.loadKeyFromSecretManager(ctx); err == nil {
				return encryptor, nil
			}
		}
	}

	// Fall back to local key for development
	if cfg.LocalKey != "" {
		key, err := base64.StdEncoding.DecodeString(cfg.LocalKey)
		if err != nil {
			// Try hex decoding
			key, err = hex.DecodeString(cfg.LocalKey)
			if err != nil {
				return nil, fmt.Errorf("invalid local key format: %w", err)
			}
		}
		if len(key) != 32 {
			return nil, ErrInvalidKeyLength
		}
		encryptor.key = key
		encryptor.keyVersion = "local-dev"
		return encryptor, nil
	}

	return nil, ErrKeyNotFound
}

// loadKeyFromSecretManager loads the encryption key from GCP Secret Manager
func (e *PIIEncryptor) loadKeyFromSecretManager(ctx context.Context) error {
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", e.projectID, e.secretName)

	result, err := e.gcpClient.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	})
	if err != nil {
		return fmt.Errorf("failed to access secret: %w", err)
	}

	key := result.Payload.Data
	if len(key) != 32 {
		return ErrInvalidKeyLength
	}

	e.mu.Lock()
	e.key = key
	e.keyVersion = result.Name
	e.lastRotation = time.Now()
	e.mu.Unlock()

	return nil
}

// RefreshKey checks and refreshes the key if needed
func (e *PIIEncryptor) RefreshKey(ctx context.Context) error {
	e.mu.RLock()
	shouldRefresh := time.Since(e.lastRotation) > e.rotationCheck
	e.mu.RUnlock()

	if shouldRefresh && e.gcpClient != nil {
		return e.loadKeyFromSecretManager(ctx)
	}
	return nil
}

// Encrypt encrypts plaintext using AES-256-GCM
func (e *PIIEncryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	e.mu.RLock()
	key := e.key
	version := e.keyVersion
	e.mu.RUnlock()

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	encoded := base64.StdEncoding.EncodeToString(ciphertext)

	// Format: $enc$v1$keyVersion$ciphertext
	return fmt.Sprintf("$enc$v1$%s$%s", version, encoded), nil
}

// Decrypt decrypts ciphertext encrypted with Encrypt
func (e *PIIEncryptor) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	// Check if it's encrypted (starts with $enc$)
	if !strings.HasPrefix(ciphertext, "$enc$") {
		// Return as-is for backwards compatibility with unencrypted data
		return ciphertext, nil
	}

	parts := strings.Split(ciphertext, "$")
	if len(parts) != 5 {
		return "", ErrInvalidCiphertext
	}

	// parts[0] = "", parts[1] = "enc", parts[2] = "v1", parts[3] = keyVersion, parts[4] = ciphertext
	encoded := parts[4]

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	e.mu.RLock()
	key := e.key
	e.mu.RUnlock()

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", ErrInvalidCiphertext
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// HashForSearch creates a deterministic hash for searchable encryption
// This allows searching on encrypted fields without decrypting everything
func (e *PIIEncryptor) HashForSearch(value string) string {
	if value == "" {
		return ""
	}

	e.mu.RLock()
	key := e.key
	e.mu.RUnlock()

	// Use HMAC-like construction for search hash
	h := sha256.New()
	h.Write(key)
	h.Write([]byte(strings.ToLower(strings.TrimSpace(value))))
	return hex.EncodeToString(h.Sum(nil))
}

// MaskEmail masks an email address for display
func MaskEmail(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}
	local := parts[0]
	domain := parts[1]

	if len(local) <= 2 {
		return local[:1] + "***@" + domain
	}
	return local[:2] + "***@" + domain
}

// MaskPhone masks a phone number for display
func MaskPhone(phone string) string {
	if phone == "" {
		return ""
	}
	if len(phone) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(phone)-4) + phone[len(phone)-4:]
}

// MaskName masks a name for display
func MaskName(name string) string {
	if name == "" {
		return ""
	}
	if len(name) <= 1 {
		return "*"
	}
	return name[:1] + strings.Repeat("*", len(name)-1)
}

// MaskAddress masks an address for display
func MaskAddress(address string) string {
	if address == "" {
		return ""
	}
	words := strings.Fields(address)
	if len(words) == 0 {
		return "***"
	}
	// Keep first word, mask the rest
	if len(words) == 1 {
		return words[0][:1] + "***"
	}
	return words[0] + " ***"
}

// Close closes the GCP client
func (e *PIIEncryptor) Close() error {
	if e.gcpClient != nil {
		return e.gcpClient.Close()
	}
	return nil
}

// Global instance for convenience
var globalEncryptor *PIIEncryptor
var globalOnce sync.Once
var globalErr error

// Initialize initializes the global encryptor
func Initialize(ctx context.Context, cfg Config) error {
	globalOnce.Do(func() {
		globalEncryptor, globalErr = NewPIIEncryptor(ctx, cfg)
	})
	return globalErr
}

// GetEncryptor returns the global encryptor instance
func GetEncryptor() *PIIEncryptor {
	return globalEncryptor
}

// Encrypt encrypts using the global encryptor
func Encrypt(plaintext string) (string, error) {
	if globalEncryptor == nil {
		return "", errors.New("encryptor not initialized")
	}
	return globalEncryptor.Encrypt(plaintext)
}

// Decrypt decrypts using the global encryptor
func Decrypt(ciphertext string) (string, error) {
	if globalEncryptor == nil {
		return "", errors.New("encryptor not initialized")
	}
	return globalEncryptor.Decrypt(ciphertext)
}

// HashForSearch hashes using the global encryptor
func HashForSearch(value string) string {
	if globalEncryptor == nil {
		return ""
	}
	return globalEncryptor.HashForSearch(value)
}
