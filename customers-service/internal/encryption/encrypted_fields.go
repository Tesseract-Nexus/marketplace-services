package encryption

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"sync"
)

// Global encryptor instance - must be initialized before use
var (
	globalEncryptor *PIIEncryptor
	encryptorMutex  sync.RWMutex
)

// InitGlobalEncryptor initializes the global PII encryptor
// Must be called during application startup
func InitGlobalEncryptor(ctx context.Context, cfg Config) error {
	encryptorMutex.Lock()
	defer encryptorMutex.Unlock()

	encryptor, err := NewPIIEncryptor(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize global encryptor: %w", err)
	}

	globalEncryptor = encryptor
	return nil
}

// GetGlobalEncryptor returns the global PII encryptor instance
func GetGlobalEncryptor() *PIIEncryptor {
	encryptorMutex.RLock()
	defer encryptorMutex.RUnlock()
	return globalEncryptor
}

// CloseGlobalEncryptor closes the global encryptor and releases resources
func CloseGlobalEncryptor() error {
	encryptorMutex.Lock()
	defer encryptorMutex.Unlock()

	if globalEncryptor != nil {
		err := globalEncryptor.Close()
		globalEncryptor = nil
		return err
	}
	return nil
}

// EncryptedString is a custom type for storing encrypted strings in the database
// It handles encryption on write and decryption on read transparently
type EncryptedString struct {
	Plaintext  string         // Decrypted value (not stored in DB)
	Encrypted  *EncryptedData // Encrypted value (stored in DB as JSON)
	TenantID   string         // Tenant ID for key retrieval
	SearchHash string         // Deterministic hash for searching
}

// Value implements the driver.Valuer interface for database writes
func (e EncryptedString) Value() (driver.Value, error) {
	if e.Encrypted == nil {
		return nil, nil
	}
	return json.Marshal(e.Encrypted)
}

// Scan implements the sql.Scanner interface for database reads
func (e *EncryptedString) Scan(value interface{}) error {
	if value == nil {
		e.Encrypted = nil
		e.Plaintext = ""
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("unsupported type for EncryptedString: %T", value)
	}

	var encrypted EncryptedData
	if err := json.Unmarshal(bytes, &encrypted); err != nil {
		return fmt.Errorf("failed to unmarshal encrypted data: %w", err)
	}

	e.Encrypted = &encrypted
	return nil
}

// Encrypt encrypts the plaintext value using the global encryptor
func (e *EncryptedString) Encrypt(ctx context.Context) error {
	if e.Plaintext == "" {
		e.Encrypted = nil
		e.SearchHash = ""
		return nil
	}

	encryptor := GetGlobalEncryptor()
	if encryptor == nil {
		return fmt.Errorf("global encryptor not initialized")
	}

	encrypted, err := encryptor.EncryptField(ctx, e.TenantID, e.Plaintext)
	if err != nil {
		return fmt.Errorf("failed to encrypt field: %w", err)
	}

	e.Encrypted = encrypted
	return nil
}

// Decrypt decrypts the encrypted value using the global encryptor
func (e *EncryptedString) Decrypt(ctx context.Context) error {
	if e.Encrypted == nil {
		e.Plaintext = ""
		return nil
	}

	encryptor := GetGlobalEncryptor()
	if encryptor == nil {
		return fmt.Errorf("global encryptor not initialized")
	}

	plaintext, err := encryptor.DecryptField(ctx, e.TenantID, e.Encrypted)
	if err != nil {
		return fmt.Errorf("failed to decrypt field: %w", err)
	}

	e.Plaintext = plaintext
	return nil
}

// SetPlaintext sets the plaintext value and marks for encryption
func (e *EncryptedString) SetPlaintext(value, tenantID string) {
	e.Plaintext = value
	e.TenantID = tenantID
	e.Encrypted = nil // Clear encrypted value to trigger re-encryption
}

// GetPlaintext returns the decrypted plaintext value
func (e *EncryptedString) GetPlaintext() string {
	return e.Plaintext
}

// EncryptedEmail is a specialized encrypted field for email addresses
// Includes a search hash for email lookup queries
type EncryptedEmail struct {
	EncryptedString
}

