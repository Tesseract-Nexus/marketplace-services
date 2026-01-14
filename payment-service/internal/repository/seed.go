package repository

import (
	"log"

	"payment-service/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SeedGatewayTemplates seeds the default payment gateway templates
// This is idempotent - it uses upsert to avoid duplicates
func SeedGatewayTemplates(db *gorm.DB) error {
	templates := []models.PaymentGatewayTemplate{
		{
			GatewayType:             models.GatewayStripe,
			DisplayName:             "Stripe",
			Description:             "Accept payments globally with Stripe. Supports cards, Apple Pay, Google Pay, and more.",
			LogoURL:                 "https://cdn.simpleicons.org/stripe/635BFF",
			SupportsPayments:        true,
			SupportsRefunds:         true,
			SupportsSubscriptions:   true,
			SupportsPlatformSplit:   true,
			SupportedCountries:      models.StringArray{"US", "GB", "AU", "CA", "DE", "FR", "IE", "NL", "SG", "NZ", "AT", "BE", "DK", "FI", "IT", "JP", "LU", "NO", "PT", "ES", "SE", "CH"},
			SupportedPaymentMethods: models.StringArray{"CARD", "APPLE_PAY", "GOOGLE_PAY", "BANK_ACCOUNT"},
			RequiredCredentials:     models.StringArray{"api_key_public", "api_key_secret", "webhook_secret"},
			SetupInstructions:       "Get your API keys from https://dashboard.stripe.com/apikeys. For platform fees, enable Stripe Connect at https://dashboard.stripe.com/connect.",
			DocumentationURL:        "https://stripe.com/docs",
			Priority:                100,
			IsActive:                true,
		},
		{
			GatewayType:             models.GatewayPayPal,
			DisplayName:             "PayPal",
			Description:             "Accept PayPal payments and credit/debit cards through PayPal checkout.",
			LogoURL:                 "https://cdn.simpleicons.org/paypal/003087",
			SupportsPayments:        true,
			SupportsRefunds:         true,
			SupportsSubscriptions:   true,
			SupportsPlatformSplit:   true,
			SupportedCountries:      models.StringArray{"US", "GB", "AU", "CA", "DE", "FR", "IN", "IT", "ES", "NL", "BE", "AT", "CH", "SG", "HK", "JP", "MX", "BR"},
			SupportedPaymentMethods: models.StringArray{"PAYPAL", "CARD"},
			RequiredCredentials:     models.StringArray{"client_id", "client_secret"},
			SetupInstructions:       "Get your PayPal API credentials from https://developer.paypal.com/dashboard/applications. Use Sandbox credentials for testing.",
			DocumentationURL:        "https://developer.paypal.com/docs",
			Priority:                90,
			IsActive:                true,
		},
		{
			GatewayType:             models.GatewayRazorpay,
			DisplayName:             "Razorpay",
			Description:             "India's leading payment gateway. Accept UPI, cards, net banking, wallets, and more.",
			LogoURL:                 "https://cdn.simpleicons.org/razorpay/0C2451",
			SupportsPayments:        true,
			SupportsRefunds:         true,
			SupportsSubscriptions:   true,
			SupportsPlatformSplit:   true,
			SupportedCountries:      models.StringArray{"IN"},
			SupportedPaymentMethods: models.StringArray{"CARD", "UPI", "NET_BANKING", "WALLET"},
			RequiredCredentials:     models.StringArray{"api_key_public", "api_key_secret", "webhook_secret"},
			SetupInstructions:       "Get your Razorpay API keys from https://dashboard.razorpay.com/app/keys. Enable Route API for platform fees.",
			DocumentationURL:        "https://razorpay.com/docs",
			Priority:                80,
			IsActive:                true,
		},
		{
			GatewayType:             models.GatewayPhonePe,
			DisplayName:             "PhonePe",
			Description:             "PhonePe payment gateway for UPI and wallet payments in India.",
			LogoURL:                 "https://cdn.simpleicons.org/phonepe/5F259F",
			SupportsPayments:        true,
			SupportsRefunds:         true,
			SupportsSubscriptions:   false,
			SupportsPlatformSplit:   false,
			SupportedCountries:      models.StringArray{"IN"},
			SupportedPaymentMethods: models.StringArray{"UPI", "WALLET"},
			RequiredCredentials:     models.StringArray{"merchant_id", "salt_key", "salt_index"},
			SetupInstructions:       "Get your PhonePe merchant credentials from the PhonePe Business dashboard.",
			DocumentationURL:        "https://developer.phonepe.com/docs",
			Priority:                70,
			IsActive:                true,
		},
		{
			GatewayType:             models.GatewayAfterpay,
			DisplayName:             "Afterpay",
			Description:             "Buy Now, Pay Later solution. Let customers split purchases into 4 interest-free payments.",
			LogoURL:                 "https://cdn.simpleicons.org/afterpay/B2FCE4",
			SupportsPayments:        true,
			SupportsRefunds:         true,
			SupportsSubscriptions:   false,
			SupportsPlatformSplit:   false,
			SupportedCountries:      models.StringArray{"AU", "NZ", "US", "GB", "CA"},
			SupportedPaymentMethods: models.StringArray{"PAY_LATER"},
			RequiredCredentials:     models.StringArray{"merchant_id", "secret_key"},
			SetupInstructions:       "Apply for an Afterpay merchant account at https://www.afterpay.com/for-retailers.",
			DocumentationURL:        "https://developers.afterpay.com",
			Priority:                60,
			IsActive:                true,
		},
		{
			GatewayType:             models.GatewayZip,
			DisplayName:             "Zip Pay",
			Description:             "Buy Now, Pay Later with Zip. Flexible payment plans for your customers.",
			LogoURL:                 "https://cdn.brandfetch.io/idT5UDBQPZ/theme/dark/symbol.svg",
			SupportsPayments:        true,
			SupportsRefunds:         true,
			SupportsSubscriptions:   false,
			SupportsPlatformSplit:   false,
			SupportedCountries:      models.StringArray{"AU", "NZ"},
			SupportedPaymentMethods: models.StringArray{"PAY_LATER"},
			RequiredCredentials:     models.StringArray{"merchant_id", "api_key"},
			SetupInstructions:       "Sign up for Zip Business at https://zip.co/business.",
			DocumentationURL:        "https://developers.zip.co",
			Priority:                50,
			IsActive:                true,
		},
		{
			GatewayType:             models.GatewayBharatPay,
			DisplayName:             "BharatPay",
			Description:             "UPI and Net Banking payments for India market.",
			LogoURL:                 "https://cdn.simpleicons.org/paytm/00BAF2",
			SupportsPayments:        true,
			SupportsRefunds:         true,
			SupportsSubscriptions:   false,
			SupportsPlatformSplit:   false,
			SupportedCountries:      models.StringArray{"IN"},
			SupportedPaymentMethods: models.StringArray{"UPI", "NET_BANKING"},
			RequiredCredentials:     models.StringArray{"merchant_id", "api_key", "api_secret"},
			SetupInstructions:       "Register at BharatPay merchant portal to get your API credentials.",
			DocumentationURL:        "https://bharatpay.com/docs",
			Priority:                40,
			IsActive:                true,
		},
		{
			GatewayType:             models.GatewayLinkt,
			DisplayName:             "Linkt",
			Description:             "Global payment solution with card, bank, and wallet support.",
			LogoURL:                 "https://cdn.simpleicons.org/wise/9FE870",
			SupportsPayments:        true,
			SupportsRefunds:         true,
			SupportsSubscriptions:   true,
			SupportsPlatformSplit:   true,
			SupportedCountries:      models.StringArray{"US", "GB", "AU", "CA", "DE", "FR", "IN", "SG", "JP"},
			SupportedPaymentMethods: models.StringArray{"CARD", "BANK_ACCOUNT", "WALLET"},
			RequiredCredentials:     models.StringArray{"api_key", "api_secret", "webhook_secret"},
			SetupInstructions:       "Get your Linkt API credentials from the merchant dashboard.",
			DocumentationURL:        "https://linkt.com/docs",
			Priority:                30,
			IsActive:                true,
		},
	}

	// Use upsert (ON CONFLICT DO UPDATE) for idempotency
	result := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "gateway_type"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"display_name",
			"description",
			"logo_url",
			"supports_payments",
			"supports_refunds",
			"supports_subscriptions",
			"supports_platform_split",
			"supported_countries",
			"supported_payment_methods",
			"required_credentials",
			"setup_instructions",
			"documentation_url",
			"priority",
			"is_active",
			"updated_at",
		}),
	}).Create(&templates)

	if result.Error != nil {
		return result.Error
	}

	log.Printf("✓ Seeded %d payment gateway templates", len(templates))
	return nil
}
