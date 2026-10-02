# API d'ingestion

Home Assistant, Node-RED, n8n ou un script peuvent envoyer des événements à AutoLedger en HTTP. English: [ingestion-api.md](ingestion-api.md).

## Authentification

Créez un jeton d'API dans la page du compte (`al_live_...`). Il est lié au compte et n'ouvre que les routes `/api/integrations/**`. Envoyez-le en jeton Bearer :

```
Authorization: Bearer al_live_xxxxxxxx
```

## Point d'entrée

`POST /api/integrations/homeassistant/event` avec un corps JSON.

| Champ | Type | Remarques |
|---|---|---|
| `event_type` | chaîne | `charging_session_end` (par défaut si vide), `odometer`, `drive`, `fuel`. `odometer_update` et `telemetry_update` sont d'anciens noms de `odometer`. |
| `vehicle_id` | chaîne | Obligatoire pour `odometer`, `drive`, `fuel`. Facultatif pour les sessions de charge. Le compte doit pouvoir modifier le véhicule. |
| `event_id` | chaîne | Votre propre identifiant de l'événement, pour l'idempotence (voir plus bas). |
| `timestamp` | RFC 3339 | Utilisé en l'absence de `data.start_time` / `data.end_time`. |
| `distance_unit` | `km` (défaut) ou `mi` | Unité des distances et du compteur de l'événement. Les valeurs sont stockées en km. |
| `data` | objet | Champs du type d'événement, ci-dessous. |

### `charging_session_end`

`data` : `start_time`, `end_time`, `energy_kwh` (ou `energy_added_kwh` ; supérieur à 0, 500 au plus), `cost` (unités monétaires) ou `cost_cents`, `location`, `charger_name`, `odometer_km`, `soc_start`, `soc_end`.

Sans coût, le plan tarifaire du véhicule s'applique s'il existe, sinon le coût reste à compléter. Véhicules électriques uniquement.

Une session sans `vehicle_id` va au véhicule par défaut de la borne domestique, sinon au seul véhicule électrique modifiable du compte, sinon elle attend dans les charges en attente (borne partagée entre plusieurs véhicules) que vous l'affectiez dans l'application.

### `odometer`

`data` : `odometer` (dans `distance_unit`) ou `odometer_km`. Le compteur du véhicule ne fait qu'avancer.

### `drive`

`data` : `start_time` (ou le `timestamp` de l'événement), `end_time` ou `duration_min`, `distance` (dans `distance_unit`, de 0,1 à 3000 km), `start_odometer`, `end_odometer`, `start_address`, `end_address`, `energy_kwh`.

Sans `energy_kwh`, l'énergie est estimée d'après la consommation moyenne du véhicule et marquée comme estimée.

### `fuel`

Véhicules thermiques uniquement. `data` : `amount` (ou `cost`), `liters`, `price_per_liter`, `fuel_type`, `is_full_tank`, `odometer` ou `odometer_km`. Le montant peut se déduire des litres et du prix au litre. L'heure est le `timestamp` de l'événement (ou `data.start_time`), maintenant à défaut.

## Réponses

| Statut | `status` | Signification |
|---|---|---|
| 201 | `recorded` | Enregistré. Le corps contient `vehicle_id` et l'identifiant du nouvel enregistrement. |
| 201 | `pending_qualification` | Session de charge en attente d'un véhicule (`pending_id`). |
| 200 | `duplicate` | Déjà enregistré ou déjà en attente ; rien n'a été écrit. |
| 200 | (compteur) | Relevé appliqué. |

Les erreurs répondent `{"error": "...", "code": "...", "params": {...}}`. Codes fréquents :

| Code | Cause |
|---|---|
| `request.invalid_body` | Le corps n'est pas du JSON valide. |
| `integration.unknown_event_type` | `event_type` mal orthographié. |
| `integration.invalid_distance_unit` | `distance_unit` n'est ni `km` ni `mi`. |
| `vehicle.not_specified` | `vehicle_id` manquant sur un événement qui l'exige. |
| `telemetry.missing_odometer` | Aucun relevé de compteur valide. |
| `charge.invalid_energy`, `charge.invalid_period`, `charge.electric_only` | Session de charge refusée. |
| `drive.missing_start_time`, `drive.invalid_distance` | Trajet refusé. |
| `fuel.combustion_only`, `fuel.amount_required` | Plein refusé. |

## Idempotence

Renvoyer le même événement est sans danger. Un enregistrement est reconnu comme déjà stocké :

1. par `event_id` (sessions de charge et trajets), quand vous en envoyez un ;
2. sinon par tolérance : une charge débutant à 30 min près avec une énergie à 0,5 kWh près, un trajet débutant à 15 min près avec une distance à 1 km près, un plein à 30 min près pour le même montant (les pleins ne sont reconnus que de cette façon).

Utilisez un `event_id` stable (identifiant de session, clé construite sur l'horodatage) dès que la source en fournit un.

## Exemples

curl :

```bash
curl -X POST https://autoledger.example.com/api/integrations/homeassistant/event \
  -H "Authorization: Bearer al_live_xxxxxxxx" -H "Content-Type: application/json" \
  -d '{"event_type":"charging_session_end","event_id":"wallbox-2026-05-01-1","vehicle_id":"<id>",
       "data":{"start_time":"2026-05-01T22:00:00Z","end_time":"2026-05-02T05:30:00Z","energy_kwh":38.4,"cost":5.76}}'
```

Un trajet en miles :

```bash
curl -X POST https://autoledger.example.com/api/integrations/homeassistant/event \
  -H "Authorization: Bearer al_live_xxxxxxxx" -H "Content-Type: application/json" \
  -d '{"event_type":"drive","vehicle_id":"<id>","distance_unit":"mi",
       "data":{"start_time":"2026-05-01T08:00:00Z","duration_min":25,"distance":12.4,"end_odometer":42310}}'
```

Home Assistant (`rest_command`) :

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

Node-RED / n8n : un nœud HTTP Request, méthode `POST`, l'URL ci-dessus, l'en-tête `Authorization` et le corps JSON de l'événement.