// Encrypt encrypts the email and generates a search hash
func (e *EncryptedEmail) Encrypt(ctx context.Context) error {
	if err := e.EncryptedString.Encrypt(ctx); err != nil {
		return err
	}

	// Generate search hash for email lookup
	encryptor := GetGlobalEncryptor()
	if encryptor != nil && e.Plaintext != "" {
		e.SearchHash = encryptor.HashEmail(e.TenantID, e.Plaintext)
	}

	return nil
}

// EncryptedPhone is a specialized encrypted field for phone numbers
// Includes a search hash for phone lookup queries
type EncryptedPhone struct {
	EncryptedString
}

// Encrypt encrypts the phone and generates a search hash
func (e *EncryptedPhone) Encrypt(ctx context.Context) error {
	if err := e.EncryptedString.Encrypt(ctx); err != nil {
		return err
	}

	// Generate search hash for phone lookup
	encryptor := GetGlobalEncryptor()
	if encryptor != nil && e.Plaintext != "" {
		e.SearchHash = encryptor.HashPhone(e.TenantID, e.Plaintext)
	}

	return nil
}

// EncryptedCustomerData stores all encrypted PII for a customer in a single column
type EncryptedCustomerData struct {
	Data     *EncryptedData     `json:"-" gorm:"type:jsonb;column:encrypted_pii"`
	PII      *CustomerPIIFields `json:"-" gorm:"-"` // Decrypted PII (not stored)
	TenantID string             `json:"-" gorm:"-"` // Tenant ID for key retrieval
}

// Value implements the driver.Valuer interface
func (e EncryptedCustomerData) Value() (driver.Value, error) {
	if e.Data == nil {
		return nil, nil
	}
	return json.Marshal(e.Data)
}

// Scan implements the sql.Scanner interface
func (e *EncryptedCustomerData) Scan(value interface{}) error {
	if value == nil {
		e.Data = nil
		e.PII = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("unsupported type for EncryptedCustomerData: %T", value)
	}

	var encrypted EncryptedData
	if err := json.Unmarshal(bytes, &encrypted); err != nil {
		return fmt.Errorf("failed to unmarshal encrypted customer data: %w", err)
	}

	e.Data = &encrypted
	return nil
}

// Encrypt encrypts the customer PII
func (e *EncryptedCustomerData) Encrypt(ctx context.Context) error {
	if e.PII == nil {
		e.Data = nil
		return nil
	}

	encryptor := GetGlobalEncryptor()
	if encryptor == nil {
		return fmt.Errorf("global encryptor not initialized")
	}

	encrypted, err := encryptor.EncryptCustomerPII(ctx, e.TenantID, e.PII)
	if err != nil {
		return fmt.Errorf("failed to encrypt customer PII: %w", err)
	}

	e.Data = encrypted
	return nil
}

// Decrypt decrypts the customer PII
func (e *EncryptedCustomerData) Decrypt(ctx context.Context) error {
	if e.Data == nil {
		e.PII = nil
		return nil
	}

	encryptor := GetGlobalEncryptor()
	if encryptor == nil {
		return fmt.Errorf("global encryptor not initialized")
	}

	pii, err := encryptor.DecryptCustomerPII(ctx, e.TenantID, e.Data)
	if err != nil {
		return fmt.Errorf("failed to decrypt customer PII: %w", err)
	}

	e.PII = pii
	return nil
}

// SetPII sets the decrypted PII data
func (e *EncryptedCustomerData) SetPII(pii *CustomerPIIFields, tenantID string) {
	e.PII = pii
	e.TenantID = tenantID
	e.Data = nil // Clear encrypted data to trigger re-encryption
}

// GetPII returns the decrypted PII data
func (e *EncryptedCustomerData) GetPII() *CustomerPIIFields {
	return e.PII
}

// EncryptedAddressData stores all encrypted PII for an address in a single column
type EncryptedAddressData struct {
	Data     *EncryptedData    `json:"-" gorm:"type:jsonb;column:encrypted_pii"`
	PII      *AddressPIIFields `json:"-" gorm:"-"` // Decrypted PII (not stored)
	TenantID string            `json:"-" gorm:"-"` // Tenant ID for key retrieval
}

// Value implements the driver.Valuer interface
func (e EncryptedAddressData) Value() (driver.Value, error) {
	if e.Data == nil {
		return nil, nil
	}
	return json.Marshal(e.Data)
}

