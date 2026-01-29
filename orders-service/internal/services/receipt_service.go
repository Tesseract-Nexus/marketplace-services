package services

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"html/template"
	goimage "image"
	"image/color"
	"image/png"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	_ "image/jpeg"

	"github.com/google/uuid"
	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	mimage "github.com/johnfercher/maroto/v2/pkg/components/image"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/extension"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/props"

	"orders-service/internal/clients"
	"orders-service/internal/models"
	"orders-service/internal/repository"
)

// ReceiptService handles receipt generation operations
type ReceiptService interface {
	// GenerateReceipt generates a receipt for an order (in-memory, not stored)
	GenerateReceipt(order *models.Order, tenantID string, req *models.ReceiptGenerationRequest) ([]byte, string, error)

	// GenerateAndStoreReceipt generates a receipt and stores it in the document service
	// Returns the receipt document with short URL for secure access
	GenerateAndStoreReceipt(ctx context.Context, order *models.Order, tenantID string, req *models.GenerateReceiptAndStoreRequest) (*models.ReceiptDocument, error)

	// GetReceiptByShortCode retrieves a receipt document by its short code and returns a presigned URL
	// This is the secure download endpoint - validates access before returning URL
	GetReceiptByShortCode(ctx context.Context, shortCode string) (*models.ReceiptDownloadResponse, *models.ReceiptDocument, error)

	// GetReceiptDocuments retrieves all receipt documents for an order
	GetReceiptDocuments(ctx context.Context, orderID uuid.UUID, tenantID string) ([]models.ReceiptDocument, error)

	// GetReceiptSettings gets receipt settings for a tenant
	GetReceiptSettings(tenantID string) (*models.ReceiptSettings, error)

	// UpdateReceiptSettings updates receipt settings for a tenant
	UpdateReceiptSettings(tenantID string, req *models.ReceiptSettingsUpdateRequest) (*models.ReceiptSettings, error)

	// GetOrCreateSettings gets or creates default settings for a tenant
	GetOrCreateSettings(tenantID string) (*models.ReceiptSettings, error)

	// GetStorageConfig returns the receipt storage configuration
	GetStorageConfig() *models.ReceiptStorageConfig
}

type receiptService struct {
	settingsRepo   *repository.ReceiptSettingsRepository
	documentRepo   *repository.ReceiptDocumentRepository
	documentClient clients.DocumentClient
	tenantClient   clients.TenantClient
	storageConfig  *models.ReceiptStorageConfig
}

// NewReceiptService creates a new receipt service
func NewReceiptService(
	settingsRepo *repository.ReceiptSettingsRepository,
	documentRepo *repository.ReceiptDocumentRepository,
	documentClient clients.DocumentClient,
	tenantClient clients.TenantClient,
) ReceiptService {
	// Load storage configuration from environment
	bucket := os.Getenv("RECEIPT_STORAGE_BUCKET")
	if bucket == "" {
		bucket = "marketplace-receipts" // Default bucket name
	}

	pathPrefix := os.Getenv("RECEIPT_STORAGE_PATH_PREFIX")
	if pathPrefix == "" {
		pathPrefix = "receipts"
	}

	shortURLBase := os.Getenv("RECEIPT_SHORT_URL_BASE")
	if shortURLBase == "" {
		shortURLBase = "/r" // Default short URL prefix
	}

	return &receiptService{
		settingsRepo:   settingsRepo,
		documentRepo:   documentRepo,
		documentClient: documentClient,
		tenantClient:   tenantClient,
		storageConfig: &models.ReceiptStorageConfig{
			Bucket:                bucket,
			PathPrefix:            pathPrefix,
			ShortURLBaseURL:       shortURLBase,
			ExpiryDays:            0, // 0 = never expires
			AutoGenerateOnPayment: true,
		},
	}
}

// GetStorageConfig returns the receipt storage configuration
func (s *receiptService) GetStorageConfig() *models.ReceiptStorageConfig {
	return s.storageConfig
}

