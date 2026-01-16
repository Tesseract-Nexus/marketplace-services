package encryption

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

// EncryptedString is a GORM-compatible type for encrypted string fields
type EncryptedString string

// Value implements driver.Valuer for database storage
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

// Scan implements sql.Scanner for database retrieval
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
		// If decryption fails, might be unencrypted legacy data
		*e = EncryptedString(strVal)
		return nil
	}
	*e = EncryptedString(decrypted)
	return nil
}

// String returns the decrypted string value
func (e EncryptedString) String() string {
	return string(e)
}

// MarshalJSON implements json.Marshaler
func (e EncryptedString) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(e))
}

// UnmarshalJSON implements json.Unmarshaler
func (e *EncryptedString) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*e = EncryptedString(s)
	return nil
}

// EncryptedEmail is a specialized type for email addresses with search support
type EncryptedEmail struct {
	Value      EncryptedString `json:"value" gorm:"column:email"`
	SearchHash string          `json:"-" gorm:"column:email_hash;index"`
}

// SetValue sets the email value and updates the search hash
func (e *EncryptedEmail) SetValue(email string) {
	e.Value = EncryptedString(email)
	e.SearchHash = HashForSearch(email)
}

// GetValue returns the decrypted email
func (e *EncryptedEmail) GetValue() string {
	return string(e.Value)
}

// GetMasked returns the masked email for display
func (e *EncryptedEmail) GetMasked() string {
	return MaskEmail(string(e.Value))
}

// EncryptedPhone is a specialized type for phone numbers with search support
type EncryptedPhone struct {
	Value      EncryptedString `json:"value" gorm:"column:phone"`
	SearchHash string          `json:"-" gorm:"column:phone_hash;index"`
}

// SetValue sets the phone value and updates the search hash
func (p *EncryptedPhone) SetValue(phone string) {
	p.Value = EncryptedString(phone)
	p.SearchHash = HashForSearch(phone)
}

// GetValue returns the decrypted phone
func (p *EncryptedPhone) GetValue() string {
	return string(p.Value)
}

// GetMasked returns the masked phone for display
func (p *EncryptedPhone) GetMasked() string {
	return MaskPhone(string(p.Value))
}

// EncryptedName is a specialized type for names
type EncryptedName struct {
	Value      EncryptedString `json:"value" gorm:"column:name"`
	SearchHash string          `json:"-" gorm:"column:name_hash;index"`
}

// SetValue sets the name value and updates the search hash
func (n *EncryptedName) SetValue(name string) {
	n.Value = EncryptedString(name)
	n.SearchHash = HashForSearch(name)
}

// GetValue returns the decrypted name
func (n *EncryptedName) GetValue() string {
	return string(n.Value)
}

// GetMasked returns the masked name for display
func (n *EncryptedName) GetMasked() string {
	return MaskName(string(n.Value))
}

// EncryptedStaffData represents encrypted staff PII fields as a single JSON blob
type EncryptedStaffData struct {
	FirstName    string `json:"firstName,omitempty"`
	LastName     string `json:"lastName,omitempty"`
	MiddleName   string `json:"middleName,omitempty"`
	Email        string `json:"email,omitempty"`
	AltEmail     string `json:"alternateEmail,omitempty"`
	Phone        string `json:"phoneNumber,omitempty"`
	Mobile       string `json:"mobileNumber,omitempty"`
}

// Value implements driver.Valuer - encrypts the entire struct as JSON
func (s EncryptedStaffData) Value() (driver.Value, error) {
	jsonData, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal staff data: %w", err)
	}
	encrypted, err := Encrypt(string(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt staff data: %w", err)
	}
	return encrypted, nil
}

// Scan implements sql.Scanner - decrypts and unmarshals the struct
func (s *EncryptedStaffData) Scan(value interface{}) error {
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
		return errors.New("unsupported type for EncryptedStaffData")
	}

	decrypted, err := Decrypt(strVal)
	if err != nil {
		// Try to unmarshal as-is (might be unencrypted legacy data)
		return json.Unmarshal([]byte(strVal), s)
	}

	return json.Unmarshal([]byte(decrypted), s)
}

// GetMasked returns a masked version of staff data
func (s *EncryptedStaffData) GetMasked() EncryptedStaffData {
	return EncryptedStaffData{
		FirstName:  MaskName(s.FirstName),
		LastName:   MaskName(s.LastName),
		MiddleName: MaskName(s.MiddleName),
		Email:      MaskEmail(s.Email),
		AltEmail:   MaskEmail(s.AltEmail),
		Phone:      MaskPhone(s.Phone),
		Mobile:     MaskPhone(s.Mobile),
	}
}

