// Package money represents monetary amounts as integer cents, exact end to end:
// JSON numbers and PostgreSQL NUMERIC values are converted without floating point.
package money

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// Cents is an amount expressed in hundredths of the currency unit.
type Cents int64

// Max is the largest amount storable in NUMERIC(10, 2).
const Max Cents = 9_999_999_999

// FromFloat converts an amount computed in floating point (rate × quantity) to cents, rounding half away from zero.
func FromFloat(v float64) Cents {
	return Cents(math.Round(v * 100))
}

// Float returns the amount in currency units, for ratio computations only.
func (c Cents) Float() float64 {
	return float64(c) / 100
}

// String formats the amount with two decimals ("-12.05").
func (c Cents) String() string {
	sign := ""
	v := int64(c)
	if v < 0 {
		sign = "-"
		v = -v
	}
	return fmt.Sprintf("%s%d.%02d", sign, v/100, v%100)
}

// Parse reads a decimal amount ("12.3", "1e2", "-0.005") and rounds it to the cent, half away from zero.
func Parse(s string) (Cents, error) {
	r, ok := new(big.Rat).SetString(strings.TrimSpace(s))
	if !ok {
		return 0, fmt.Errorf("invalid amount %q", s)
	}
	return fromRat(r)
}

func fromRat(r *big.Rat) (Cents, error) {
	v, err := scaleRat(r, 100)
	return Cents(v), err
}

// scaleRat multiplies r by scale and rounds to an integer, half away from zero.
func scaleRat(r *big.Rat, scale int64) (int64, error) {
	scaled := new(big.Rat).Mul(r, big.NewRat(scale, 1))
	num, den := scaled.Num(), scaled.Denom()
	quo, rem := new(big.Int).QuoRem(num, den, new(big.Int))
	// Round half away from zero: |2·rem| >= den
	if new(big.Int).Abs(new(big.Int).Mul(rem, big.NewInt(2))).Cmp(den) >= 0 {
		if num.Sign() < 0 {
			quo.Sub(quo, big.NewInt(1))
		} else {
			quo.Add(quo, big.NewInt(1))
		}
	}
	if !quo.IsInt64() {
		return 0, errors.New("amount out of range")
	}
	return quo.Int64(), nil
}

// MulRate multiplies an amount by a rate (e.g. a currency conversion) and rounds to the cent.
func (c Cents) MulRate(rate float64) Cents {
	return FromFloat(c.Float() * rate)
}

// Split divides an amount into n parts that add up exactly to the total; the remainder goes to the last part.
func Split(total Cents, n int) []Cents {
	parts := make([]Cents, n)
	if n <= 0 {
		return parts
	}
	base := total / Cents(n)
	for i := range parts {
		parts[i] = base
	}
	parts[n-1] += total - base*Cents(n)
	return parts
}

// MarshalJSON encodes the amount as a JSON number with two decimals.
func (c Cents) MarshalJSON() ([]byte, error) {
	return []byte(c.String()), nil
}

// UnmarshalJSON accepts a JSON number or a numeric string; null leaves the value unchanged.
func (c *Cents) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" {
		return nil
	}
	s = strings.Trim(s, `"`)
	if s == "" {
		*c = 0
		return nil
	}
	v, err := Parse(s)
	if err != nil {
		return err
	}
	*c = v
	return nil
}

// ScanNumeric implements pgtype.NumericScanner.
func (c *Cents) ScanNumeric(n pgtype.Numeric) error {
	if !n.Valid {
		return errors.New("cannot scan NULL into money.Cents")
	}
	if n.NaN || n.InfinityModifier != pgtype.Finite {
		return errors.New("cannot scan non-finite numeric into money.Cents")
	}
	r := new(big.Rat).SetInt(n.Int)
	if n.Exp > 0 {
		r.Mul(r, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n.Exp)), nil)))
	} else if n.Exp < 0 {
		r.Quo(r, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-n.Exp)), nil)))
	}
	v, err := fromRat(r)
	if err != nil {
		return err
	}
	*c = v
	return nil
}

// NumericValue implements pgtype.NumericValuer.
func (c Cents) NumericValue() (pgtype.Numeric, error) {
	return pgtype.Numeric{Int: big.NewInt(int64(c)), Exp: -2, Valid: true}, nil
}
