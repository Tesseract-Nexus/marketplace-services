package encryption

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

// EncryptedString is a GORM-compatible type for encrypted string fields
type EncryptedString string

func (e EncryptedString) Value() (driver.Value, error) {
	if e == "" {
		return nil, nil
	}
	encrypted, err := Encrypt(string(e))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt value: %w", err)
	}
	return encrypted, nil
}

func (e *EncryptedString) Scan(value interface{}) error {
	if value == nil {
		*e = ""
		return nil
	}

	var strVal string
	switch v := value.(type) {
	case []byte:
		strVal = string(v)
	case string:
		strVal = v
	default:
		return errors.New("unsupported type for EncryptedString")
	}

	decrypted, err := Decrypt(strVal)
	if err != nil {
		*e = EncryptedString(strVal)
		return nil
	}
	*e = EncryptedString(decrypted)
	return nil
}

func (e EncryptedString) String() string {
	return string(e)
}

func (e EncryptedString) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(e))
}

func (e *EncryptedString) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*e = EncryptedString(s)
	return nil
}

// EncryptedVendorContact represents encrypted vendor contact information
type EncryptedVendorContact struct {
	PrimaryContact   string `json:"primaryContact,omitempty"`
	SecondaryContact string `json:"secondaryContact,omitempty"`
	Email            string `json:"email,omitempty"`
}

func (c EncryptedVendorContact) Value() (driver.Value, error) {
	jsonData, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal contact data: %w", err)
	}
	encrypted, err := Encrypt(string(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt contact data: %w", err)
	}
	return encrypted, nil
}

func (c *EncryptedVendorContact) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	var strVal string
	switch v := value.(type) {
	case []byte:
		strVal = string(v)
	case string:
		strVal = v
	default:
		return errors.New("unsupported type for EncryptedVendorContact")
	}

	decrypted, err := Decrypt(strVal)
	if err != nil {
		return json.Unmarshal([]byte(strVal), c)
	}

	return json.Unmarshal([]byte(decrypted), c)
}

func (c *EncryptedVendorContact) GetMasked() EncryptedVendorContact {
	return EncryptedVendorContact{
		PrimaryContact:   MaskPhone(c.PrimaryContact),
		SecondaryContact: MaskPhone(c.SecondaryContact),
		Email:            MaskEmail(c.Email),
	}
}

// EncryptedVendorAddress represents encrypted vendor address
type EncryptedVendorAddress struct {
	AddressLine1 string `json:"addressLine1,omitempty"`
	AddressLine2 string `json:"addressLine2,omitempty"`
	City         string `json:"city,omitempty"`
	State        string `json:"state,omitempty"`
	PostalCode   string `json:"postalCode,omitempty"`
	Country      string `json:"country,omitempty"`
}

func (a EncryptedVendorAddress) Value() (driver.Value, error) {
	jsonData, err := json.Marshal(a)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal address data: %w", err)
	}
	encrypted, err := Encrypt(string(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt address data: %w", err)
	}
	return encrypted, nil
}

func (a *EncryptedVendorAddress) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	var strVal string
	switch v := value.(type) {
	case []byte:
		strVal = string(v)
	case string:
		strVal = v
	default:
		return errors.New("unsupported type for EncryptedVendorAddress")
	}

	decrypted, err := Decrypt(strVal)
	if err != nil {
		return json.Unmarshal([]byte(strVal), a)
	}

	return json.Unmarshal([]byte(decrypted), a)
}

func (a *EncryptedVendorAddress) GetMasked() EncryptedVendorAddress {
	maskedPostal := a.PostalCode
	if len(maskedPostal) > 3 {
		maskedPostal = maskedPostal[:3] + "***"
	}
	return EncryptedVendorAddress{
		AddressLine1: MaskAddress(a.AddressLine1),
		AddressLine2: MaskAddress(a.AddressLine2),
		City:         a.City,
		State:        a.State,
		PostalCode:   maskedPostal,
		Country:      a.Country,
	}
}

