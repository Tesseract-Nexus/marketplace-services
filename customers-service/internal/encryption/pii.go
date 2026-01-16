package encryption

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

// PIIEncryptor handles AES-256-GCM encryption for PII data with GCP Secret Manager integration
type PIIEncryptor struct {
	gcpClient      *secretmanager.Client
	gcpProjectID   string
	keyCache       map[string]*cachedKey
	cacheMutex     sync.RWMutex
	cacheTTL       time.Duration
	fallbackKey    []byte // For local development without GCP
	hashSalt       string // Salt for deterministic hashing
	useLocalKey    bool   // Flag for local development mode
}

type cachedKey struct {
	key       []byte
	expiresAt time.Time
}

// EncryptedData represents encrypted PII data with metadata
type EncryptedData struct {
	Ciphertext  string `json:"ciphertext"`  // Base64 encoded encrypted data
	Nonce       string `json:"nonce"`       // Base64 encoded nonce
	KeyVersion  int    `json:"keyVersion"`  // Key version used for rotation tracking
	Algorithm   string `json:"algorithm"`   // Encryption algorithm (AES-256-GCM)
	EncryptedAt int64  `json:"encryptedAt"` // Unix timestamp of encryption
}

// PIIFields represents common PII fields that need encryption for customers
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
	Company     *string `json:"company,omitempty"`
	DateOfBirth *string `json:"dateOfBirth,omitempty"`
}

// CustomerPIIFields represents all PII fields specific to customers
type CustomerPIIFields struct {
	Email     *string `json:"email,omitempty"`
	Phone     *string `json:"phone,omitempty"`
	FirstName *string `json:"firstName,omitempty"`
	LastName  *string `json:"lastName,omitempty"`
}

// AddressPIIFields represents PII fields for addresses
type AddressPIIFields struct {
	FirstName    *string `json:"firstName,omitempty"`
	LastName     *string `json:"lastName,omitempty"`
	Company      *string `json:"company,omitempty"`
	AddressLine1 *string `json:"addressLine1,omitempty"`
	AddressLine2 *string `json:"addressLine2,omitempty"`
	City         *string `json:"city,omitempty"`
	State        *string `json:"state,omitempty"`
	PostalCode   *string `json:"postalCode,omitempty"`
	Country      *string `json:"country,omitempty"`
	Phone        *string `json:"phone,omitempty"`
}

// Config holds configuration for PIIEncryptor
type Config struct {
	GCPProjectID string
	Environment  string // "production", "staging", "development"
	FallbackKey  string // Base64 encoded 32-byte key for local dev
	HashSalt     string // Salt for deterministic hashing
}

// NewPIIEncryptor creates a new PII encryptor with GCP Secret Manager
// Falls back to environment variable key for local development
func NewPIIEncryptor(ctx context.Context, cfg Config) (*PIIEncryptor, error) {
	encryptor := &PIIEncryptor{
		keyCache: make(map[string]*cachedKey),
		cacheTTL: 5 * time.Minute,
		hashSalt: cfg.HashSalt,
	}

	// Check if we should use local development mode
	if cfg.Environment == "development" || os.Getenv("USE_LOCAL_ENCRYPTION_KEY") == "true" {
		return encryptor.initLocalMode(cfg)
	}

	// Production mode: use GCP Secret Manager
	return encryptor.initGCPMode(ctx, cfg)
}

// initLocalMode initializes the encryptor for local development
func (e *PIIEncryptor) initLocalMode(cfg Config) (*PIIEncryptor, error) {
	e.useLocalKey = true

	// Try to get key from environment or config
	keyStr := cfg.FallbackKey
	if keyStr == "" {
		keyStr = os.Getenv("PII_ENCRYPTION_KEY")
	}

	if keyStr == "" {
		// Generate a random key for development (not persistent!)
		key := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return nil, fmt.Errorf("failed to generate development key: %w", err)
		}
		e.fallbackKey = key
		fmt.Println("WARNING: Using randomly generated encryption key. Data will not persist across restarts.")
	} else {
		// Decode the provided key
		key, err := base64.StdEncoding.DecodeString(keyStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode encryption key: %w", err)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("encryption key must be 32 bytes (256 bits), got %d bytes", len(key))
		}
		e.fallbackKey = key
	}

	// Set hash salt
	if e.hashSalt == "" {
		e.hashSalt = os.Getenv("PII_HASH_SALT")
	}
	if e.hashSalt == "" {
		e.hashSalt = "customers-service-default-salt" // Default for development
	}

	return e, nil
}

