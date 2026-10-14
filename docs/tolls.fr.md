# Péages

Les péages s'enregistrent de deux façons, dont une seule dépend du pays.

| | Disponible | Ce que ça fait |
| --- | --- | --- |
| Péages et parkings manuels | Partout | Ajoute un péage ou un parking à un trajet ou à un voyage, ou depuis la page des dépenses. Il compte dans le coût d'usage comme toute autre dépense. |
| Détection automatique | Là où la détection des péages est active | Compare la trace GPS d'un trajet TeslaMate aux gares de péage des autoroutes françaises, estime le tarif et l'enregistre comme péage automatique. Elle liste aussi les trajets autoroutiers restant à qualifier (un péage saisi, ou « pas de péage »). |

Les données de référence de la détection automatique viennent d'[OpenTollData](https://github.com/louis2038/OpenTollData) et ne couvrent que les autoroutes françaises.

## Réglage de la détection des péages

Chaque véhicule a un réglage **Détection des péages (France)** dans son formulaire :

| Mode | Effet |
| --- | --- |
| Automatique (défaut) | Active quand le véhicule montre des signes de conduite en France : une détection GPS qui a trouvé une gare de péage, un péage automatique déjà enregistré, un trajet dont l'adresse nomme la France, ou un propriétaire qui utilise l'interface française. |
| Toujours activée | Active quoi qu'ait fait le véhicule jusque-là, par exemple pour un premier voyage en France. |
| Désactivée | Jamais active, même si les signes ci-dessus existent. |

Le serveur décide : le véhicule renvoie `toll_detection` (le mode) et `toll_detection_active` (le résultat).

Quand la détection n'est pas active :

- le filtre « À qualifier » et son compteur, le filtre de source du péage, l'action de lot « Péage auto », les boutons de qualification des cartes de trajet et le bouton de détection de la modale de coût sont masqués ;
- les points d'accès de détection et de péages en lot répondent `409` avec le code `toll.detection_disabled` ;
- les trajets autoroutiers ne comptent pas dans le score de complétude du tableau de bord ;
- les péages manuels, le filtre « Avec péage » et la ligne péages de la ventilation des coûts restent disponibles, et les péages déjà détectés sont conservés.
