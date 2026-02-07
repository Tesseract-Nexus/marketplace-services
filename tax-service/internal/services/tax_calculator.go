package services

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"tax-service/internal/models"
	"tax-service/internal/repository"
)

// TaxCalculator handles tax calculation logic
type TaxCalculator struct {
	repo     *repository.TaxRepository
	cacheTTL time.Duration
}

// NewTaxCalculator creates a new tax calculator
func NewTaxCalculator(repo *repository.TaxRepository, cacheTTL time.Duration) *TaxCalculator {
	return &TaxCalculator{
		repo:     repo,
		cacheTTL: cacheTTL,
	}
}

// CalculateTax calculates tax for a transaction
func (c *TaxCalculator) CalculateTax(ctx context.Context, req models.CalculateTaxRequest) (*models.TaxCalculationResponse, error) {
	// Check cache first
	cacheKey := c.generateCacheKey(req)
	cached, err := c.repo.GetCachedTaxCalculation(ctx, cacheKey)
	if err == nil && cached != nil {
		var response models.TaxCalculationResponse
		if err := json.Unmarshal([]byte(cached.CalculationResult), &response); err == nil {
			return &response, nil
		}
	}

	// Check customer exemption
	var isExempt bool
	var exemptReason string
	if req.CustomerID != nil {
		exemption, err := c.repo.GetCustomerExemption(ctx, req.TenantID, *req.CustomerID)
		if err == nil && exemption != nil {
			isExempt = true
			exemptReason = fmt.Sprintf("Customer exempt: %s (%s)", exemption.CertificateType, exemption.CertificateNumber)
		}
	}

	// If customer is exempt, return zero tax
	if isExempt {
		subtotal := c.calculateSubtotal(req.LineItems)
		return &models.TaxCalculationResponse{
			Subtotal:       subtotal,
			ShippingAmount: req.ShippingAmount,
			TaxAmount:      0,
			Total:          subtotal + req.ShippingAmount,
			TaxBreakdown:   []models.TaxBreakdown{},
			IsExempt:       true,
			ExemptReason:   exemptReason,
		}, nil
	}

	// Determine country code
	countryCode := req.ShippingAddress.CountryCode
	if countryCode == "" {
		countryCode = req.ShippingAddress.Country
	}

	// Check tax nexus - only collect tax if tenant has nexus in the destination jurisdiction
	hasNexus := c.checkNexus(ctx, req.TenantID, countryCode, req.ShippingAddress.StateCode)
	if !hasNexus {
		// No nexus - no tax collection obligation
		subtotal := c.calculateSubtotal(req.LineItems)
		return &models.TaxCalculationResponse{
			Subtotal:       subtotal,
			ShippingAmount: req.ShippingAmount,
			TaxAmount:      0,
			Total:          subtotal + req.ShippingAmount,
			TaxBreakdown:   []models.TaxBreakdown{},
			IsExempt:       false,
			ExemptReason:   "No tax nexus in destination jurisdiction",
		}, nil
	}

	// Route to country-specific tax calculation
	switch countryCode {
	case "IN":
		return c.calculateIndiaGST(ctx, req)
	case "DE", "FR", "IT", "ES", "NL", "BE", "AT", "PL", "SE", "DK", "FI", "IE", "PT", "GR", "CZ", "RO", "HU":
		return c.calculateEUVAT(ctx, req)
	case "GB":
		return c.calculateUKVAT(ctx, req)
	case "CA":
		return c.calculateCanadaTax(ctx, req)
	default:
		return c.calculateStandardTax(ctx, req)
	}
}

// checkNexus verifies if the tenant has tax nexus in the destination jurisdiction
func (c *TaxCalculator) checkNexus(ctx context.Context, tenantID, countryCode, stateCode string) bool {
	// Check country-level nexus first
	_, err := c.repo.GetNexusByCountry(ctx, tenantID, countryCode)
	if err == nil {
		return true // Has country-level nexus
	}

	// For US, check state-level nexus (economic nexus requirements)
	if countryCode == "US" && stateCode != "" {
		jurisdiction, err := c.repo.GetJurisdictionByStateCode(ctx, tenantID, stateCode)
		if err != nil {
			return false
		}
		_, err = c.repo.GetNexusByJurisdiction(ctx, tenantID, jurisdiction.ID)
		return err == nil
	}

	// For India, check state-level nexus (GST registration per state)
	if countryCode == "IN" && stateCode != "" {
		jurisdiction, err := c.repo.GetJurisdictionByStateCode(ctx, tenantID, stateCode)
		if err != nil {
			return false
		}
		_, err = c.repo.GetNexusByJurisdiction(ctx, tenantID, jurisdiction.ID)
		return err == nil
	}

	// For other countries, assume nexus if no specific check fails
	// In production, this should be more strict based on local laws
	return true
}

