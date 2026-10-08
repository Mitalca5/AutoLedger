# CSV import and export

AutoLedger imports four kinds of files: charges, drives, fill-ups and odometer readings. Français : [csv-import.fr.md](csv-import.fr.md).

The same columns are used by **Export** (manual tracking page, or `GET /api/vehicles/{id}/export?type=...&format=csv|json&from=YYYY-MM-DD&to=YYYY-MM-DD`): an exported file can be imported again, into another vehicle or another instance. The export also covers expenses and maintenance, which have no import.

## File format

- Delimiter: comma, semicolon or tab, detected from the file. UTF-8, with or without a BOM.
- First line: headers. Case, accents and surrounding spaces are ignored, and the aliases below are accepted.
- Dates: `2026-01-15`, `2026-01-15 18:30`, `2026-01-15T18:30:00Z` (a date without an offset is read in the instance timezone, `APP_TIMEZONE`). Dates with `/` or `.` are read day first by default (`15/01/2026`); the import dialog lets you choose month first or year first.
- Numbers: `.` or `,` as decimal separator, detected. When a file mixes thousands separators, choose the separator in the import dialog.
- Distances: columns suffixed `_km` or `_mi` are explicit. Without a suffix, the column is read in the distance unit of the account.
- A single invalid row cancels the whole file: nothing is written until every row is valid. Rows already present are skipped when "skip duplicates" is on.
- The type (charges, drives, fuel, odometer) is detected from the headers and can be chosen manually.

## Charges

| Column | Aliases | Required | Notes |
| --- | --- | --- | --- |
| `date` | `time`, `start_time`, `datetime` | yes | |
| `kwh` | `kwh_added`, `energy`, `energy_kwh` | yes | 0 to 1000 |
| `cost` | `amount`, `price`, `total_cost` | yes | |
| `currency` | `curr` | no | ISO code; defaults to the vehicle currency |
| `fx_rate` | `fx`, `rate` | if the currency differs | conversion to the vehicle currency |
| `odometer_km` / `odometer_mi` | `odometer`, `odo` | no | |
| `location` | `address`, `station`, `place` | no | |

## Drives

| Column | Aliases | Required | Notes |
| --- | --- | --- | --- |
| `start_time` | `date`, `start`, `datetime` | yes | |
| `distance_km` / `distance_mi` | `distance`, `dist` | yes | 0 to 3000 km |
| `end_time` | `end` | no | otherwise derived from the distance |
| `kwh` | `energy`, `energy_consumed_kwh` | no | |
| `start_address` | `start_location`, `origin` | no | |
| `end_address` | `end_location`, `destination` | no | |
| `tag` | `tags`, `purpose` | no | several tags separated by `;` |

## Fill-ups

| Column | Aliases | Required | Notes |
| --- | --- | --- | --- |
| `date` | `time`, `datetime` | yes | |
| `liters` | `litres`, `volume`, `quantity` | with `price_per_liter`, or use `amount` | 0 to 500 |
| `price_per_liter` | `price_per_litre`, `unit_price`, `price` | | 0 to 10 |
| `amount` | `cost`, `total`, `total_cost` | or use litres and price | |
| `fuel_type` | `fuel` | no | `SP95_E10`, `SP98`, `DIESEL`, `E85`, `GPL` |
| `is_full_tank` | `full_tank`, `full` | no | defaults to yes |
| `odometer_km` / `odometer_mi` | `odometer`, `odo` | no | |
| `notes` | `note`, `comment` | no | |

## Odometer readings

| Column | Aliases | Required |
| --- | --- | --- |
| `date` | `time`, `datetime` | yes |
| `odometer_km` / `odometer_mi` | `odometer`, `odo` | yes |
| `notes` | `note`, `comment` | no |

## Mileage by tag and allowance scales

`type=mileage` (or `GET /api/vehicles/{id}/mileage-report?from=&to=&tag=&rates=`) totals the trips of the period per tag: number of trips, distance, tolls and, when a scale is chosen, the mileage allowance. A trip carrying several tags counts in each of them; untagged trips have an empty tag.

An allowance scale is data you type, never assumed by the application. Each slice (`POST /api/mileage-rates/`) has a name, a year, a first and last kilometre (the last one empty for no limit) and a rate per kilometre. Slices sharing a name form a scale, and they price the **cumulative distance of the year**: kilometres driven before the period start still count to find the slice, so a September report continues the January total. The scale is chosen with `rates=<name>`; without it no allowance is computed.

## Column mapping and saved profiles

When a header is not recognised, the import dialog shows every column of the file with the field it was matched to and lets you pick another one, or ignore the column. The mapping, the date order and the decimal separator can be saved as a **profile** under a name; selecting it later applies them to any file with the same layout, on any vehicle of the account.

## Recipe: OBD2 logger export (Car Scanner, Torque, ...)

The core has no parser for a specific app. A trip or fill-up log exported by an OBD2 application is imported through a profile, once.

1. Export the log from the app as CSV (Car Scanner: *Trips* or *Fuel* log, Torque: *Trip log*).
2. Open **Odometer → Import a CSV** (or **Energy → Fill-ups → Import CSV**), choose the file and the type (`Drives` for trips, `Fill-ups` for fuel logs).
3. In the mapping step, match the app columns to the fields. Typical matches:

   | App column (example) | AutoLedger field |
   | --- | --- |
   | `Start time`, `Trip start` | `start_time` |
   | `End time`, `Trip end` | `end_time` |
   | `Distance (km)`, `Trip distance` | `distance_km` |
   | `Fuel used (l)`, `Volume` | `liters` |
   | `Cost`, `Total price` | `amount` |
   | anything else (speed, RPM, temperatures) | ignore |

4. Set the date order and the decimal separator that the app uses (many apps on a French phone write `15/01/2026` and `12,5`).
5. Save the mapping as a profile (for example "Car Scanner trips"). Next exports from the same app need only the profile to be selected.

Column names vary between app versions and languages: the preview lists the unmatched columns and the rows it rejects with their line number, so the mapping can be adjusted before anything is written. An imported batch can be undone from the same dialog.
