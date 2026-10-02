package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

const providerTeslaMate = "TESLAMATE"

// teslaMateSourceConfig is the non-secret part of a TeslaMate source, stored as JSON.
type teslaMateSourceConfig struct {
	APIURL     *string         `json:"api_url,omitempty"`
	AuthType   models.AuthMode `json:"auth_type,omitempty"`
	CarID      *int            `json:"car_id,omitempty"`
	GrafanaURL *string         `json:"grafana_url,omitempty"`
	BasicUser  *string         `json:"basic_user,omitempty"`
}

func nonBlank(s *string) bool { return s != nil && strings.TrimSpace(*s) != "" }

// hasTeslaMateConfig reports whether the vehicle carries any TeslaMate setting worth keeping as a source.
func hasTeslaMateConfig(v *models.Vehicle) bool {
	return nonBlank(v.TeslaMateAPIURL) || nonBlank(v.TeslaMateGrafanaURL) || v.TeslaMateCarID != nil ||
		nonBlank(v.TeslaMateBasicUser) || nonBlank(v.TeslaMateAPIKeyEncrypted) || nonBlank(v.TeslaMateBasicPassEnc) ||
		(v.TeslaMateAuthType != "" && v.TeslaMateAuthType != models.AuthModeNone)
}

// saveTeslaMateSource keeps the vehicle's TeslaMate source equal to its TeslaMate settings: created or updated
// when the vehicle carries any, removed otherwise.
func saveTeslaMateSource(ctx context.Context, q querier, v *models.Vehicle) error {
	if !hasTeslaMateConfig(v) {
		_, err := q.Exec(ctx, `DELETE FROM vehicle_data_sources WHERE vehicle_id = $1 AND provider = $2`, v.ID, providerTeslaMate)
		if err != nil {
			return fmt.Errorf("failed to remove the TeslaMate source: %w", err)
		}
		return nil
	}
	cfg, err := json.Marshal(teslaMateSourceConfig{
		APIURL: v.TeslaMateAPIURL, AuthType: v.TeslaMateAuthType, CarID: v.TeslaMateCarID,
		GrafanaURL: v.TeslaMateGrafanaURL, BasicUser: v.TeslaMateBasicUser,
	})
	if err != nil {
		return fmt.Errorf("failed to encode the TeslaMate source: %w", err)
	}
	_, err = q.Exec(ctx, `
		INSERT INTO vehicle_data_sources (vehicle_id, provider, config, secret_encrypted, basic_secret_encrypted)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (vehicle_id) WHERE provider = 'TESLAMATE' DO UPDATE
		SET config = EXCLUDED.config, secret_encrypted = EXCLUDED.secret_encrypted,
		    basic_secret_encrypted = EXCLUDED.basic_secret_encrypted, updated_at = NOW()`,
		v.ID, providerTeslaMate, cfg, v.TeslaMateAPIKeyEncrypted, v.TeslaMateBasicPassEnc)
	if err != nil {
		return fmt.Errorf("failed to save the TeslaMate source: %w", err)
	}
	return nil
}

// loadTeslaMateSources fills the TeslaMate fields of the vehicles from their sources; a vehicle without a
// source has none.
func (r *Repository) loadTeslaMateSources(ctx context.Context, vehicles []*models.Vehicle) error {
	if len(vehicles) == 0 {
		return nil
	}
	ids := make([]string, len(vehicles))
	for i, v := range vehicles {
		ids[i] = v.ID
	}
	rows, err := r.pool.Query(ctx, `
		SELECT vehicle_id::text, config, secret_encrypted, basic_secret_encrypted
		FROM vehicle_data_sources
		WHERE provider = $1 AND vehicle_id::text = ANY($2)`, providerTeslaMate, ids)
	if err != nil {
		return fmt.Errorf("failed to load the TeslaMate sources: %w", err)
	}
	defer rows.Close()

	type source struct {
		cfg              teslaMateSourceConfig
		secret, basicSec *string
	}
	byVehicle := make(map[string]source, len(vehicles))
	for rows.Next() {
		var id string
		var raw []byte
		var s source
		if err := rows.Scan(&id, &raw, &s.secret, &s.basicSec); err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &s.cfg); err != nil {
			return fmt.Errorf("invalid TeslaMate source of vehicle %s: %w", id, err)
		}
		byVehicle[id] = s
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, v := range vehicles {
		s := byVehicle[v.ID]
		v.TeslaMateAPIURL, v.TeslaMateCarID, v.TeslaMateGrafanaURL, v.TeslaMateBasicUser = s.cfg.APIURL, s.cfg.CarID, s.cfg.GrafanaURL, s.cfg.BasicUser
		v.TeslaMateAuthType = s.cfg.AuthType
		if v.TeslaMateAuthType == "" {
			v.TeslaMateAuthType = models.AuthModeNone
		}
		v.TeslaMateAPIKeyEncrypted, v.TeslaMateBasicPassEnc = s.secret, s.basicSec
	}
	return nil
}

// SourceActivity is what one origin (TESLAMATE, WEBHOOK, CSV, MANUAL) has written for a vehicle.
type SourceActivity struct {
	Origin           string     `json:"origin"`
	Drives           int        `json:"drives"`
	Charges          int        `json:"charges"`
	OdometerReadings int        `json:"odometer_readings"`
	LastAt           *time.Time `json:"last_at,omitempty"`
}

// ListSourceActivity counts the drives, charges and odometer readings each origin wrote for a vehicle.
// An origin that wrote nothing is absent.
func (r *Repository) ListSourceActivity(ctx context.Context, vehicleID string) ([]SourceActivity, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT origin,
		       COUNT(*) FILTER (WHERE kind = 'drive'),
		       COUNT(*) FILTER (WHERE kind = 'charge'),
		       COUNT(*) FILTER (WHERE kind = 'odometer'),
		       MAX(at)
		FROM (
			SELECT origin, 'drive' AS kind, start_time AS at FROM drives WHERE vehicle_id = $1
			UNION ALL
			SELECT origin, 'charge', date FROM charge_logs WHERE vehicle_id = $1
			UNION ALL
			SELECT CASE source WHEN 'HA' THEN 'WEBHOOK' ELSE 'MANUAL' END, 'odometer', date::timestamptz
			FROM odometer_checkpoints WHERE vehicle_id = $1
		) x
		GROUP BY origin
		ORDER BY origin;
	`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SourceActivity{}
	for rows.Next() {
		var a SourceActivity
		if err := rows.Scan(&a.Origin, &a.Drives, &a.Charges, &a.OdometerReadings, &a.LastAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