// calculateIndiaGST calculates India GST (CGST+SGST for intrastate, IGST for interstate)
func (c *TaxCalculator) calculateIndiaGST(ctx context.Context, req models.CalculateTaxRequest) (*models.TaxCalculationResponse, error) {
	subtotal := c.calculateSubtotal(req.LineItems)

	// Determine if interstate or intrastate
	originStateCode := ""
	destStateCode := req.ShippingAddress.StateCode

	if req.OriginAddress != nil {
		originStateCode = req.OriginAddress.StateCode
	}

	// If origin state code not provided, try to get from tenant's nexus
	if originStateCode == "" {
		nexus, err := c.repo.GetNexusByCountry(ctx, req.TenantID, "IN")
		if err == nil && nexus != nil {
			originStateCode = nexus.Jurisdiction.StateCode
		}
	}

	isInterstate := originStateCode != "" && destStateCode != "" && originStateCode != destStateCode

	var totalTax float64
	var taxBreakdown []models.TaxBreakdown
	gstSummary := &models.GSTSummary{IsInterstate: isInterstate}

	// Calculate tax for each line item
	for _, item := range req.LineItems {
		// Determine GST slab from HSN/SAC code or category
		gstSlab := c.getGSTSlab(ctx, req.TenantID, item)

		if gstSlab == 0 {
			continue // Exempt or nil-rated
		}

		if isInterstate {
			// IGST = full GST rate
			igstAmount := item.Subtotal * (gstSlab / 100.0)
			totalTax += igstAmount
			gstSummary.IGST += igstAmount

			taxBreakdown = append(taxBreakdown, models.TaxBreakdown{
				JurisdictionName: "India",
				TaxType:          "IGST",
				Rate:             gstSlab,
				TaxableAmount:    item.Subtotal,
				TaxAmount:        igstAmount,
				HSNCode:          item.HSNCode,
				SACCode:          item.SACCode,
			})
		} else {
			// CGST + SGST = half each
			halfRate := gstSlab / 2.0
			cgstAmount := item.Subtotal * (halfRate / 100.0)
			sgstAmount := item.Subtotal * (halfRate / 100.0)
			totalTax += cgstAmount + sgstAmount
			gstSummary.CGST += cgstAmount
			gstSummary.SGST += sgstAmount

			taxBreakdown = append(taxBreakdown, models.TaxBreakdown{
				JurisdictionName: "India - Central",
				TaxType:          "CGST",
				Rate:             halfRate,
				TaxableAmount:    item.Subtotal,
				TaxAmount:        cgstAmount,
				HSNCode:          item.HSNCode,
				SACCode:          item.SACCode,
			})
			taxBreakdown = append(taxBreakdown, models.TaxBreakdown{
				JurisdictionName: req.ShippingAddress.State,
				TaxType:          "SGST",
				Rate:             halfRate,
				TaxableAmount:    item.Subtotal,
				TaxAmount:        sgstAmount,
				HSNCode:          item.HSNCode,
				SACCode:          item.SACCode,
			})
		}
	}

	// Calculate shipping tax (same logic)
	if req.ShippingAmount > 0 {
		shippingGSTSlab := 18.0 // Default shipping GST rate
		if isInterstate {
			igstAmount := req.ShippingAmount * (shippingGSTSlab / 100.0)
			totalTax += igstAmount
			gstSummary.IGST += igstAmount
		} else {
			halfRate := shippingGSTSlab / 2.0
			cgstAmount := req.ShippingAmount * (halfRate / 100.0)
			sgstAmount := req.ShippingAmount * (halfRate / 100.0)
			totalTax += cgstAmount + sgstAmount
			gstSummary.CGST += cgstAmount
			gstSummary.SGST += sgstAmount
		}
	}

	gstSummary.TotalGST = totalTax

	response := &models.TaxCalculationResponse{
		Subtotal:       subtotal,
		ShippingAmount: req.ShippingAmount,
		TaxAmount:      totalTax,
		Total:          subtotal + req.ShippingAmount + totalTax,
		TaxBreakdown:   taxBreakdown,
		IsExempt:       false,
		GSTSummary:     gstSummary,
	}

	// Cache the result
	cacheKey := c.generateCacheKey(req)
	c.cacheResult(ctx, cacheKey, response)
	return response, nil
}

