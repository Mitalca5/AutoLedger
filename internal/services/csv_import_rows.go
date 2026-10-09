package services

import (
	"context"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/internal/services/ingest"
)

const (
	kmPerMile     = 1.609344
	maxOdometerKm = 2_000_000
	maxFxRate     = 999_999
	maxFuelLiters = 500
	maxFuelPrice  = 10
)

var currencyCode = regexp.MustCompile(`^[A-Z]{3}$`)

type parsedRow struct {
	insert      func(ctx context.Context, tx *database.Repository, batchID string) error
	isDuplicate func(ctx context.Context, repo *database.Repository) (bool, error)
}

type rowParser func(rc *rowContext, row []string, line int) (*parsedRow, *apierror.Error)

// validImportAmount checks the source value before converting it to integer cents.
// Non-finite or overflowing floats must not wrap into a negative amount during conversion.
func validImportAmount(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= money.Max.Float()
}

var rowParsers = map[ImportType]rowParser{
	ImportTypeCharges:  parseChargeRow,
	ImportTypeDrives:   parseDriveRow,
	ImportTypeFuel:     parseFuelRow,
	ImportTypeOdometer: parseOdometerRow,
}

type rowContext struct {
	columns map[string]int
	vehicle *models.Vehicle
	loc     *time.Location
	unit    string
	// decimal and dateOrder override the detection of numbers and slash dates; empty means detect.
	decimal   string
	dateOrder string
}

func (rc *rowContext) float(raw string) (float64, error) { return parseFloatWith(raw, rc.decimal) }

func (rc *rowContext) time(raw string) (time.Time, error) {
	return parseTimeWith(raw, rc.loc, rc.dateOrder)
}

// col returns the first non-empty cell among the named columns.
func (rc *rowContext) col(row []string, names ...string) string {
	for _, n := range names {
		if idx, ok := rc.columns[n]; ok && idx < len(row) {
			if v := strings.TrimSpace(row[idx]); v != "" {
				return v
			}
		}
	}
	return ""
}

// distance reads a distance in km from the first filled column among base_km, base_mi and base (read in the
// account's unit) for each base name, then from a bare km / mi column. found is false when no cell is filled.
func (rc *rowContext) distance(row []string, bases ...string) (km float64, found bool, err error) {
	type candidate struct {
		name string
		mi   bool
	}
	var candidates []candidate
	for _, b := range bases {
		candidates = append(candidates, candidate{b + "_km", false}, candidate{b + "_mi", true}, candidate{b, rc.unit == "mi"})
	}
	candidates = append(candidates, candidate{"km", false}, candidate{"mi", true})

	for _, c := range candidates {
		raw := rc.col(row, c.name)
		if raw == "" {
			continue
		}
		v, err := rc.float(raw)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, true, err
		}
		if c.mi {
			v *= kmPerMile
		}
		return v, true, nil
	}
	return 0, false, nil
}

func (rc *rowContext) date(row []string, names ...string) (time.Time, bool) {
	raw := rc.col(row, names...)
	t, err := rc.time(raw)
	return t, err == nil
}

func optionalText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// odometerCell reads the optional odometer column: absent is nil, an out-of-range reading is an error.
func (rc *rowContext) odometerCell(row []string, line int) (*float64, *apierror.Error) {
	km, found, err := rc.distance(row, "odometer", "odo")
	if !found {
		return nil, nil
	}
	if err != nil || km < 0 || km > maxOdometerKm {
		return nil, apierror.Newf("import.row.invalid_odometer", "Line %d: invalid odometer reading", line)
	}
	if km == 0 {
		return nil, nil
	}
	return &km, nil
}