// Scan implements the sql.Scanner interface
func (e *EncryptedAddressData) Scan(value interface{}) error {
	if value == nil {
		e.Data = nil
		e.PII = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("unsupported type for EncryptedAddressData: %T", value)
	}

	var encrypted EncryptedData
	if err := json.Unmarshal(bytes, &encrypted); err != nil {
		return fmt.Errorf("failed to unmarshal encrypted address data: %w", err)
	}

	e.Data = &encrypted
	return nil
}

// Encrypt encrypts the address PII
func (e *EncryptedAddressData) Encrypt(ctx context.Context) error {
	if e.PII == nil {
		e.Data = nil
		return nil
	}

	encryptor := GetGlobalEncryptor()
	if encryptor == nil {
		return fmt.Errorf("global encryptor not initialized")
	}

	encrypted, err := encryptor.EncryptAddressPII(ctx, e.TenantID, e.PII)
	if err != nil {
		return fmt.Errorf("failed to encrypt address PII: %w", err)
	}

	e.Data = encrypted
	return nil
}

// Decrypt decrypts the address PII
func (e *EncryptedAddressData) Decrypt(ctx context.Context) error {
	if e.Data == nil {
		e.PII = nil
		return nil
	}

	encryptor := GetGlobalEncryptor()
	if encryptor == nil {
		return fmt.Errorf("global encryptor not initialized")
	}

	pii, err := encryptor.DecryptAddressPII(ctx, e.TenantID, e.Data)
	if err != nil {
		return fmt.Errorf("failed to decrypt address PII: %w", err)
	}

	e.PII = pii
	return nil
}

// SetPII sets the decrypted PII data
func (e *EncryptedAddressData) SetPII(pii *AddressPIIFields, tenantID string) {
	e.PII = pii
	e.TenantID = tenantID
	e.Data = nil
}

// GetPII returns the decrypted PII data
func (e *EncryptedAddressData) GetPII() *AddressPIIFields {
	return e.PII
}

// EncryptedCustomer wraps a customer model with encryption/decryption capabilities
// This provides a transparent encryption layer that can be used with existing customer models
type EncryptedCustomer struct {
	// Original fields (stored as-is in DB for backwards compatibility)
	Email     string `json:"email" gorm:"type:varchar(255)"`
	FirstName string `json:"firstName" gorm:"type:varchar(100)"`
	LastName  string `json:"lastName" gorm:"type:varchar(100)"`
	Phone     string `json:"phone" gorm:"type:varchar(50)"`

	// Encrypted PII data (stores encrypted versions)
	EncryptedPII EncryptedCustomerData `json:"-" gorm:"type:jsonb;column:encrypted_pii"`

	// Search hashes for encrypted field lookups
	EmailHash string `json:"-" gorm:"type:varchar(64);index:idx_encrypted_email_hash"`
	PhoneHash string `json:"-" gorm:"type:varchar(64);index:idx_encrypted_phone_hash"`

	// Tenant ID for encryption key lookup
	TenantID string `json:"tenantId" gorm:"type:varchar(255);not null"`
}

// EncryptPII encrypts PII fields and generates search hashes
func (c *EncryptedCustomer) EncryptPII(ctx context.Context) error {
	encryptor := GetGlobalEncryptor()
	if encryptor == nil {
		return fmt.Errorf("global encryptor not initialized")
	}

	// Build PII structure from plain fields
	pii := &CustomerPIIFields{}
	if c.Email != "" {
		pii.Email = &c.Email
	}
	if c.FirstName != "" {
		pii.FirstName = &c.FirstName
	}
	if c.LastName != "" {
		pii.LastName = &c.LastName
	}
	if c.Phone != "" {
		pii.Phone = &c.Phone
	}

	// Encrypt PII
	c.EncryptedPII.TenantID = c.TenantID
	c.EncryptedPII.SetPII(pii, c.TenantID)
	if err := c.EncryptedPII.Encrypt(ctx); err != nil {
		return err
	}

	// Generate search hashes
	c.EmailHash = encryptor.HashEmail(c.TenantID, c.Email)
	c.PhoneHash = encryptor.HashPhone(c.TenantID, c.Phone)

	return nil
}

