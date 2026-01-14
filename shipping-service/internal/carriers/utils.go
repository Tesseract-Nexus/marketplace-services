package carriers

// CleanPhoneNumber normalizes phone numbers to 10-digit Indian format
// Handles various input formats including:
// - 10 digits: 9876543210
// - With leading 0: 09876543210
// - With country code: 919876543210, +919876543210
func CleanPhoneNumber(phone string) string {
	// Remove all non-digit characters
	digits := ""
	for _, c := range phone {
		if c >= '0' && c <= '9' {
			digits += string(c)
		}
	}

	// Handle various formats
	switch len(digits) {
	case 10:
		// Already 10 digits - perfect
		return digits
	case 11:
		// Might have leading 0 (e.g., 09876543210)
		if digits[0] == '0' {
			return digits[1:]
		}
		return digits[:10]
	case 12:
		// Might have 91 prefix (e.g., 919876543210)
		if digits[:2] == "91" {
			return digits[2:]
		}
		return digits[:10]
	case 13:
		// Might have +91 converted to 91 with leading 0
		if digits[:2] == "91" {
			return digits[2:12]
		}
		return digits[:10]
	default:
		if len(digits) > 10 {
			// Take last 10 digits
			return digits[len(digits)-10:]
		}
		// Less than 10 digits - return empty to indicate invalid
		return ""
	}
}