// GenerateAndStoreReceipt generates a receipt and stores it in the document service.
// If a receipt already exists for this order, it returns the existing one unless force regeneration is requested.
func (s *receiptService) GenerateAndStoreReceipt(ctx context.Context, order *models.Order, tenantID string, req *models.GenerateReceiptAndStoreRequest) (*models.ReceiptDocument, error) {
	// Check if a receipt already exists for this order
	existing, err := s.documentRepo.GetLatestByOrderID(order.ID, tenantID)
	if err != nil {
		log.Printf("WARNING: Failed to check existing receipt: %v", err)
	}
	if existing != nil && (req == nil || !req.ForceRegenerate) {
		return existing, nil
	}

	// Set defaults
	docType := models.ReceiptDocumentTypeReceipt
	if req != nil && req.DocumentType != "" {
		docType = req.DocumentType
	}

	format := models.ReceiptFormatPDF
	if req != nil && req.Format != "" {
		format = req.Format
	}

	tmpl := models.ReceiptTemplateDefault
	if req != nil && req.Template != "" {
		tmpl = req.Template
	}

	// Generate receipt data
	var locale string
	if req != nil {
		locale = req.Locale
	}
	genReq := &models.ReceiptGenerationRequest{
		Format:   format,
		Template: tmpl,
		Locale:   locale,
	}

	data, contentType, err := s.GenerateReceipt(order, tenantID, genReq)
	if err != nil {
		return nil, fmt.Errorf("failed to generate receipt: %w", err)
	}

	// Generate receipt and invoice numbers
	receiptNumber := repository.GenerateReceiptNumber(order.OrderNumber)
	var invoiceNumber string
	if docType == models.ReceiptDocumentTypeInvoice || docType == models.ReceiptDocumentTypeTaxInvoice {
		invoiceNumber = repository.GenerateInvoiceNumber(order.OrderNumber)
	}

	// Calculate checksum for integrity
	checksum := md5.Sum(data)
	checksumStr := hex.EncodeToString(checksum[:])

	// Build storage path: {prefix}/{tenant_id}/{year}/{month}/{order_number}_{receipt_number}.pdf
	now := time.Now()
	storagePath := fmt.Sprintf("%s/%s/%d/%02d/%s_%s",
		s.storageConfig.PathPrefix,
		tenantID,
		now.Year(),
		now.Month(),
		order.OrderNumber,
		receiptNumber,
	)
	if format == models.ReceiptFormatPDF {
		storagePath += ".pdf"
	} else {
		storagePath += ".html"
	}

	// Determine filename
	filename := fmt.Sprintf("receipt-%s.pdf", order.OrderNumber)
	if docType == models.ReceiptDocumentTypeInvoice || docType == models.ReceiptDocumentTypeTaxInvoice {
		filename = fmt.Sprintf("invoice-%s.pdf", order.OrderNumber)
	}

	// Upload to document service (PRIVATE bucket - isPublic: false)
	uploadReq := &clients.DocumentUploadRequest{
		TenantID:    tenantID,
		Bucket:      s.storageConfig.Bucket,
		Path:        storagePath,
		Filename:    filename,
		ContentType: contentType,
		Data:        data,
		IsPublic:    false, // IMPORTANT: Receipts are PRIVATE
		Tags: map[string]string{
			"tenant_id":      tenantID,
			"order_id":       order.ID.String(),
			"order_number":   order.OrderNumber,
			"receipt_number": receiptNumber,
			"document_type":  string(docType),
		},
		EntityType: "receipt",
		EntityID:   order.ID.String(),
		ProductID:  "marketplace",
	}

	var docID string
	if s.documentClient != nil {
		uploadResp, err := s.documentClient.UploadDocument(ctx, uploadReq)
		if err != nil {
			log.Printf("WARNING: Failed to upload receipt to document service: %v (continuing without storage)", err)
		} else {
			docID = uploadResp.ID
		}
	}

	// Create receipt document record
	receiptDoc := &models.ReceiptDocument{
		ID:              uuid.New(),
		TenantID:        tenantID,
		OrderID:         order.ID,
		VendorID:        order.VendorID,
		ReceiptNumber:   receiptNumber,
		InvoiceNumber:   invoiceNumber,
		DocumentType:    docType,
		Format:          format,
		Template:        tmpl,
		StorageBucket:   s.storageConfig.Bucket,
		StoragePath:     storagePath,
		DocumentID:      docID,
		FileSize:        int64(len(data)),
		ContentChecksum: checksumStr,
		CustomerEmail:   "",
		OrderTotal:      order.Total,
		Currency:        order.Currency,
	}

	if order.Customer != nil {
		receiptDoc.CustomerEmail = order.Customer.Email
	}

	// Generate short URL after we have the receipt doc ID
	if err := s.documentRepo.Create(receiptDoc); err != nil {
		return nil, fmt.Errorf("failed to save receipt document: %w", err)
	}

	// Build the short URL using the generated short code
	receiptDoc.ShortURL = fmt.Sprintf("%s/%s", s.storageConfig.ShortURLBaseURL, receiptDoc.ShortCode)
	if err := s.documentRepo.Update(receiptDoc); err != nil {
		log.Printf("WARNING: Failed to update receipt short URL: %v", err)
	}

	return receiptDoc, nil
}

// GetReceiptByShortCode retrieves a receipt document by its short code
// Returns a presigned URL for secure download
func (s *receiptService) GetReceiptByShortCode(ctx context.Context, shortCode string) (*models.ReceiptDownloadResponse, *models.ReceiptDocument, error) {
	// Get receipt document by short code
	doc, err := s.documentRepo.GetByShortCode(shortCode)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get receipt document: %w", err)
	}
	if doc == nil {
		return nil, nil, fmt.Errorf("receipt not found")
	}

	// Check if expired (if expiry is set)
	if doc.ExpiresAt != nil && time.Now().After(*doc.ExpiresAt) {
		return nil, nil, fmt.Errorf("receipt link has expired")
	}

	// Generate presigned URL for download (15 minute expiry)
	var downloadURL string
	var expiresAt time.Time

	if s.documentClient != nil {
		presignReq := &clients.PresignedURLRequest{
			TenantID:  doc.TenantID,
			Bucket:    doc.StorageBucket,
			Path:      doc.StoragePath,
			Method:    "GET",
			ExpiresIn: 900, // 15 minutes
		}

		presignResp, err := s.documentClient.GetPresignedURL(ctx, presignReq)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate download URL: %w", err)
		}

		downloadURL = presignResp.URL
		expiresAt = presignResp.ExpiresAt
	} else {
		// Fallback: return direct API endpoint (for development)
		downloadURL = fmt.Sprintf("/api/v1/receipts/%s/download", doc.ID.String())
		expiresAt = time.Now().Add(15 * time.Minute)
	}

	// Increment access count for audit
	if err := s.documentRepo.IncrementAccessCount(doc.ID); err != nil {
		log.Printf("WARNING: Failed to increment access count: %v", err)
	}

	// Determine order number from receipt number
	orderNumber := ""
	if strings.HasPrefix(doc.ReceiptNumber, "RCP-") {
		// Extract order timestamp from receipt number
		// Receipt: RCP-YYYYMMDD-XXXXX -> Order: ORD-XXXXX
		parts := strings.Split(doc.ReceiptNumber, "-")
		if len(parts) >= 3 {
			orderNumber = "ORD-" + parts[2]
		}
	}

	response := &models.ReceiptDownloadResponse{
		ReceiptNumber: doc.ReceiptNumber,
		OrderNumber:   orderNumber,
		DownloadURL:   downloadURL,
		ExpiresAt:     expiresAt,
		Format:        string(doc.Format),
		FileSize:      doc.FileSize,
	}

	return response, doc, nil
}

// GetReceiptDocuments retrieves all receipt documents for an order
func (s *receiptService) GetReceiptDocuments(ctx context.Context, orderID uuid.UUID, tenantID string) ([]models.ReceiptDocument, error) {
	return s.documentRepo.GetByOrderID(orderID, tenantID)
}