// EncryptedPaymentInfo represents encrypted vendor payment/banking information
// This is highly sensitive financial data requiring strong encryption
type EncryptedPaymentInfo struct {
	AccountHolderName string `json:"accountHolderName,omitempty"`
	BankName          string `json:"bankName,omitempty"`
	AccountNumber     string `json:"accountNumber,omitempty"`
	RoutingNumber     string `json:"routingNumber,omitempty"`
	SwiftCode         string `json:"swiftCode,omitempty"`
	TaxIdentifier     string `json:"taxIdentifier,omitempty"`
	Currency          string `json:"currency,omitempty"`
	PaymentMethod     string `json:"paymentMethod,omitempty"`
}

func (p EncryptedPaymentInfo) Value() (driver.Value, error) {
	jsonData, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payment data: %w", err)
	}
	encrypted, err := Encrypt(string(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt payment data: %w", err)
	}
	return encrypted, nil
}

func (p *EncryptedPaymentInfo) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	var strVal string
	switch v := value.(type) {
	case []byte:
		strVal = string(v)
	case string:
		strVal = v
	default:
		return errors.New("unsupported type for EncryptedPaymentInfo")
	}

	decrypted, err := Decrypt(strVal)
	if err != nil {
		return json.Unmarshal([]byte(strVal), p)
	}

	return json.Unmarshal([]byte(decrypted), p)
}

func (p *EncryptedPaymentInfo) GetMasked() EncryptedPaymentInfo {
	return EncryptedPaymentInfo{
		AccountHolderName: MaskName(p.AccountHolderName),
		BankName:          p.BankName, // Bank name is not sensitive
		AccountNumber:     MaskBankAccount(p.AccountNumber),
		RoutingNumber:     MaskBankAccount(p.RoutingNumber),
		SwiftCode:         p.SwiftCode, // SWIFT codes are public
		TaxIdentifier:     MaskTaxID(p.TaxIdentifier),
		Currency:          p.Currency,
		PaymentMethod:     p.PaymentMethod,
	}
}

// EncryptedBusinessInfo represents encrypted business registration details
type EncryptedBusinessInfo struct {
	BusinessRegistrationNumber string `json:"businessRegistrationNumber,omitempty"`
	TaxIdentificationNumber    string `json:"taxIdentificationNumber,omitempty"`
}

func (b EncryptedBusinessInfo) Value() (driver.Value, error) {
	jsonData, err := json.Marshal(b)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal business data: %w", err)
	}
	encrypted, err := Encrypt(string(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt business data: %w", err)
	}
	return encrypted, nil
}

func (b *EncryptedBusinessInfo) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	var strVal string
	switch v := value.(type) {
	case []byte:
		strVal = string(v)
	case string:
		strVal = v
	default:
		return errors.New("unsupported type for EncryptedBusinessInfo")
	}

	decrypted, err := Decrypt(strVal)
	if err != nil {
		return json.Unmarshal([]byte(strVal), b)
	}

	return json.Unmarshal([]byte(decrypted), b)
}

func (b *EncryptedBusinessInfo) GetMasked() EncryptedBusinessInfo {
	return EncryptedBusinessInfo{
		BusinessRegistrationNumber: MaskTaxID(b.BusinessRegistrationNumber),
		TaxIdentificationNumber:    MaskTaxID(b.TaxIdentificationNumber),
	}
}

// EncryptedEmail for vendor email with search support
type EncryptedEmail struct {
	Value      EncryptedString `json:"value" gorm:"column:email"`
	SearchHash string          `json:"-" gorm:"column:email_hash;index"`
}

func (e *EncryptedEmail) SetValue(email string) {
	e.Value = EncryptedString(email)
	e.SearchHash = HashForSearch(email)
}

func (e *EncryptedEmail) GetValue() string {
	return string(e.Value)
}

func (e *EncryptedEmail) GetMasked() string {
	return MaskEmail(string(e.Value))
}

// EncryptedPhone for vendor phone with search support
type EncryptedPhone struct {
	Value      EncryptedString `json:"value" gorm:"column:phone"`
	SearchHash string          `json:"-" gorm:"column:phone_hash;index"`
}

func (p *EncryptedPhone) SetValue(phone string) {
	p.Value = EncryptedString(phone)
	p.SearchHash = HashForSearch(phone)
}

func (p *EncryptedPhone) GetValue() string {
	return string(p.Value)
}

func (p *EncryptedPhone) GetMasked() string {
	return MaskPhone(string(p.Value))
}
