// Package geocode turns coordinates into a short address through a Nominatim reverse-geocoding endpoint.
package geocode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultURL is the public OpenStreetMap instance. Its usage policy allows one request per second and
	// requires an identifying User-Agent.
	DefaultURL       = "https://nominatim.openstreetmap.org"
	DefaultUserAgent = "AutoLedger (+https://github.com/Rem7474/AutoLedger)"

	minInterval = time.Second
	maxCache    = 4096
	// cacheDecimals rounds the coordinates of the cache key to about 11 m, so a place visited every day is
	// looked up once.
	cacheDecimals = 4
)

// Client is a reverse geocoder with a cache and a request rate limit. The zero value is not usable: use New.
type Client struct {
	baseURL   string
	userAgent string
	http      *http.Client
	interval  time.Duration

	mu    sync.Mutex
	cache map[string]string
	last  time.Time
}

// New returns a client for a Nominatim instance. An empty baseURL or userAgent selects the default.
func New(baseURL, userAgent string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultURL
	}
	if strings.TrimSpace(userAgent) == "" {
		userAgent = DefaultUserAgent
	}
	return &Client{
		baseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		userAgent: userAgent,
		http:      &http.Client{Timeout: 10 * time.Second},
		interval:  minInterval,
		cache:     map[string]string{},
	}
}

type reverseResponse struct {
	DisplayName string            `json:"display_name"`
	Address     map[string]string `json:"address"`
	Error       string            `json:"error"`
}

// Reverse returns a short address for a position, or an error when none is found or the service fails.
func (c *Client) Reverse(ctx context.Context, lat, lon float64) (string, error) {
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return "", errors.New("coordinates out of range")
	}
	key := fmt.Sprintf("%.*f,%.*f", cacheDecimals, lat, cacheDecimals, lon)
	if addr, ok := c.cached(key); ok {
		return addr, nil
	}
	if err := c.wait(ctx); err != nil {
		return "", err
	}

	q := url.Values{}
	q.Set("format", "jsonv2")
	q.Set("lat", fmt.Sprintf("%.6f", lat))
	q.Set("lon", fmt.Sprintf("%.6f", lon))
	q.Set("zoom", "18")
	q.Set("addressdetails", "1")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/reverse?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("geocoder answered %d", resp.StatusCode)
	}
	var body reverseResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	addr := label(body)
	if addr == "" {
		return "", errors.New("no address for this position")
	}
	c.store(key, addr)
	return addr, nil
}

// label builds "12 Rue Example, Annecy" from the address parts, falling back to the full display name.
func label(r reverseResponse) string {
	a := r.Address
	street := strings.TrimSpace(a["house_number"] + " " + firstOf(a, "road", "pedestrian", "footway", "path", "square"))
	place := firstOf(a, "city", "town", "village", "municipality", "hamlet", "suburb")
	switch {
	case street != "" && place != "":
		return street + ", " + place
	case street != "":
		return street
	case place != "":
		if poi := firstOf(a, "amenity", "shop", "tourism", "building"); poi != "" {
			return poi + ", " + place
		}
		return place
	}
	return strings.TrimSpace(r.DisplayName)
}

func firstOf(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(m[k]); v != "" {
			return v
		}
	}
	return ""
}

func (c *Client) cached(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.cache[key]
	return v, ok
}

func (c *Client) store(key, addr string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.cache) >= maxCache {
		for k := range c.cache {
			delete(c.cache, k)
			break
		}
	}
	c.cache[key] = addr
}

// wait blocks until a request may be sent without exceeding the rate limit.
func (c *Client) wait(ctx context.Context) error {
	c.mu.Lock()
	next := c.last.Add(c.interval)
	if now := time.Now(); next.Before(now) {
		next = now
	}
	c.last = next
	c.mu.Unlock()

	delay := time.Until(next)
	if delay <= 0 {
		return nil
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