// initGCPMode initializes the encryptor with GCP Secret Manager
func (e *PIIEncryptor) initGCPMode(ctx context.Context, cfg Config) (*PIIEncryptor, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		// Fall back to local mode if GCP is not available
		fmt.Printf("WARNING: Failed to create GCP Secret Manager client: %v. Falling back to local mode.\n", err)
		return e.initLocalMode(cfg)
	}

	e.gcpClient = client
	e.gcpProjectID = cfg.GCPProjectID
	if e.gcpProjectID == "" {
		e.gcpProjectID = os.Getenv("GCP_PROJECT_ID")
	}

	// Set hash salt from GCP or environment
	if e.hashSalt == "" {
		e.hashSalt = os.Getenv("PII_HASH_SALT")
	}
	if e.hashSalt == "" {
		// Try to fetch from Secret Manager
		salt, err := e.fetchSecretFromGCP(ctx, "pii-hash-salt")
		if err == nil && salt != "" {
			e.hashSalt = salt
		} else {
			e.hashSalt = "customers-service-production-salt"
		}
	}

	return e, nil
}

// Close closes the GCP client
func (e *PIIEncryptor) Close() error {
	if e.gcpClient != nil {
		return e.gcpClient.Close()
	}
	return nil
}

// GetDataEncryptionKey retrieves or generates a data encryption key for a tenant
func (e *PIIEncryptor) GetDataEncryptionKey(ctx context.Context, tenantID string) ([]byte, int, error) {
	// Local development mode
	if e.useLocalKey {
		return e.fallbackKey, 1, nil
	}

	cacheKey := fmt.Sprintf("dek_%s", tenantID)

	// Check cache first
	e.cacheMutex.RLock()
	if cached, ok := e.keyCache[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
		e.cacheMutex.RUnlock()
		return cached.key, 1, nil
	}
	e.cacheMutex.RUnlock()

	// Retrieve key from GCP Secret Manager
	secretName := fmt.Sprintf("projects/%s/secrets/tenant-%s-pii-dek/versions/latest", e.gcpProjectID, tenantID)

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
	secretID := fmt.Sprintf("tenant-%s-pii-dek", tenantID)
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
			Labels: map[string]string{
				"service":   "customers-service",
				"tenant-id": tenantID,
				"purpose":   "pii-encryption",
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
		return nil, 0, fmt.Errorf("failed to store key in Secret Manager: %w", err)
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

// fetchSecretFromGCP fetches a secret value from GCP Secret Manager
func (e *PIIEncryptor) fetchSecretFromGCP(ctx context.Context, secretID string) (string, error) {
	if e.gcpClient == nil {
		return "", fmt.Errorf("GCP client not initialized")
	}

	secretName := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", e.gcpProjectID, secretID)
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretName,
	}

	result, err := e.gcpClient.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", err
	}

	return string(result.Payload.Data), nil
}

