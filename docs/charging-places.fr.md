# Lieux de recharge

English: [charging-places.md](charging-places.md).

Chaque recharge rapporte un lieu sous forme de texte : un nom de géofence TeslaMate (`Maison`), une adresse (`Valence Supercharger, Valence`), un tag Home Assistant ou le nom donné à la borne. AutoLedger ne déduit jamais un lieu de la vitesse de charge ni de la source d'une recharge.

## Classer les lieux

**Énergie → Recharges → Lieux** liste les textes distincts rapportés par les recharges du véhicule, avec leur nombre de sessions et leur énergie. Chaque texte est associé à un lieu en un choix ; toutes les recharges qui rapportent ce texte, passées et futures, sont classées d'un coup. Deux écritures qui ne diffèrent que par la casse ou les espaces sont le même texte.

Seul le domicile est à déclarer. Choisir **Domicile** pour un texte crée le lieu domicile s'il n'existe pas encore, et plusieurs textes peuvent y être associés. Dès que le compte a un lieu domicile, toute recharge qui rapporte un lieu associé à aucun lieu compte comme **Extérieur** ; une recharge qui ne rapporte aucun lieu reste non classée. Sans lieu domicile, rien n'est supposé hors domicile.

Un lieu a un nom et une catégorie : **Domicile**, **Travail**, **Public** ou **Autre**. Les lieux appartiennent au propriétaire du véhicule et sont partagés par tous ceux qui y ont accès. Supprimer un lieu laisse ses recharges non classées.

Une recharge peut recevoir son propre lieu dans le formulaire de recharge, qui l'emporte alors sur son texte rapporté.

## Recharges par lieu

**Énergie → Recharges** s'ouvre sur une synthèse des recharges du mois en cours. La période se choisit avec les préréglages (1 mois, 3 mois, 6 mois, 1 an, tout) ou avec deux jours pris sur un calendrier. **Énergie / Coût** bascule les barres mensuelles et la répartition par lieu entre kWh et coût enregistré :

- l'énergie, le montant dépensé et le prix moyen au kWh ;
- l'énergie de chaque mois par lieu, et le prix moyen de chaque mois ;
- une répartition par lieu, en énergie ou en coût, avec l'énergie, le coût, le prix moyen, les sessions gratuites et les sessions sans coût de chaque lieu ;
- **Si tout était rechargé à domicile** : ce qu'a coûté l'énergie rechargée dans les lieux classés hors **Domicile**, extérieur compris, comparé à la même énergie au prix moyen des lieux **Domicile**.

Comment les chiffres sont comptés :

- Le prix moyen est le coût enregistré rapporté à l'énergie des sessions dont le coût est connu. Une session gratuite (coût 0) compte pour zéro ; une session sans coût enregistré est laissée hors du prix, et son énergie et son nombre sont affichés à côté.
- Les sessions qui n'ont enregistré aucune énergie ne sont pas comptées et sont signalées à part.
- L'énergie des recharges sans lieu rapporté est affichée comme non classée et n'est jamais supposée hors domicile : elle est laissée hors du chiffre domicile. Les recharges en extérieur sont comptées hors domicile.
- Le chiffre domicile est négatif (« de plus ») quand recharger ailleurs a coûté moins que le prix domicile, par exemple une recharge gratuite au travail.
- Les coûts sont les montants du registre des coûts : ils correspondent aux totaux d'énergie mensuels du tableau de bord.

## Home Assistant

L'intégration envoie le lieu d'une session depuis une **entité de lieu** optionnelle (par exemple le suivi de position du véhicule, dont l'état est le nom de la zone) ou depuis un **tag de lieu par défaut**. Sans l'un ni l'autre, la session est placée au nom donné à sa borne.
