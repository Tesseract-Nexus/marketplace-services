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

// EncryptedOrderCustomer represents encrypted customer information in an order
type EncryptedOrderCustomer struct {
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
}

func (c EncryptedOrderCustomer) Value() (driver.Value, error) {
	jsonData, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal customer data: %w", err)
	}
	encrypted, err := Encrypt(string(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt customer data: %w", err)
	}
	return encrypted, nil
}

func (c *EncryptedOrderCustomer) Scan(value interface{}) error {
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
		return errors.New("unsupported type for EncryptedOrderCustomer")
	}

	decrypted, err := Decrypt(strVal)
	if err != nil {
		return json.Unmarshal([]byte(strVal), c)
	}

	return json.Unmarshal([]byte(decrypted), c)
}

func (c *EncryptedOrderCustomer) GetMasked() EncryptedOrderCustomer {
	return EncryptedOrderCustomer{
		FirstName: MaskName(c.FirstName),
		LastName:  MaskName(c.LastName),
		Email:     MaskEmail(c.Email),
		Phone:     MaskPhone(c.Phone),
	}
}

// EncryptedShippingAddress represents encrypted shipping address in an order
type EncryptedShippingAddress struct {
	Street      string `json:"street,omitempty"`
	City        string `json:"city,omitempty"`
	State       string `json:"state,omitempty"`
	StateCode   string `json:"stateCode,omitempty"`
	PostalCode  string `json:"postalCode,omitempty"`
	Country     string `json:"country,omitempty"`
	CountryCode string `json:"countryCode,omitempty"`
}

func (s EncryptedShippingAddress) Value() (driver.Value, error) {
	jsonData, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal shipping address: %w", err)
	}
	encrypted, err := Encrypt(string(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt shipping address: %w", err)
	}
	return encrypted, nil
}

func (s *EncryptedShippingAddress) Scan(value interface{}) error {
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
		return errors.New("unsupported type for EncryptedShippingAddress")
	}

	decrypted, err := Decrypt(strVal)
	if err != nil {
		return json.Unmarshal([]byte(strVal), s)
	}

	return json.Unmarshal([]byte(decrypted), s)
}

func (s *EncryptedShippingAddress) GetMasked() EncryptedShippingAddress {
	maskedPostal := s.PostalCode
	if len(maskedPostal) > 3 {
		maskedPostal = maskedPostal[:3] + "***"
	}
	return EncryptedShippingAddress{
		Street:      MaskAddress(s.Street),
		City:        s.City,
		State:       s.State,
		StateCode:   s.StateCode,
		PostalCode:  maskedPostal,
		Country:     s.Country,
		CountryCode: s.CountryCode,
	}
}

// EncryptedTaxInfo represents encrypted tax identification for B2B orders
type EncryptedTaxInfo struct {
	CustomerGSTIN     string `json:"customerGstin,omitempty"`
	CustomerVATNumber string `json:"customerVatNumber,omitempty"`
}

func (t EncryptedTaxInfo) Value() (driver.Value, error) {
	jsonData, err := json.Marshal(t)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tax info: %w", err)
	}
	encrypted, err := Encrypt(string(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt tax info: %w", err)
	}
	return encrypted, nil
}

func (t *EncryptedTaxInfo) Scan(value interface{}) error {
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
		return errors.New("unsupported type for EncryptedTaxInfo")
	}

	decrypted, err := Decrypt(strVal)
	if err != nil {
		return json.Unmarshal([]byte(strVal), t)
	}

	return json.Unmarshal([]byte(decrypted), t)
}

func (t *EncryptedTaxInfo) GetMasked() EncryptedTaxInfo {
	return EncryptedTaxInfo{
		CustomerGSTIN:     MaskGSTIN(t.CustomerGSTIN),
		CustomerVATNumber: MaskVAT(t.CustomerVATNumber),
	}
}

// EncryptedEmail for order emails with search support
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

// EncryptedPhone for order phones with search support
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

// EncryptedName for customer names with search support
type EncryptedName struct {
	Value      EncryptedString `json:"value" gorm:"column:name"`
	SearchHash string          `json:"-" gorm:"column:name_hash;index"`
}

func (n *EncryptedName) SetValue(name string) {
	n.Value = EncryptedString(name)
	n.SearchHash = HashForSearch(name)
}

func (n *EncryptedName) GetValue() string {
	return string(n.Value)
}

func (n *EncryptedName) GetMasked() string {
	return MaskName(string(n.Value))
}
