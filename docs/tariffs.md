# Tariff plans

A tariff plan prices the energy of a home charging session when no cost is given. Three plan types exist:

| `plan_type` | Meaning |
|---|---|
| `FLAT` | One price per kWh (`flat_rate_cents`). |
| `TIME_OF_USE` | Peak and off-peak prices with off-peak `time_windows`. Stored as `BANDS` since migration 51 and still accepted by the API. |
| `BANDS` | Any number of named bands, rules that assign a band to a time range, and a default band. |

## Bands

```json
{
  "name": "Day plan",
  "plan_type": "BANDS",
  "currency": "EUR",
  "default_band": "white",
  "bands": [
    { "name": "blue",  "rate_cents": 0.10 },
    { "name": "white", "rate_cents": 0.20 },
    { "name": "red",   "rate_cents": 0.50 }
  ],
  "rules": [
    { "days": [1,2,3,4,5], "start": "22:00", "end": "06:00", "band": "blue" },
    { "days": [0,6],       "start": "00:00", "end": "24:00", "band": "blue" },
    { "days": [1,2,3,4,5], "start": "18:00", "end": "20:00", "band": "red" }
  ]
}
```

- `rate_cents` is the price per kWh in currency units, like every amount of the API.
- `days` uses 0 = Sunday to 6 = Saturday; omitted means every day.
- A rule whose end is before its start crosses midnight. The part after midnight belongs to the day on which the rule starts.
- The first matching rule wins; time outside any rule is priced at `default_band`.
- Times are wall-clock times in `APP_TIMEZONE`, so a window keeps its hours across daylight-saving changes.
- A session spanning several bands is priced in proportion to the time spent in each.

## Validity and versions

`valid_from` and `valid_to` (`YYYY-MM-DD`, both optional) bound the days a plan prices. Plans of the same owner sharing a
name (case-insensitive) are versions of one tariff: the version whose range covers the day of the session is used, so a
new price is added as a new plan and never changes costs already stored. A session on a day no version covers is
left to the manual cost entry.

`standing_charge_cents` records an optional monthly standing charge for the plan.
