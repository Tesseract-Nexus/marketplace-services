package carriers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"shipping-service/internal/models"
)

// DelhiveryCarrier implements the Carrier interface for Delhivery
type DelhiveryCarrier struct {
	config     CarrierConfig
	httpClient *http.Client
}

// NewDelhiveryCarrierImpl creates a new Delhivery carrier instance (real implementation)
func NewDelhiveryCarrierImpl(config CarrierConfig) *DelhiveryCarrier {
	return &DelhiveryCarrier{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetName returns the carrier name
func (d *DelhiveryCarrier) GetName() models.CarrierType {
	return models.CarrierDelhivery
}

// TestConnection tests the carrier credentials by making a test API call
func (d *DelhiveryCarrier) TestConnection() error {
	// Test connection by checking serviceability for a known pincode pair
	endpoint := fmt.Sprintf("%s/c/api/pin-codes/json/", d.config.BaseURL)

	query := url.Values{}
	query.Set("filter_codes", "560001") // Bangalore pincode

	req, err := http.NewRequest("GET", fmt.Sprintf("%s?%s", endpoint, query.Encode()), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Token %s", d.config.APIKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid API token: authentication failed")
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// GetRates retrieves shipping rates from Delhivery
func (d *DelhiveryCarrier) GetRates(request models.GetRatesRequest) ([]models.ShippingRate, error) {
	// Delhivery rate calculation endpoint
	endpoint := fmt.Sprintf("%s/api/kinko/v1/invoice/charges/.json", d.config.BaseURL)

	// Calculate weight in grams (Delhivery uses grams)
	weightInGrams := request.Weight * 1000

	// Calculate volumetric weight: (L * B * H) / 5000, convert to grams
	volumetricWeight := (request.Length * request.Width * request.Height) / 5000 * 1000
	chargeableWeight := weightInGrams
	if volumetricWeight > chargeableWeight {
		chargeableWeight = volumetricWeight
	}

	log.Printf("Delhivery GetRates: Actual=%.2fg, Volumetric=%.2fg (%.0fx%.0fx%.0fcm), Chargeable=%.2fg",
		weightInGrams, volumetricWeight, request.Length, request.Width, request.Height, chargeableWeight)

	query := url.Values{}
	query.Set("md", "E")                                      // Mode: E=Express, S=Surface
	query.Set("cgm", fmt.Sprintf("%.0f", chargeableWeight))  // Charged weight in grams
	query.Set("o_pin", request.FromAddress.PostalCode)        // Origin pincode
	query.Set("d_pin", request.ToAddress.PostalCode)          // Destination pincode
	query.Set("ss", "DTO")                                    // Shipment status: DTO=Delivered to Origin

	req, err := http.NewRequest("GET", fmt.Sprintf("%s?%s", endpoint, query.Encode()), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Token %s", d.config.APIKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read and log raw response for debugging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	log.Printf("Delhivery raw API response: %s", string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Delhivery rate response format
	var ratesResp []struct {
		Status            string  `json:"status"`
		TotalAmount       float64 `json:"total_amount"`
		ChargedWeight     float64 `json:"charged_weight"`
		Zone              string  `json:"zone"`
		OriginCity        string  `json:"origin_city"`
		DestinationCity   string  `json:"destination_city"`
		FreightCharge     float64 `json:"charge_freight"`
		CodAmount         float64 `json:"charge_cod"`
		FuelSurcharge     float64 `json:"charge_fs"`
		HandlingCharge    float64 `json:"charge_handling"`
		OtherCharge       float64 `json:"charge_other"`
		GSTPercent        float64 `json:"gst_percent"`
	}

	if err := json.Unmarshal(bodyBytes, &ratesResp); err != nil {
		// Try alternate single object response format
		var singleResp struct {
			Status            string  `json:"status"`
			TotalAmount       float64 `json:"total_amount"`
			ChargedWeight     float64 `json:"charged_weight"`
			Zone              string  `json:"zone"`
			OriginCity        string  `json:"origin_city"`
			DestinationCity   string  `json:"destination_city"`
			FreightCharge     float64 `json:"charge_freight"`
			CodAmount         float64 `json:"charge_cod"`
			FuelSurcharge     float64 `json:"charge_fs"`
			HandlingCharge    float64 `json:"charge_handling"`
			OtherCharge       float64 `json:"charge_other"`
			GSTPercent        float64 `json:"gst_percent"`
		}
		if err := json.Unmarshal(bodyBytes, &singleResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		ratesResp = append(ratesResp, struct {
			Status            string  `json:"status"`
			TotalAmount       float64 `json:"total_amount"`
			ChargedWeight     float64 `json:"charged_weight"`
			Zone              string  `json:"zone"`
			OriginCity        string  `json:"origin_city"`
			DestinationCity   string  `json:"destination_city"`
			FreightCharge     float64 `json:"charge_freight"`
			CodAmount         float64 `json:"charge_cod"`
			FuelSurcharge     float64 `json:"charge_fs"`
			HandlingCharge    float64 `json:"charge_handling"`
			OtherCharge       float64 `json:"charge_other"`
			GSTPercent        float64 `json:"gst_percent"`
		}(singleResp))
	}

	rates := make([]models.ShippingRate, 0)

	for _, rate := range ratesResp {
		if rate.Status != "success" && rate.Status != "ok" && rate.Status != "" {
			log.Printf("Delhivery: Skipping rate with status=%s", rate.Status)
			continue
		}

		// Calculate total rate
		totalRate := rate.TotalAmount
		if totalRate == 0 {
			// Calculate from components if total not provided
			baseRate := rate.FreightCharge + rate.FuelSurcharge + rate.HandlingCharge + rate.OtherCharge
			gstAmount := baseRate * (rate.GSTPercent / 100)
			totalRate = baseRate + gstAmount
		}

		log.Printf("Delhivery Rate: Zone=%s, Base=%.2f, Fuel=%.2f, Total=%.2f, ChargeableWt=%.2fg",
			rate.Zone, rate.FreightCharge, rate.FuelSurcharge, totalRate, rate.ChargedWeight)

		// Estimate delivery days based on zone
		estimatedDays := getEstimatedDaysForZone(rate.Zone)
		estimatedDelivery := time.Now().AddDate(0, 0, estimatedDays)

		serviceName := "Delhivery Express"
		if rate.Zone != "" {
			serviceName = fmt.Sprintf("Delhivery Express (%s)", rate.Zone)
		}

		rates = append(rates, models.ShippingRate{
			Carrier:           models.CarrierDelhivery,
			ServiceName:       serviceName,
			ServiceCode:       "EXPRESS",
			Rate:              totalRate,
			Currency:          "INR",
			EstimatedDays:     estimatedDays,
			EstimatedDelivery: &estimatedDelivery,
			Available:         true,
		})
	}

	// Also get Surface rates if requested weight is suitable (>500g)
	if chargeableWeight >= 500 {
		surfaceRates, err := d.getSurfaceRates(request.FromAddress.PostalCode, request.ToAddress.PostalCode, chargeableWeight)
		if err != nil {
			log.Printf("Delhivery: Failed to get surface rates: %v", err)
		} else {
			rates = append(rates, surfaceRates...)
		}
	}

	return rates, nil
}

// getSurfaceRates gets surface (economy) shipping rates
func (d *DelhiveryCarrier) getSurfaceRates(originPin, destPin string, weightInGrams float64) ([]models.ShippingRate, error) {
	endpoint := fmt.Sprintf("%s/api/kinko/v1/invoice/charges/.json", d.config.BaseURL)

	query := url.Values{}
	query.Set("md", "S")                                      // Mode: S=Surface
	query.Set("cgm", fmt.Sprintf("%.0f", weightInGrams))
	query.Set("o_pin", originPin)
	query.Set("d_pin", destPin)
	query.Set("ss", "DTO")

	req, err := http.NewRequest("GET", fmt.Sprintf("%s?%s", endpoint, query.Encode()), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Token %s", d.config.APIKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("surface rates API returned status %d", resp.StatusCode)
	}

	var ratesResp []struct {
		TotalAmount   float64 `json:"total_amount"`
		ChargedWeight float64 `json:"charged_weight"`
		Zone          string  `json:"zone"`
	}

	if err := json.Unmarshal(bodyBytes, &ratesResp); err != nil {
		var singleResp struct {
			TotalAmount   float64 `json:"total_amount"`
			ChargedWeight float64 `json:"charged_weight"`
			Zone          string  `json:"zone"`
		}
		if err := json.Unmarshal(bodyBytes, &singleResp); err != nil {
			return nil, err
		}
		ratesResp = append(ratesResp, singleResp)
	}

	rates := make([]models.ShippingRate, 0)
	for _, rate := range ratesResp {
		// Surface is typically 5-7 days
		estimatedDays := 5 + getEstimatedDaysForZone(rate.Zone)
		estimatedDelivery := time.Now().AddDate(0, 0, estimatedDays)

		rates = append(rates, models.ShippingRate{
			Carrier:           models.CarrierDelhivery,
			ServiceName:       fmt.Sprintf("Delhivery Surface (%s)", rate.Zone),
			ServiceCode:       "SURFACE",
			Rate:              rate.TotalAmount,
			Currency:          "INR",
			EstimatedDays:     estimatedDays,
			EstimatedDelivery: &estimatedDelivery,
			Available:         true,
		})
	}

	return rates, nil
}

// getEstimatedDaysForZone estimates delivery days based on zone
func getEstimatedDaysForZone(zone string) int {
	switch strings.ToUpper(zone) {
	case "A", "LOCAL":
		return 2
	case "B", "WITHIN_STATE":
		return 3
	case "C", "METRO":
		return 3
	case "D", "ROI":
		return 4
	case "E", "SPECIAL":
		return 5
	default:
		return 4
	}
}

// CreateShipment creates a shipment with Delhivery
func (d *DelhiveryCarrier) CreateShipment(request models.CreateShipmentRequest) (*models.Shipment, error) {
	// Derive pickup location name from warehouse/from address
	// Use a consistent naming convention for the tenant's warehouse
	pickupLocationName := d.config.APISecret // Can be configured as pickup location name
	if pickupLocationName == "" {
		// Generate a name from the from address
		pickupLocationName = sanitizePickupLocationName(request.FromAddress.Name)
		if pickupLocationName == "" {
			pickupLocationName = "TesserixWarehouse"
		}
	}

	// Ensure the pickup location exists in Delhivery before creating shipment
	if err := d.EnsurePickupLocation(pickupLocationName, request.FromAddress); err != nil {
		log.Printf("Delhivery: Warning - Failed to ensure pickup location: %v", err)
		// Continue anyway - the location might already exist
	}

	endpoint := fmt.Sprintf("%s/api/cmu/create.json", d.config.BaseURL)

	// Build shipment data for Delhivery
	// Delhivery requires a specific format for the shipment data
	shipmentData := buildDelhiveryShipmentData(request)

	// Delhivery uses form-urlencoded data
	formData := url.Values{}
	formData.Set("format", "json")
	formData.Set("data", shipmentData)

	// Use the pickup location name we ensured exists
	formData.Set("pickup_location", pickupLocationName)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Token %s", d.config.APIKey))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("Delhivery CreateShipment response: %s", string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var createResp struct {
		Success       bool   `json:"success"`
		RMK           string `json:"rmk"`
		CashToCollect int    `json:"cash_to_collect"`
		Packages      []struct {
			Waybill       string          `json:"waybill"`
			Refnum        string          `json:"refnum"`
			Status        string          `json:"status"`
			Remarks       json.RawMessage `json:"remarks"` // Can be string or array
			CashToCollect int             `json:"cash_to_collect"`
		} `json:"packages"`
		UploadWbn string `json:"upload_wbn"`
	}

	if err := json.Unmarshal(bodyBytes, &createResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Helper to extract remarks (handles both string and array)
	getRemarksString := func(raw json.RawMessage) string {
		if len(raw) == 0 {
			return ""
		}
		// Try as string first
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
		// Try as array of strings
		var arr []string
		if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
			return strings.Join(arr, "; ")
		}
		return string(raw)
	}

	if !createResp.Success || len(createResp.Packages) == 0 {
		errMsg := createResp.RMK
		if len(createResp.Packages) > 0 {
			remarks := getRemarksString(createResp.Packages[0].Remarks)
			if remarks != "" {
				errMsg = remarks
			}
		}
		return nil, fmt.Errorf("shipment creation failed: %s", errMsg)
	}

	pkg := createResp.Packages[0]
	waybill := pkg.Waybill

	log.Printf("Delhivery: Order created - Waybill=%s, RefNum=%s, Status=%s", waybill, pkg.Refnum, pkg.Status)

	// Generate label URL for the shipment
	labelURL := d.generateLabelURL(waybill)
	log.Printf("Delhivery: Label URL generated - %s", labelURL)

	// Use pre-calculated shipping cost from checkout if provided
	shippingCost := request.ShippingCost
	if shippingCost == 0 {
		// Get estimated cost from rates
		rateReq := models.GetRatesRequest{
			FromAddress: request.FromAddress,
			ToAddress:   request.ToAddress,
			Weight:      request.Weight,
			Length:      request.Length,
			Width:       request.Width,
			Height:      request.Height,
		}
		if rates, err := d.GetRates(rateReq); err == nil && len(rates) > 0 {
			shippingCost = rates[0].Rate
		}
	}

	shipment := &models.Shipment{
		OrderID:           request.OrderID,
		OrderNumber:       request.OrderNumber,
		Carrier:           models.CarrierDelhivery,
		CarrierShipmentID: waybill,
		TrackingNumber:    waybill,
		TrackingURL:       fmt.Sprintf("https://www.delhivery.com/track/package/%s", waybill),
		LabelURL:          labelURL,
		Status:            models.ShipmentStatusCreated,
		FromAddress:       request.FromAddress,
		ToAddress:         request.ToAddress,
		Weight:            request.Weight,
		Length:            request.Length,
		Width:             request.Width,
		Height:            request.Height,
		ShippingCost:      shippingCost,
		Currency:          "INR",
		Metadata:          fmt.Sprintf(`{"upload_wbn":"%s","ref_num":"%s"}`, createResp.UploadWbn, pkg.Refnum),
	}

	return shipment, nil
}

// buildDelhiveryShipmentData builds the JSON data for Delhivery shipment creation
func buildDelhiveryShipmentData(request models.CreateShipmentRequest) string {
	// Build order items
	var subTotal float64
	orderItems := ""
	if len(request.Items) > 0 {
		items := make([]string, 0)
		for _, item := range request.Items {
			items = append(items, fmt.Sprintf("%s (%d)", item.Name, item.Quantity))
			subTotal += item.Price * float64(item.Quantity)
		}
		orderItems = strings.Join(items, ", ")
	} else {
		subTotal = request.OrderValue
		if subTotal <= 0 {
			subTotal = 100
		}
		orderItems = fmt.Sprintf("Order %s", request.OrderNumber)
	}

	// Clean phone number - phone is required for delivery
	phone := CleanPhoneNumber(request.ToAddress.Phone)
	if phone == "" {
		// Log warning but use a placeholder - the order should have valid phone
		log.Printf("Delhivery: Warning - Invalid or missing phone number for order %s", request.OrderNumber)
		phone = "0000000000" // Placeholder - Delhivery will reject if truly invalid
	}

	shipmentData := map[string]interface{}{
		"shipments": []map[string]interface{}{
			{
				"name":                request.ToAddress.Name,
				"add":                 request.ToAddress.Street,
				"pin":                 request.ToAddress.PostalCode,
				"city":                request.ToAddress.City,
				"state":               request.ToAddress.State,
				"country":             request.ToAddress.Country,
				"phone":               phone,
				"order":               request.OrderNumber,
				"payment_mode":        "Prepaid",
				"return_pin":          request.FromAddress.PostalCode,
				"return_city":         request.FromAddress.City,
				"return_phone":        CleanPhoneNumber(request.FromAddress.Phone),
				"return_add":          request.FromAddress.Street,
				"return_state":        request.FromAddress.State,
				"return_country":      request.FromAddress.Country,
				"return_name":         request.FromAddress.Name,
				"products_desc":       orderItems,
				"hsn_code":            "",
				"cod_amount":          0,
				"order_date":          time.Now().Format("2006-01-02 15:04:05"),
				"total_amount":        subTotal,
				"seller_add":          request.FromAddress.Street,
				"seller_name":         request.FromAddress.Name,
				"seller_inv":          request.OrderNumber,
				"quantity":            1,
				"waybill":             "", // Delhivery will generate
				"shipment_width":      request.Width,
				"shipment_height":     request.Height,
				"weight":              request.Weight * 1000, // Convert to grams
				"shipment_length":     request.Length,
				"fragile_shipment":    false,
				"dangerous_goods":     false,
				"document_shipment":   false,
				"extra_parameters":    map[string]interface{}{},
			},
		},
	}

	jsonBytes, _ := json.Marshal(shipmentData)
	return string(jsonBytes)
}

// GetTracking retrieves tracking information from Delhivery
func (d *DelhiveryCarrier) GetTracking(trackingNumber string) (*models.TrackShipmentResponse, error) {
	endpoint := fmt.Sprintf("%s/api/v1/packages/json/", d.config.BaseURL)

	query := url.Values{}
	query.Set("waybill", trackingNumber)

	req, err := http.NewRequest("GET", fmt.Sprintf("%s?%s", endpoint, query.Encode()), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Token %s", d.config.APIKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var trackResp struct {
		ShipmentData []struct {
			Shipment struct {
				Status struct {
					Status         string `json:"Status"`
					StatusLocation string `json:"StatusLocation"`
					StatusDateTime string `json:"StatusDateTime"`
					StatusType     string `json:"StatusType"`
					Instructions   string `json:"Instructions"`
				} `json:"Status"`
				ReferenceNo       string `json:"ReferenceNo"`
				DeliveryDate      string `json:"DeliveryDate"`
				DestRecieveDate   string `json:"DestRecieveDate"`
				PickupDate        string `json:"PickUpDate"`
				Scans             []struct {
					ScanDetail struct {
						Scan           string `json:"Scan"`
						ScanDateTime   string `json:"ScanDateTime"`
						ScannedLocation string `json:"ScannedLocation"`
						Instructions   string `json:"Instructions"`
						StatusDateTime string `json:"StatusDateTime"`
						ScanType       string `json:"ScanType"`
					} `json:"ScanDetail"`
				} `json:"Scans"`
				ExpectedDeliveryDate string `json:"ExpectedDeliveryDate"`
			} `json:"Shipment"`
		} `json:"ShipmentData"`
	}

	if err := json.Unmarshal(bodyBytes, &trackResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(trackResp.ShipmentData) == 0 {
		return nil, fmt.Errorf("no tracking data found for waybill: %s", trackingNumber)
	}

	shipmentData := trackResp.ShipmentData[0].Shipment
	status := mapDelhiveryStatus(shipmentData.Status.StatusType)

	// Parse expected delivery date
	var estimatedDelivery *time.Time
	if shipmentData.ExpectedDeliveryDate != "" {
		if parsed, err := time.Parse("2006-01-02T15:04:05", shipmentData.ExpectedDeliveryDate); err == nil {
			estimatedDelivery = &parsed
		}
	}

	// Parse actual delivery date
	var actualDelivery *time.Time
	if shipmentData.DeliveryDate != "" && status == models.ShipmentStatusDelivered {
		if parsed, err := time.Parse("2006-01-02T15:04:05", shipmentData.DeliveryDate); err == nil {
			actualDelivery = &parsed
		}
	}

	// Parse tracking events
	var events []models.ShipmentTracking
	for _, scan := range shipmentData.Scans {
		detail := scan.ScanDetail
		timestamp, _ := time.Parse("2006-01-02T15:04:05", detail.ScanDateTime)

		events = append(events, models.ShipmentTracking{
			Status:      detail.Scan,
			Location:    detail.ScannedLocation,
			Description: detail.Instructions,
			Timestamp:   timestamp,
		})
	}

	return &models.TrackShipmentResponse{
		TrackingNumber:    trackingNumber,
		Carrier:           models.CarrierDelhivery,
		Status:            status,
		EstimatedDelivery: estimatedDelivery,
		ActualDelivery:    actualDelivery,
		Events:            events,
	}, nil
}

// mapDelhiveryStatus maps Delhivery status type to internal shipment status
func mapDelhiveryStatus(statusType string) models.ShipmentStatus {
	switch strings.ToUpper(statusType) {
	case "DL", "DELIVERED":
		return models.ShipmentStatusDelivered
	case "IT", "IN_TRANSIT", "IN TRANSIT":
		return models.ShipmentStatusInTransit
	case "OFD", "OUT_FOR_DELIVERY", "OUT FOR DELIVERY":
		return models.ShipmentStatusOutForDelivery
	case "PU", "PICKED_UP", "PICKED UP", "PKD":
		return models.ShipmentStatusPickedUp
	case "PP", "PENDING_PICKUP", "PENDING PICKUP", "PKP":
		return models.ShipmentStatusPending
	case "RT", "RTO", "RETURNED", "RTO_IN_TRANSIT":
		return models.ShipmentStatusReturned
	case "CN", "CANCELLED":
		return models.ShipmentStatusCancelled
	case "UD", "UNDELIVERED", "NOT_DELIVERED":
		return models.ShipmentStatusFailed
	default:
		return models.ShipmentStatusInTransit
	}
}

// CancelShipment cancels a shipment with Delhivery
func (d *DelhiveryCarrier) CancelShipment(shipmentID string) error {
	endpoint := fmt.Sprintf("%s/api/p/edit", d.config.BaseURL)

	// Delhivery cancellation payload
	cancelData := map[string]interface{}{
		"waybill": shipmentID,
		"cancellation": true,
	}

	jsonBytes, err := json.Marshal(cancelData)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	formData := url.Values{}
	formData.Set("format", "json")
	formData.Set("data", string(jsonBytes))

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Token %s", d.config.APIKey))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var cancelResp struct {
		Status  bool   `json:"status"`
		Error   string `json:"error"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(bodyBytes, &cancelResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !cancelResp.Status {
		errMsg := cancelResp.Error
		if cancelResp.Message != "" {
			errMsg = cancelResp.Message
		}
		return fmt.Errorf("cancellation failed: %s", errMsg)
	}

	return nil
}

// IsAvailable checks if Delhivery is available for the route
func (d *DelhiveryCarrier) IsAvailable(fromCountry, toCountry string) bool {
	// Delhivery only serves India domestic shipments
	return fromCountry == "IN" && toCountry == "IN"
}

// SupportsRegion checks if Delhivery supports a region
func (d *DelhiveryCarrier) SupportsRegion(countryCode string) bool {
	return countryCode == "IN"
}

// GenerateReturnLabel generates a return shipping label
func (d *DelhiveryCarrier) GenerateReturnLabel(request models.ReturnLabelRequest) (*models.ReturnLabelResponse, error) {
	// Create a reverse shipment for returns
	createReq := models.CreateShipmentRequest{
		OrderID:     request.OrderID,
		OrderNumber: request.RMANumber,
		FromAddress: request.CustomerAddress, // Customer is the origin for returns
		ToAddress:   request.ReturnAddress,   // Warehouse is the destination
		Weight:      request.Weight,
		Length:      request.Length,
		Width:       request.Width,
		Height:      request.Height,
	}

	shipment, err := d.CreateShipment(createReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create return shipment: %w", err)
	}

	// Set expiry to 30 days from now
	expiresAt := time.Now().AddDate(0, 0, 30)

	return &models.ReturnLabelResponse{
		ReturnID:       request.ReturnID,
		RMANumber:      request.RMANumber,
		Carrier:        models.CarrierDelhivery,
		TrackingNumber: shipment.TrackingNumber,
		LabelURL:       fmt.Sprintf("https://www.delhivery.com/track/package/%s", shipment.TrackingNumber),
		Status:         models.ShipmentStatusCreated,
		ExpiresAt:      &expiresAt,
	}, nil
}

// generateLabelURL generates the packing slip/label URL for a waybill
// Delhivery provides labels via the packing_slip API endpoint
func (d *DelhiveryCarrier) generateLabelURL(waybill string) string {
	// Delhivery label/packing slip endpoint
	// The API returns a PDF when pdf=true parameter is included
	return fmt.Sprintf("%s/api/p/packing_slip?wbns=%s&pdf=true", d.config.BaseURL, waybill)
}

// GetLabel fetches the label for a waybill (returns PDF content)
func (d *DelhiveryCarrier) GetLabel(waybill string) ([]byte, error) {
	endpoint := fmt.Sprintf("%s/api/p/packing_slip", d.config.BaseURL)

	query := url.Values{}
	query.Set("wbns", waybill)
	query.Set("pdf", "true")

	req, err := http.NewRequest("GET", fmt.Sprintf("%s?%s", endpoint, query.Encode()), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Token %s", d.config.APIKey))

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check if response is JSON (contains pdf_download_link) or direct PDF
	if len(bodyBytes) >= 4 && string(bodyBytes[:4]) == "%PDF" {
		// Direct PDF response
		return bodyBytes, nil
	}

	// Parse JSON response to get PDF download link
	var labelResp struct {
		Packages []struct {
			Waybill         string `json:"wbn"`
			PDFDownloadLink string `json:"pdf_download_link"`
		} `json:"packages"`
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(bodyBytes, &labelResp); err != nil {
		log.Printf("Delhivery GetLabel: Failed to parse JSON response: %s", string(bodyBytes[:min(200, len(bodyBytes))]))
		return nil, fmt.Errorf("failed to parse label response: %w", err)
	}

	if labelResp.Error {
		return nil, fmt.Errorf("Delhivery API error: %s", labelResp.Message)
	}

	if len(labelResp.Packages) == 0 {
		return nil, fmt.Errorf("no packages found in Delhivery response")
	}

	pdfURL := labelResp.Packages[0].PDFDownloadLink
	if pdfURL == "" {
		log.Printf("Delhivery GetLabel: No pdf_download_link in response: %s", string(bodyBytes))
		return nil, fmt.Errorf("no PDF download link in Delhivery response")
	}

	log.Printf("Delhivery GetLabel: Fetching PDF from: %s", pdfURL)

	// Fetch the actual PDF from the download link
	pdfReq, err := http.NewRequest("GET", pdfURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create PDF download request: %w", err)
	}

	pdfResp, err := d.httpClient.Do(pdfReq)
	if err != nil {
		return nil, fmt.Errorf("failed to download PDF: %w", err)
	}
	defer pdfResp.Body.Close()

	if pdfResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PDF download failed with status %d", pdfResp.StatusCode)
	}

	// Limit response size to prevent memory issues (10MB max for PDF labels)
	const maxLabelSize = 10 * 1024 * 1024
	limitedReader := io.LimitReader(pdfResp.Body, maxLabelSize+1)
	labelData, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF data: %w", err)
	}

	// Check if we hit the limit (data might be truncated)
	if len(labelData) > maxLabelSize {
		return nil, fmt.Errorf("label data exceeds maximum size of %d bytes", maxLabelSize)
	}

	// Verify it's a valid PDF (starts with %PDF)
	if len(labelData) < 4 || string(labelData[:4]) != "%PDF" {
		log.Printf("Delhivery GetLabel: Downloaded content doesn't appear to be a PDF, first 50 bytes: %s", string(labelData[:min(50, len(labelData))]))
		return nil, fmt.Errorf("invalid PDF from Delhivery download link")
	}

	return labelData, nil
}

// CheckPincodeServiceability checks if Delhivery can service origin/destination pincodes
func (d *DelhiveryCarrier) CheckPincodeServiceability(originPin, destPin string) (bool, error) {
	endpoint := fmt.Sprintf("%s/c/api/pin-codes/json/", d.config.BaseURL)

	query := url.Values{}
	query.Set("filter_codes", fmt.Sprintf("%s,%s", originPin, destPin))

	req, err := http.NewRequest("GET", fmt.Sprintf("%s?%s", endpoint, query.Encode()), nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Token %s", d.config.APIKey))

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("serviceability check failed with status %d", resp.StatusCode)
	}

	var serviceResp struct {
		DeliveryCodesAvailable []struct {
			PostalCode struct {
				Pin     string `json:"pin"`
				Prepaid string `json:"pre_paid"`
				COD     string `json:"cod"`
			} `json:"postal_code"`
		} `json:"delivery_codes"`
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &serviceResp); err != nil {
		return false, err
	}

	// Check if both pincodes are serviceable
	originServiceable := false
	destServiceable := false

	for _, dc := range serviceResp.DeliveryCodesAvailable {
		if dc.PostalCode.Pin == originPin && (dc.PostalCode.Prepaid == "Y" || dc.PostalCode.Prepaid == "y") {
			originServiceable = true
		}
		if dc.PostalCode.Pin == destPin && (dc.PostalCode.Prepaid == "Y" || dc.PostalCode.Prepaid == "y") {
			destServiceable = true
		}
	}

	return originServiceable && destServiceable, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// sanitizePickupLocationName sanitizes a name for use as a Delhivery pickup location
// Delhivery requires alphanumeric names without special characters
func sanitizePickupLocationName(name string) string {
	// Remove special characters, keep only alphanumeric and spaces
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		}
	}
	sanitized := result.String()
	// Limit to 50 characters (Delhivery limit)
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}
	return sanitized
}

// PickupLocationRequest represents a request to create a pickup location
type PickupLocationRequest struct {
	Name          string `json:"name"`
	Phone         string `json:"phone"`
	Address       string `json:"address"`
	City          string `json:"city"`
	State         string `json:"state"`
	Country       string `json:"country"`
	Pin           string `json:"pin"`
	ReturnAddress string `json:"return_address"`
	ReturnCity    string `json:"return_city"`
	ReturnState   string `json:"return_state"`
	ReturnCountry string `json:"return_country"`
	ReturnPin     string `json:"return_pin"`
}

// CreatePickupLocation creates a pickup location/warehouse in Delhivery
// This should be called when the carrier is first configured
func (d *DelhiveryCarrier) CreatePickupLocation(req PickupLocationRequest) error {
	endpoint := fmt.Sprintf("%s/api/backend/clientwarehouse/create/", d.config.BaseURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("Token %s", d.config.APIKey))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Check for success in response (XML format)
	if !strings.Contains(string(bodyBytes), "<success>True</success>") {
		return fmt.Errorf("failed to create pickup location: %s", string(bodyBytes))
	}

	log.Printf("Delhivery: Pickup location '%s' created successfully", req.Name)
	return nil
}

// EnsurePickupLocation creates a pickup location if it doesn't exist
func (d *DelhiveryCarrier) EnsurePickupLocation(name string, address models.Address) error {
	// Try to create the pickup location
	// If it already exists, Delhivery may return an error or update it
	req := PickupLocationRequest{
		Name:          name,
		Phone:         CleanPhoneNumber(address.Phone),
		Address:       address.Street,
		City:          address.City,
		State:         address.State,
		Country:       "India",
		Pin:           address.PostalCode,
		ReturnAddress: address.Street,
		ReturnCity:    address.City,
		ReturnState:   address.State,
		ReturnCountry: "India",
		ReturnPin:     address.PostalCode,
	}

	err := d.CreatePickupLocation(req)
	if err != nil {
		// Log but don't fail - pickup location might already exist
		log.Printf("Delhivery: Note - pickup location creation returned: %v", err)
	}
	return nil
}