// DecryptPII decrypts PII fields from encrypted storage
func (c *EncryptedCustomer) DecryptPII(ctx context.Context) error {
	c.EncryptedPII.TenantID = c.TenantID
	if err := c.EncryptedPII.Decrypt(ctx); err != nil {
		return err
	}

	// Populate plain fields from decrypted PII
	if c.EncryptedPII.PII != nil {
		if c.EncryptedPII.PII.Email != nil {
			c.Email = *c.EncryptedPII.PII.Email
		}
		if c.EncryptedPII.PII.FirstName != nil {
			c.FirstName = *c.EncryptedPII.PII.FirstName
		}
		if c.EncryptedPII.PII.LastName != nil {
			c.LastName = *c.EncryptedPII.PII.LastName
		}
		if c.EncryptedPII.PII.Phone != nil {
			c.Phone = *c.EncryptedPII.PII.Phone
		}
	}

	return nil
}

// GetMaskedPII returns masked PII for API responses
func (c *EncryptedCustomer) GetMaskedPII() *MaskedCustomer {
	return &MaskedCustomer{
		Email:     MaskEmail(c.Email),
		Phone:     MaskPhone(c.Phone),
		FirstName: MaskName(c.FirstName),
		LastName:  MaskName(c.LastName),
	}
}

// EncryptedAddress wraps an address model with encryption/decryption capabilities
type EncryptedAddress struct {
	// Original fields (stored as-is in DB for backwards compatibility)
	FirstName    string `json:"firstName" gorm:"type:varchar(100)"`
	LastName     string `json:"lastName" gorm:"type:varchar(100)"`
	Company      string `json:"company" gorm:"type:varchar(255)"`
	AddressLine1 string `json:"addressLine1" gorm:"type:varchar(255)"`
	AddressLine2 string `json:"addressLine2" gorm:"type:varchar(255)"`
	City         string `json:"city" gorm:"type:varchar(100)"`
	State        string `json:"state" gorm:"type:varchar(100)"`
	PostalCode   string `json:"postalCode" gorm:"type:varchar(20)"`
	Country      string `json:"country" gorm:"type:varchar(2)"`
	Phone        string `json:"phone" gorm:"type:varchar(50)"`

	// Encrypted PII data
	EncryptedPII EncryptedAddressData `json:"-" gorm:"type:jsonb;column:encrypted_pii"`

	// Tenant ID for encryption key lookup
	TenantID string `json:"tenantId" gorm:"type:varchar(255);not null"`
}

// EncryptPII encrypts address PII fields
func (a *EncryptedAddress) EncryptPII(ctx context.Context) error {
	encryptor := GetGlobalEncryptor()
	if encryptor == nil {
		return fmt.Errorf("global encryptor not initialized")
	}

	// Build PII structure from plain fields
	pii := &AddressPIIFields{}
	if a.FirstName != "" {
		pii.FirstName = &a.FirstName
	}
	if a.LastName != "" {
		pii.LastName = &a.LastName
	}
	if a.Company != "" {
		pii.Company = &a.Company
	}
	if a.AddressLine1 != "" {
		pii.AddressLine1 = &a.AddressLine1
	}
	if a.AddressLine2 != "" {
		pii.AddressLine2 = &a.AddressLine2
	}
	if a.City != "" {
		pii.City = &a.City
	}
	if a.State != "" {
		pii.State = &a.State
	}
	if a.PostalCode != "" {
		pii.PostalCode = &a.PostalCode
	}
	if a.Country != "" {
		pii.Country = &a.Country
	}
	if a.Phone != "" {
		pii.Phone = &a.Phone
	}

	// Encrypt PII
	a.EncryptedPII.TenantID = a.TenantID
	a.EncryptedPII.SetPII(pii, a.TenantID)
	return a.EncryptedPII.Encrypt(ctx)
}