// getGSTSlab determines the GST slab rate for an item
func (c *TaxCalculator) getGSTSlab(ctx context.Context, tenantID string, item models.LineItemInput) float64 {
	// Try HSN code first (for goods)
	if item.HSNCode != "" {
		category, err := c.repo.GetProductCategoryByHSN(ctx, tenantID, item.HSNCode)
		if err == nil && category != nil {
			if category.IsTaxExempt || category.IsNilRated {
				return 0
			}
			return category.GSTSlab
		}
	}

	// Try SAC code (for services)
	if item.SACCode != "" {
		category, err := c.repo.GetProductCategoryBySAC(ctx, tenantID, item.SACCode)
		if err == nil && category != nil {
			if category.IsTaxExempt || category.IsNilRated {
				return 0
			}
			return category.GSTSlab
		}
	}

	// Try category ID
	if item.CategoryID != nil && *item.CategoryID != uuid.Nil {
		category, err := c.repo.GetProductCategory(ctx, *item.CategoryID)
		if err == nil && category != nil {
			if category.IsTaxExempt || category.IsNilRated {
				return 0
			}
			return category.GSTSlab
		}
	}

	// Default to 18% GST (most common slab)
	return 18.0
}

// calculateEUVAT calculates EU VAT with reverse-charge support
func (c *TaxCalculator) calculateEUVAT(ctx context.Context, req models.CalculateTaxRequest) (*models.TaxCalculationResponse, error) {
	subtotal := c.calculateSubtotal(req.LineItems)

	// Check for B2B reverse charge
	// If buyer has valid VAT number and it's a cross-border EU transaction, reverse charge applies
	reverseCharge := false
	if req.IsB2B && req.CustomerGSTIN != "" {
		// CustomerGSTIN is used for EU VAT number in this context
		reverseCharge = true
	}

	// If reverse charge, no VAT is charged (buyer accounts for VAT)
	if reverseCharge {
		return &models.TaxCalculationResponse{
			Subtotal:       subtotal,
			ShippingAmount: req.ShippingAmount,
			TaxAmount:      0,
			Total:          subtotal + req.ShippingAmount,
			TaxBreakdown:   []models.TaxBreakdown{},
			IsExempt:       false,
			ReverseCharge:  true,
			VATSummary: &models.VATSummary{
				IsReverseCharge: true,
				BuyerVATNumber:  req.CustomerGSTIN,
			},
		}, nil
	}

	// Get VAT rate for the country
	countryCode := req.ShippingAddress.CountryCode
	if countryCode == "" {
		countryCode = req.ShippingAddress.Country
	}

	jurisdictions, err := c.repo.GetJurisdictionByLocation(ctx, req.TenantID, countryCode, "", "", "")
	if err != nil || len(jurisdictions) == 0 {
		// No jurisdiction, return zero tax
		return &models.TaxCalculationResponse{
			Subtotal:       subtotal,
			ShippingAmount: req.ShippingAmount,
			TaxAmount:      0,
			Total:          subtotal + req.ShippingAmount,
			TaxBreakdown:   []models.TaxBreakdown{},
			IsExempt:       false,
		}, nil
	}

	// Get VAT rates
	var totalTax float64
	var taxBreakdown []models.TaxBreakdown

	for _, jurisdiction := range jurisdictions {
		rates, err := c.repo.GetActiveTaxRates(ctx, []uuid.UUID{jurisdiction.ID})
		if err != nil {
			continue
		}

		for _, rate := range rates {
			if rate.TaxType != models.TaxTypeVAT {
				continue
			}

			taxableAmount := subtotal
			if rate.AppliesToShipping {
				taxableAmount += req.ShippingAmount
			}

			taxAmount := taxableAmount * (rate.Rate / 100.0)
			totalTax += taxAmount

			taxBreakdown = append(taxBreakdown, models.TaxBreakdown{
				JurisdictionID:   jurisdiction.ID,
				JurisdictionName: jurisdiction.Name,
				TaxType:          "VAT",
				Rate:             rate.Rate,
				TaxableAmount:    taxableAmount,
				TaxAmount:        taxAmount,
			})
			break // Only apply one VAT rate per jurisdiction
		}
	}

	vatSummary := &models.VATSummary{
		VATAmount: totalTax,
	}
	if len(taxBreakdown) > 0 {
		vatSummary.VATRate = taxBreakdown[0].Rate
	}

	return &models.TaxCalculationResponse{
		Subtotal:       subtotal,
		ShippingAmount: req.ShippingAmount,
		TaxAmount:      totalTax,
		Total:          subtotal + req.ShippingAmount + totalTax,
		TaxBreakdown:   taxBreakdown,
		IsExempt:       false,
		VATSummary:     vatSummary,
	}, nil
}

