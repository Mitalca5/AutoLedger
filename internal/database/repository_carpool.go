package database

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
)

// Carpool trips, legs and passenger cost-sharing.
// CreateCarpoolTrip stores a new carpool trip with its legs and passengers.
func (r *Repository) CreateCarpoolTrip(ctx context.Context, trip *models.CarpoolTrip, legs []models.CarpoolLeg, passengers []models.CarpoolPassenger) error {
	trip.ID = ""
	return r.saveCarpoolTrip(ctx, trip, legs, passengers)
}

// UpdateCarpoolTrip replaces a carpool trip, its legs and its passengers.
func (r *Repository) UpdateCarpoolTrip(ctx context.Context, trip *models.CarpoolTrip, legs []models.CarpoolLeg, passengers []models.CarpoolPassenger) error {
	if trip.ID == "" {
		return ErrNotFound
	}
	return r.saveCarpoolTrip(ctx, trip, legs, passengers)
}

// saveCarpoolTrip writes a trip atomically. Trip totals are the sums of its legs; passengers' stops must
// reference existing stops (0..len(legs)).
func (r *Repository) saveCarpoolTrip(ctx context.Context, trip *models.CarpoolTrip, legs []models.CarpoolLeg, passengers []models.CarpoolPassenger) error {
	if len(legs) == 0 {
		return apierror.New("carpool.needs_leg", "A carpool needs at least one leg")
	}
	for _, p := range passengers {
		if p.BoardStopIndex < 0 || p.AlightStopIndex <= p.BoardStopIndex || p.AlightStopIndex > len(legs) {
			return apierror.Newf("carpool.invalid_stops", "Invalid pick-up and drop-off stops for %s", p.PassengerName)
		}
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := ensureCarpoolLinksOwned(ctx, tx, trip); err != nil {
		return err
	}
	var driveIDs []string
	for _, l := range legs {
		if l.DriveID != nil && *l.DriveID != "" {
			driveIDs = append(driveIDs, *l.DriveID)
		}
	}
	if err := ensureDrivesOwned(ctx, tx, trip.VehicleID, driveIDs); err != nil {
		return err
	}

	trip.DistanceKm, trip.ElectricityCost, trip.TollsCost, trip.TiresCost = 0, 0, 0, 0
	trip.MaintenanceCost, trip.InsuranceCost, trip.OtherCost, trip.TotalRevenue = 0, 0, 0, 0
	for _, l := range legs {
		trip.DistanceKm += l.DistanceKm
		trip.ElectricityCost += l.ElectricityCost
		trip.TollsCost += l.TollsCost
		trip.TiresCost += l.TiresCost
		trip.MaintenanceCost += l.MaintenanceCost
		trip.InsuranceCost += l.InsuranceCost
		trip.OtherCost += l.OtherCost
	}
	for _, p := range passengers {
		trip.TotalRevenue += p.AmountPaid
	}
	trip.DistanceKm = math.Round(trip.DistanceKm*100) / 100
	trip.TotalCost = trip.ElectricityCost + trip.TollsCost + trip.TiresCost + trip.MaintenanceCost + trip.InsuranceCost + trip.OtherCost
	trip.NetCost = trip.TotalCost - trip.TotalRevenue

	if trip.ID == "" {
		err = tx.QueryRow(ctx, `
			INSERT INTO carpool_trips (
				vehicle_id, drive_id, trip_group_id, title, date, distance_km,
				electricity_cost, tolls_cost, tires_cost, maintenance_cost, insurance_cost, other_cost,
				total_cost, total_revenue, net_cost, notes
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
			RETURNING id, created_at, updated_at;
		`,
			trip.VehicleID, trip.DriveID, trip.TripGroupID, trip.Title, trip.Date, trip.DistanceKm,
			trip.ElectricityCost, trip.TollsCost, trip.TiresCost, trip.MaintenanceCost, trip.InsuranceCost, trip.OtherCost,
			trip.TotalCost, trip.TotalRevenue, trip.NetCost, trip.Notes,
		).Scan(&trip.ID, &trip.CreatedAt, &trip.UpdatedAt)
	} else {
		err = tx.QueryRow(ctx, `
			UPDATE carpool_trips
			SET drive_id = $1, trip_group_id = $2, title = $3, date = $4, distance_km = $5,
			    electricity_cost = $6, tolls_cost = $7, tires_cost = $8, maintenance_cost = $9,
			    insurance_cost = $10, other_cost = $11, total_cost = $12, total_revenue = $13,
			    net_cost = $14, notes = $15, updated_at = NOW()
			WHERE id::text = $16 AND vehicle_id = $17
			RETURNING created_at, updated_at;
		`,
			trip.DriveID, trip.TripGroupID, trip.Title, trip.Date, trip.DistanceKm,
			trip.ElectricityCost, trip.TollsCost, trip.TiresCost, trip.MaintenanceCost,
			trip.InsuranceCost, trip.OtherCost, trip.TotalCost, trip.TotalRevenue,
			trip.NetCost, trip.Notes, trip.ID, trip.VehicleID,
		).Scan(&trip.CreatedAt, &trip.UpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
	}
	if err != nil {
		return fmt.Errorf("failed to save carpool trip: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM carpool_legs WHERE carpool_trip_id = $1;`, trip.ID); err != nil {
		return fmt.Errorf("failed to clear legs: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM carpool_passengers WHERE carpool_trip_id = $1;`, trip.ID); err != nil {
		return fmt.Errorf("failed to clear passengers: %w", err)
	}

	for i := range legs {
		l := &legs[i]
		l.CarpoolTripID, l.OrderIndex = trip.ID, i
		if err := tx.QueryRow(ctx, `
			INSERT INTO carpool_legs (
				carpool_trip_id, order_index, drive_id, start_label, end_label, distance_km,
				electricity_cost, tolls_cost, tires_cost, maintenance_cost, insurance_cost, other_cost
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING id;
		`, l.CarpoolTripID, l.OrderIndex, l.DriveID, l.StartLabel, l.EndLabel, l.DistanceKm,
			l.ElectricityCost, l.TollsCost, l.TiresCost, l.MaintenanceCost, l.InsuranceCost, l.OtherCost,
		).Scan(&l.ID); err != nil {
			return fmt.Errorf("failed to insert carpool leg: %w", err)
		}
	}

	// A carpool made of several drives is a trip: it gets its own trip group so it shows up with the trips.
	if trip.TripGroupID == nil {
		if ids := uniqueStrings(driveIDs); len(ids) >= 2 {
			tg := &models.TripGroup{VehicleID: trip.VehicleID, Name: trip.Title}
			if err := insertTripGroup(ctx, tx, tg, ids); err != nil {
				return fmt.Errorf("failed to group the carpool drives: %w", err)
			}
			if _, err := tx.Exec(ctx, `UPDATE carpool_trips SET trip_group_id = $1 WHERE id = $2;`, tg.ID, trip.ID); err != nil {
				return fmt.Errorf("failed to link the carpool to its trip: %w", err)
			}
			trip.TripGroupID = &tg.ID
		}
	}

	for i := range passengers {
		p := &passengers[i]
		p.CarpoolTripID = trip.ID
		if err := tx.QueryRow(ctx, `
			INSERT INTO carpool_passengers (
				carpool_trip_id, passenger_name, origin, destination, seats, amount_paid, notes,
				board_stop_index, alight_stop_index
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING id, created_at;
		`, p.CarpoolTripID, p.PassengerName, p.Origin, p.Destination, p.Seats, p.AmountPaid, p.Notes,
			p.BoardStopIndex, p.AlightStopIndex,
		).Scan(&p.ID, &p.CreatedAt); err != nil {
			return fmt.Errorf("failed to insert carpool passenger: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func ensureCarpoolLinksOwned(ctx context.Context, tx pgx.Tx, trip *models.CarpoolTrip) error {
	if trip.DriveID != nil && *trip.DriveID == "" {
		trip.DriveID = nil
	}
	if trip.TripGroupID != nil && *trip.TripGroupID == "" {
		trip.TripGroupID = nil
	}
	if trip.DriveID != nil {
		if err := ensureDrivesOwned(ctx, tx, trip.VehicleID, []string{*trip.DriveID}); err != nil {
			return err
		}
	}
	if trip.TripGroupID != nil {
		if err := ensureTripGroupOwned(ctx, tx, trip.VehicleID, *trip.TripGroupID); err != nil {
			return err
		}
	}
	return nil
}

const carpoolTripColumns = `
	id, vehicle_id, drive_id, trip_group_id, title, date, distance_km,
	electricity_cost, tolls_cost, tires_cost, maintenance_cost, insurance_cost, other_cost,
	total_cost, total_revenue, net_cost, notes, created_at, updated_at
`

func scanCarpoolTrip(row pgx.Row, t *models.CarpoolTripWithPassengers) error {
	return row.Scan(
		&t.ID, &t.VehicleID, &t.DriveID, &t.TripGroupID, &t.Title, &t.Date, &t.DistanceKm,
		&t.ElectricityCost, &t.TollsCost, &t.TiresCost, &t.MaintenanceCost, &t.InsuranceCost, &t.OtherCost,
		&t.TotalCost, &t.TotalRevenue, &t.NetCost, &t.Notes, &t.CreatedAt, &t.UpdatedAt,
	)
}

// ListCarpoolTrips returns the carpool trips of a vehicle with their legs and passengers.
func (r *Repository) ListCarpoolTrips(ctx context.Context, vehicleID string) ([]models.CarpoolTripWithPassengers, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+carpoolTripColumns+` FROM carpool_trips WHERE vehicle_id = $1 ORDER BY date DESC, created_at DESC;`, vehicleID)
	if err != nil {
		return nil, fmt.Errorf("failed to list carpool trips: %w", err)
	}
	trips := []models.CarpoolTripWithPassengers{}
	for rows.Next() {
		var t models.CarpoolTripWithPassengers
		if err := scanCarpoolTrip(rows, &t); err != nil {
			rows.Close()
			return nil, err
		}
		trips = append(trips, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.loadCarpoolDetails(ctx, trips); err != nil {
		return nil, err
	}
	return trips, nil
}

func (r *Repository) GetCarpoolTrip(ctx context.Context, id, vehicleID string) (*models.CarpoolTripWithPassengers, error) {
	var t models.CarpoolTripWithPassengers
	err := scanCarpoolTrip(r.pool.QueryRow(ctx, `SELECT `+carpoolTripColumns+` FROM carpool_trips WHERE id::text = $1 AND vehicle_id = $2;`, id, vehicleID), &t)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get carpool trip: %w", err)
	}
	trips := []models.CarpoolTripWithPassengers{t}
	if err := r.loadCarpoolDetails(ctx, trips); err != nil {
		return nil, err
	}
	return &trips[0], nil
}

// loadCarpoolDetails loads legs and passengers of several trips in two queries.
func (r *Repository) loadCarpoolDetails(ctx context.Context, trips []models.CarpoolTripWithPassengers) error {
	if len(trips) == 0 {
		return nil
	}
	ids := make([]string, len(trips))
	index := make(map[string]int, len(trips))
	for i := range trips {
		ids[i] = trips[i].ID
		index[trips[i].ID] = i
		trips[i].Legs = []models.CarpoolLeg{}
		trips[i].Passengers = []models.CarpoolPassenger{}
	}

	legRows, err := r.pool.Query(ctx, `
		SELECT id, carpool_trip_id, order_index, drive_id, start_label, end_label, distance_km,
		       electricity_cost, tolls_cost, tires_cost, maintenance_cost, insurance_cost, other_cost
		FROM carpool_legs
		WHERE carpool_trip_id::text = ANY($1::text[])
		ORDER BY carpool_trip_id, order_index;
	`, ids)
	if err != nil {
		return fmt.Errorf("failed to load carpool legs: %w", err)
	}
	for legRows.Next() {
		var l models.CarpoolLeg
		if err := legRows.Scan(&l.ID, &l.CarpoolTripID, &l.OrderIndex, &l.DriveID, &l.StartLabel, &l.EndLabel, &l.DistanceKm,
			&l.ElectricityCost, &l.TollsCost, &l.TiresCost, &l.MaintenanceCost, &l.InsuranceCost, &l.OtherCost); err != nil {
			legRows.Close()
			return err
		}
		t := &trips[index[l.CarpoolTripID]]
		t.Legs = append(t.Legs, l)
	}
	legRows.Close()
	if err := legRows.Err(); err != nil {
		return err
	}

	pRows, err := r.pool.Query(ctx, `
		SELECT id, carpool_trip_id, passenger_name, origin, destination, seats, amount_paid, notes, created_at,
		       board_stop_index, alight_stop_index
		FROM carpool_passengers
		WHERE carpool_trip_id::text = ANY($1::text[])
		ORDER BY created_at ASC;
	`, ids)
	if err != nil {
		return fmt.Errorf("failed to load carpool passengers: %w", err)
	}
	defer pRows.Close()
	for pRows.Next() {
		var p models.CarpoolPassenger
		if err := pRows.Scan(&p.ID, &p.CarpoolTripID, &p.PassengerName, &p.Origin, &p.Destination,
			&p.Seats, &p.AmountPaid, &p.Notes, &p.CreatedAt, &p.BoardStopIndex, &p.AlightStopIndex); err != nil {
			return err
		}
		t := &trips[index[p.CarpoolTripID]]
		t.Passengers = append(t.Passengers, p)
	}
	return pRows.Err()
}

func (r *Repository) DeleteCarpoolTrip(ctx context.Context, id, vehicleID string) error {
	query := `DELETE FROM carpool_trips WHERE id::text = $1 AND vehicle_id = $2;`
	cmd, err := r.pool.Exec(ctx, query, id, vehicleID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) GetCarpoolSummary(ctx context.Context, vehicleID string) (*models.CarpoolSummary, error) {
	query := `
		SELECT
			COUNT(*) AS total_trips,
			COALESCE(SUM(distance_km), 0) AS total_distance,
			COALESCE(SUM(total_cost), 0) AS total_cost,
			COALESCE(SUM(total_revenue), 0) AS total_revenue,
			COALESCE(SUM(net_cost), 0) AS total_net_cost
		FROM carpool_trips
		WHERE vehicle_id = $1;
	`
	var s models.CarpoolSummary
	err := r.pool.QueryRow(ctx, query, vehicleID).Scan(
		&s.TotalTrips, &s.TotalDistanceKm, &s.TotalRealCost, &s.TotalRevenue, &s.TotalNetCost,
	)
	if err != nil {
		return nil, err
	}

	// Count passengers
	_ = r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM carpool_passengers cp
		JOIN carpool_trips ct ON cp.carpool_trip_id = ct.id
		WHERE ct.vehicle_id = $1;
	`, vehicleID).Scan(&s.TotalPassengers)

	if s.TotalRealCost > 0 {
		rate := math.Round((s.TotalRevenue.Float()/s.TotalRealCost.Float())*1000) / 10
		s.CoverageRatePct = &rate
		s.TotalSaved = s.TotalRevenue
	}
	if s.TotalDistanceKm > 0 {
		s.NetCostPerKm = math.Round((s.TotalNetCost.Float()/s.TotalDistanceKm)*1000) / 1000
	}

	return &s, nil
}
