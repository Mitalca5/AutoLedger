# Import et export CSV

AutoLedger importe quatre types de fichiers : recharges, trajets, pleins et relevés kilométriques. English: [csv-import.md](csv-import.md).

Les mêmes colonnes sont utilisées par l'**export** (page de suivi manuel, ou `GET /api/vehicles/{id}/export?type=...&format=csv|json&from=AAAA-MM-JJ&to=AAAA-MM-JJ`) : un fichier exporté peut être réimporté, dans un autre véhicule ou une autre instance. L'export couvre aussi les dépenses et l'entretien, qui n'ont pas d'import.

## Format du fichier

- Séparateur : virgule, point-virgule ou tabulation, détecté dans le fichier. UTF-8, avec ou sans BOM.
- Première ligne : les en-têtes. La casse, les accents et les espaces autour sont ignorés, et les alias ci-dessous sont acceptés.
- Dates : `2026-01-15`, `2026-01-15 18:30`, `2026-01-15T18:30:00Z` (une date sans décalage est lue dans le fuseau de l'instance, `APP_TIMEZONE`). Les dates avec `/` ou `.` sont lues jour en premier par défaut (`15/01/2026`) ; la fenêtre d'import permet de choisir mois en premier ou année en premier.
- Nombres : `.` ou `,` comme séparateur décimal, détecté. Si un fichier contient des séparateurs de milliers, choisissez le séparateur dans la fenêtre d'import.
- Distances : les colonnes suffixées `_km` ou `_mi` sont explicites. Sans suffixe, la colonne est lue dans l'unité de distance du compte.
- Une seule ligne invalide annule tout le fichier : rien n'est écrit tant que toutes les lignes ne sont pas valides. Les lignes déjà présentes sont ignorées si « ignorer les doublons » est activé.
- Le type (recharges, trajets, pleins, relevés) est détecté d'après les en-têtes et peut être choisi à la main.

## Recharges

| Colonne | Alias | Obligatoire | Remarques |
| --- | --- | --- | --- |
| `date` | `time`, `start_time`, `datetime` | oui | |
| `kwh` | `kwh_added`, `energy`, `energy_kwh` | oui | 0 à 1000 |
| `cost` | `amount`, `price`, `total_cost` | oui | |
| `currency` | `curr` | non | code ISO ; devise du véhicule par défaut |
| `fx_rate` | `fx`, `rate` | si la devise diffère | conversion vers la devise du véhicule |
| `odometer_km` / `odometer_mi` | `odometer`, `odo` | non | |
| `location` | `address`, `station`, `place` | non | |

## Trajets

| Colonne | Alias | Obligatoire | Remarques |
| --- | --- | --- | --- |
| `start_time` | `date`, `start`, `datetime` | oui | |
| `distance_km` / `distance_mi` | `distance`, `dist` | oui | 0 à 3000 km |
| `end_time` | `end` | non | sinon déduit de la distance |
| `kwh` | `energy`, `energy_consumed_kwh` | non | |
| `start_address` | `start_location`, `origin` | non | |
| `end_address` | `end_location`, `destination` | non | |
| `tag` | `tags`, `purpose` | non | plusieurs étiquettes séparées par `;` |

## Pleins

| Colonne | Alias | Obligatoire | Remarques |
| --- | --- | --- | --- |
| `date` | `time`, `datetime` | oui | |
| `liters` | `litres`, `volume`, `quantity` | avec `price_per_liter`, ou utiliser `amount` | 0 à 500 |
| `price_per_liter` | `price_per_litre`, `unit_price`, `price` | | 0 à 10 |
| `amount` | `cost`, `total`, `total_cost` | ou litres et prix au litre | |
| `fuel_type` | `fuel` | non | `SP95_E10`, `SP98`, `DIESEL`, `E85`, `GPL` |
| `is_full_tank` | `full_tank`, `full` | non | oui par défaut |
| `odometer_km` / `odometer_mi` | `odometer`, `odo` | non | |
| `notes` | `note`, `comment` | non | |

## Relevés kilométriques

| Colonne | Alias | Obligatoire |
| --- | --- | --- |
| `date` | `time`, `datetime` | oui |
| `odometer_km` / `odometer_mi` | `odometer`, `odo` | oui |
| `notes` | `note`, `comment` | non |

## Correspondance des colonnes et profils

Quand un en-tête n'est pas reconnu, la fenêtre d'import affiche chaque colonne du fichier avec le champ auquel elle est associée et permet d'en choisir un autre, ou d'ignorer la colonne. La correspondance, l'ordre des dates et le séparateur décimal peuvent être enregistrés dans un **profil** nommé ; le sélectionner plus tard les applique à tout fichier de même structure, sur n'importe quel véhicule du compte.

## Recette : export d'un enregistreur OBD2 (Car Scanner, Torque, ...)

Le cœur n'a pas d'analyseur propre à une application. Un journal de trajets ou de pleins exporté par une application OBD2 s'importe via un profil, configuré une seule fois.

1. Exportez le journal de l'application en CSV (Car Scanner : journal des *trajets* ou du *carburant*, Torque : *journal de trajets*).
2. Ouvrez **Suivi manuel → Importer un CSV**, choisissez le fichier et le type (`Trajets` pour les trajets, `Pleins` pour les journaux de carburant).
3. À l'étape de correspondance, associez les colonnes de l'application aux champs. Correspondances usuelles :

   | Colonne de l'application (exemple) | Champ AutoLedger |
   | --- | --- |
   | `Heure de début`, `Début du trajet` | `start_time` |
   | `Heure de fin`, `Fin du trajet` | `end_time` |
   | `Distance (km)`, `Distance du trajet` | `distance_km` |
   | `Carburant utilisé (l)`, `Volume` | `liters` |
   | `Coût`, `Prix total` | `amount` |
   | toute autre colonne (vitesse, régime, températures) | ignorer |

4. Réglez l'ordre des dates et le séparateur décimal utilisés par l'application (beaucoup d'applications sur un téléphone en français écrivent `15/01/2026` et `12,5`).
5. Enregistrez la correspondance dans un profil (par exemple « Car Scanner trajets »). Les exports suivants de la même application demandent seulement de sélectionner le profil.

Les noms de colonnes varient selon la version et la langue de l'application : l'aperçu liste les colonnes non associées et les lignes refusées avec leur numéro de ligne, ce qui permet d'ajuster la correspondance avant toute écriture. Un lot importé peut être annulé depuis la même fenêtre.
