# Grilles tarifaires

Une grille tarifaire chiffre l'énergie d'une session de recharge à domicile lorsqu'aucun coût n'est donné. English : [tariffs.md](tariffs.md). Trois types de grille existent :

| `plan_type` | Signification |
|---|---|
| `FLAT` | Un prix par kWh (`flat_rate_cents`). |
| `TIME_OF_USE` | Prix heures pleines et heures creuses, avec des `time_windows` pour les heures creuses. Stocké comme `BANDS` depuis la migration 51 et toujours accepté par l'API. |
| `BANDS` | Un nombre quelconque de tranches nommées, des règles qui associent une tranche à une plage horaire, et une tranche par défaut. |

## Modifier les grilles

**Compte → Grilles tarifaires d'électricité** liste les grilles regroupées par nom de tarif, la version la plus récente en premier. Une grille se crée et se modifie dans un formulaire (prix unique ou plusieurs prix, plages horaires avec leurs jours, dates de validité, abonnement mensuel). Chaque ligne propose la modification, **Nouvelle version** (mêmes prix, à partir du lendemain de la fin de la version précédente), la duplication et la suppression. Le sélecteur de grille du formulaire véhicule renvoie vers cette section. Les mêmes opérations existent via `POST/PUT/DELETE /api/tariffs/plans` ; les prix sont stockés au centime par kWh.

## Tranches

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

- `rate_cents` est le prix par kWh en unités de la devise, comme tous les montants de l'API.
- `days` va de 0 = dimanche à 6 = samedi ; omis, il vaut tous les jours.
- Une règle dont la fin précède le début passe minuit. La partie après minuit appartient au jour où la règle commence.
- La première règle qui correspond l'emporte ; le temps hors de toute règle est facturé à `default_band`.
- Les heures sont des heures locales dans `APP_TIMEZONE` : une plage garde ses heures lors des changements d'heure.
- Une session à cheval sur plusieurs tranches est facturée au prorata du temps passé dans chacune.

## Validité et versions

`valid_from` et `valid_to` (`AAAA-MM-JJ`, facultatifs) bornent les jours qu'une grille chiffre. Les grilles d'un même propriétaire qui portent le même nom (sans tenir compte de la casse) sont des versions d'un même tarif : la version dont la période couvre le jour de la session est utilisée, si bien qu'un nouveau prix s'ajoute comme une nouvelle grille et ne modifie jamais les coûts déjà enregistrés. Une session un jour qu'aucune version ne couvre est laissée à la saisie manuelle du coût.

## Où la grille chiffre une session

- Les sessions envoyées par Home Assistant ou un script sans coût, et les recharges en attente rattachées à un véhicule, utilisent la grille du véhicule au jour du début de la session.
- Le formulaire de recharge (Énergie → Recharges → Ajouter) chiffre une session de la même façon dès que le début, la fin facultative et l'énergie sont renseignés : `POST /api/tariffs/calculate-session` avec `vehicle_id`, `start_time`, `end_time` et `kwh` renvoie le `cost` et le nom de la grille `plan` (`null` si aucune grille ne chiffre ce jour). Le formulaire envoie des instants ; le serveur les lit dans `APP_TIMEZONE`, donc une session qui passe minuit est répartie entre les tranches où elle se déroule. Un prix ou un coût saisi à la main remplace le calcul.
- Le même calcul renseigne le coût d'une recharge synchronisée qui n'en a pas, d'après son début et sa fin enregistrés.

`standing_charge_cents` enregistre un abonnement mensuel facultatif pour la grille.
