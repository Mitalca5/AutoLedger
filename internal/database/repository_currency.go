package database

import (
	"fmt"
)

// amountInVehicleCurrencyExpr converts an expense amount to its vehicle's own currency (NULL when a
// foreign amount has no conversion rate). vehicleIDExpr is a trusted SQL fragment — a column
// reference or bound parameter already scoped to the right vehicle in the surrounding query, never
// user input — naming the vehicle to compare currencies against.
func amountInVehicleCurrencyExpr(vehicleIDExpr string) string {
	return fmt.Sprintf(`(CASE WHEN currency = (SELECT currency FROM vehicles WHERE id = %s) THEN amount ELSE amount * fx_rate END)`, vehicleIDExpr)
}
