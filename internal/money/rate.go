package money

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// Rate is a price per unit of energy expressed in millionths of the currency unit, so that
// prices such as 0.2516 or 0.34925 per kWh are kept exactly.
type Rate int64

const (
	rateScale    = 1_000_000
	rateDecimals = 6
	// MaxRate is the largest rate storable in NUMERIC(12, 6).
	MaxRate Rate = 999_999_999_999
)

// Cost prices a quantity at this rate and rounds the result to the cent, half away from zero.
func (r Rate) Cost(quantity float64) Cents {
	return FromFloat(quantity * r.Float())
}

// Float returns the rate in currency units, for ratio computations only.
func (r Rate) Float() float64 {
	return float64(r) / rateScale
}

// String formats the rate with at least two and at most six decimals ("0.20", "0.2516").
func (r Rate) String() string {
	sign := ""
	v := int64(r)
	if v < 0 {
		sign = "-"
		v = -v
	}
	frac := strings.TrimRight(fmt.Sprintf("%0*d", rateDecimals, v%rateScale), "0")
	for len(frac) < 2 {
		frac += "0"
	}
	return fmt.Sprintf("%s%d.%s", sign, v/rateScale, frac)
}

// ParseRate reads a decimal rate ("0.2516", "1e-1") and rounds it to six decimals, half away from zero.
func ParseRate(s string) (Rate, error) {
	r, ok := new(big.Rat).SetString(strings.TrimSpace(s))
	if !ok {
		return 0, fmt.Errorf("invalid rate %q", s)
	}
	return rateFromRat(r)
}

func rateFromRat(r *big.Rat) (Rate, error) {
	v, err := scaleRat(r, rateScale)
	if err != nil || v > int64(MaxRate) || v < -int64(MaxRate) {
		return 0, errors.New("rate out of range")
	}
	return Rate(v), nil
}

// MarshalJSON encodes the rate as a JSON number.
func (r Rate) MarshalJSON() ([]byte, error) {
	return []byte(r.String()), nil
}

// UnmarshalJSON accepts a JSON number or a numeric string; null leaves the value unchanged.
func (r *Rate) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" {
		return nil
	}
	s = strings.Trim(s, `"`)
	if s == "" {
		*r = 0
		return nil
	}
	v, err := ParseRate(s)
	if err != nil {
		return err
	}
	*r = v
	return nil
}

// ScanNumeric implements pgtype.NumericScanner.
func (r *Rate) ScanNumeric(n pgtype.Numeric) error {
	if !n.Valid {
		return errors.New("cannot scan NULL into money.Rate")
	}
	if n.NaN || n.InfinityModifier != pgtype.Finite {
		return errors.New("cannot scan non-finite numeric into money.Rate")
	}
	rat := new(big.Rat).SetInt(n.Int)
	exp := int64(n.Exp)
	if exp < 0 {
		exp = -exp
	}
	pow := new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(exp), nil))
	if n.Exp > 0 {
		rat.Mul(rat, pow)
	} else if n.Exp < 0 {
		rat.Quo(rat, pow)
	}
	v, err := rateFromRat(rat)
	if err != nil {
		return err
	}
	*r = v
	return nil
}

// NumericValue implements pgtype.NumericValuer.
func (r Rate) NumericValue() (pgtype.Numeric, error) {
	return pgtype.Numeric{Int: big.NewInt(int64(r)), Exp: -rateDecimals, Valid: true}, nil
}