// GenerateReceipt generates a receipt for an order
func (s *receiptService) GenerateReceipt(order *models.Order, tenantID string, req *models.ReceiptGenerationRequest) ([]byte, string, error) {
	// Get receipt settings
	settings, err := s.GetOrCreateSettings(tenantID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get receipt settings: %w", err)
	}

	// Set defaults
	format := models.ReceiptFormatPDF
	if req != nil && req.Format != "" {
		format = req.Format
	}

	tmpl := settings.DefaultTemplate
	if req != nil && req.Template != "" {
		tmpl = req.Template
	}

	locale := "en-US"
	if req != nil && req.Locale != "" {
		locale = req.Locale
	}

	// Build receipt data
	receiptData := s.buildReceiptData(order, settings, format, tmpl, locale)

	// Generate based on format
	var data []byte
	var contentType string

	switch format {
	case models.ReceiptFormatPDF:
		data, err = s.generatePDF(receiptData)
		contentType = "application/pdf"
	case models.ReceiptFormatHTML:
		data, err = s.generateHTML(receiptData)
		contentType = "text/html"
	default:
		return nil, "", fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return nil, "", fmt.Errorf("failed to generate receipt: %w", err)
	}

	return data, contentType, nil
}

// buildReceiptData constructs the receipt data structure
func (s *receiptService) buildReceiptData(order *models.Order, settings *models.ReceiptSettings, format models.ReceiptFormat, tmpl models.ReceiptTemplate, locale string) *models.ReceiptData {
	// Safely extract suffix from order number (e.g., ORD-xxx → RCP-xxx)
	orderSuffix := order.OrderNumber
	if len(order.OrderNumber) > 4 && strings.HasPrefix(order.OrderNumber, "ORD-") {
		orderSuffix = order.OrderNumber[4:]
	}
	data := &models.ReceiptData{
		ReceiptNumber: fmt.Sprintf("RCP-%s", orderSuffix),
		GeneratedAt:   time.Now(),
		Order:         order,
		Settings:      settings,
		Format:        format,
		Template:      tmpl,
		Locale:        locale,
	}

	// Format currency values
	currencySymbol := getCurrencySymbol(order.Currency)
	data.FormattedSubtotal = formatCurrency(order.Subtotal, currencySymbol)
	data.FormattedTax = formatCurrency(order.TaxAmount, currencySymbol)
	data.FormattedShipping = formatCurrency(order.ShippingCost, currencySymbol)
	data.FormattedDiscount = formatCurrency(order.DiscountAmount, currencySymbol)
	data.FormattedTotal = formatCurrency(order.Total, currencySymbol)

	// Build tax lines based on order tax data
	data.TaxLines = s.buildTaxLines(order, currencySymbol)

	// Build QR code URL for order tracking
	if order.StorefrontHost != "" {
		data.QRCodeURL = fmt.Sprintf("https://%s/orders/%s", order.StorefrontHost, order.OrderNumber)
	}

	return data
}

// buildTaxLines builds tax breakdown lines for display
func (s *receiptService) buildTaxLines(order *models.Order, currencySymbol string) []models.ReceiptTaxLine {
	var lines []models.ReceiptTaxLine

	// India GST taxes
	if order.CGST > 0 {
		lines = append(lines, models.ReceiptTaxLine{
			Name:   "CGST",
			Rate:   calculateTaxRate(order.CGST, order.Subtotal),
			Amount: formatCurrency(order.CGST, currencySymbol),
		})
	}
	if order.SGST > 0 {
		lines = append(lines, models.ReceiptTaxLine{
			Name:   "SGST",
			Rate:   calculateTaxRate(order.SGST, order.Subtotal),
			Amount: formatCurrency(order.SGST, currencySymbol),
		})
	}
	if order.IGST > 0 {
		lines = append(lines, models.ReceiptTaxLine{
			Name:   "IGST",
			Rate:   calculateTaxRate(order.IGST, order.Subtotal),
			Amount: formatCurrency(order.IGST, currencySymbol),
		})
	}
	if order.UTGST > 0 {
		lines = append(lines, models.ReceiptTaxLine{
			Name:   "UTGST",
			Rate:   calculateTaxRate(order.UTGST, order.Subtotal),
			Amount: formatCurrency(order.UTGST, currencySymbol),
		})
	}
	if order.GSTCess > 0 {
		lines = append(lines, models.ReceiptTaxLine{
			Name:   "GST Cess",
			Rate:   calculateTaxRate(order.GSTCess, order.Subtotal),
			Amount: formatCurrency(order.GSTCess, currencySymbol),
		})
	}

	// EU VAT
	if order.VATAmount > 0 {
		lines = append(lines, models.ReceiptTaxLine{
			Name:   "VAT",
			Rate:   calculateTaxRate(order.VATAmount, order.Subtotal),
			Amount: formatCurrency(order.VATAmount, currencySymbol),
		})
	}

	// Generic tax if no specific breakdown
	if len(lines) == 0 && order.TaxAmount > 0 {
		lines = append(lines, models.ReceiptTaxLine{
			Name:   "Tax",
			Rate:   calculateTaxRate(order.TaxAmount, order.Subtotal),
			Amount: formatCurrency(order.TaxAmount, currencySymbol),
		})
	}

	return lines
}

// PDF color palette - clean, professional grays and a subtle accent
var (
	pdfDarkText   = &props.Color{Red: 33, Green: 37, Blue: 41}    // #212529
	pdfMediumText = &props.Color{Red: 73, Green: 80, Blue: 87}    // #495057
	pdfLightText  = &props.Color{Red: 108, Green: 117, Blue: 125} // #6C757D
	pdfHeaderBg   = &props.Color{Red: 248, Green: 249, Blue: 250} // #F8F9FA
	pdfTableBg    = &props.Color{Red: 233, Green: 236, Blue: 239} // #E9ECEF
	pdfAccent     = &props.Color{Red: 13, Green: 110, Blue: 253}  // #0D6EFD
	pdfWhite      = &props.Color{Red: 255, Green: 255, Blue: 255}
	pdfTotalBg    = &props.Color{Red: 33, Green: 37, Blue: 41}    // #212529
)

// generatePDF generates a PDF receipt using maroto
func (s *receiptService) generatePDF(data *models.ReceiptData) ([]byte, error) {
	cfg := config.NewBuilder().
		WithPageNumber().
		WithLeftMargin(15).
		WithTopMargin(15).
		WithRightMargin(15).
		Build()

	m := maroto.New(cfg)

	s.addPDFHeader(m, data)
	s.addPDFReceiptDetails(m, data)
	s.addPDFAddresses(m, data)
	s.addPDFItemsTable(m, data)
	s.addPDFTotals(m, data)

	if data.Settings.ShowPaymentDetails && data.Order.Payment != nil {
		s.addPDFPaymentInfo(m, data)
	}

	s.addPDFFooter(m, data)

	pdfDoc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}

	return pdfDoc.GetBytes(), nil
}