// calculateUKVAT calculates UK VAT (post-Brexit)
func (c *TaxCalculator) calculateUKVAT(ctx context.Context, req models.CalculateTaxRequest) (*models.TaxCalculationResponse, error) {
	// Similar to EU VAT but no reverse charge within UK
	return c.calculateEUVAT(ctx, req)
}

// calculateCanadaTax calculates Canadian GST/HST/PST/QST
func (c *TaxCalculator) calculateCanadaTax(ctx context.Context, req models.CalculateTaxRequest) (*models.TaxCalculationResponse, error) {
	subtotal := c.calculateSubtotal(req.LineItems)

	// Get province from state code
	provinceCode := req.ShippingAddress.StateCode
	if provinceCode == "" {
		provinceCode = req.ShippingAddress.State
	}

	jurisdictions, err := c.repo.GetJurisdictionByLocation(ctx, req.TenantID, "CA", provinceCode, "", "")
	if err != nil || len(jurisdictions) == 0 {
		return &models.TaxCalculationResponse{
			Subtotal:       subtotal,
			ShippingAmount: req.ShippingAmount,
			TaxAmount:      0,
			Total:          subtotal + req.ShippingAmount,
			TaxBreakdown:   []models.TaxBreakdown{},
			IsExempt:       false,
		}, nil
	}

	var totalTax float64
	var taxBreakdown []models.TaxBreakdown
	taxableAmount := subtotal + req.ShippingAmount

	for _, jurisdiction := range jurisdictions {
		rates, err := c.repo.GetActiveTaxRates(ctx, []uuid.UUID{jurisdiction.ID})
		if err != nil {
			continue
		}

		var compoundBase = taxableAmount
		for _, rate := range rates {
			var taxAmount float64
			if rate.IsCompound {
				// QST is compound (calculated on subtotal + GST)
				taxAmount = compoundBase * (rate.Rate / 100.0)
			} else {
				taxAmount = taxableAmount * (rate.Rate / 100.0)
				compoundBase += taxAmount // Add to base for compound calculation
			}

			totalTax += taxAmount

			taxBreakdown = append(taxBreakdown, models.TaxBreakdown{
				JurisdictionID:   jurisdiction.ID,
				JurisdictionName: jurisdiction.Name,
				TaxType:          string(rate.TaxType),
				Rate:             rate.Rate,
				TaxableAmount:    taxableAmount,
				TaxAmount:        taxAmount,
				IsCompound:       rate.IsCompound,
			})
		}
	}

	return &models.TaxCalculationResponse{
		Subtotal:       subtotal,
		ShippingAmount: req.ShippingAmount,
		TaxAmount:      totalTax,
		Total:          subtotal + req.ShippingAmount + totalTax,
		TaxBreakdown:   taxBreakdown,
		IsExempt:       false,
	}, nil
}