// DecryptPII decrypts address PII fields from encrypted storage
func (a *EncryptedAddress) DecryptPII(ctx context.Context) error {
	a.EncryptedPII.TenantID = a.TenantID
	if err := a.EncryptedPII.Decrypt(ctx); err != nil {
		return err
	}

	// Populate plain fields from decrypted PII
	if a.EncryptedPII.PII != nil {
		if a.EncryptedPII.PII.FirstName != nil {
			a.FirstName = *a.EncryptedPII.PII.FirstName
		}
		if a.EncryptedPII.PII.LastName != nil {
			a.LastName = *a.EncryptedPII.PII.LastName
		}
		if a.EncryptedPII.PII.Company != nil {
			a.Company = *a.EncryptedPII.PII.Company
		}
		if a.EncryptedPII.PII.AddressLine1 != nil {
			a.AddressLine1 = *a.EncryptedPII.PII.AddressLine1
		}
		if a.EncryptedPII.PII.AddressLine2 != nil {
			a.AddressLine2 = *a.EncryptedPII.PII.AddressLine2
		}
		if a.EncryptedPII.PII.City != nil {
			a.City = *a.EncryptedPII.PII.City
		}
		if a.EncryptedPII.PII.State != nil {
			a.State = *a.EncryptedPII.PII.State
		}
		if a.EncryptedPII.PII.PostalCode != nil {
			a.PostalCode = *a.EncryptedPII.PII.PostalCode
		}
		if a.EncryptedPII.PII.Country != nil {
			a.Country = *a.EncryptedPII.PII.Country
		}
		if a.EncryptedPII.PII.Phone != nil {
			a.Phone = *a.EncryptedPII.PII.Phone
		}
	}

	return nil
}

// GetMaskedPII returns masked PII for API responses
func (a *EncryptedAddress) GetMaskedPII() *MaskedAddress {
	return MaskAddressPII(&AddressPIIFields{
		FirstName:    &a.FirstName,
		LastName:     &a.LastName,
		Company:      &a.Company,
		AddressLine1: &a.AddressLine1,
		AddressLine2: &a.AddressLine2,
		City:         &a.City,
		State:        &a.State,
		PostalCode:   &a.PostalCode,
		Country:      &a.Country,
		Phone:        &a.Phone,
	})
}

// PIIEncryptionService provides high-level encryption operations for customer data
type PIIEncryptionService struct {
	encryptor *PIIEncryptor
}

// NewPIIEncryptionService creates a new PII encryption service
func NewPIIEncryptionService() *PIIEncryptionService {
	return &PIIEncryptionService{
		encryptor: GetGlobalEncryptor(),
	}
}

// EncryptCustomerFields encrypts individual customer fields and returns encrypted data + hashes
func (s *PIIEncryptionService) EncryptCustomerFields(ctx context.Context, tenantID, email, firstName, lastName, phone string) (*EncryptedData, string, string, error) {
	if s.encryptor == nil {
		return nil, "", "", fmt.Errorf("encryptor not initialized")
	}

	// Build PII structure
	pii := &CustomerPIIFields{}
	if email != "" {
		pii.Email = &email
	}
	if firstName != "" {
		pii.FirstName = &firstName
	}
	if lastName != "" {
		pii.LastName = &lastName
	}
	if phone != "" {
		pii.Phone = &phone
	}

	// Encrypt PII
	encrypted, err := s.encryptor.EncryptCustomerPII(ctx, tenantID, pii)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to encrypt customer PII: %w", err)
	}

	// Generate search hashes
	emailHash := s.encryptor.HashEmail(tenantID, email)
	phoneHash := s.encryptor.HashPhone(tenantID, phone)

	return encrypted, emailHash, phoneHash, nil
}

// DecryptCustomerFields decrypts customer PII and returns individual fields
func (s *PIIEncryptionService) DecryptCustomerFields(ctx context.Context, tenantID string, encrypted *EncryptedData) (email, firstName, lastName, phone string, err error) {
	if s.encryptor == nil {
		return "", "", "", "", fmt.Errorf("encryptor not initialized")
	}

	pii, err := s.encryptor.DecryptCustomerPII(ctx, tenantID, encrypted)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to decrypt customer PII: %w", err)
	}

	if pii != nil {
		if pii.Email != nil {
			email = *pii.Email
		}
		if pii.FirstName != nil {
			firstName = *pii.FirstName
		}
		if pii.LastName != nil {
			lastName = *pii.LastName
		}
		if pii.Phone != nil {
			phone = *pii.Phone
		}
	}

	return email, firstName, lastName, phone, nil
}

// FindByEmailHash generates an email hash for searching
func (s *PIIEncryptionService) FindByEmailHash(tenantID, email string) string {
	if s.encryptor == nil {
		return ""
	}
	return s.encryptor.HashEmail(tenantID, email)
}

// FindByPhoneHash generates a phone hash for searching
func (s *PIIEncryptionService) FindByPhoneHash(tenantID, phone string) string {
	if s.encryptor == nil {
		return ""
	}
	return s.encryptor.HashPhone(tenantID, phone)
}