// EncryptedAddressData represents encrypted address fields
type EncryptedAddressData struct {
	StreetAddress  string  `json:"streetAddress,omitempty"`
	StreetAddress2 string  `json:"streetAddress2,omitempty"`
	City           string  `json:"city,omitempty"`
	State          string  `json:"state,omitempty"`
	PostalCode     string  `json:"postalCode,omitempty"`
	Country        string  `json:"country,omitempty"`
	Latitude       float64 `json:"latitude,omitempty"`
	Longitude      float64 `json:"longitude,omitempty"`
}

// Value implements driver.Valuer
func (a EncryptedAddressData) Value() (driver.Value, error) {
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

// Scan implements sql.Scanner
func (a *EncryptedAddressData) Scan(value interface{}) error {
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
		return errors.New("unsupported type for EncryptedAddressData")
	}

	decrypted, err := Decrypt(strVal)
	if err != nil {
		return json.Unmarshal([]byte(strVal), a)
	}

	return json.Unmarshal([]byte(decrypted), a)
}

// GetMasked returns a masked version of address data
func (a *EncryptedAddressData) GetMasked() EncryptedAddressData {
	return EncryptedAddressData{
		StreetAddress:  MaskAddress(a.StreetAddress),
		StreetAddress2: MaskAddress(a.StreetAddress2),
		City:           a.City, // City and country are typically not considered highly sensitive
		State:          a.State,
		PostalCode:     a.PostalCode[:3] + "***",
		Country:        a.Country,
	}
}

// EncryptedIPAddress represents an encrypted IP address
type EncryptedIPAddress EncryptedString

// MaskIP masks an IP address for display
func MaskIP(ip string) string {
	if ip == "" {
		return ""
	}
	// For IPv4, mask last two octets
	parts := splitIP(ip)
	if len(parts) >= 4 {
		return parts[0] + "." + parts[1] + ".***.*"
	}
	return "***"
}

func splitIP(ip string) []string {
	var parts []string
	current := ""
	for _, c := range ip {
		if c == '.' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// EncryptedTrustedDevice represents encrypted trusted device data
type EncryptedTrustedDevice struct {
	DeviceFingerprint string  `json:"deviceFingerprint"`
	DeviceName        string  `json:"deviceName"`
	DeviceType        string  `json:"deviceType,omitempty"`
	OperatingSystem   string  `json:"operatingSystem,omitempty"`
	Browser           string  `json:"browser,omitempty"`
	IPAddress         string  `json:"ipAddress,omitempty"`
	Location          string  `json:"location,omitempty"`
	TrustedAt         string  `json:"trustedAt"`
	LastUsedAt        string  `json:"lastUsedAt,omitempty"`
	ExpiresAt         string  `json:"expiresAt,omitempty"`
}

// EncryptedTrustedDevices is a slice of encrypted trusted devices
type EncryptedTrustedDevices []EncryptedTrustedDevice

// Value implements driver.Valuer
func (d EncryptedTrustedDevices) Value() (driver.Value, error) {
	if len(d) == 0 {
		return nil, nil
	}
	jsonData, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal devices: %w", err)
	}
	encrypted, err := Encrypt(string(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt devices: %w", err)
	}
	return encrypted, nil
}

// Scan implements sql.Scanner
func (d *EncryptedTrustedDevices) Scan(value interface{}) error {
	if value == nil {
		*d = nil
		return nil
	}

	var strVal string
	switch v := value.(type) {
	case []byte:
		strVal = string(v)
	case string:
		strVal = v
	default:
		return errors.New("unsupported type for EncryptedTrustedDevices")
	}

	decrypted, err := Decrypt(strVal)
	if err != nil {
		return json.Unmarshal([]byte(strVal), d)
	}

	return json.Unmarshal([]byte(decrypted), d)
}

// GetMasked returns a masked version of trusted devices
func (d EncryptedTrustedDevices) GetMasked() EncryptedTrustedDevices {
	masked := make(EncryptedTrustedDevices, len(d))
	for i, device := range d {
		masked[i] = EncryptedTrustedDevice{
			DeviceFingerprint: device.DeviceFingerprint[:8] + "***",
			DeviceName:        device.DeviceName,
			DeviceType:        device.DeviceType,
			OperatingSystem:   device.OperatingSystem,
			Browser:           device.Browser,
			IPAddress:         MaskIP(device.IPAddress),
			Location:          device.Location,
			TrustedAt:         device.TrustedAt,
			LastUsedAt:        device.LastUsedAt,
			ExpiresAt:         device.ExpiresAt,
		}
	}
	return masked
}
