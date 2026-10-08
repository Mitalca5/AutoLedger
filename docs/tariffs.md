# Tariff plans

Français : [tariffs.fr.md](tariffs.fr.md).

A tariff plan prices the energy of a home charging session when no cost is given. Three plan types exist:

| `plan_type` | Meaning |
|---|---|
| `FLAT` | One price per kWh (`flat_rate_cents`). |
| `TIME_OF_USE` | Peak and off-peak prices with off-peak `time_windows`. Stored as `BANDS` since migration 51 and still accepted by the API. |
| `BANDS` | Any number of named bands, rules that assign a band to a time range, and a default band. |

## Editing plans

**Account → Electricity tariff plans** lists the plans grouped by tariff name, newest version first. A plan is created and edited in a form (single price or several prices, time ranges with their days, validity dates, monthly subscription). A row offers edit, **New version** (same prices, starting the day after the previous version ends), duplicate and delete. The vehicle form's plan selector links to this section. The same operations are available through `POST/PUT/DELETE /api/tariffs/plans`; prices are stored to the cent per kWh.

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

## Where the plan prices a session

- Sessions pushed by Home Assistant or a script without a cost, and pending charges assigned to a vehicle, use the plan of the vehicle on the day the session started.
- The charge form (Energy → Charges → Add) prices a session the same way as soon as the start, the optional end and the energy are filled: `POST /api/tariffs/calculate-session` with `vehicle_id`, `start_time`, `end_time` and `kwh` returns the `cost` and the `plan` name (`null` when no plan prices that day). The form sends instants; the server reads them in `APP_TIMEZONE`, so a session crossing midnight is split between the bands it spends time in. A price or a cost typed by hand replaces the calculation.
- The same calculation fills the cost of a synchronised charge that has none, from its recorded start and end.

`standing_charge_cents` records an optional monthly standing charge for the plan.
