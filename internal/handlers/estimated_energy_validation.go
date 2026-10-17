package handlers

import (
	"math"
	"strconv"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/money"
)

func validateEstimatedEnergyRate(rate *float64) error {
	if rate == nil {
		return nil
	}
	maximum := money.MaxRate.Float()
	if math.IsNaN(*rate) || math.IsInf(*rate, 0) || *rate < 0 || *rate > maximum {
		return apierror.Newf("vehicle.estimate_rate_range", "The electricity rate must be between 0 and %s per kWh", strconv.FormatFloat(maximum, 'f', -1, 64))
	}
	return nil
}