// fetchAndCircleCropLogo downloads a logo from URL and crops it into a circle, returning PNG bytes
func (s *receiptService) fetchAndCircleCropLogo(logoURL string) ([]byte, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(logoURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch logo: status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // 5MB limit
	if err != nil {
		return nil, err
	}

	src, _, err := goimage.Decode(bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	// Crop to circle
	bounds := src.Bounds()
	size := bounds.Dx()
	if bounds.Dy() < size {
		size = bounds.Dy()
	}

	// Create a square RGBA image
	dst := goimage.NewRGBA(goimage.Rect(0, 0, size, size))
	center := float64(size) / 2.0
	radius := center

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - center
			dy := float64(y) - center
			if math.Sqrt(dx*dx+dy*dy) <= radius {
				dst.Set(x, y, src.At(bounds.Min.X+x, bounds.Min.Y+y))
			} else {
				dst.Set(x, y, color.Transparent)
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// addPDFHeader adds the header section to the PDF
func (s *receiptService) addPDFHeader(m core.Maroto, data *models.ReceiptData) {
	// Try to fetch logo
	var logoBytes []byte
	if data.Settings.LogoURL != "" {
		var err error
		logoBytes, err = s.fetchAndCircleCropLogo(data.Settings.LogoURL)
		if err != nil {
			log.Printf("WARNING: Failed to fetch logo for receipt: %v", err)
		}
	}

	// Business info components
	businessCols := []core.Col{}
	if logoBytes != nil {
		businessCols = append(businessCols,
			col.New(1).Add(
				mimage.NewFromBytes(logoBytes, extension.Png, props.Rect{
					Center:  true,
					Percent: 85,
				}),
			),
		)
		businessCols = append(businessCols,
			col.New(5).Add(
				text.New(data.Settings.BusinessName, props.Text{
					Size:  14,
					Style: fontstyle.Bold,
					Color: pdfDarkText,
					Top:   2,
				}),
				text.New(s.buildBusinessContactLine(data.Settings), props.Text{
					Size:  8,
					Color: pdfLightText,
					Top:   9,
				}),
			),
		)
	} else {
		businessCols = append(businessCols,
			col.New(6).Add(
				text.New(data.Settings.BusinessName, props.Text{
					Size:  14,
					Style: fontstyle.Bold,
					Color: pdfDarkText,
				}),
				text.New(s.buildBusinessContactLine(data.Settings), props.Text{
					Size:  8,
					Color: pdfLightText,
					Top:   7,
				}),
			),
		)
	}

	// Receipt title on the right
	businessCols = append(businessCols,
		col.New(6).Add(
			text.New("RECEIPT", props.Text{
				Size:  22,
				Style: fontstyle.Bold,
				Align: align.Right,
				Color: pdfDarkText,
			}),
			text.New(data.ReceiptNumber, props.Text{
				Size:  9,
				Top:   9,
				Align: align.Right,
				Color: pdfAccent,
			}),
			text.New(data.GeneratedAt.Format("January 02, 2006"), props.Text{
				Size:  8,
				Top:   14,
				Align: align.Right,
				Color: pdfLightText,
			}),
		),
	)

	m.AddRow(22, businessCols...).WithStyle(&props.Cell{
		BackgroundColor: pdfHeaderBg,
	})

	// Thin accent line under header
	m.AddRow(1).WithStyle(&props.Cell{
		BackgroundColor: pdfAccent,
	})

	// Spacing
	m.AddRow(4)
}

// buildBusinessContactLine creates a single-line contact summary
func (s *receiptService) buildBusinessContactLine(settings *models.ReceiptSettings) string {
	parts := []string{}
	if settings.BusinessAddress != "" {
		// Take just the first line of address for the header
		addrLine := strings.Split(settings.BusinessAddress, "\n")[0]
		parts = append(parts, addrLine)
	}
	if settings.BusinessPhone != "" {
		parts = append(parts, settings.BusinessPhone)
	}
	if settings.BusinessEmail != "" {
		parts = append(parts, settings.BusinessEmail)
	}
	if settings.BusinessWebsite != "" {
		parts = append(parts, settings.BusinessWebsite)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "  |  ")
}

// addPDFReceiptDetails adds receipt metadata in a clean grid
func (s *receiptService) addPDFReceiptDetails(m core.Maroto, data *models.ReceiptData) {
	order := data.Order

	// Row with order details in a light background box
	m.AddRow(18,
		col.New(3).Add(
			text.New("Order Number", props.Text{Size: 7, Color: pdfLightText, Top: 2}),
			text.New(order.OrderNumber, props.Text{Size: 9, Style: fontstyle.Bold, Color: pdfDarkText, Top: 7}),
		),
		col.New(3).Add(
			text.New("Order Date", props.Text{Size: 7, Color: pdfLightText, Top: 2}),
			text.New(order.CreatedAt.Format("Jan 02, 2006"), props.Text{Size: 9, Color: pdfDarkText, Top: 7}),
		),
		col.New(3).Add(
			text.New("Status", props.Text{Size: 7, Color: pdfLightText, Align: align.Right, Top: 2}),
			text.New(string(order.Status), props.Text{Size: 9, Style: fontstyle.Bold, Color: pdfDarkText, Align: align.Right, Top: 7}),
		),
		col.New(3).Add(
			text.New("Payment", props.Text{Size: 7, Color: pdfLightText, Align: align.Right, Top: 2}),
			text.New(string(order.PaymentStatus), props.Text{Size: 9, Style: fontstyle.Bold, Color: pdfDarkText, Align: align.Right, Top: 7}),
		),
	).WithStyle(&props.Cell{BackgroundColor: pdfHeaderBg})

	m.AddRow(4)
}

// addPDFAddresses adds billing and shipping addresses
func (s *receiptService) addPDFAddresses(m core.Maroto, data *models.ReceiptData) {
	order := data.Order

	var customerName, customerEmail string
	if order.Customer != nil {
		customerName = fmt.Sprintf("%s %s", order.Customer.FirstName, order.Customer.LastName)
		customerEmail = order.Customer.Email
	}

	var shippingAddr string
	if order.Shipping != nil && data.Settings.ShowShippingDetails {
		parts := []string{order.Shipping.Street}
		cityLine := order.Shipping.City
		if order.Shipping.State != "" {
			cityLine += ", " + order.Shipping.State
		}
		if order.Shipping.PostalCode != "" {
			cityLine += " " + order.Shipping.PostalCode
		}
		parts = append(parts, cityLine)
		if order.Shipping.Country != "" {
			parts = append(parts, order.Shipping.Country)
		}
		shippingAddr = strings.Join(parts, "\n")
	}

	// Section labels
	m.AddRow(6,
		col.New(6).Add(
			text.New("BILL TO", props.Text{Size: 8, Style: fontstyle.Bold, Color: pdfLightText}),
		),
		col.New(6).Add(
			text.New("SHIP TO", props.Text{Size: 8, Style: fontstyle.Bold, Color: pdfLightText}),
		),
	)

	// Address content
	m.AddRow(20,
		col.New(6).Add(
			text.New(customerName, props.Text{Size: 10, Style: fontstyle.Bold, Color: pdfDarkText}),
			text.New(customerEmail, props.Text{Size: 8, Color: pdfMediumText, Top: 5}),
		),
		col.New(6).Add(
			text.New(shippingAddr, props.Text{Size: 9, Color: pdfDarkText}),
		),
	)

	// Tax identifiers for B2B
	if order.CustomerGSTIN != "" || order.CustomerVATNumber != "" {
		m.AddRow(8,
			col.New(12).Add(
				text.New(s.buildCustomerTaxID(order), props.Text{Size: 8, Color: pdfMediumText}),
			),
		)
	}

	m.AddRow(4)
}

// addPDFItemsTable adds the items table to the PDF
func (s *receiptService) addPDFItemsTable(m core.Maroto, data *models.ReceiptData) {
	// Table header with dark background
	m.AddRow(9,
		col.New(5).Add(text.New("ITEM", props.Text{Size: 8, Style: fontstyle.Bold, Color: pdfWhite, Top: 2})),
		col.New(2).Add(text.New("SKU", props.Text{Size: 8, Style: fontstyle.Bold, Color: pdfWhite, Align: align.Center, Top: 2})),
		col.New(1).Add(text.New("QTY", props.Text{Size: 8, Style: fontstyle.Bold, Color: pdfWhite, Align: align.Center, Top: 2})),
		col.New(2).Add(text.New("PRICE", props.Text{Size: 8, Style: fontstyle.Bold, Color: pdfWhite, Align: align.Right, Top: 2})),
		col.New(2).Add(text.New("TOTAL", props.Text{Size: 8, Style: fontstyle.Bold, Color: pdfWhite, Align: align.Right, Top: 2})),
	).WithStyle(&props.Cell{BackgroundColor: pdfTotalBg})

	currencySymbol := getCurrencySymbol(data.Order.Currency)

	// Table rows with alternating backgrounds
	for i, item := range data.Order.Items {
		rowHeight := 8.0

		itemName := item.ProductName
		if data.Settings.ShowHSNSACCodes && (item.HSNCode != "" || item.SACCode != "") {
			code := item.HSNCode
			if code == "" {
				code = item.SACCode
			}
			itemName = fmt.Sprintf("%s  (HSN: %s)", item.ProductName, code)
		}

		r := m.AddRow(rowHeight,
			col.New(5).Add(text.New(itemName, props.Text{Size: 9, Color: pdfDarkText, Top: 1})),
			col.New(2).Add(text.New(item.SKU, props.Text{Size: 8, Color: pdfMediumText, Align: align.Center, Top: 1})),
			col.New(1).Add(text.New(fmt.Sprintf("%d", item.Quantity), props.Text{Size: 9, Color: pdfDarkText, Align: align.Center, Top: 1})),
			col.New(2).Add(text.New(formatCurrency(item.UnitPrice, currencySymbol), props.Text{Size: 9, Color: pdfDarkText, Align: align.Right, Top: 1})),
			col.New(2).Add(text.New(formatCurrency(item.TotalPrice, currencySymbol), props.Text{Size: 9, Style: fontstyle.Bold, Color: pdfDarkText, Align: align.Right, Top: 1})),
		)

		// Alternate row background
		if i%2 == 0 {
			r.WithStyle(&props.Cell{BackgroundColor: pdfHeaderBg})
		}
	}

	m.AddRow(2)
}

// addPDFTotals adds the totals section
func (s *receiptService) addPDFTotals(m core.Maroto, data *models.ReceiptData) {
	addTotalLine := func(label, value string, labelColor, valueColor *props.Color, bold bool, size float64) {
		style := fontstyle.Normal
		if bold {
			style = fontstyle.Bold
		}
		m.AddRow(7,
			col.New(7),
			col.New(3).Add(text.New(label, props.Text{Size: size, Color: labelColor, Align: align.Right, Top: 1})),
			col.New(2).Add(text.New(value, props.Text{Size: size, Style: style, Color: valueColor, Align: align.Right, Top: 1})),
		)
	}

	addTotalLine("Subtotal", data.FormattedSubtotal, pdfMediumText, pdfDarkText, false, 9)

	// Tax breakdown
	if data.Settings.ShowTaxBreakdown {
		for _, taxLine := range data.TaxLines {
			addTotalLine(fmt.Sprintf("%s (%.1f%%)", taxLine.Name, taxLine.Rate), taxLine.Amount, pdfMediumText, pdfDarkText, false, 8)
		}
	} else if data.Order.TaxAmount > 0 {
		addTotalLine("Tax", data.FormattedTax, pdfMediumText, pdfDarkText, false, 9)
	}

	if data.Order.ShippingCost > 0 {
		addTotalLine("Shipping", data.FormattedShipping, pdfMediumText, pdfDarkText, false, 9)
	}

	if data.Order.DiscountAmount > 0 {
		addTotalLine("Discount", "-"+data.FormattedDiscount, pdfMediumText, pdfAccent, false, 9)
	}

	// Grand total with dark background
	m.AddRow(10,
		col.New(7),
		col.New(3).Add(text.New("TOTAL", props.Text{Size: 11, Style: fontstyle.Bold, Color: pdfWhite, Align: align.Right, Top: 2})),
		col.New(2).Add(text.New(data.FormattedTotal, props.Text{Size: 11, Style: fontstyle.Bold, Color: pdfWhite, Align: align.Right, Top: 2})),
	).WithStyle(&props.Cell{BackgroundColor: pdfTotalBg})

	m.AddRow(3)
}

// addPDFPaymentInfo adds payment information
func (s *receiptService) addPDFPaymentInfo(m core.Maroto, data *models.ReceiptData) {
	payment := data.Order.Payment
	if payment == nil {
		return
	}

	m.AddRow(6,
		col.New(12).Add(
			text.New("PAYMENT DETAILS", props.Text{Size: 8, Style: fontstyle.Bold, Color: pdfLightText}),
		),
	)

	m.AddRow(12,
		col.New(3).Add(
			text.New("Method", props.Text{Size: 7, Color: pdfLightText, Top: 2}),
			text.New(strings.ToUpper(payment.Method), props.Text{Size: 9, Style: fontstyle.Bold, Color: pdfDarkText, Top: 6}),
		),
		col.New(5).Add(
			text.New("Transaction ID", props.Text{Size: 7, Color: pdfLightText, Top: 2}),
			text.New(maskTransactionID(payment.TransactionID), props.Text{Size: 8, Color: pdfMediumText, Top: 6}),
		),
		col.New(4).Add(
			text.New("Payment Status", props.Text{Size: 7, Color: pdfLightText, Align: align.Right, Top: 2}),
			text.New(string(data.Order.PaymentStatus), props.Text{Size: 9, Style: fontstyle.Bold, Color: pdfDarkText, Align: align.Right, Top: 6}),
		),
	).WithStyle(&props.Cell{BackgroundColor: pdfHeaderBg})

	m.AddRow(4)
}

// addPDFFooter adds the footer section
func (s *receiptService) addPDFFooter(m core.Maroto, data *models.ReceiptData) {
	// Thin line separator
	m.AddRow(1).WithStyle(&props.Cell{BackgroundColor: pdfTableBg})
	m.AddRow(5)

	// Footer text
	if data.Settings.FooterText != "" {
		m.AddRow(8,
			col.New(12).Add(
				text.New(data.Settings.FooterText, props.Text{
					Size:  9,
					Align: align.Center,
					Color: pdfMediumText,
					Style: fontstyle.BoldItalic,
				}),
			),
		)
	}

	// Terms
	if data.Settings.TermsText != "" {
		m.AddRow(3)
		m.AddRow(14,
			col.New(12).Add(
				text.New("Terms & Conditions", props.Text{Size: 7, Style: fontstyle.Bold, Color: pdfLightText}),
				text.New(data.Settings.TermsText, props.Text{Size: 7, Color: pdfLightText, Top: 4}),
			),
		)
	}

	// Business details in footer (full address, tax IDs)
	settings := data.Settings
	hasBusinessDetails := settings.BusinessAddress != "" || settings.GSTIN != "" || settings.VATNumber != "" || settings.TaxID != ""
	if hasBusinessDetails {
		m.AddRow(3)
		m.AddRow(1).WithStyle(&props.Cell{BackgroundColor: pdfTableBg})
		m.AddRow(3)

		footerDetails := []string{}
		if settings.BusinessAddress != "" {
			footerDetails = append(footerDetails, settings.BusinessAddress)
		}
		taxParts := []string{}
		if settings.GSTIN != "" {
			taxParts = append(taxParts, "GSTIN: "+settings.GSTIN)
		}
		if settings.VATNumber != "" {
			taxParts = append(taxParts, "VAT: "+settings.VATNumber)
		}
		if settings.TaxID != "" {
			taxParts = append(taxParts, "Tax ID: "+settings.TaxID)
		}
		if len(taxParts) > 0 {
			footerDetails = append(footerDetails, strings.Join(taxParts, "  |  "))
		}

		m.AddRow(12,
			col.New(12).Add(
				text.New(data.Settings.BusinessName, props.Text{Size: 8, Style: fontstyle.Bold, Color: pdfMediumText, Align: align.Center}),
				text.New(strings.Join(footerDetails, "  |  "), props.Text{Size: 7, Color: pdfLightText, Align: align.Center, Top: 4}),
			),
		)
	}

	// Generated timestamp
	m.AddRow(5)
	m.AddRow(6,
		col.New(12).Add(
			text.New(fmt.Sprintf("Generated on %s", data.GeneratedAt.Format("January 02, 2006 at 3:04 PM MST")), props.Text{
				Size:  7,
				Align: align.Center,
				Color: pdfLightText,
			}),
		),
	)
}

// buildCustomerTaxID builds the customer tax ID string
func (s *receiptService) buildCustomerTaxID(order *models.Order) string {
	var parts []string
	if order.CustomerGSTIN != "" {
		parts = append(parts, fmt.Sprintf("Customer GSTIN: %s", order.CustomerGSTIN))
	}
	if order.CustomerVATNumber != "" {
		parts = append(parts, fmt.Sprintf("Customer VAT: %s", order.CustomerVATNumber))
	}
	return strings.Join(parts, " | ")
}

// generateHTML generates an HTML receipt
func (s *receiptService) generateHTML(data *models.ReceiptData) ([]byte, error) {
	funcMap := template.FuncMap{
		"maskTxnID": maskTransactionID,
	}
	tmpl, err := template.New("receipt").Funcs(funcMap).Parse(receiptHTMLTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute HTML template: %w", err)
	}

	return buf.Bytes(), nil
}

// GetReceiptSettings gets receipt settings for a tenant
func (s *receiptService) GetReceiptSettings(tenantID string) (*models.ReceiptSettings, error) {
	return s.settingsRepo.GetByTenantID(tenantID)
}

// GetOrCreateSettings gets or creates default settings for a tenant
func (s *receiptService) GetOrCreateSettings(tenantID string) (*models.ReceiptSettings, error) {
	settings, err := s.settingsRepo.GetByTenantID(tenantID)
	if err != nil {
		return nil, err
	}

	if settings != nil {
		// If business name is still the default, try to fetch the actual store name
		if settings.BusinessName == "Your Store" && s.tenantClient != nil {
			if name := s.tenantClient.GetTenantName(context.Background(), tenantID); name != "" {
				settings.BusinessName = name
				_ = s.settingsRepo.Update(settings)
			}
		}
		return settings, nil
	}

	// Fetch actual store name from tenant service
	businessName := "Your Store"
	if s.tenantClient != nil {
		if name := s.tenantClient.GetTenantName(context.Background(), tenantID); name != "" {
			businessName = name
		}
	}

	// Create default settings
	settings = &models.ReceiptSettings{
		ID:                  uuid.New(),
		TenantID:            tenantID,
		DefaultTemplate:     models.ReceiptTemplateDefault,
		PrimaryColor:        "#1a73e8",
		SecondaryColor:      "#5f6368",
		BusinessName:        businessName,
		ShowTaxBreakdown:    true,
		ShowHSNSACCodes:     true,
		ShowPaymentDetails:  true,
		ShowShippingDetails: true,
		IncludeQRCode:       true,
		FooterText:          "Thank you for your purchase!",
	}

	if err := s.settingsRepo.Create(settings); err != nil {
		return nil, err
	}

	return settings, nil
}

// UpdateReceiptSettings updates receipt settings for a tenant
func (s *receiptService) UpdateReceiptSettings(tenantID string, req *models.ReceiptSettingsUpdateRequest) (*models.ReceiptSettings, error) {
	settings, err := s.GetOrCreateSettings(tenantID)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if req.DefaultTemplate != nil {
		settings.DefaultTemplate = *req.DefaultTemplate
	}
	if req.LogoURL != nil {
		settings.LogoURL = *req.LogoURL
	}
	if req.PrimaryColor != nil {
		settings.PrimaryColor = *req.PrimaryColor
	}
	if req.SecondaryColor != nil {
		settings.SecondaryColor = *req.SecondaryColor
	}
	if req.BusinessName != nil {
		settings.BusinessName = *req.BusinessName
	}
	if req.BusinessAddress != nil {
		settings.BusinessAddress = *req.BusinessAddress
	}
	if req.BusinessPhone != nil {
		settings.BusinessPhone = *req.BusinessPhone
	}
	if req.BusinessEmail != nil {
		settings.BusinessEmail = *req.BusinessEmail
	}
	if req.BusinessWebsite != nil {
		settings.BusinessWebsite = *req.BusinessWebsite
	}
	if req.GSTIN != nil {
		settings.GSTIN = *req.GSTIN
	}
	if req.VATNumber != nil {
		settings.VATNumber = *req.VATNumber
	}
	if req.TaxID != nil {
		settings.TaxID = *req.TaxID
	}
	if req.HeaderText != nil {
		settings.HeaderText = *req.HeaderText
	}
	if req.FooterText != nil {
		settings.FooterText = *req.FooterText
	}
	if req.TermsText != nil {
		settings.TermsText = *req.TermsText
	}
	if req.ShowTaxBreakdown != nil {
		settings.ShowTaxBreakdown = *req.ShowTaxBreakdown
	}
	if req.ShowHSNSACCodes != nil {
		settings.ShowHSNSACCodes = *req.ShowHSNSACCodes
	}
	if req.ShowPaymentDetails != nil {
		settings.ShowPaymentDetails = *req.ShowPaymentDetails
	}
	if req.ShowShippingDetails != nil {
		settings.ShowShippingDetails = *req.ShowShippingDetails
	}
	if req.IncludeQRCode != nil {
		settings.IncludeQRCode = *req.IncludeQRCode
	}
	if req.ShowItemImages != nil {
		settings.ShowItemImages = *req.ShowItemImages
	}

	if err := s.settingsRepo.Update(settings); err != nil {
		return nil, err
	}

	return settings, nil
}

// Helper functions

func getCurrencySymbol(currency string) string {
	symbols := map[string]string{
		"USD": "$",
		"EUR": "\u20ac",
		"GBP": "\u00a3",
		"INR": "\u20b9",
		"JPY": "\u00a5",
		"AUD": "A$",
		"CAD": "C$",
	}
	if symbol, ok := symbols[strings.ToUpper(currency)]; ok {
		return symbol
	}
	return currency + " "
}

func formatCurrency(amount float64, symbol string) string {
	return fmt.Sprintf("%s%.2f", symbol, amount)
}

// maskTransactionID masks a transaction ID, showing only the last 4 characters
// e.g., "txn_1234567890" → "••••••••••7890"
func maskTransactionID(txnID string) string {
	if len(txnID) <= 4 {
		return txnID
	}
	masked := strings.Repeat("•", len(txnID)-4) + txnID[len(txnID)-4:]
	return masked
}

func calculateTaxRate(taxAmount, subtotal float64) float64 {
	if subtotal == 0 {
		return 0
	}
	return (taxAmount / subtotal) * 100
}

// HTML template for receipt
const receiptHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Receipt - {{.ReceiptNumber}}</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            line-height: 1.5;
            color: #333;
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
        }
        .receipt {
            border: 1px solid #ddd;
            border-radius: 8px;
            padding: 30px;
            background: #fff;
        }
        .header {
            display: flex;
            justify-content: space-between;
            align-items: flex-start;
            margin-bottom: 30px;
            padding-bottom: 20px;
            border-bottom: 2px solid {{.Settings.PrimaryColor}};
        }
        .business-info h1 {
            color: {{.Settings.PrimaryColor}};
            font-size: 24px;
            margin-bottom: 5px;
        }
        .business-info p {
            color: #666;
            font-size: 14px;
        }
        .receipt-title {
            text-align: right;
        }
        .receipt-title h2 {
            font-size: 28px;
            color: {{.Settings.PrimaryColor}};
        }
        .receipt-title p {
            color: #666;
            font-size: 14px;
        }
        .details {
            display: flex;
            justify-content: space-between;
            margin-bottom: 30px;
        }
        .details-section h3 {
            font-size: 12px;
            text-transform: uppercase;
            color: #666;
            margin-bottom: 10px;
        }
        .details-section p {
            font-size: 14px;
            margin-bottom: 3px;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            margin-bottom: 30px;
        }
        th {
            background: #f5f5f5;
            padding: 12px;
            text-align: left;
            font-size: 12px;
            text-transform: uppercase;
            color: #666;
        }
        td {
            padding: 12px;
            border-bottom: 1px solid #eee;
            font-size: 14px;
        }
        .text-right { text-align: right; }
        .text-center { text-align: center; }
        .totals {
            margin-left: auto;
            width: 300px;
        }
        .totals-row {
            display: flex;
            justify-content: space-between;
            padding: 8px 0;
            font-size: 14px;
        }
        .totals-row.total {
            border-top: 2px solid #333;
            font-weight: bold;
            font-size: 18px;
            padding-top: 12px;
        }
        .payment-info {
            background: #f9f9f9;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 20px;
        }
        .payment-info h3 {
            font-size: 14px;
            margin-bottom: 10px;
            color: #666;
        }
        .footer {
            text-align: center;
            color: #666;
            font-size: 14px;
            padding-top: 20px;
            border-top: 1px solid #eee;
        }
        .generated {
            font-size: 12px;
            color: #999;
            margin-top: 10px;
        }
        .hsn-code {
            font-size: 11px;
            color: #666;
        }
    </style>
</head>
<body>
    <div class="receipt">
        <div class="header">
            <div class="business-info">
                <h1>{{.Settings.BusinessName}}</h1>
                {{if .Settings.BusinessAddress}}<p>{{.Settings.BusinessAddress}}</p>{{end}}
                {{if .Settings.BusinessPhone}}<p>{{.Settings.BusinessPhone}}</p>{{end}}
                {{if .Settings.BusinessEmail}}<p>{{.Settings.BusinessEmail}}</p>{{end}}
                {{if .Settings.GSTIN}}<p>GSTIN: {{.Settings.GSTIN}}</p>{{end}}
                {{if .Settings.VATNumber}}<p>VAT: {{.Settings.VATNumber}}</p>{{end}}
            </div>
            <div class="receipt-title">
                <h2>RECEIPT</h2>
                <p># {{.ReceiptNumber}}</p>
                <p>Order: {{.Order.OrderNumber}}</p>
                <p>{{.Order.CreatedAt.Format "Jan 02, 2006"}}</p>
            </div>
        </div>

        <div class="details">
            <div class="details-section">
                <h3>Bill To</h3>
                {{if .Order.Customer}}
                <p><strong>{{.Order.Customer.FirstName}} {{.Order.Customer.LastName}}</strong></p>
                <p>{{.Order.Customer.Email}}</p>
                {{if .Order.Customer.Phone}}<p>{{.Order.Customer.Phone}}</p>{{end}}
                {{end}}
                {{if .Order.CustomerGSTIN}}<p>GSTIN: {{.Order.CustomerGSTIN}}</p>{{end}}
                {{if .Order.CustomerVATNumber}}<p>VAT: {{.Order.CustomerVATNumber}}</p>{{end}}
            </div>
            {{if .Settings.ShowShippingDetails}}
            {{if .Order.Shipping}}
            <div class="details-section">
                <h3>Ship To</h3>
                <p>{{.Order.Shipping.Street}}</p>
                <p>{{.Order.Shipping.City}}, {{.Order.Shipping.State}} {{.Order.Shipping.PostalCode}}</p>
                <p>{{.Order.Shipping.Country}}</p>
            </div>
            {{end}}
            {{end}}
            <div class="details-section">
                <h3>Order Status</h3>
                <p>Status: {{.Order.Status}}</p>
                <p>Payment: {{.Order.PaymentStatus}}</p>
                <p>Fulfillment: {{.Order.FulfillmentStatus}}</p>
            </div>
        </div>

        <table>
            <thead>
                <tr>
                    <th>Item</th>
                    <th>SKU</th>
                    <th class="text-center">Qty</th>
                    <th class="text-right">Price</th>
                    <th class="text-right">Total</th>
                </tr>
            </thead>
            <tbody>
                {{range .Order.Items}}
                <tr>
                    <td>
                        {{.ProductName}}
                        {{if $.Settings.ShowHSNSACCodes}}
                        {{if .HSNCode}}<br><span class="hsn-code">HSN: {{.HSNCode}}</span>{{end}}
                        {{if .SACCode}}<br><span class="hsn-code">SAC: {{.SACCode}}</span>{{end}}
                        {{end}}
                    </td>
                    <td>{{.SKU}}</td>
                    <td class="text-center">{{.Quantity}}</td>
                    <td class="text-right">{{printf "%.2f" .UnitPrice}}</td>
                    <td class="text-right">{{printf "%.2f" .TotalPrice}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>

        <div class="totals">
            <div class="totals-row">
                <span>Subtotal</span>
                <span>{{.FormattedSubtotal}}</span>
            </div>
            {{if .Settings.ShowTaxBreakdown}}
            {{range .TaxLines}}
            <div class="totals-row">
                <span>{{.Name}} ({{printf "%.1f" .Rate}}%)</span>
                <span>{{.Amount}}</span>
            </div>
            {{end}}
            {{else}}
            {{if gt .Order.TaxAmount 0}}
            <div class="totals-row">
                <span>Tax</span>
                <span>{{.FormattedTax}}</span>
            </div>
            {{end}}
            {{end}}
            {{if gt .Order.ShippingCost 0}}
            <div class="totals-row">
                <span>Shipping</span>
                <span>{{.FormattedShipping}}</span>
            </div>
            {{end}}
            {{if gt .Order.DiscountAmount 0}}
            <div class="totals-row">
                <span>Discount</span>
                <span>-{{.FormattedDiscount}}</span>
            </div>
            {{end}}
            <div class="totals-row total">
                <span>Total</span>
                <span>{{.FormattedTotal}}</span>
            </div>
        </div>

        {{if .Settings.ShowPaymentDetails}}
        {{if .Order.Payment}}
        <div class="payment-info">
            <h3>Payment Information</h3>
            <p>Method: {{.Order.Payment.Method}}</p>
            {{if .Order.Payment.TransactionID}}<p>Transaction ID: {{maskTxnID .Order.Payment.TransactionID}}</p>{{end}}
            <p>Status: {{.Order.Payment.Status}}</p>
        </div>
        {{end}}
        {{end}}

        <div class="footer">
            {{if .Settings.FooterText}}<p>{{.Settings.FooterText}}</p>{{end}}
            {{if .Settings.TermsText}}
            <p style="font-size: 11px; margin-top: 15px; color: #999;">{{.Settings.TermsText}}</p>
            {{end}}
            <p class="generated">Generated on {{.GeneratedAt.Format "Jan 02, 2006 15:04 MST"}}</p>
        </div>
    </div>
</body>
</html>`
