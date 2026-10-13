# Comparatif de véhicules

English : [vehicle-comparison.md](vehicle-comparison.md).

Choisissez un véhicule électrique, hybride rechargeable (PHEV) ou à prolongateur d'autonomie (REEV), ouvrez **Comparatif** et créez un comparatif **véhicule suivi**. Toutes les marques et tous les modèles utilisent le même calcul.

Renseignez la consommation du véhicule thermique de référence en L/100 km, le prix au litre, les prix d'achat et de revente, l'entretien annuel, l'assurance et les taxes. Les préréglages de carburant sont des points de départ, pas un catalogue de modèles : remplacez-les par des hypothèses adaptées à votre véhicule, votre marché et votre devise.

Le côté suivi utilise les coûts enregistrés divisés par le kilométrage connu, puis applique la distance annuelle et la durée saisies. Les hybrides incluent les pleins et les recharges. Une variation du prix du carburant s'applique à la part carburant des deux véhicules ; une variation du prix de l'électricité ne s'applique qu'aux recharges. Le graphique de sensibilité au carburant modifie aussi les deux parts carburant.

Les résultats montrent les coûts d'énergie, l'entretien, l'assurance, la dépréciation, le coût total, le coût par distance et le point mort d'achat projeté. Comparez les deux lignes **Énergie** pour l'économie d'énergie ; l'économie totale inclut les autres catégories de coûts. Les coûts enregistrés ne sont jamais modifiés.

Les prix du carburant et de l’électricité respectent les limites de stockage, quelle que soit la devise, y compris dans les champs de simulation.

## Données et hypothèses

- Enregistrez chaque plein et chaque recharge, y compris les recharges gratuites, et saisissez les relevés kilométriques. Des coûts ou un kilométrage manquants rendent l'estimation peu fiable. Un hybride qui n'a que des pleins ou que des recharges peut être comparé ; le résultat porte alors un avertissement, car le calcul ne peut pas deviner les achats non enregistrés.
- Les coûts représentent des achats du journal, pas l'énergie mesurée en roulant. Une variation du carburant restant dans le réservoir ou de la charge de la batterie peut fausser un historique court.
- Les deux côtés utilisent la même distance annuelle. Avec moins de trois mois d'historique, la distance annuelle vaut 12 000 km par défaut ; modifiez-la selon votre usage.
- Assurance : une prime récurrente en cours est annualisée selon son intervalle de facturation (100 par mois, 300 par trimestre et 1 200 tous les douze mois donnent 1 200 par an) ; une prime ponctuelle avec période de couverture est annualisée sur cette période. Les paiements sans l'un ni l'autre utilisent leur total des douze derniers mois, avec une hypothèse affichée dans le résultat. Les contrats futurs et expirés sont exclus : renseignez une date de fin sur un contrat récurrent remplacé. Les deux côtés appliquent la même inflation de l'assurance, et le kilométrage annuel ne change jamais un total d'assurance.
- Le côté suivi est étiqueté « projeté depuis vos coûts saisis » : il extrapole ce que vous avez saisi, ce n'est pas un total mesuré. Le résultat indique sous le verdict, à l'écran, à l'impression et dans l'export CSV, ce qui n'est pas comparé : péages, stationnement, financement, abonnements et autres coûts.
- Les financements, prêts et locations sont exclus. La dépréciation suit le calcul achat/revente existant ; le point mort compare l'achat et les dépenses courantes sans déduire la revente.
- La **projection** reste un scénario électrique contre thermique saisi à la main. Un comparatif d'hybride suivi demande des données d'hybride enregistrées.

## Point mort

Le point mort est l'année à partir de laquelle les dépenses cumulées du véhicule suivi (prix d'achat plus énergie, entretien, assurance et taxes de chaque année, avec l'inflation du scénario) passent sous celles du thermique. La revente n'est pas déduite ; lorsqu'une valeur de revente est saisie, un second chiffre crédite chaque véhicule de sa valeur à cette date. La recherche s'arrête à la fin de la période et ne prolonge jamais au-delà.

Lorsqu'il n'est pas atteint, le résultat en donne la raison :

- **la période est trop courte** : les économies d'usage réduisent l'écart d'achat sans le combler dans la période ;
- **aucune économie d'usage** : le véhicule suivi ne coûte pas moins cher à l'usage que le thermique ;
- **inversé** : le véhicule suivi est moins cher à l'achat mais finit plus cher que le thermique sur la période.

## Et si

Le résultat d'une **projection** comporte une section **Et si… ?**. Déplacez les curseurs, ou saisissez une valeur, pour la distance annuelle, le prix du carburant au litre et le prix de l'électricité au kWh ; les curseurs couvrent de la moitié au double de la valeur enregistrée. **Autres hypothèses** ouvre la durée (années entières), les prix d'achat et de revente des deux véhicules et l'inflation du carburant, de l'électricité et des autres coûts, avec les mêmes limites que le formulaire du scénario. Les chiffres explorés s'affichent à côté des chiffres enregistrés : l'économie d'énergie (carburant et électricité seulement), l'économie totale (tous les coûts du scénario) et le point mort sur le coût complet sont trois chiffres distincts. L'écart du point mort est exprimé en années, ou indique que le point mort devient atteint ou cesse de l'être. **Réinitialiser** revient aux valeurs enregistrées et **Enregistrer comme nouvelle comparaison** ouvre le formulaire avec les valeurs explorées, sans modifier la comparaison d'origine.

Les valeurs explorées passent par le même calcul qu'un scénario enregistré et ne sont pas stockées. Un prix hypothétique ne recalcule jamais une recharge ou un plein enregistré. Les distances, volumes et la devise suivent les réglages du compte. La section n'existe que pour les projections.

## Coûts de recharge gratuits et inconnus

Dans une projection manuelle, saisissez un prix de l’électricité de 0 pour une énergie gratuite ; un champ vide ne signifie pas zéro.

Saisissez un coût explicite de **0** pour une recharge gratuite, quelle qu'en soit la raison. Zéro est un coût connu ; un coût absent reste inconnu. Les recharges gratuites enregistrées comptent comme des recharges dans les comparatifs d'hybrides suivis et ne déclenchent pas l'avertissement de source d'énergie manquante. Les recharges dont le coût est inconnu reçoivent un rappel distinct pour saisir leur coût réel, ou zéro si elles étaient gratuites.

La devise d’une comparaison suivie est celle du véhicule de référence ; une projection conserve la devise choisie. La modifier ne convertit pas les montants. Une ancienne projection prend la devise commune à tous les véhicules de son propriétaire ; si elles diffèrent, le propriétaire confirme la devise avant de la consulter. Une projection enregistrée sans devise prend cette même devise commune ; si les véhicules diffèrent, la requête est refusée tant qu’une devise n’est pas choisie. Les comparaisons de devises différentes ne partagent pas un graphique ou un CSV.

Le panneau « Et si » d’une projection avertit avant d’utiliser un prix de carburant ou d’électricité supérieur à trois fois la médiane d’au moins cinq achats au coût connu pour le véhicule sélectionné. Les pleins doivent correspondre au carburant du scénario. Les recharges dans une autre devise sont converties avec leur taux enregistré ; les coûts inconnus sont exclus et les recharges gratuites comptent pour zéro. Sans historique adapté, avec une médiane nulle ou une devise de scénario différente, aucun avertissement n’est affiché. Vous pouvez confirmer le prix et continuer ; la confirmation ne modifie ni la valeur ni les données enregistrées. Un changement de prix ou de référence demande une nouvelle confirmation.
