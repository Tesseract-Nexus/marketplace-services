package encryption

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

// PIIEncryptor handles envelope encryption for PII data
type PIIEncryptor struct {
	gcpClient     *secretmanager.Client
	gcpProjectID  string
	keyCache      map[string]*cachedKey
	cacheMutex    sync.RWMutex
	cacheTTL      time.Duration
}

type cachedKey struct {
	key       []byte
	expiresAt time.Time
}

// NewPIIEncryptor creates a new PII encryptor with GCP KMS
func NewPIIEncryptor(ctx context.Context, gcpProjectID string) (*PIIEncryptor, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret manager client: %w", err)
	}

	return &PIIEncryptor{
		gcpClient:    client,
		gcpProjectID: gcpProjectID,
		keyCache:     make(map[string]*cachedKey),
		cacheTTL:     5 * time.Minute,
	}, nil
}

// Close closes the GCP client
func (e *PIIEncryptor) Close() error {
	return e.gcpClient.Close()
}

// EncryptedData represents encrypted PII data
type EncryptedData struct {
	Ciphertext  string `json:"ciphertext"`  // Base64 encoded encrypted data
	Nonce       string `json:"nonce"`       // Base64 encoded nonce
	KeyVersion  int    `json:"keyVersion"`  // Key version used
	Algorithm   string `json:"algorithm"`   // Encryption algorithm
	EncryptedAt int64  `json:"encryptedAt"` // Unix timestamp
}

// PIIFields represents common PII fields that need encryption
type PIIFields struct {
	Email       *string `json:"email,omitempty"`
	Phone       *string `json:"phone,omitempty"`
	FirstName   *string `json:"firstName,omitempty"`
	LastName    *string `json:"lastName,omitempty"`
	Address1    *string `json:"address1,omitempty"`
	Address2    *string `json:"address2,omitempty"`
	City        *string `json:"city,omitempty"`
	State       *string `json:"state,omitempty"`
	PostalCode  *string `json:"postalCode,omitempty"`
	Country     *string `json:"country,omitempty"`
	IPAddress   *string `json:"ipAddress,omitempty"`
	DateOfBirth *string `json:"dateOfBirth,omitempty"`
}

// GetDataEncryptionKey retrieves or generates a data encryption key for a tenant
func (e *PIIEncryptor) GetDataEncryptionKey(ctx context.Context, tenantID string) ([]byte, int, error) {
	cacheKey := fmt.Sprintf("dek_%s", tenantID)

	// Check cache first
	e.cacheMutex.RLock()
	if cached, ok := e.keyCache[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
		e.cacheMutex.RUnlock()
		return cached.key, 1, nil
	}
	e.cacheMutex.RUnlock()

	// Retrieve key from GCP Secret Manager
	secretName := fmt.Sprintf("projects/%s/secrets/tenant-%s-dek/versions/latest", e.gcpProjectID, tenantID)

	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretName,
	}

	result, err := e.gcpClient.AccessSecretVersion(ctx, req)
	if err != nil {
		// If key doesn't exist, generate a new one
		return e.generateAndStoreKey(ctx, tenantID)
	}

	key := result.Payload.Data

	// Cache the key
	e.cacheMutex.Lock()
	e.keyCache[cacheKey] = &cachedKey{
		key:       key,
		expiresAt: time.Now().Add(e.cacheTTL),
	}
	e.cacheMutex.Unlock()

	return key, 1, nil
}

// generateAndStoreKey generates a new DEK and stores it in Secret Manager
func (e *PIIEncryptor) generateAndStoreKey(ctx context.Context, tenantID string) ([]byte, int, error) {
	// Generate a 256-bit key
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, 0, fmt.Errorf("failed to generate key: %w", err)
	}

	// Store in Secret Manager
	secretID := fmt.Sprintf("tenant-%s-dek", tenantID)
	parent := fmt.Sprintf("projects/%s", e.gcpProjectID)

	// Create the secret
	createReq := &secretmanagerpb.CreateSecretRequest{
		Parent:   parent,
		SecretId: secretID,
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	}

	secret, err := e.gcpClient.CreateSecret(ctx, createReq)
	if err != nil {
		// Secret might already exist, try to add a version
		secret = &secretmanagerpb.Secret{
			Name: fmt.Sprintf("%s/secrets/%s", parent, secretID),
		}
	}

	// Add the secret version
	addReq := &secretmanagerpb.AddSecretVersionRequest{
		Parent: secret.Name,
		Payload: &secretmanagerpb.SecretPayload{
			Data: key,
		},
	}

	_, err = e.gcpClient.AddSecretVersion(ctx, addReq)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to store key: %w", err)
	}

	// Cache the key
	cacheKey := fmt.Sprintf("dek_%s", tenantID)
	e.cacheMutex.Lock()
	e.keyCache[cacheKey] = &cachedKey{
		key:       key,
		expiresAt: time.Now().Add(e.cacheTTL),
	}
	e.cacheMutex.Unlock()

	return key, 1, nil
}

// EncryptPII encrypts PII fields for a tenant
func (e *PIIEncryptor) EncryptPII(ctx context.Context, tenantID string, pii *PIIFields) (*EncryptedData, error) {
	if pii == nil {
		return nil, nil
	}

	// Get the data encryption key
	key, keyVersion, err := e.GetDataEncryptionKey(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get encryption key: %w", err)
	}

	// Serialize PII to JSON
	plaintext, err := json.Marshal(pii)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize PII: %w", err)
	}

	// Create AES-GCM cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	return &EncryptedData{
		Ciphertext:  base64.StdEncoding.EncodeToString(ciphertext),
		Nonce:       base64.StdEncoding.EncodeToString(nonce),
		KeyVersion:  keyVersion,
		Algorithm:   "AES-256-GCM",
		EncryptedAt: time.Now().Unix(),
	}, nil
}

// DecryptPII decrypts PII fields for a tenant
func (e *PIIEncryptor) DecryptPII(ctx context.Context, tenantID string, encrypted *EncryptedData) (*PIIFields, error) {
	if encrypted == nil {
		return nil, nil
	}

	// Get the data encryption key
	key, _, err := e.GetDataEncryptionKey(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get encryption key: %w", err)
	}

	// Decode ciphertext and nonce
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(encrypted.Nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to decode nonce: %w", err)
	}

	// Create AES-GCM cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	// Deserialize PII
	var pii PIIFields
	if err := json.Unmarshal(plaintext, &pii); err != nil {
		return nil, fmt.Errorf("failed to deserialize PII: %w", err)
	}

	return &pii, nil
}

// HashPII creates a one-way hash of PII for searching without exposing data
func (e *PIIEncryptor) HashPII(value string, salt string) string {
	hash := sha256.Sum256([]byte(value + salt))
	return base64.StdEncoding.EncodeToString(hash[:])
}

// MaskEmail masks an email address for display
func MaskEmail(email string) string {
	if len(email) < 5 {
		return "***"
	}
	atIndex := -1
	for i, c := range email {
		if c == '@' {
			atIndex = i
			break
		}
	}
	if atIndex < 1 {
		return "***"
	}

	// Show first char and domain
	return string(email[0]) + "***" + email[atIndex:]
}

// MaskPhone masks a phone number for display
func MaskPhone(phone string) string {
	if len(phone) < 4 {
		return "***"
	}
	return "***" + phone[len(phone)-4:]
}

// MaskName masks a name for display
func MaskName(name string) string {
	if len(name) < 2 {
		return "*"
	}
	return string(name[0]) + "***"
}
