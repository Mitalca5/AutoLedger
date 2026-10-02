package services

import "strings"

var accentFolds = strings.NewReplacer(
	"à", "a", "â", "a", "ä", "a", "é", "e", "è", "e", "ê", "e", "ë", "e",
	"î", "i", "ï", "i", "ô", "o", "ö", "o", "ù", "u", "û", "u", "ü", "u", "ç", "c", "œ", "oe",
)

// headerAliases maps the spellings found in French files and in the app's own exports to the column names the
// row parsers read. Keys are already folded (lower case, no accent, words joined by "_", unit suffix removed).
var headerAliases = map[string]string{
	"jour": "date", "date_heure": "datetime", "horodatage": "datetime",
	"debut": "start_time", "fin": "end_time",
	"depart": "start_address", "arrivee": "end_address", "destination": "end_address",
	"duree": "duration", "duree_min": "duration_min",
	"energie": "kwh", "energie_kwh": "kwh", "energy_kwh": "kwh", "kwh_ajoutes": "kwh", "kwh_charges": "kwh",
	"cout": "cost", "montant": "amount", "prix": "price", "prix_total": "total_cost", "cout_total": "total_cost",
	"devise": "currency", "taux_de_change": "fx_rate", "taux": "fx_rate",
	"lieu": "location", "adresse": "address", "borne": "station",
	"kilometrage": "odometer", "compteur": "odometer", "odometre": "odometer",
	"etiquette": "tag", "etiquettes": "tags",
	"litre": "liters", "volume_litres": "liters", "prix_par_litre": "price_per_liter", "prix_litre": "price_per_liter",
	"carburant": "fuel_type", "plein_complet": "is_full_tank",
	"commentaire": "notes", "commentaires": "notes", "remarque": "notes",
}

// unitSuffixes are the distance units a header may end with, kept when the base name is aliased.
var unitSuffixes = []string{"_km", "_mi"}

// foldHeader lower-cases a header, drops accents and symbols, and joins its words with "_":
// "Distance (km)" and " Odomètre-MI " become "distance_km" and "odometre_mi".
func foldHeader(h string) string {
	h = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\uFEFF")))
	h = accentFolds.Replace(h)
	var b strings.Builder
	pendingSep := false
	for _, r := range h {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if pendingSep && b.Len() > 0 {
				b.WriteByte('_')
			}
			pendingSep = false
			b.WriteRune(r)
		} else {
			pendingSep = true
		}
	}
	return b.String()
}

func normalizeHeader(h string) string {
	folded := foldHeader(h)
	if alias, ok := headerAliases[folded]; ok {
		return alias
	}
	for _, suffix := range unitSuffixes {
		if base, found := strings.CutSuffix(folded, suffix); found {
			if alias, ok := headerAliases[base]; ok {
				return alias + suffix
			}
		}
	}
	return folded
}

// canonicalHeaders returns the parser name of each header. In the app's own drive export "Start" and "End" are
// addresses next to a "Date" column, whereas alone they are the start and end times.
func canonicalHeaders(headers []string) []string {
	out := make([]string, len(headers))
	hasDate := false
	for i, h := range headers {
		out[i] = normalizeHeader(h)
		if out[i] == "date" || out[i] == "start_time" {
			hasDate = true
		}
	}
	if hasDate {
		for i, name := range out {
			switch name {
			case "start":
				out[i] = "start_address"
			case "end":
				out[i] = "end_address"
			}
		}
	}
	return out
}