func parseChargeRow(rc *rowContext, row []string, line int) (*parsedRow, *apierror.Error) {
	date, ok := rc.date(row, "date", "time", "start_time", "datetime")
	if !ok {
		return nil, apierror.Newf("import.row.invalid_date", "Line %d: invalid date %q", line, rc.col(row, "date", "time", "start_time", "datetime"))
	}

	kwhRaw := rc.col(row, "kwh", "kwh_added", "energy", "energy_kwh")
	kwh, err := rc.float(kwhRaw)
	if err != nil || !ingest.ValidChargeEnergy(kwh) {
		return nil, apierror.Newf("import.row.invalid_kwh", "Line %d: invalid energy %q (0 to %d kWh)", line, kwhRaw, ingest.MaxChargeKwh)
	}

	costRaw := rc.col(row, "cost", "amount", "price", "total_cost")
	if costRaw == "" {
		return nil, apierror.Newf("import.row.cost_required", "Line %d: the cost of a charge is required", line)
	}
	costValue, err := rc.float(costRaw)
	if err != nil || !validImportAmount(costValue) {
		return nil, apierror.Newf("import.row.invalid_cost", "Line %d: invalid cost %q", line, costRaw)
	}
	cost := money.FromFloat(costValue)

	currency := strings.ToUpper(rc.col(row, "currency", "curr"))
	if currency == "" {
		currency = rc.vehicle.Currency
	}
	if !currencyCode.MatchString(currency) {
		return nil, apierror.Newf("import.row.invalid_currency", "Line %d: invalid currency %q", line, currency)
	}
	var fxRate *float64
	if currency != rc.vehicle.Currency {
		fx, err := rc.float(rc.col(row, "fx_rate", "fx", "rate"))
		if err != nil || fx <= 0 || fx > maxFxRate || math.IsNaN(fx) {
			return nil, apierror.Newf("import.row.fx_required", "Line %d: a conversion rate to %s is required for a charge in %s", line, rc.vehicle.Currency, currency)
		}
		fxRate = &fx
	}

	odometer, oerr := rc.odometerCell(row, line)
	if oerr != nil {
		return nil, oerr
	}

	charge := &models.ChargeLog{
		VehicleID: rc.vehicle.ID,
		Origin:    "CSV",
		Date:      date,
		KwhAdded:  kwh,
		Cost:      &cost,
		Currency:  currency,
		FxRate:    fxRate,
		Odometer:  odometer,
		Address:   optionalText(rc.col(row, "location", "address", "station", "place")),
	}
	return &parsedRow{
		insert: func(ctx context.Context, tx *database.Repository, batchID string) error {
			charge.SourceBatchID = &batchID
			return tx.CreateManualCharge(ctx, charge)
		},
		isDuplicate: func(ctx context.Context, repo *database.Repository) (bool, error) {
			return repo.HasDuplicateCharge(ctx, charge.VehicleID, date, kwh)
		},
	}, nil
}

func parseDriveRow(rc *rowContext, row []string, line int) (*parsedRow, *apierror.Error) {
	startTime, ok := rc.date(row, "start_time", "date", "start", "datetime")
	if !ok {
		return nil, apierror.Newf("import.row.invalid_date", "Line %d: invalid date %q", line, rc.col(row, "start_time", "date", "start", "datetime"))
	}

	dist, found, err := rc.distance(row, "distance", "dist")
	if !found || err != nil || !ingest.ValidDriveDistance(dist) {
		return nil, apierror.Newf("import.row.invalid_distance", "Line %d: invalid distance (0 to %d km)", line, ingest.MaxDriveKm)
	}

	var endTime *time.Time
	if endStr := rc.col(row, "end_time", "end"); endStr != "" {
		if et, err := rc.time(endStr); err == nil {
			endTime = &et
		}
	}
	var typedEnergy *float64
	if kwhStr := rc.col(row, "kwh", "energy", "energy_consumed_kwh"); kwhStr != "" {
		if parsed, err := rc.float(kwhStr); err == nil {
			typedEnergy = &parsed
		}
	}
	timings := ingest.NormalizeDrive(ingest.DriveInput{
		Start:           startTime,
		End:             endTime,
		DistanceKm:      dist,
		EnergyKwh:       typedEnergy,
		VehicleKwh100km: rc.vehicle.EstimatedKwh100km,
	})

	tags := []string{}
	if tag := rc.col(row, "tag", "tags", "purpose"); tag != "" {
		tags = append(tags, tag)
	}

	drive := &models.Drive{
		VehicleID:           rc.vehicle.ID,
		StartTime:           startTime,
		EndTime:             timings.End,
		DistanceKm:          dist,
		DurationMin:         timings.DurationMin,
		StartAddress:        optionalText(rc.col(row, "start_address", "start_location", "origin")),
		EndAddress:          optionalText(rc.col(row, "end_address", "end_location", "destination")),
		EnergyConsumedKwh:   &timings.EnergyKwh,
		ConsumptionKwh100km: &timings.Kwh100km,
		Tags:                tags,
		EnergyEstimated:     timings.EnergyEstimated,
		Origin:              "CSV",
	}
	return &parsedRow{
		insert: func(ctx context.Context, tx *database.Repository, batchID string) error {
			drive.SourceBatchID = &batchID
			return tx.CreateManualDrive(ctx, drive)
		},
		isDuplicate: func(ctx context.Context, repo *database.Repository) (bool, error) {
			return repo.HasDuplicateDrive(ctx, drive.VehicleID, startTime, dist)
		},
	}, nil
}

func (rc *rowContext) optionalPositive(raw string, max float64) (value *float64, valid bool) {
	if raw == "" {
		return nil, true
	}
	v, err := rc.float(raw)
	if err != nil || math.IsNaN(v) || v < 0 || v > max {
		return nil, false
	}
	if v == 0 {
		return nil, true
	}
	return &v, true
}

