package teslamate

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/teslacost/teslacost/internal/apierror"
)

// AuthType defines how to authenticate against teslamateapi.
type AuthType string

const (
	AuthNone   AuthType = "NONE"
	AuthBearer AuthType = "BEARER"
	AuthBasic  AuthType = "BASIC"
)

// Config holds configuration for connecting to teslamateapi.
type Config struct {
	BaseURL   string
	AuthType  AuthType
	APIToken  string // Used when AuthType is BEARER
	Username  string // Used when AuthType is BASIC
	Password  string // Used when AuthType is BASIC
	Timeout   time.Duration
	UserAgent string
}

// Client is a client for the TeslaMate REST API (teslamateapi).
type Client struct {
	baseURL    string
	authType   AuthType
	apiToken   string
	username   string
	password   string
	httpClient *http.Client
	userAgent  string
}

// NewClient creates a new TeslaMate API client.
func NewClient(cfg Config) (*Client, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		return nil, fmt.Errorf("teslamate baseURL cannot be empty")
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}

	userAgent := cfg.UserAgent
	if userAgent == "" {
		userAgent = "AutoLedger/1.0"
	}

	return &Client{
		baseURL:  baseURL,
		authType: cfg.AuthType,
		apiToken: cfg.APIToken,
		username: cfg.Username,
		password: cfg.Password,
		httpClient: &http.Client{
			Timeout: timeout,
			// Never follow redirects: the API URL is user-provided and must not bounce to other hosts.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		userAgent: userAgent,
	}, nil
}

// doRequest performs an HTTP request with configured headers and auth.
func (c *Client) doRequest(ctx context.Context, method, endpoint string, queryParams url.Values, target any) error {
	fullURL := fmt.Sprintf("%s%s", c.baseURL, endpoint)
	if len(queryParams) > 0 {
		fullURL = fmt.Sprintf("%s?%s", fullURL, queryParams.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	// Apply authentication
	switch c.authType {
	case AuthBearer:
		if c.apiToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.apiToken)
		}
	case AuthBasic:
		if c.username != "" || c.password != "" {
			auth := base64.StdEncoding.EncodeToString([]byte(c.username + ":" + c.password))
			req.Header.Set("Authorization", "Basic "+auth)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("teslamateapi request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// The upstream body is not echoed back: the URL is user-provided and could target internal services.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return apierror.Newf("teslamate.auth_refused", "teslamateapi returned status %d: authentication refused", resp.StatusCode)
		case http.StatusNotFound:
			return apierror.Newf("teslamate.not_found", "teslamateapi returned status %d: resource not found (wrong URL or vehicle identifier)", resp.StatusCode)
		}
		return fmt.Errorf("teslamateapi returned status %d", resp.StatusCode)
	}

	if target != nil {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			return fmt.Errorf("failed to decode response JSON: %w", err)
		}
	}

	return nil
}

// GetCars retrieves all cars logged in TeslaMate.
func (c *Client) GetCars(ctx context.Context) ([]Car, error) {
	var resp CarsResponse
	err := c.doRequest(ctx, http.MethodGet, "/api/v1/cars", nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Data.Cars, nil
}

// GetCar retrieves information for a specific car ID.
func (c *Client) GetCar(ctx context.Context, carID int) (*Car, error) {
	cars, err := c.GetCars(ctx)
	if err != nil {
		return nil, err
	}
	for _, car := range cars {
		if car.CarID == carID {
			return &car, nil
		}
	}
	return nil, fmt.Errorf("car with ID %d not found in teslamate", carID)
}

// GetCarStatus retrieves current status telemetry (odometer, state, battery) for a car.
func (c *Client) GetCarStatus(ctx context.Context, carID int) (*StatusDetails, *Units, error) {
	var resp StatusResponse
	endpoint := fmt.Sprintf("/api/v1/cars/%d/status", carID)
	err := c.doRequest(ctx, http.MethodGet, endpoint, nil, &resp)
	if err != nil {
		return nil, nil, err
	}
	return &resp.Data.Status, &resp.Data.Units, nil
}

// GetDrives retrieves drives history for a specific car ID with optional filters.
func (c *Client) GetDrives(ctx context.Context, carID int, opts DriveFilterOptions) ([]Drive, *Units, error) {
	params := make(url.Values)

	if opts.Page > 0 {
		params.Set("page", strconv.Itoa(opts.Page))
	}
	if opts.Show > 0 {
		params.Set("show", strconv.Itoa(opts.Show))
	}
	if opts.StartDate != "" {
		params.Set("startDate", opts.StartDate)
	}
	if opts.EndDate != "" {
		params.Set("endDate", opts.EndDate)
	}
	if opts.MinDistance > 0 {
		params.Set("minDistance", fmt.Sprintf("%.2f", opts.MinDistance))
	}

	var resp DrivesResponse
	endpoint := fmt.Sprintf("/api/v1/cars/%d/drives", carID)
	err := c.doRequest(ctx, http.MethodGet, endpoint, params, &resp)
	if err != nil {
		return nil, nil, err
	}

	return resp.Data.Drives, &resp.Data.Units, nil
}

// GetDriveDetails retrieves the full GPS trace (drive_details) for a single drive, with the units its speeds are in.
func (c *Client) GetDriveDetails(ctx context.Context, carID, driveID int) ([]DrivePosition, *Units, error) {
	var resp DriveDetailResponse
	endpoint := fmt.Sprintf("/api/v1/cars/%d/drives/%d", carID, driveID)
	err := c.doRequest(ctx, http.MethodGet, endpoint, nil, &resp)
	if err != nil {
		return nil, nil, err
	}
	return resp.Data.Drive.DriveDetails, &resp.Data.Units, nil
}

// GetCharges retrieves charges history for a specific car ID with optional filters.
func (c *Client) GetCharges(ctx context.Context, carID int, opts ChargeFilterOptions) ([]Charge, *Units, error) {
	params := make(url.Values)

	if opts.Page > 0 {
		params.Set("page", strconv.Itoa(opts.Page))
	}
	if opts.Show > 0 {
		params.Set("show", strconv.Itoa(opts.Show))
	}
	if opts.StartDate != "" {
		params.Set("startDate", opts.StartDate)
	}
	if opts.EndDate != "" {
		params.Set("endDate", opts.EndDate)
	}

	var resp ChargesResponse
	endpoint := fmt.Sprintf("/api/v1/cars/%d/charges", carID)
	err := c.doRequest(ctx, http.MethodGet, endpoint, params, &resp)
	if err != nil {
		return nil, nil, err
	}

	return resp.Data.Charges, &resp.Data.Units, nil
}

// GetBatteryHealth retrieves the battery health computed by TeslaMateApi. Older versions of TeslaMateApi do not
// expose the endpoint.
func (c *Client) GetBatteryHealth(ctx context.Context, carID int) (*BatteryHealth, error) {
	var resp BatteryHealthResponse
	endpoint := fmt.Sprintf("/api/v1/cars/%d/battery-health", carID)
	if err := c.doRequest(ctx, http.MethodGet, endpoint, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data.BatteryHealth, nil
}

// ConvertTemperatureToC converts a temperature to Celsius based on the reported unit.
func ConvertTemperatureToC(val float64, unit string) float64 {
	if strings.EqualFold(unit, "F") {
		return (val - 32) * 5 / 9
	}
	return val
}

// ConvertDistanceToKm converts a distance to kilometers based on the reported unit.
func ConvertDistanceToKm(val float64, unit string) float64 {
	if strings.ToLower(unit) == "mi" {
		return val * 1.609344
	}
	return val
}
