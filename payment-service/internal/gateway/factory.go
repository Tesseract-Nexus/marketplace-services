package gateway

import (
	"fmt"
	"sync"

	"payment-service/internal/models"
)

// GatewayFactory creates payment gateway instances
type GatewayFactory struct {
	mu       sync.RWMutex
	gateways map[string]PaymentGateway // cache of gateway instances
}

// NewGatewayFactory creates a new gateway factory
func NewGatewayFactory() *GatewayFactory {
	return &GatewayFactory{
		gateways: make(map[string]PaymentGateway),
	}
}

// CreateGateway creates a new gateway instance from configuration
func (f *GatewayFactory) CreateGateway(config *models.PaymentGatewayConfig) (PaymentGateway, error) {
	if config == nil {
		return nil, fmt.Errorf("gateway config is required")
	}

	// Generate cache key
	cacheKey := fmt.Sprintf("%s_%s_%t", config.TenantID, config.GatewayType, config.IsTestMode)

	// Check cache first
	f.mu.RLock()
	if gw, exists := f.gateways[cacheKey]; exists {
		f.mu.RUnlock()
		return gw, nil
	}
	f.mu.RUnlock()

	// Create new gateway instance
	var gw PaymentGateway
	var err error

	switch config.GatewayType {
	case models.GatewayStripe:
		gw, err = NewStripeGateway(config)
	case models.GatewayRazorpay:
		gw, err = NewRazorpayGateway(config)
	case models.GatewayPayPal:
		gw, err = NewPayPalGateway(config)
	case models.GatewayPhonePe:
		gw, err = NewPhonePeGateway(config)
	case models.GatewayBharatPay:
		gw, err = NewBharatPayGateway(config)
	case models.GatewayAfterpay:
		gw, err = NewAfterpayGateway(config)
	case models.GatewayZip:
		gw, err = NewZipGateway(config)
	case models.GatewayLinkt:
		gw, err = NewLinktGateway(config)
	default:
		return nil, fmt.Errorf("unsupported gateway type: %s", config.GatewayType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create %s gateway: %w", config.GatewayType, err)
	}

	// Cache the gateway instance
	f.mu.Lock()
	f.gateways[cacheKey] = gw
	f.mu.Unlock()

	return gw, nil
}

// InvalidateCache removes a gateway from the cache
func (f *GatewayFactory) InvalidateCache(tenantID string, gatewayType models.GatewayType, isTestMode bool) {
	cacheKey := fmt.Sprintf("%s_%s_%t", tenantID, gatewayType, isTestMode)
	f.mu.Lock()
	delete(f.gateways, cacheKey)
	f.mu.Unlock()
}

// ClearCache removes all gateways from the cache
func (f *GatewayFactory) ClearCache() {
	f.mu.Lock()
	f.gateways = make(map[string]PaymentGateway)
	f.mu.Unlock()
}

// GetSupportedGatewayTypes returns all supported gateway types
func GetSupportedGatewayTypes() []models.GatewayType {
	return []models.GatewayType{
		models.GatewayStripe,
		models.GatewayPayPal,
		models.GatewayRazorpay,
		models.GatewayPhonePe,
		models.GatewayBharatPay,
		models.GatewayAfterpay,
		models.GatewayZip,
		models.GatewayLinkt,
	}
}

// GetGatewayDisplayName returns the display name for a gateway type
func GetGatewayDisplayName(gatewayType models.GatewayType) string {
	names := map[models.GatewayType]string{
		models.GatewayStripe:    "Stripe",
		models.GatewayPayPal:    "PayPal",
		models.GatewayRazorpay:  "Razorpay",
		models.GatewayPhonePe:   "PhonePe",
		models.GatewayBharatPay: "BharatPay",
		models.GatewayAfterpay:  "Afterpay",
		models.GatewayZip:       "Zip Pay",
		models.GatewayLinkt:     "Linkt",
		models.GatewayPayU:      "PayU India",
		models.GatewayCashfree:  "Cashfree",
		models.GatewayPaytm:     "Paytm",
	}

	if name, ok := names[gatewayType]; ok {
		return name
	}
	return string(gatewayType)
}

// GetGatewayCountries returns the supported countries for a gateway type
func GetGatewayCountries(gatewayType models.GatewayType) []string {
	countries := map[models.GatewayType][]string{
		models.GatewayStripe:    {"US", "GB", "AU", "CA", "NZ", "SG", "DE", "FR", "IT", "ES", "NL", "IE", "JP", "HK"},
		models.GatewayPayPal:    {"US", "GB", "AU", "CA", "DE", "FR", "IT", "ES", "NL", "IN", "SG", "HK"},
		models.GatewayRazorpay:  {"IN"},
		models.GatewayPhonePe:   {"IN"},
		models.GatewayBharatPay: {"IN"},
		models.GatewayAfterpay:  {"AU", "NZ", "US", "GB", "CA"},
		models.GatewayZip:       {"AU", "NZ"},
		models.GatewayLinkt:     {"US", "GB", "AU", "EU", "IN", "SG", "HK", "JP", "CA", "NZ"},
	}

	if c, ok := countries[gatewayType]; ok {
		return c
	}
	return []string{"US"}
}

// GetGatewayPaymentMethods returns the supported payment methods for a gateway type
func GetGatewayPaymentMethods(gatewayType models.GatewayType) []models.PaymentMethodType {
	methods := map[models.GatewayType][]models.PaymentMethodType{
		models.GatewayStripe: {
			models.MethodCard,
			models.MethodApplePay,
			models.MethodGooglePay,
			models.MethodBankAccount,
			models.MethodSEPA,
			models.MethodIDeal,
			models.MethodKlarna,
		},
		models.GatewayPayPal: {
			models.MethodPayPal,
			models.MethodCard,
		},
		models.GatewayRazorpay: {
			models.MethodCard,
			models.MethodUPI,
			models.MethodNetBanking,
			models.MethodWallet,
			models.MethodEMI,
			models.MethodPayLater,
		},
		models.GatewayPhonePe: {
			models.MethodUPI,
			models.MethodWallet,
			models.MethodCard,
		},
		models.GatewayBharatPay: {
			models.MethodUPI,
			models.MethodNetBanking,
			models.MethodRuPay,
		},
		models.GatewayAfterpay: {
			models.MethodPayLater,
		},
		models.GatewayZip: {
			models.MethodPayLater,
		},
		models.GatewayLinkt: {
			models.MethodCard,
			models.MethodBankAccount,
			models.MethodWallet,
			models.MethodApplePay,
			models.MethodGooglePay,
		},
	}

	if m, ok := methods[gatewayType]; ok {
		return m
	}
	return []models.PaymentMethodType{models.MethodCard}
}

// GetPaymentMethodIcon returns the icon name for a payment method
func GetPaymentMethodIcon(method models.PaymentMethodType) string {
	icons := map[models.PaymentMethodType]string{
		models.MethodCard:        "credit-card",
		models.MethodUPI:         "smartphone",
		models.MethodNetBanking:  "building-2",
		models.MethodWallet:      "wallet",
		models.MethodEMI:         "calendar",
		models.MethodPayLater:    "clock",
		models.MethodBankAccount: "landmark",
		models.MethodPayPal:      "paypal",
		models.MethodApplePay:    "apple",
		models.MethodGooglePay:   "google",
		models.MethodSEPA:        "euro",
		models.MethodIDeal:       "ideal",
		models.MethodKlarna:      "klarna",
		models.MethodRuPay:       "credit-card",
	}

	if icon, ok := icons[method]; ok {
		return icon
	}
	return "credit-card"
}

// GetPaymentMethodDisplayName returns the display name for a payment method
func GetPaymentMethodDisplayName(method models.PaymentMethodType) string {
	names := map[models.PaymentMethodType]string{
		models.MethodCard:        "Credit/Debit Card",
		models.MethodUPI:         "UPI",
		models.MethodNetBanking:  "Net Banking",
		models.MethodWallet:      "Digital Wallet",
		models.MethodEMI:         "EMI",
		models.MethodPayLater:    "Buy Now, Pay Later",
		models.MethodBankAccount: "Bank Account",
		models.MethodPayPal:      "PayPal",
		models.MethodApplePay:    "Apple Pay",
		models.MethodGooglePay:   "Google Pay",
		models.MethodSEPA:        "SEPA Direct Debit",
		models.MethodIDeal:       "iDEAL",
		models.MethodKlarna:      "Klarna",
		models.MethodRuPay:       "RuPay",
		models.MethodCardlessEMI: "Cardless EMI",
	}

	if name, ok := names[method]; ok {
		return name
	}
	return string(method)
}