func parseFlag(raw string, defaultValue bool) bool {
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "oui", "y", "o", "full", "plein":
		return true
	case "0", "false", "no", "non", "n", "partial", "partiel":
		return false
	}
	return defaultValue
}

func parseFuelRow(rc *rowContext, row []string, line int) (*parsedRow, *apierror.Error) {
	date, ok := rc.date(row, "date", "time", "datetime")
	if !ok {
		return nil, apierror.Newf("import.row.invalid_date", "Line %d: invalid date %q", line, rc.col(row, "date", "time", "datetime"))
	}

	liters, ok := rc.optionalPositive(rc.col(row, "liters", "litres", "volume", "quantity"), maxFuelLiters)
	if !ok {
		return nil, apierror.Newf("import.row.invalid_liters", "Line %d: invalid quantity (0 to %d L)", line, maxFuelLiters)
	}
	price, ok := rc.optionalPositive(rc.col(row, "price_per_liter", "price_per_litre", "unit_price", "price"), maxFuelPrice)
	if !ok {
		return nil, apierror.Newf("import.row.invalid_price", "Line %d: invalid price per litre (0 to %d)", line, maxFuelPrice)
	}

	var amount money.Cents
	if raw := rc.col(row, "amount", "cost", "total", "total_cost"); raw != "" {
		v, err := rc.float(raw)
		if err != nil || !validImportAmount(v) {
			return nil, apierror.Newf("import.row.invalid_amount", "Line %d: invalid amount %q", line, raw)
		}
		amount = money.FromFloat(v)
	}
	switch {
	case amount == 0 && liters != nil && price != nil:
		amount = money.FromFloat(*liters * *price)
	case amount > 0 && liters != nil && price == nil:
		p := math.Round(amount.Float() / *liters * 1000) / 1000
		price = &p
	case amount > 0 && liters == nil && price != nil:
		l := math.Round(amount.Float() / *price * 100) / 100
		liters = &l
	}
	if amount <= 0 {
		return nil, apierror.Newf("import.row.amount_required", "Line %d: the amount of a fill-up is required (or litres and price per litre)", line)
	}

	var fuelType *string
	if raw := strings.ToUpper(rc.col(row, "fuel_type", "fuel")); raw != "" {
		if !models.FuelTypes[raw] {
			return nil, apierror.Newf("import.row.invalid_fuel_type", "Line %d: invalid fuel %q", line, raw)
		}
		fuelType = &raw
	}

	odometer, oerr := rc.odometerCell(row, line)
	if oerr != nil {
		return nil, oerr
	}

	fuel := &models.FuelLog{
		VehicleID:     rc.vehicle.ID,
		Date:          date,
		Odometer:      odometer,
		Amount:        amount,
		Liters:        liters,
		PricePerLiter: price,
		FuelType:      fuelType,
		IsFullTank:    parseFlag(rc.col(row, "is_full_tank", "full_tank", "full"), true),
		Notes:         optionalText(rc.col(row, "notes", "note", "comment")),
	}
	return &parsedRow{
		insert: func(ctx context.Context, tx *database.Repository, batchID string) error {
			fuel.SourceBatchID = &batchID
			return tx.CreateFuelLog(ctx, fuel)
		},
		isDuplicate: func(ctx context.Context, repo *database.Repository) (bool, error) {
			return repo.HasDuplicateFuelLog(ctx, fuel.VehicleID, date, amount)
		},
	}, nil
}

func parseOdometerRow(rc *rowContext, row []string, line int) (*parsedRow, *apierror.Error) {
	date, ok := rc.date(row, "date", "time", "datetime")
	if !ok {
		return nil, apierror.Newf("import.row.invalid_date", "Line %d: invalid date %q", line, rc.col(row, "date", "time", "datetime"))
	}
	odometer, oerr := rc.odometerCell(row, line)
	if oerr != nil {
		return nil, oerr
	}
	if odometer == nil {
		return nil, apierror.Newf("import.row.invalid_odometer", "Line %d: invalid odometer reading", line)
	}

	checkpoint := &models.OdometerCheckpoint{
		VehicleID: rc.vehicle.ID,
		Date:      date,
		Odometer:  *odometer,
		Notes:     optionalText(rc.col(row, "notes", "note", "comment")),
	}
	return &parsedRow{
		insert: func(ctx context.Context, tx *database.Repository, batchID string) error {
			checkpoint.SourceBatchID = &batchID
			return tx.CreateOdometerCheckpoint(ctx, checkpoint)
		},
		isDuplicate: func(ctx context.Context, repo *database.Repository) (bool, error) {
			return repo.HasDuplicateOdometerCheckpoint(ctx, checkpoint.VehicleID, date, *odometer)
		},
	}, nil
}
