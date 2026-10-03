# Wallbox recipes

AutoLedger has no connector for a given wallbox brand. Any charger that Home Assistant can read (Easee, Wallbox, Zaptec, an OCPP box, a smart plug with an energy meter...) feeds it the same way: when a session ends, Home Assistant posts a `charging_session_end` event to the [ingestion API](ingestion-api.md). Français : [wallbox-recipes.fr.md](wallbox-recipes.fr.md).

The [HACS integration](https://github.com/Rem7474/autoledger-homeassistant) brings AutoLedger data into Home Assistant; the recipes below go the other way, from the charger to the ledger, with the token and endpoint of the ingestion API.

## What the event needs

| Event field | Comes from the charger as |
|---|---|
| `data.energy_kwh` | The energy of the finished session, in kWh. |
| `data.start_time`, `data.end_time` | When the session started and ended (RFC 3339). |
| `event_id` | A key that is the same every time the same session is sent: charger name + start time. |
| `vehicle_id` | Optional. Without it, the session goes to the home charger's default vehicle, then to the only electric vehicle of the account, then waits in the pending charges. |
| `data.cost` | Optional. Without it, the vehicle's [tariff plan](tariffs.md) prices the session. |

## Mapping the sensors

Entity names differ by integration and by installation: open *Developer tools > States* and look for your charger. Three things are needed, whatever the brand:

| Need | Typical entity | Notes |
|---|---|---|
| Charger status | `sensor.<charger>_status` | Its states include one for "charging" and others for "completed", "disconnected" or "awaiting start". Use the exact strings you see. |
| Energy of the session | `sensor.<charger>_session_energy` | kWh, and often reset when the car is unplugged or a new session begins. Read it when the status leaves "charging", before the reset. |
| Session start | none | Note the time when the status becomes "charging" in an `input_text` helper. |

If the box only reports a lifetime energy counter, store the counter value at the start (a second `input_number` helper) and send the difference.

## Recipe

Helper (Settings > Devices & services > Helpers): a text helper `input_text.wallbox_session_start` (maximum length 64) holding the start of the session in ISO 8601.

```yaml
rest_command:
  autoledger_charging_session:
    url: "https://autoledger.example.com/api/integrations/homeassistant/event"
    method: POST
    headers:
      Authorization: "Bearer al_live_xxxxxxxx"
      Content-Type: "application/json"
    payload: >
      {"event_type":"charging_session_end",
       "event_id":"{{ event_id }}",
       "vehicle_id":"{{ vehicle_id }}",
       "data":{"start_time":"{{ start_time }}","end_time":"{{ end_time }}",
               "energy_kwh":{{ energy_kwh }},"charger_name":"Home wallbox"}}

automation:
  - alias: "Wallbox: remember the session start"
    trigger:
      - platform: state
        entity_id: sensor.wallbox_status
        to: "charging"
    condition:
      - condition: template
        value_template: "{{ trigger.from_state.state not in ['charging', 'unavailable', 'unknown'] }}"
    action:
      - service: input_text.set_value
        target: { entity_id: input_text.wallbox_session_start }
        data: { value: "{{ utcnow().isoformat() }}" }

  - alias: "Wallbox: send the finished session to AutoLedger"
    trigger:
      - platform: state
        entity_id: sensor.wallbox_status
        from: "charging"
    condition:
      - condition: template
        value_template: "{{ trigger.to_state.state not in ['unavailable', 'unknown'] and states('sensor.wallbox_session_energy') | float(0) > 0 }}"
    action:
      - service: rest_command.autoledger_charging_session
        data:
          vehicle_id: "<vehicle id, optional: delete the line to let AutoLedger choose>"
          event_id: "wallbox-{{ as_timestamp(states('input_text.wallbox_session_start')) | int }}"
          start_time: "{{ states('input_text.wallbox_session_start') }}"
          end_time: "{{ utcnow().isoformat() }}"
          energy_kwh: "{{ states('sensor.wallbox_session_energy') | float(0) }}"
```

Adapt `sensor.wallbox_status`, `sensor.wallbox_session_energy` and the "charging" state to your entities.

## Idempotence

`event_id` is derived from the session start, so Home Assistant restarting in the middle of the automation, a retry or a double trigger never creates a second charge: AutoLedger answers `duplicate` and writes nothing. Add the charger name to the key when several chargers feed the same account.

A status that flaps between "charging" and a waiting state (a charge paused by a load balancer) sends one event per resumed stretch; each has its own start time, so each is recorded. To send a single event per session, trigger on a status that only appears when the cable is unplugged.

## Cost

- With a [tariff plan](tariffs.md) on the vehicle (flat, peak / off-peak or any number of bands), leave `cost` out: the session is priced from its start and end times.
- With a dynamic tariff or a per-session price from the charger, send `data.cost` in currency units.
- The tariff plan also applies to sessions that wait in the pending charges when you assign them.