// calculateStandardTax calculates tax using the standard method (US and other countries)
func (c *TaxCalculator) calculateStandardTax(ctx context.Context, req models.CalculateTaxRequest) (*models.TaxCalculationResponse, error) {
	subtotal := c.calculateSubtotal(req.LineItems)

	// Resolve country and state codes (prefer ISO codes over full names for jurisdiction matching)
	countryCode := req.ShippingAddress.CountryCode
	if countryCode == "" {
		countryCode = req.ShippingAddress.Country
	}
	stateCode := req.ShippingAddress.StateCode
	if stateCode == "" {
		stateCode = req.ShippingAddress.State
	}

	// Resolve jurisdictions from address
	jurisdictions, err := c.repo.GetJurisdictionByLocation(
		ctx,
		req.TenantID,
		countryCode,
		stateCode,
		req.ShippingAddress.City,
		req.ShippingAddress.Zip,
	)
	if err != nil || len(jurisdictions) == 0 {
		return &models.TaxCalculationResponse{
			Subtotal:       subtotal,
			ShippingAmount: req.ShippingAmount,
			TaxAmount:      0,
			Total:          subtotal + req.ShippingAmount,
			TaxBreakdown:   []models.TaxBreakdown{},
			IsExempt:       false,
		}, nil
	}

	// Calculate tax for each line item
	var totalTax float64
	var taxBreakdownMap = make(map[uuid.UUID]*models.TaxBreakdown)

	for _, item := range req.LineItems {
		itemTax, breakdown, err := c.calculateItemTax(ctx, item, jurisdictions)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate tax for item: %w", err)
		}

		totalTax += itemTax

		for _, bd := range breakdown {
			if existing, exists := taxBreakdownMap[bd.JurisdictionID]; exists {
				existing.TaxableAmount += bd.TaxableAmount
				existing.TaxAmount += bd.TaxAmount
			} else {
				taxBreakdownMap[bd.JurisdictionID] = &bd
			}
		}
	}

	// Calculate shipping tax
	if req.ShippingAmount > 0 {
		shippingTax, shippingBreakdown := c.calculateShippingTax(ctx, req.ShippingAmount, jurisdictions)
		totalTax += shippingTax

		for _, bd := range shippingBreakdown {
			if existing, exists := taxBreakdownMap[bd.JurisdictionID]; exists {
				existing.TaxableAmount += bd.TaxableAmount
				existing.TaxAmount += bd.TaxAmount
			} else {
				taxBreakdownMap[bd.JurisdictionID] = &bd
			}
		}
	}

	var taxBreakdown []models.TaxBreakdown
	for _, bd := range taxBreakdownMap {
		taxBreakdown = append(taxBreakdown, *bd)
	}

	response := &models.TaxCalculationResponse{
		Subtotal:       subtotal,
		ShippingAmount: req.ShippingAmount,
		TaxAmount:      totalTax,
		Total:          subtotal + req.ShippingAmount + totalTax,
		TaxBreakdown:   taxBreakdown,
		IsExempt:       false,
	}

	// Cache the result
	cacheKey := c.generateCacheKey(req)
	c.cacheResult(ctx, cacheKey, response)
	return response, nil
}

// calculateItemTax calculates tax for a single line item
func (c *TaxCalculator) calculateItemTax(ctx context.Context, item models.LineItemInput, jurisdictions []models.TaxJurisdiction) (float64, []models.TaxBreakdown, error) {
	var totalTax float64
	var breakdown []models.TaxBreakdown

	// Check if category is tax-exempt
	if item.CategoryID != nil && *item.CategoryID != uuid.Nil {
		category, err := c.repo.GetProductCategory(ctx, *item.CategoryID)
		if err == nil && category != nil && category.IsTaxExempt {
			// Product category is exempt
			return 0, breakdown, nil
		}
	}

	// Get tax rates for each jurisdiction
	for _, jurisdiction := range jurisdictions {
		var rates []models.TaxRate
		var overrides []models.TaxRateCategoryOverride

		if item.CategoryID != nil && *item.CategoryID != uuid.Nil {
			r, o, err := c.repo.GetTaxRatesForJurisdictionAndCategory(ctx, jurisdiction.ID, *item.CategoryID)
			if err != nil {
				continue
			}
			rates = r
			overrides = o
		} else {
			jurisdictionIDs := []uuid.UUID{jurisdiction.ID}
			r, err := c.repo.GetActiveTaxRates(ctx, jurisdictionIDs)
			if err != nil {
				continue
			}
			rates = r
		}

		// Apply rates (compound calculation based on priority)
		var compoundBase = item.Subtotal
		for _, rate := range rates {
			if !rate.AppliesToProducts {
				continue
			}

			// Check for category override
			effectiveRate := rate.Rate
			for _, override := range overrides {
				if override.TaxRateID == rate.ID {
					if override.IsExempt {
						effectiveRate = 0
					} else if override.OverrideRate != nil {
						effectiveRate = *override.OverrideRate
					}
					break
				}
			}

			if effectiveRate == 0 {
				continue
			}

			// Calculate tax for this rate
			var taxAmount float64
			if rate.IsCompound {
				// Compound: tax is applied on subtotal + previous taxes
				taxAmount = compoundBase * (effectiveRate / 100.0)
				compoundBase += taxAmount
			} else {
				// Simple: tax is applied on original subtotal
				taxAmount = item.Subtotal * (effectiveRate / 100.0)
			}

			totalTax += taxAmount

			// Add to breakdown
			breakdown = append(breakdown, models.TaxBreakdown{
				JurisdictionID:   jurisdiction.ID,
				JurisdictionName: jurisdiction.Name,
				TaxType:          string(rate.TaxType),
				Rate:             effectiveRate,
				TaxableAmount:    item.Subtotal,
				TaxAmount:        taxAmount,
			})
		}
	}

	return totalTax, breakdown, nil
}

