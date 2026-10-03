# Recettes borne de recharge

AutoLedger n'a pas de connecteur propre à une marque de borne. Toute borne que Home Assistant sait lire (Easee, Wallbox, Zaptec, une borne OCPP, une prise connectée avec compteur d'énergie...) l'alimente de la même façon : à la fin d'une session, Home Assistant envoie un événement `charging_session_end` à l'[API d'ingestion](ingestion-api.fr.md). English: [wallbox-recipes.md](wallbox-recipes.md).

L'[intégration HACS](https://github.com/Rem7474/autoledger-homeassistant) apporte les données d'AutoLedger dans Home Assistant ; les recettes ci-dessous font le chemin inverse, de la borne vers le registre, avec le jeton et l'adresse de l'API d'ingestion.

## Ce dont l'événement a besoin

| Champ de l'événement | Provenance côté borne |
|---|---|
| `data.energy_kwh` | L'énergie de la session terminée, en kWh. |
| `data.start_time`, `data.end_time` | Début et fin de la session (RFC 3339). |
| `event_id` | Une clé identique à chaque envoi de la même session : nom de la borne + heure de début. |
| `vehicle_id` | Facultatif. Sans lui, la session va au véhicule par défaut de la borne domicile, puis au seul véhicule électrique du compte, sinon elle attend dans les recharges en attente. |
| `data.cost` | Facultatif. Sans lui, la [grille tarifaire](tariffs.md) du véhicule chiffre la session. |

## Associer les capteurs

Les noms d'entités varient selon l'intégration et l'installation : ouvrez *Outils de développement > États* et cherchez votre borne. Trois éléments sont nécessaires, quelle que soit la marque :

| Besoin | Entité typique | Remarques |
|---|---|---|
| État de la borne | `sensor.<borne>_status` | Ses états comprennent un état « en charge » et d'autres pour « terminé », « déconnecté » ou « en attente ». Utilisez les chaînes exactes que vous voyez. |
| Énergie de la session | `sensor.<borne>_session_energy` | En kWh, souvent remise à zéro au débranchement ou au début d'une nouvelle session. Lisez-la quand l'état quitte « en charge », avant la remise à zéro. |
| Début de session | aucune | Notez l'heure où l'état devient « en charge » dans un assistant `input_text`. |

Si la borne ne publie qu'un compteur d'énergie cumulé, enregistrez sa valeur au début (un second assistant `input_number`) et envoyez la différence.

## Recette

Assistant (Paramètres > Appareils et services > Assistants) : un assistant texte `input_text.wallbox_session_start` (longueur maximale 64) qui contient le début de la session en ISO 8601.

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

Adaptez `sensor.wallbox_status`, `sensor.wallbox_session_energy` et l'état `"charging"` à vos entités.

## Idempotence

L'`event_id` est dérivé du début de session : un redémarrage de Home Assistant au milieu de l'automatisation, une nouvelle tentative ou un double déclenchement ne crée jamais une seconde recharge. AutoLedger répond `duplicate` et n'écrit rien. Ajoutez le nom de la borne à la clé quand plusieurs bornes alimentent le même compte.

Un état qui oscille entre « en charge » et un état d'attente (charge suspendue par un délesteur) envoie un événement par période reprise ; chacun a sa propre heure de début, donc chacun est enregistré. Pour n'envoyer qu'un événement par session, déclenchez sur un état qui n'apparaît qu'au débranchement du câble.

## Coût

- Avec une [grille tarifaire](tariffs.md) sur le véhicule (tarif fixe, heures pleines / creuses ou un nombre quelconque de tranches), omettez `cost` : la session est chiffrée d'après ses heures de début et de fin.
- Avec un tarif dynamique ou un prix par session fourni par la borne, envoyez `data.cost` en unités monétaires.
- La grille s'applique aussi aux sessions en attente quand vous les affectez.
