package services

import (
	"context"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
)

func TestTireConditionHonoursConfiguredMinimum(t *testing.T) {
	for _, c := range []struct {
		minimum, depth float64
		want           string
	}{{4, 3, "CRITICAL"}, {4, 4, "CRITICAL"}, {4, 4.1, "WARNING"}, {4, 5.5, "WARNING"}, {4, 5.6, "GOOD"}, {5, 5.1, "WARNING"}, {5, 6.6, "GOOD"}, {1.6, 2.5, "CRITICAL"}, {1.6, 3, "WARNING"}} {
		tire := wearTire(models.TirePosStorage)
		tire.MinLegalDepthMm = c.minimum
		store := &fakeTireWearStore{logs: []models.TireLog{{DepthMm: c.depth}}}
		got, err := NewTireWearService(store).CalculateTireWear(context.Background(), tire, 0)
		if err != nil {
			t.Fatal(err)
		}
		if got.Condition != c.want {
			t.Fatalf("min=%v depth=%v: %s, want %s", c.minimum, c.depth, got.Condition, c.want)
		}
	}
}
