# Ingestion API

Home Assistant, Node-RED, n8n or any script can push events to AutoLedger over HTTP. Français : [ingestion-api.fr.md](ingestion-api.fr.md).

## Authentication

Create an API token in the account page (`al_live_...`). It is account-level and only opens the `/api/integrations/**` routes. Send it as a bearer token:

```
Authorization: Bearer al_live_xxxxxxxx
```

## Endpoint

`POST /api/integrations/homeassistant/event` with a JSON body.

| Field | Type | Notes |
|---|---|---|
| `event_type` | string | `charging_session_end` (default when empty), `odometer`, `drive`, `fuel`. `odometer_update` and `telemetry_update` are older names of `odometer`. |
| `vehicle_id` | string | Required for `odometer`, `drive`, `fuel`. Optional for charging sessions. The account needs edit access to the vehicle. |
| `event_id` | string | Your own identifier for the event, used for idempotence (see below). |
| `timestamp` | RFC 3339 | Used when `data.start_time` / `data.end_time` is absent. |
| `distance_unit` | `km` (default) or `mi` | Unit of the distances and odometer values of the event. Values are stored in km. |
| `data` | object | Fields of the event type, below. |

### `charging_session_end`

`data`: `start_time`, `end_time`, `energy_kwh` (or `energy_added_kwh`; above 0, at most 500), `cost` (currency units) or `cost_cents`, `location`, `charger_name`, `odometer_km`, `soc_start`, `soc_end`.

Without a cost, the vehicle's tariff plan is applied when there is one, otherwise the cost is left to complete. Electric vehicles only.

A session without `vehicle_id` goes to the home charger's default vehicle, otherwise to the only electric vehicle the account can edit, otherwise it waits in the pending charges (a charger shared by several vehicles) until you assign it in the app.

### `odometer`

`data`: `odometer` (in `distance_unit`) or `odometer_km`. The vehicle's odometer only moves forward.

### `drive`

`data`: `start_time` (or the event `timestamp`), `end_time` or `duration_min`, `distance` (in `distance_unit`, 0.1 to 3000 km), `start_odometer`, `end_odometer`, `start_address`, `end_address`, `energy_kwh`.

`distance` is optional. When it is absent, it is the difference of `end_odometer` and `start_odometer` if both are sent. Otherwise the drive is stored without distance (shown as 0), with no energy estimate and no consumption figure; a `distance` sent as 0 is refused. `start_lat`/`start_lon` and `end_lat`/`end_lon` give the positions: they stand in for `start_address` / `end_address` when these are absent (stored as `lat, lon` text).

Without `energy_kwh`, the energy is estimated from the vehicle's average consumption and flagged as estimated.

### `fuel`

Combustion vehicles only. `data`: `amount` (or `cost`), `liters`, `price_per_liter`, `fuel_type`, `is_full_tank`, `odometer` or `odometer_km`. The amount can be derived from litres and price per litre. The time is the event `timestamp` (or `data.start_time`), now when absent.

## Responses

| Status | `status` | Meaning |
|---|---|---|
| 201 | `recorded` | Stored. The body carries `vehicle_id` and the id of the new record. |
| 201 | `pending_qualification` | A charging session waiting for a vehicle (`pending_id`). |
| 200 | `duplicate` | Already recorded or already pending; nothing was written. |
| 200 | (odometer) | Reading applied. |

Errors answer `{"error": "...", "code": "...", "params": {...}}`. Frequent codes:

| Code | Cause |
|---|---|
| `request.invalid_body` | The body is not valid JSON. |
| `integration.unknown_event_type` | Misspelt `event_type`. |
| `integration.invalid_distance_unit` | `distance_unit` is neither `km` nor `mi`. |
| `vehicle.not_specified` | `vehicle_id` missing on an event that needs it. |
| `telemetry.missing_odometer` | No valid odometer reading. |
| `charge.invalid_energy`, `charge.invalid_period`, `charge.electric_only` | Charging session rejected. |
| `drive.missing_start_time`, `drive.invalid_distance` | Drive rejected. |
| `fuel.combustion_only`, `fuel.amount_required` | Fill-up rejected. |

## Idempotence

Sending the same event again is safe. A record is recognised as already stored:

1. by `event_id` (charging sessions and drives), when you send one;
2. otherwise by tolerance: a charge starting within 30 min with an energy within 0.5 kWh, a drive starting within 15 min with a distance within 1 km, a fill-up within 30 min with the same amount (fill-ups are only recognised this way).

Use a stable `event_id` (a session id, a timestamp-based key) whenever the source has one.

## Examples

curl:

```bash
curl -X POST https://autoledger.example.com/api/integrations/homeassistant/event \
  -H "Authorization: Bearer al_live_xxxxxxxx" -H "Content-Type: application/json" \
  -d '{"event_type":"charging_session_end","event_id":"wallbox-2026-05-01-1","vehicle_id":"<id>",
       "data":{"start_time":"2026-05-01T22:00:00Z","end_time":"2026-05-02T05:30:00Z","energy_kwh":38.4,"cost":5.76}}'
```

A drive in miles:

```bash
curl -X POST https://autoledger.example.com/api/integrations/homeassistant/event \
  -H "Authorization: Bearer al_live_xxxxxxxx" -H "Content-Type: application/json" \
  -d '{"event_type":"drive","vehicle_id":"<id>","distance_unit":"mi",
       "data":{"start_time":"2026-05-01T08:00:00Z","duration_min":25,"distance":12.4,"end_odometer":42310}}'
```

Home Assistant (`rest_command`):

```yaml
rest_command:
  autoledger_odometer:
    url: "https://autoledger.example.com/api/integrations/homeassistant/event"
    method: POST
    headers:
      Authorization: "Bearer al_live_xxxxxxxx"
      Content-Type: "application/json"
    payload: >
      {"event_type":"odometer","vehicle_id":"<id>","data":{"odometer_km":{{ states('sensor.car_odometer') | float }}}}
```

Node-RED / n8n: an HTTP Request node, method `POST`, the URL above, the `Authorization` header and the JSON body of the event.
