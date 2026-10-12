package models

// Categories of a charging place.
const (
	PlaceHome   = "HOME"
	PlaceWork   = "WORK"
	PlacePublic = "PUBLIC"
	PlaceOther  = "OTHER"
)

// ValidPlaceCategory reports whether c is one of the place categories.
func ValidPlaceCategory(c string) bool {
	switch c {
	case PlaceHome, PlaceWork, PlacePublic, PlaceOther:
		return true
	}
	return false
}

// ChargingPlace is a place an account charges at; Aliases are the reported texts that resolve to it.
type ChargingPlace struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Aliases  []string `json:"aliases"`
}

// ChargingLocation is a distinct text charges report as their location, with the place it is linked to.
type ChargingLocation struct {
	Text     string  `json:"text"`
	Sessions int     `json:"sessions"`
	KwhAdded float64 `json:"kwh_added"`
	PlaceID  *string `json:"place_id"`
}