// calculateShippingTax calculates tax on shipping amount
func (c *TaxCalculator) calculateShippingTax(ctx context.Context, shippingAmount float64, jurisdictions []models.TaxJurisdiction) (float64, []models.TaxBreakdown) {
	var totalTax float64
	var breakdown []models.TaxBreakdown

	for _, jurisdiction := range jurisdictions {
		jurisdictionIDs := []uuid.UUID{jurisdiction.ID}
		rates, err := c.repo.GetActiveTaxRates(ctx, jurisdictionIDs)
		if err != nil {
			continue
		}

		var compoundBase = shippingAmount
		for _, rate := range rates {
			if !rate.AppliesToShipping {
				continue
			}

			// Calculate tax
			var taxAmount float64
			if rate.IsCompound {
				taxAmount = compoundBase * (rate.Rate / 100.0)
				compoundBase += taxAmount
			} else {
				taxAmount = shippingAmount * (rate.Rate / 100.0)
			}

			totalTax += taxAmount

			breakdown = append(breakdown, models.TaxBreakdown{
				JurisdictionID:   jurisdiction.ID,
				JurisdictionName: jurisdiction.Name,
				TaxType:          string(rate.TaxType),
				Rate:             rate.Rate,
				TaxableAmount:    shippingAmount,
				TaxAmount:        taxAmount,
			})
		}
	}

	return totalTax, breakdown
}

// calculateSubtotal calculates the subtotal from line items
func (c *TaxCalculator) calculateSubtotal(items []models.LineItemInput) float64 {
	var subtotal float64
	for _, item := range items {
		subtotal += item.Subtotal
	}
	return subtotal
}

// generateCacheKey generates a cache key for the tax calculation
func (c *TaxCalculator) generateCacheKey(req models.CalculateTaxRequest) string {
	// Create a deterministic string from request
	key := fmt.Sprintf("%s:%s:%s:%s:%s:%f",
		req.TenantID,
		req.ShippingAddress.Country,
		req.ShippingAddress.State,
		req.ShippingAddress.City,
		req.ShippingAddress.Zip,
		req.ShippingAmount,
	)

	// Add line items
	for _, item := range req.LineItems {
		categoryID := "nil"
		if item.CategoryID != nil {
			categoryID = item.CategoryID.String()
		}
		key += fmt.Sprintf(":%s:%f", categoryID, item.Subtotal)
	}

	// Add customer ID if present
	if req.CustomerID != nil {
		key += fmt.Sprintf(":%s", req.CustomerID.String())
	}

	// Hash the key
	hash := md5.Sum([]byte(key))
	return fmt.Sprintf("%x", hash)
}

// cacheResult caches the tax calculation result
func (c *TaxCalculator) cacheResult(ctx context.Context, cacheKey string, response *models.TaxCalculationResponse) {
	resultJSON, err := json.Marshal(response)
	if err != nil {
		return
	}

	cache := &models.TaxCalculationCache{
		CacheKey:          cacheKey,
		CalculationResult: string(resultJSON),
		ExpiresAt:         time.Now().Add(c.cacheTTL),
	}

	c.repo.CacheTaxCalculation(ctx, cache)
}

// ValidateAddress validates an address (basic validation, can be extended with external API)
func (c *TaxCalculator) ValidateAddress(ctx context.Context, address models.AddressInput) (*models.ValidateAddressResponse, error) {
	// Basic validation
	isValid := true
	if address.City == "" || address.Country == "" {
		isValid = false
	}

	// For now, just return the address as standardized
	// In production, integrate with USPS, Google Maps, or similar service
	response := &models.ValidateAddressResponse{
		IsValid:             isValid,
		StandardizedAddress: address,
		Suggestions:         []models.AddressInput{},
	}

	return response, nil
}
