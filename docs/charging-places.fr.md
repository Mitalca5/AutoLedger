# Lieux de recharge

English: [charging-places.md](charging-places.md).

Chaque recharge rapporte un lieu sous forme de texte : un nom de géofence TeslaMate (`Maison`), une adresse (`Valence Supercharger, Valence`), un tag Home Assistant ou le nom donné à la borne. AutoLedger ne déduit jamais un lieu de la vitesse de charge ni de la source d'une recharge.

## Classer les lieux

**Énergie → Recharges → Lieux** liste les textes distincts rapportés par les recharges du véhicule, avec leur nombre de sessions et leur énergie. Chaque texte est associé à un lieu en un choix ; toutes les recharges qui rapportent ce texte, passées et futures, sont classées d'un coup. Deux écritures qui ne diffèrent que par la casse ou les espaces sont le même texte.

Un lieu a un nom et une catégorie : **Domicile**, **Travail**, **Public** ou **Autre**. Les lieux appartiennent au propriétaire du véhicule et sont partagés par tous ceux qui y ont accès. Supprimer un lieu laisse ses recharges non classées.

Une recharge peut recevoir son propre lieu dans le formulaire de recharge, qui l'emporte alors sur son texte rapporté.

## Home Assistant

L'intégration envoie le lieu d'une session depuis une **entité de lieu** optionnelle (par exemple le suivi de position du véhicule, dont l'état est le nom de la zone) ou depuis un **tag de lieu par défaut**. Sans l'un ni l'autre, la session est placée au nom donné à sa borne.