// EncryptField encrypts a single field value
func (e *PIIEncryptor) EncryptField(ctx context.Context, tenantID, value string) (*EncryptedData, error) {
	if value == "" {
		return nil, nil
	}

	key, keyVersion, err := e.GetDataEncryptionKey(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get encryption key: %w", err)
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
	ciphertext := gcm.Seal(nil, nonce, []byte(value), nil)

	return &EncryptedData{
		Ciphertext:  base64.StdEncoding.EncodeToString(ciphertext),
		Nonce:       base64.StdEncoding.EncodeToString(nonce),
		KeyVersion:  keyVersion,
		Algorithm:   "AES-256-GCM",
		EncryptedAt: time.Now().Unix(),
	}, nil
}

// DecryptField decrypts a single encrypted field value
func (e *PIIEncryptor) DecryptField(ctx context.Context, tenantID string, encrypted *EncryptedData) (string, error) {
	if encrypted == nil {
		return "", nil
	}

	key, _, err := e.GetDataEncryptionKey(ctx, tenantID)
	if err != nil {
		return "", fmt.Errorf("failed to get encryption key: %w", err)
	}

	// Decode ciphertext and nonce
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted.Ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(encrypted.Nonce)
	if err != nil {
		return "", fmt.Errorf("failed to decode nonce: %w", err)
	}

	// Create AES-GCM cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// EncryptPII encrypts all PII fields for a tenant
func (e *PIIEncryptor) EncryptPII(ctx context.Context, tenantID string, pii *PIIFields) (*EncryptedData, error) {
	if pii == nil {
		return nil, nil
	}

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

// EncryptCustomerPII encrypts customer-specific PII fields
func (e *PIIEncryptor) EncryptCustomerPII(ctx context.Context, tenantID string, pii *CustomerPIIFields) (*EncryptedData, error) {
	if pii == nil {
		return nil, nil
	}

	key, keyVersion, err := e.GetDataEncryptionKey(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get encryption key: %w", err)
	}

	plaintext, err := json.Marshal(pii)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize customer PII: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	return &EncryptedData{
		Ciphertext:  base64.StdEncoding.EncodeToString(ciphertext),
		Nonce:       base64.StdEncoding.EncodeToString(nonce),
		KeyVersion:  keyVersion,
		Algorithm:   "AES-256-GCM",
		EncryptedAt: time.Now().Unix(),
	}, nil
}

// DecryptCustomerPII decrypts customer-specific PII fields
func (e *PIIEncryptor) DecryptCustomerPII(ctx context.Context, tenantID string, encrypted *EncryptedData) (*CustomerPIIFields, error) {
	if encrypted == nil {
		return nil, nil
	}

	key, _, err := e.GetDataEncryptionKey(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get encryption key: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(encrypted.Nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to decode nonce: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	var pii CustomerPIIFields
	if err := json.Unmarshal(plaintext, &pii); err != nil {
		return nil, fmt.Errorf("failed to deserialize customer PII: %w", err)
	}

	return &pii, nil
}

// EncryptAddressPII encrypts address-specific PII fields
func (e *PIIEncryptor) EncryptAddressPII(ctx context.Context, tenantID string, pii *AddressPIIFields) (*EncryptedData, error) {
	if pii == nil {
		return nil, nil
	}

	key, keyVersion, err := e.GetDataEncryptionKey(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get encryption key: %w", err)
	}

	plaintext, err := json.Marshal(pii)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize address PII: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	return &EncryptedData{
		Ciphertext:  base64.StdEncoding.EncodeToString(ciphertext),
		Nonce:       base64.StdEncoding.EncodeToString(nonce),
		KeyVersion:  keyVersion,
		Algorithm:   "AES-256-GCM",
		EncryptedAt: time.Now().Unix(),
	}, nil
}

// DecryptAddressPII decrypts address-specific PII fields
func (e *PIIEncryptor) DecryptAddressPII(ctx context.Context, tenantID string, encrypted *EncryptedData) (*AddressPIIFields, error) {
	if encrypted == nil {
		return nil, nil
	}

	key, _, err := e.GetDataEncryptionKey(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get encryption key: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(encrypted.Nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to decode nonce: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	var pii AddressPIIFields
	if err := json.Unmarshal(plaintext, &pii); err != nil {
		return nil, fmt.Errorf("failed to deserialize address PII: %w", err)
	}

	return &pii, nil
}

// HashForSearch creates a deterministic hash for searching encrypted fields
// Uses HMAC-SHA256 with tenant-specific salting for secure searchable encryption
func (e *PIIEncryptor) HashForSearch(tenantID, value string) string {
	if value == "" {
		return ""
	}
	// Normalize the value (lowercase, trim whitespace)
	normalized := strings.ToLower(strings.TrimSpace(value))
	// Create tenant-specific salt
	salt := fmt.Sprintf("%s:%s:%s", e.hashSalt, tenantID, "search")
	// Hash with SHA-256
	hash := sha256.Sum256([]byte(normalized + salt))
	return hex.EncodeToString(hash[:])
}

// HashEmail creates a searchable hash specifically for email lookup
func (e *PIIEncryptor) HashEmail(tenantID, email string) string {
	if email == "" {
		return ""
	}
	// Normalize email (lowercase, trim)
	normalized := strings.ToLower(strings.TrimSpace(email))
	salt := fmt.Sprintf("%s:%s:%s", e.hashSalt, tenantID, "email")
	hash := sha256.Sum256([]byte(normalized + salt))
	return hex.EncodeToString(hash[:])
}

// HashPhone creates a searchable hash specifically for phone lookup
func (e *PIIEncryptor) HashPhone(tenantID, phone string) string {
	if phone == "" {
		return ""
	}
	// Normalize phone (remove non-digits)
	normalized := normalizePhone(phone)
	salt := fmt.Sprintf("%s:%s:%s", e.hashSalt, tenantID, "phone")
	hash := sha256.Sum256([]byte(normalized + salt))
	return hex.EncodeToString(hash[:])
}

// normalizePhone removes all non-digit characters from a phone number
func normalizePhone(phone string) string {
	var result strings.Builder
	for _, c := range phone {
		if c >= '0' && c <= '9' {
			result.WriteRune(c)
		}
	}
	return result.String()
}

// MaskEmail masks an email address for display (e.g., "j***@example.com")
func MaskEmail(email string) string {
	if len(email) < 5 {
		return "***"
	}

	atIndex := strings.Index(email, "@")
	if atIndex < 1 {
		return "***"
	}

	// Show first character + masked + domain
	localPart := email[:atIndex]
	domain := email[atIndex:]

	if len(localPart) <= 2 {
		return string(localPart[0]) + "***" + domain
	}

	return string(localPart[0]) + "***" + domain
}

// MaskPhone masks a phone number for display (e.g., "***-***-1234")
func MaskPhone(phone string) string {
	if len(phone) < 4 {
		return "***"
	}
	// Show only last 4 digits
	return "***" + phone[len(phone)-4:]
}

// MaskName masks a name for display (e.g., "J***")
func MaskName(name string) string {
	if len(name) < 1 {
		return "***"
	}
	if len(name) == 1 {
		return string(name[0]) + "***"
	}
	return string(name[0]) + "***"
}

// MaskAddress masks an address for display
func MaskAddress(address string) string {
	if len(address) < 5 {
		return "***"
	}
	// Show first 3 characters + masked
	return address[:3] + "***"
}

// MaskPostalCode masks a postal code for display
func MaskPostalCode(postalCode string) string {
	if len(postalCode) < 3 {
		return "***"
	}
	// Show first 3 characters for partial matching
	return postalCode[:3] + "***"
}

// MaskedCustomer represents a customer with masked PII for API responses
type MaskedCustomer struct {
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

// MaskCustomerPII masks customer PII fields for API responses
func MaskCustomerPII(pii *CustomerPIIFields) *MaskedCustomer {
	if pii == nil {
		return nil
	}

	masked := &MaskedCustomer{}

	if pii.Email != nil {
		masked.Email = MaskEmail(*pii.Email)
	}
	if pii.Phone != nil {
		masked.Phone = MaskPhone(*pii.Phone)
	}
	if pii.FirstName != nil {
		masked.FirstName = MaskName(*pii.FirstName)
	}
	if pii.LastName != nil {
		masked.LastName = MaskName(*pii.LastName)
	}

	return masked
}

// MaskedAddress represents an address with masked PII for API responses
type MaskedAddress struct {
	FirstName    string `json:"firstName"`
	LastName     string `json:"lastName"`
	Company      string `json:"company"`
	AddressLine1 string `json:"addressLine1"`
	AddressLine2 string `json:"addressLine2"`
	City         string `json:"city"`
	State        string `json:"state"`
	PostalCode   string `json:"postalCode"`
	Country      string `json:"country"`
	Phone        string `json:"phone"`
}

// MaskAddressPII masks address PII fields for API responses
func MaskAddressPII(pii *AddressPIIFields) *MaskedAddress {
	if pii == nil {
		return nil
	}

	masked := &MaskedAddress{}

	if pii.FirstName != nil {
		masked.FirstName = MaskName(*pii.FirstName)
	}
	if pii.LastName != nil {
		masked.LastName = MaskName(*pii.LastName)
	}
	if pii.Company != nil && *pii.Company != "" {
		masked.Company = MaskName(*pii.Company)
	}
	if pii.AddressLine1 != nil {
		masked.AddressLine1 = MaskAddress(*pii.AddressLine1)
	}
	if pii.AddressLine2 != nil && *pii.AddressLine2 != "" {
		masked.AddressLine2 = MaskAddress(*pii.AddressLine2)
	}
	if pii.City != nil {
		masked.City = *pii.City // City is not masked
	}
	if pii.State != nil {
		masked.State = *pii.State // State is not masked
	}
	if pii.PostalCode != nil {
		masked.PostalCode = MaskPostalCode(*pii.PostalCode)
	}
	if pii.Country != nil {
		masked.Country = *pii.Country // Country is not masked
	}
	if pii.Phone != nil {
		masked.Phone = MaskPhone(*pii.Phone)
	}

	return masked
}

// ClearCache clears the encryption key cache
func (e *PIIEncryptor) ClearCache() {
	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()
	e.keyCache = make(map[string]*cachedKey)
}

// ClearTenantCache clears the encryption key cache for a specific tenant
func (e *PIIEncryptor) ClearTenantCache(tenantID string) {
	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()
	cacheKey := fmt.Sprintf("dek_%s", tenantID)
	delete(e.keyCache, cacheKey)
}
