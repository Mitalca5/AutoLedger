package services

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services/ingest"
	"github.com/teslacost/teslacost/internal/teslamate"
)

func (s *SyncService) syncDrives(ctx context.Context, client *teslamate.Client, v *models.Vehicle, carID int) resourceSyncStats {
	var st resourceSyncStats
	if s.repo == nil {
		return st
	}
	stopBefore := s.incrementalStopBefore(ctx, v.ID, "drives", s.repo.GetLatestTeslaMateDriveStartTime)

	completed := false
	for page := 1; page <= syncMaxPages; page++ {
		driveList, driveUnits, err := client.GetDrives(ctx, carID, teslamate.DriveFilterOptions{
			Page: page,
			Show: syncPageSize,
		})
		if err != nil {
			formattedErr := formatTeslaMateError(err, *v.TeslaMateAPIURL)
			slog.Warn("could not fetch drives", "component", "sync", "vehicle_id", v.ID, "page", page, "error", formattedErr)
			st.warnings = append(st.warnings, apierror.NewMessagef("sync.page_failed", "%s (page %d): %v - the import resumes at the next synchronization", "kw:drives", page, formattedErr))
			break
		}
		if len(driveList) == 0 {
			completed = true
			break
		}

		reachedKnownHistory := false
		for _, td := range driveList {
			startTime, err := td.ParsedStartTime()
			if err != nil || startTime.IsZero() {
				st.recordUpsert(false, fmt.Errorf("invalid start date (%q)", td.StartDate), "kw:drive", td.DriveID)
				continue
			}
			endTime, _ := td.ParsedEndTime()
			if endTime.IsZero() {
				endTime = startTime.Add(time.Duration(td.DurationMin) * time.Minute)
			}

			if stopBefore != nil && startTime.Before(*stopBefore) {
				reachedKnownHistory = true
			}
			st.see(td.DriveID, startTime)

			isInserted, err := s.repo.UpsertTeslaMateDrive(ctx, buildDrive(v.ID, td, driveUnits, startTime, endTime))
			st.recordUpsert(isInserted, err, "kw:drive", td.DriveID)
		}

		if len(driveList) < syncPageSize || reachedKnownHistory {
			completed = true
			break
		}
		if page == syncMaxPages {
			st.warnings = append(st.warnings, apierror.NewMessagef("sync.page_limit", "%s: limit of %d pages reached, history partially imported", "kw:drives", syncMaxPages))
		}
	}

	s.finishResourceSync(ctx, &st, v.ID, "drives", completed, stopBefore == nil)
	return st
}

func buildDrive(vehicleID string, td teslamate.Drive, units *teslamate.Units, startTime, endTime time.Time) *models.Drive {
	distKm := td.OdometerDetails.OdometerDistance
	startOdo := td.OdometerDetails.OdometerStart
	endOdo := td.OdometerDetails.OdometerEnd
	if units != nil {
		distKm = teslamate.ConvertDistanceToKm(distKm, units.UnitOfLength)
		startOdo = teslamate.ConvertDistanceToKm(startOdo, units.UnitOfLength)
		endOdo = teslamate.ConvertDistanceToKm(endOdo, units.UnitOfLength)
	}

	var speedAvg *float64
	if td.SpeedAvg > 0 {
		speedAvg = &td.SpeedAvg
	}

	var startAddr, endAddr *string
	if td.StartAddress != "" {
		startAddr = &td.StartAddress
	}
	if td.EndAddress != "" {
		endAddr = &td.EndAddress
	}

	var speedMax, powerMax, powerMin *int
	if td.SpeedMax > 0 {
		speedMax = &td.SpeedMax
	}
	if td.PowerMax != 0 {
		powerMax = &td.PowerMax
	}
	if td.PowerMin != 0 {
		powerMin = &td.PowerMin
	}

	startLevel, endLevel := batteryLevels(td.BatteryDetails)

	// TeslaMate reports its own "consumption_net" in Wh per distance unit; the stored figure is kWh/100 km,
	// derived here from the energy and the distance in km.
	var consumption *float64
	if td.EnergyConsumedNet != nil && *td.EnergyConsumedNet > 0 && distKm > 0 {
		c := ingest.Consumption100km(*td.EnergyConsumedNet, distKm)
		consumption = &c
	}

	tmDriveID := td.DriveID
	return &models.Drive{
		VehicleID:           vehicleID,
		TeslaMateDriveID:    &tmDriveID,
		StartTime:           startTime,
		EndTime:             endTime,
		StartOdometer:       &startOdo,
		EndOdometer:         &endOdo,
		DistanceKm:          distKm,
		DurationMin:         td.DurationMin,
		SpeedAvg:            speedAvg,
		SpeedMax:            speedMax,
		PowerMax:            powerMax,
		PowerMin:            powerMin,
		StartAddress:        startAddr,
		EndAddress:          endAddr,
		EnergyConsumedKwh:   td.EnergyConsumedNet,
		ConsumptionKwh100km: consumption,
		StartBatteryLevel:   startLevel,
		EndBatteryLevel:     endLevel,
		OutsideTempC:        tempCelsius(td.OutsideTempAvg, units),
		Tags:                []string{},
	}
}

// batteryLevels returns the state of charge at both ends of a drive or charge; a missing end level (reported as
// 0) means the reading is unknown.
func batteryLevels(b teslamate.BatteryDetails) (start, end *int) {
	if b.EndBatteryLevel <= 0 {
		return nil, nil
	}
	s, e := b.StartBatteryLevel, b.EndBatteryLevel
	return &s, &e
}

// tempCelsius converts the average outside temperature reported in the TeslaMate unit, to one decimal.
func tempCelsius(v *float64, units *teslamate.Units) *float64 {
	if v == nil {
		return nil
	}
	c := *v
	if units != nil {
		c = teslamate.ConvertTemperatureToC(c, units.UnitOfTemperature)
	}
	c = math.Round(c*10) / 10
	return &c
}
