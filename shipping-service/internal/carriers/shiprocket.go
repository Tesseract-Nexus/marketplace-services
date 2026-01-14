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

// ShiprocketCarrier implements the Carrier interface for Shiprocket
type ShiprocketCarrier struct {
	config     CarrierConfig
	httpClient *http.Client
	authToken  string
	tokenExpiry time.Time
}

// NewShiprocketCarrier creates a new Shiprocket carrier instance
func NewShiprocketCarrier(config CarrierConfig) *ShiprocketCarrier {
	return &ShiprocketCarrier{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetName returns the carrier name
func (s *ShiprocketCarrier) GetName() models.CarrierType {
	return models.CarrierShiprocket
}

// TestConnection tests the carrier credentials by authenticating with Shiprocket API
func (s *ShiprocketCarrier) TestConnection() error {
	// Force re-authentication to verify credentials
	s.authToken = ""
	s.tokenExpiry = time.Time{}
	return s.authenticate()
}

// authenticate gets an auth token from Shiprocket
func (s *ShiprocketCarrier) authenticate() error {
	// Check if we have a valid token
	if s.authToken != "" && time.Now().Before(s.tokenExpiry) {
		return nil
	}

	url := fmt.Sprintf("%s/v1/external/auth/login", s.config.BaseURL)
	payload := map[string]string{
		"email":    s.config.APIKey,
		"password": s.config.APISecret,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal auth request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create auth request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send auth request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("auth failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var authResp struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("failed to decode auth response: %w", err)
	}

	s.authToken = authResp.Token
	s.tokenExpiry = time.Now().Add(10 * 24 * time.Hour) // Shiprocket tokens valid for 10 days

	return nil
}

// GetRates retrieves shipping rates from Shiprocket
func (s *ShiprocketCarrier) GetRates(request models.GetRatesRequest) ([]models.ShippingRate, error) {
	if err := s.authenticate(); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	endpoint := fmt.Sprintf("%s/v1/external/courier/serviceability", s.config.BaseURL)

	// Calculate volumetric weight for accurate pricing
	// Shiprocket uses the higher of actual weight vs volumetric weight (L*B*H/5000)
	volumetricWeight := (request.Length * request.Width * request.Height) / 5000
	chargeableWeight := request.Weight
	if volumetricWeight > chargeableWeight {
		chargeableWeight = volumetricWeight
	}
	log.Printf("Shiprocket GetRates: Actual=%.2fkg, Volumetric=%.2fkg (%.0fx%.0fx%.0fcm), Chargeable=%.2fkg",
		request.Weight, volumetricWeight, request.Length, request.Width, request.Height, chargeableWeight)

	query := url.Values{}
	query.Set("pickup_postcode", request.FromAddress.PostalCode)
	query.Set("delivery_postcode", request.ToAddress.PostalCode)
	query.Set("weight", fmt.Sprintf("%.2f", chargeableWeight))
	query.Set("length", fmt.Sprintf("%.2f", request.Length))
	query.Set("breadth", fmt.Sprintf("%.2f", request.Width))
	query.Set("height", fmt.Sprintf("%.2f", request.Height))
	query.Set("cod", "0")
	// Pass declared value for accurate rate calculation (affects insurance/handling charges)
	if request.DeclaredValue > 0 {
		query.Set("declared_value", fmt.Sprintf("%.2f", request.DeclaredValue))
		log.Printf("Shiprocket GetRates: Including declared_value=%.2f", request.DeclaredValue)
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s?%s", endpoint, query.Encode()), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.authToken))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read and log raw response for debugging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	log.Printf("Shiprocket raw API response (first 5000 chars): %s", string(bodyBytes[:min(len(bodyBytes), 5000)]))

	var ratesResp struct {
		Data struct {
			AvailableCouriers []struct {
				CourierName           string      `json:"courier_name"`
				CourierCompanyID      int         `json:"courier_company_id"`
				Rate                  float64     `json:"rate"`
				FreightCharge         float64     `json:"freight_charge"`
				OtherCharges          float64     `json:"other_charges"`
				CodCharges            float64     `json:"cod_charges"`
				RtoCharges            float64     `json:"rto_charges"`
				SupressDate           string      `json:"suppress_date"`
				MinWeight             float64     `json:"min_weight"`
				CallBeforeDelivery    float64     `json:"call_before_delivery_charges"`
				EstimatedDeliveryDays interface{} `json:"estimated_delivery_days"` // Can be string or int
				ChargeableWeight      float64     `json:"chargeable_weight"`
			} `json:"available_courier_companies"`
		} `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &ratesResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	rates := make([]models.ShippingRate, 0)
	for _, courier := range ratesResp.Data.AvailableCouriers {
		// Calculate total rate including all charges
		// Shiprocket API returns base rates WITHOUT GST (18%)
		// We add GST estimate to provide more accurate rates matching dashboard prices
		baseRate := courier.Rate
		if baseRate == 0 {
			baseRate = courier.FreightCharge
		}
		// Add other charges if present (fuel surcharge, handling, etc.)
		baseRate += courier.OtherCharges

		// Add GST (18%) - Shiprocket charges GST on shipping services
		const gstRate = 0.18
		totalRate := baseRate * (1 + gstRate)

		log.Printf("Shiprocket Rate: %s (ID=%d) - Base=%.2f, +GST(18%%)=%.2f, Total=%.2f, ChargeableWt=%.2fkg",
			courier.CourierName, courier.CourierCompanyID, baseRate, baseRate*gstRate, totalRate, courier.ChargeableWeight)

		// Parse estimated delivery days - can be int or string like "3-5"
		estimatedDays := 5 // default
		switch v := courier.EstimatedDeliveryDays.(type) {
		case float64:
			estimatedDays = int(v)
		case int:
			estimatedDays = v
		case string:
			// Parse first number from string like "3-5" or "3"
			if _, err := fmt.Sscanf(v, "%d", &estimatedDays); err != nil {
				estimatedDays = 5 // fallback
			}
		}
		estimatedDelivery := time.Now().AddDate(0, 0, estimatedDays)
		rates = append(rates, models.ShippingRate{
			Carrier:           models.CarrierShiprocket,
			ServiceName:       courier.CourierName,
			ServiceCode:       fmt.Sprintf("%d", courier.CourierCompanyID),
			Rate:              totalRate,
			Currency:          "INR",
			EstimatedDays:     estimatedDays,
			EstimatedDelivery: &estimatedDelivery,
			Available:         true,
		})
	}

	return rates, nil
}

// CreateShipment creates a shipment with Shiprocket
func (s *ShiprocketCarrier) CreateShipment(request models.CreateShipmentRequest) (*models.Shipment, error) {
	if err := s.authenticate(); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Get pickup locations from Shiprocket to find the right one to use
	pickupLocations, err := s.GetPickupLocations()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pickup locations: %w. Please configure a pickup location in your Shiprocket dashboard", err)
	}

	if len(pickupLocations) == 0 {
		return nil, fmt.Errorf("no pickup locations configured in Shiprocket. Please add a pickup location in your Shiprocket dashboard at Settings > Pickup Addresses")
	}

	// Use the first primary location, or the first available location
	pickupLocationCode := ""
	for _, loc := range pickupLocations {
		if loc.IsPrimary() {
			pickupLocationCode = loc.PickupCode
			break
		}
	}
	if pickupLocationCode == "" {
		// No primary location found, use the first one
		pickupLocationCode = pickupLocations[0].PickupCode
	}

	url := fmt.Sprintf("%s/v1/external/orders/create/adhoc", s.config.BaseURL)

	// Split name into first and last name (Shiprocket requires both)
	firstName, lastName := splitName(request.ToAddress.Name)

	// Clean and validate phone number (Shiprocket requires 10-digit Indian format)
	phone := CleanPhoneNumber(request.ToAddress.Phone)
	log.Printf("Shiprocket: Original phone='%s', Cleaned phone='%s'", request.ToAddress.Phone, phone)
	if phone == "" || len(phone) != 10 {
		// Log warning but use placeholder - the order should have valid phone
		log.Printf("Shiprocket: Warning - Invalid phone number for order %s", request.OrderNumber)
		phone = "0000000000" // Placeholder - Shiprocket will validate
	}

	// Build order items from request or use fallback
	var orderItems []map[string]interface{}
	var subTotal float64

	if len(request.Items) > 0 {
		// Use actual order items
		for _, item := range request.Items {
			orderItems = append(orderItems, map[string]interface{}{
				"name":          item.Name,
				"sku":           item.SKU,
				"units":         item.Quantity,
				"selling_price": item.Price,
			})
			subTotal += item.Price * float64(item.Quantity)
		}
	} else {
		// Fallback to order value if no items provided
		subTotal = request.OrderValue
		if subTotal <= 0 {
			subTotal = 100 // Minimum fallback value
		}
		orderItems = []map[string]interface{}{
			{
				"name":          "Order Items",
				"sku":           request.OrderNumber,
				"units":         1,
				"selling_price": subTotal,
			},
		}
	}

	payload := map[string]interface{}{
		"order_id":               request.OrderNumber,
		"order_date":             time.Now().Format("2006-01-02 15:04"),
		"pickup_location":        pickupLocationCode,
		"billing_customer_name":  firstName,
		"billing_last_name":      lastName,
		"billing_address":        request.ToAddress.Street,
		"billing_city":           request.ToAddress.City,
		"billing_pincode":        request.ToAddress.PostalCode,
		"billing_state":          request.ToAddress.State,
		"billing_country":        request.ToAddress.Country,
		"billing_email":          request.ToAddress.Email,
		"billing_phone":          phone,
		"shipping_is_billing":    true,
		"order_items":            orderItems,
		"payment_method":         "Prepaid",
		"sub_total":              subTotal,
		"length":                 request.Length,
		"breadth":                request.Width,
		"height":                 request.Height,
		"weight":                 request.Weight,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.authToken))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var createResp struct {
		OrderID      int    `json:"order_id"`
		ShipmentID   int    `json:"shipment_id"`
		Status       string `json:"status"`
		AWBCode      string `json:"awb_code"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("Shiprocket: Order created - OrderID=%d, ShipmentID=%d, AWB=%s", createResp.OrderID, createResp.ShipmentID, createResp.AWBCode)

	// Generate AWB if not already assigned
	var awbCode string
	var shippingCost float64
	var courierName string

	// Use pre-calculated shipping cost from checkout if provided
	if request.ShippingCost > 0 {
		shippingCost = request.ShippingCost
		log.Printf("Shiprocket: Using pre-calculated shipping cost from checkout: %.2f", shippingCost)
	}

	if createResp.AWBCode != "" {
		awbCode = createResp.AWBCode
	} else if createResp.ShipmentID > 0 {
		// Need to generate AWB separately - pass customer's selected courier if available
		awbResult, err := s.generateAWB(createResp.ShipmentID, request.CourierServiceCode)
		if err != nil {
			log.Printf("Shiprocket: Failed to auto-generate AWB: %v", err)
			// If we don't have a pre-calculated cost, try to get estimated cost
			if shippingCost == 0 {
				rateReq := models.GetRatesRequest{
					FromAddress: request.FromAddress,
					ToAddress:   request.ToAddress,
					Weight:      request.Weight,
					Length:      request.Length,
					Width:       request.Width,
					Height:      request.Height,
				}
				if rates, err := s.GetRates(rateReq); err == nil && len(rates) > 0 {
					// Get the lowest rate as estimate
					for _, rate := range rates {
						if rate.Available && (shippingCost == 0 || rate.Rate < shippingCost) {
							shippingCost = rate.Rate
						}
					}
					log.Printf("Shiprocket: Using estimated shipping cost from rates: %.2f", shippingCost)
				}
			}
		} else {
			awbCode = awbResult.AWBCode
			courierName = awbResult.CourierName
			// Only use AWB result cost if we don't have a pre-calculated cost
			if shippingCost == 0 {
				shippingCost = awbResult.ShippingCost
			}
			log.Printf("Shiprocket: AWB generated - AWB=%s, Cost=%.2f, Courier=%s", awbCode, shippingCost, courierName)
		}
	}

	// Generate label if we have AWB
	var labelURL string
	if createResp.ShipmentID > 0 {
		labelURL, _ = s.generateLabel(createResp.ShipmentID)
		if labelURL != "" {
			log.Printf("Shiprocket: Label generated - URL=%s", labelURL)
		}
	}

	shipment := &models.Shipment{
		OrderID:           request.OrderID,
		OrderNumber:       request.OrderNumber,
		Carrier:           models.CarrierShiprocket,
		CarrierShipmentID: fmt.Sprintf("%d", createResp.ShipmentID),
		TrackingNumber:    awbCode,
		TrackingURL:       fmt.Sprintf("https://shiprocket.co/tracking/%s", awbCode),
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
		Metadata:          fmt.Sprintf(`{"courier_name":"%s","shiprocket_order_id":%d}`, courierName, createResp.OrderID),
	}

	return shipment, nil
}

// AWBResult holds the result of AWB generation
type AWBResult struct {
	AWBCode      string
	CourierName  string
	ShippingCost float64
}

// generateAWB assigns a courier and generates AWB for the shipment
// If courierID is provided (from customer's checkout selection), it will auto-assign that specific courier
// Otherwise, Shiprocket will show a list of available couriers for manual selection
func (s *ShiprocketCarrier) generateAWB(shipmentID int, courierID string) (*AWBResult, error) {
	url := fmt.Sprintf("%s/v1/external/courier/assign/awb", s.config.BaseURL)

	payload := map[string]interface{}{
		"shipment_id": shipmentID,
	}

	// If customer selected a specific courier at checkout, include it for auto-assignment
	if courierID != "" {
		payload["courier_id"] = courierID
		log.Printf("Shiprocket: Using customer-selected courier_id=%s for auto-assignment", courierID)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.authToken))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var awbResp struct {
		Response struct {
			Data struct {
				AWBCode         string  `json:"awb_code"`
				CourierName     string  `json:"courier_name"`
				CourierCompanyID int    `json:"courier_company_id"`
				ShippingCharge  float64 `json:"applied_weight_slab_charge"`
				FreightCharge   float64 `json:"freight_charge"`
				Rate            float64 `json:"rate"`
			} `json:"data"`
		} `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&awbResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Use the best available cost field
	cost := awbResp.Response.Data.Rate
	if cost == 0 {
		cost = awbResp.Response.Data.FreightCharge
	}
	if cost == 0 {
		cost = awbResp.Response.Data.ShippingCharge
	}

	return &AWBResult{
		AWBCode:      awbResp.Response.Data.AWBCode,
		CourierName:  awbResp.Response.Data.CourierName,
		ShippingCost: cost,
	}, nil
}

// generateLabel generates the shipping label for a shipment
func (s *ShiprocketCarrier) generateLabel(shipmentID int) (string, error) {
	url := fmt.Sprintf("%s/v1/external/courier/generate/label", s.config.BaseURL)

	payload := map[string]interface{}{
		"shipment_id": []int{shipmentID},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.authToken))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var labelResp struct {
		LabelURL      string `json:"label_url"`
		LabelCreated  int    `json:"label_created"`
		NotCreated    []int  `json:"not_created"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&labelResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return labelResp.LabelURL, nil
}

// ShiprocketTrackingResponse represents Shiprocket tracking API response
type ShiprocketTrackingResponse struct {
	TrackingData struct {
		AWB               string `json:"awb"`
		TrackStatus       int    `json:"track_status"`
		ShipmentStatus    string `json:"shipment_status"`
		ShipmentStatusID  int    `json:"shipment_status_id"`
		CurrentStatus     string `json:"current_status"`
		CurrentStatusID   int    `json:"current_status_id"`
		ETD               string `json:"etd"`
		ShipmentTrack     []struct {
			Date     string `json:"date"`
			Status   string `json:"status"`
			Activity string `json:"activity"`
			Location string `json:"location"`
		} `json:"shipment_track"`
		ShipmentTrackActivities []struct {
			Date     string `json:"date"`
			Status   string `json:"status"`
			Activity string `json:"activity"`
			Location string `json:"location"`
		} `json:"shipment_track_activities"`
	} `json:"tracking_data"`
}

// mapShiprocketStatus maps Shiprocket status ID to internal shipment status
func mapShiprocketStatus(statusID int) models.ShipmentStatus {
	// Shiprocket status IDs: https://apidocs.shiprocket.in/
	// 1 - Pickup Pending, 2 - Pickup Queued, 3 - Pickup Scheduled
	// 4 - Out for Pickup, 5 - Picked Up, 6 - In Transit
	// 7 - Delivered, 8 - Cancelled, 9 - RTO Initiated, 10 - RTO Delivered
	// 11 - Lost, 12 - Damaged, 13 - Pickup Rescheduled
	switch statusID {
	case 1, 2, 3, 4, 13:
		return models.ShipmentStatusPending
	case 5:
		return models.ShipmentStatusPickedUp
	case 6:
		return models.ShipmentStatusInTransit
	case 7:
		return models.ShipmentStatusDelivered
	case 8:
		return models.ShipmentStatusCancelled
	case 9, 10:
		return models.ShipmentStatusReturned
	case 11, 12:
		return models.ShipmentStatusFailed
	default:
		return models.ShipmentStatusInTransit
	}
}

// GetTracking retrieves tracking information
func (s *ShiprocketCarrier) GetTracking(trackingNumber string) (*models.TrackShipmentResponse, error) {
	if err := s.authenticate(); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	url := fmt.Sprintf("%s/v1/external/courier/track/awb/%s", s.config.BaseURL, trackingNumber)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.authToken))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse Shiprocket tracking response
	var shiprocketResp ShiprocketTrackingResponse
	if err := json.NewDecoder(resp.Body).Decode(&shiprocketResp); err != nil {
		return nil, fmt.Errorf("failed to parse tracking response: %w", err)
	}

	trackingData := shiprocketResp.TrackingData

	// Map Shiprocket status to internal status
	status := mapShiprocketStatus(trackingData.CurrentStatusID)

	// Parse estimated delivery date
	var estimatedDelivery *time.Time
	if trackingData.ETD != "" {
		if parsed, err := time.Parse("2006-01-02", trackingData.ETD); err == nil {
			estimatedDelivery = &parsed
		}
	}

	// Parse actual delivery date if delivered
	var actualDelivery *time.Time
	if status == models.ShipmentStatusDelivered {
		// Find the delivery event timestamp
		activities := trackingData.ShipmentTrackActivities
		if len(activities) == 0 {
			activities = make([]struct {
				Date     string `json:"date"`
				Status   string `json:"status"`
				Activity string `json:"activity"`
				Location string `json:"location"`
			}, len(trackingData.ShipmentTrack))
			for i, track := range trackingData.ShipmentTrack {
				activities[i] = struct {
					Date     string `json:"date"`
					Status   string `json:"status"`
					Activity string `json:"activity"`
					Location string `json:"location"`
				}{
					Date:     track.Date,
					Status:   track.Status,
					Activity: track.Activity,
					Location: track.Location,
				}
			}
		}
		for _, activity := range activities {
			if strings.Contains(strings.ToLower(activity.Status), "delivered") {
				if parsed, err := time.Parse("2006-01-02 15:04:05", activity.Date); err == nil {
					actualDelivery = &parsed
					break
				}
				if parsed, err := time.Parse("2006-01-02", activity.Date); err == nil {
					actualDelivery = &parsed
					break
				}
			}
		}
	}

	// Parse tracking events
	var events []models.ShipmentTracking
	activities := trackingData.ShipmentTrackActivities
	if len(activities) == 0 {
		// Fallback to shipment_track if activities not available
		for _, track := range trackingData.ShipmentTrack {
			timestamp, _ := time.Parse("2006-01-02 15:04:05", track.Date)
			if timestamp.IsZero() {
				timestamp, _ = time.Parse("2006-01-02", track.Date)
			}
			events = append(events, models.ShipmentTracking{
				Status:      track.Status,
				Location:    track.Location,
				Description: track.Activity,
				Timestamp:   timestamp,
			})
		}
	} else {
		for _, activity := range activities {
			timestamp, _ := time.Parse("2006-01-02 15:04:05", activity.Date)
			if timestamp.IsZero() {
				timestamp, _ = time.Parse("2006-01-02", activity.Date)
			}
			events = append(events, models.ShipmentTracking{
				Status:      activity.Status,
				Location:    activity.Location,
				Description: activity.Activity,
				Timestamp:   timestamp,
			})
		}
	}

	trackingResp := &models.TrackShipmentResponse{
		TrackingNumber:    trackingNumber,
		Carrier:           models.CarrierShiprocket,
		Status:            status,
		EstimatedDelivery: estimatedDelivery,
		ActualDelivery:    actualDelivery,
		Events:            events,
	}

	return trackingResp, nil
}

// CancelShipment cancels a shipment
func (s *ShiprocketCarrier) CancelShipment(shipmentID string) error {
	if err := s.authenticate(); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	url := fmt.Sprintf("%s/v1/external/orders/cancel/shipment/%s", s.config.BaseURL, shipmentID)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.authToken))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// IsAvailable checks if Shiprocket is available for the route
func (s *ShiprocketCarrier) IsAvailable(fromCountry, toCountry string) bool {
	// Shiprocket primarily serves India
	return fromCountry == "IN" || toCountry == "IN"
}

// SupportsRegion checks if Shiprocket supports a region
func (s *ShiprocketCarrier) SupportsRegion(countryCode string) bool {
	// Shiprocket supports India and some international destinations
	region := GetRegionForCountry(countryCode)
	return region == RegionIndia
}

// GenerateReturnLabel generates a return shipping label using Shiprocket's return order API
func (s *ShiprocketCarrier) GenerateReturnLabel(request models.ReturnLabelRequest) (*models.ReturnLabelResponse, error) {
	if err := s.authenticate(); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Create return order using Shiprocket's create return API
	url := fmt.Sprintf("%s/v1/external/orders/create/return", s.config.BaseURL)

	payload := map[string]interface{}{
		"order_id":            request.OrderNumber,
		"order_date":          time.Now().Format("2006-01-02"),
		"channel_id":          "",
		"pickup_customer_name": request.CustomerAddress.Name,
		"pickup_address":       request.CustomerAddress.Street,
		"pickup_address_2":     request.CustomerAddress.Street2,
		"pickup_city":          request.CustomerAddress.City,
		"pickup_state":         request.CustomerAddress.State,
		"pickup_country":       request.CustomerAddress.Country,
		"pickup_pincode":       request.CustomerAddress.PostalCode,
		"pickup_email":         request.CustomerAddress.Email,
		"pickup_phone":         request.CustomerAddress.Phone,
		"shipping_customer_name": request.ReturnAddress.Name,
		"shipping_address":     request.ReturnAddress.Street,
		"shipping_address_2":   request.ReturnAddress.Street2,
		"shipping_city":        request.ReturnAddress.City,
		"shipping_state":       request.ReturnAddress.State,
		"shipping_country":     request.ReturnAddress.Country,
		"shipping_pincode":     request.ReturnAddress.PostalCode,
		"shipping_email":       request.ReturnAddress.Email,
		"shipping_phone":       request.ReturnAddress.Phone,
		"order_items": []map[string]interface{}{
			{
				"name":          fmt.Sprintf("Return - %s", request.RMANumber),
				"sku":           request.RMANumber,
				"units":         1,
				"selling_price": 0,
				"qc_enable":     true,
			},
		},
		"payment_method": "Prepaid",
		"sub_total":      0,
		"length":         request.Length,
		"breadth":        request.Width,
		"height":         request.Height,
		"weight":         request.Weight,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.authToken))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var returnResp struct {
		OrderID      int    `json:"order_id"`
		ShipmentID   int    `json:"shipment_id"`
		Status       string `json:"status"`
		AWBCode      string `json:"awb_code"`
		LabelURL     string `json:"label_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&returnResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Generate label URL if not provided directly
	labelURL := returnResp.LabelURL
	if labelURL == "" && returnResp.ShipmentID > 0 {
		// Fetch label separately
		labelURL, _ = s.fetchReturnLabel(returnResp.ShipmentID)
	}

	// Set expiry to 30 days from now (typical return label validity)
	expiresAt := time.Now().AddDate(0, 0, 30)

	return &models.ReturnLabelResponse{
		ReturnID:       request.ReturnID,
		RMANumber:      request.RMANumber,
		Carrier:        models.CarrierShiprocket,
		TrackingNumber: returnResp.AWBCode,
		LabelURL:       labelURL,
		Status:         models.ShipmentStatusCreated,
		ExpiresAt:      &expiresAt,
	}, nil
}

// fetchReturnLabel fetches the label URL for a return shipment
func (s *ShiprocketCarrier) fetchReturnLabel(shipmentID int) (string, error) {
	url := fmt.Sprintf("%s/v1/external/courier/generate/label", s.config.BaseURL)

	payload := map[string]interface{}{
		"shipment_id": []int{shipmentID},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.authToken))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var labelResp struct {
		LabelURL string `json:"label_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&labelResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return labelResp.LabelURL, nil
}

// PickupLocation represents a Shiprocket pickup location
type PickupLocation struct {
	ID                int    `json:"id"`
	PickupCode        string `json:"pickup_location"`
	Name              string `json:"name"`
	Email             string `json:"email"`
	Phone             string `json:"phone"`
	Address           string `json:"address"`
	Address2          string `json:"address_2"`
	City              string `json:"city"`
	State             string `json:"state"`
	Country           string `json:"country"`
	PinCode           string `json:"pin_code"`
	IsPrimaryLocation int    `json:"is_primary_location"` // 0 or 1 from Shiprocket API
}

// IsPrimary returns true if this is the primary pickup location
func (p PickupLocation) IsPrimary() bool {
	return p.IsPrimaryLocation == 1
}

// AddPickupLocation creates or updates a pickup location in Shiprocket
func (s *ShiprocketCarrier) AddPickupLocation(location PickupLocation) error {
	if err := s.authenticate(); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	url := fmt.Sprintf("%s/v1/external/settings/company/addpickup", s.config.BaseURL)

	payload := map[string]interface{}{
		"pickup_location": location.PickupCode,
		"name":            location.Name,
		"email":           location.Email,
		"phone":           location.Phone,
		"address":         location.Address,
		"address_2":       location.Address2,
		"city":            location.City,
		"state":           location.State,
		"country":         location.Country,
		"pin_code":        location.PinCode,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.authToken))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// GetPickupLocations retrieves all pickup locations from Shiprocket
func (s *ShiprocketCarrier) GetPickupLocations() ([]PickupLocation, error) {
	if err := s.authenticate(); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	url := fmt.Sprintf("%s/v1/external/settings/company/pickup", s.config.BaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.authToken))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var pickupResp struct {
		Data struct {
			ShippingAddress []PickupLocation `json:"shipping_address"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&pickupResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return pickupResp.Data.ShippingAddress, nil
}

// splitName splits a full name into first and last name
func splitName(fullName string) (firstName, lastName string) {
	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return "Customer", "."
	}
	if len(parts) == 1 {
		return parts[0], "."
	}
	// First part is first name, rest is last name
	return parts[0], strings.Join(parts[1:], " ")
}

// Note: CleanPhoneNumber is now in utils.go for shared use across carriers
