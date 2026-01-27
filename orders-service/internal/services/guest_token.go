package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const guestTokenExpiry = 30 * 24 * time.Hour // 30 days

// GuestTokenService generates and validates stateless HMAC-SHA256 tokens for guest order access.
type GuestTokenService struct {
	secret []byte
}

// NewGuestTokenService creates a new GuestTokenService using the GUEST_ORDER_TOKEN_SECRET env var.
func NewGuestTokenService() *GuestTokenService {
	secret := os.Getenv("GUEST_ORDER_TOKEN_SECRET")
	if secret == "" {
		secret = "default-guest-order-token-secret-change-me"
		fmt.Println("WARNING: GUEST_ORDER_TOKEN_SECRET not set, using insecure default")
	}
	return &GuestTokenService{secret: []byte(secret)}
}

// GenerateToken produces a base64url-encoded token: expiry_unix|HMAC-SHA256(orderID|orderNumber|email|expiry, secret)
func (s *GuestTokenService) GenerateToken(orderID, orderNumber, email string) string {
	expiry := time.Now().Add(guestTokenExpiry).Unix()
	expiryStr := strconv.FormatInt(expiry, 10)

	mac := s.computeHMAC(orderID, orderNumber, email, expiryStr)

	payload := expiryStr + "|" + base64.RawURLEncoding.EncodeToString(mac)
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

// ValidateToken decodes the token, checks expiry, recomputes HMAC, and performs constant-time comparison.
func (s *GuestTokenService) ValidateToken(token, orderID, orderNumber, email string) error {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return fmt.Errorf("invalid token")
	}

	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid token")
	}

	expiryStr := parts[0]
	providedMAC, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("invalid token")
	}

	// Check expiry
	expiry, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid token")
	}
	if time.Now().Unix() > expiry {
		return fmt.Errorf("token expired")
	}

	// Recompute and constant-time compare
	expectedMAC := s.computeHMAC(orderID, orderNumber, email, expiryStr)
	if subtle.ConstantTimeCompare(providedMAC, expectedMAC) != 1 {
		return fmt.Errorf("invalid token")
	}

	return nil
}

func (s *GuestTokenService) computeHMAC(orderID, orderNumber, email, expiry string) []byte {
	h := hmac.New(sha256.New, s.secret)
	h.Write([]byte(orderID + "|" + orderNumber + "|" + email + "|" + expiry))
	return h.Sum(nil)
}
