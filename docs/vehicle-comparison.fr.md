# Comparatif de véhicules

English : [vehicle-comparison.md](vehicle-comparison.md).

Choisissez un véhicule électrique, hybride rechargeable (PHEV) ou à prolongateur d'autonomie (REEV), ouvrez **Comparatif** et créez un comparatif **véhicule suivi**. Toutes les marques et tous les modèles utilisent le même calcul.

Renseignez la consommation du véhicule thermique de référence en L/100 km, le prix au litre, les prix d'achat et de revente, l'entretien annuel, l'assurance et les taxes. Les préréglages de carburant sont des points de départ, pas un catalogue de modèles : remplacez-les par des hypothèses adaptées à votre véhicule, votre marché et votre devise.

Le côté suivi utilise les coûts enregistrés divisés par le kilométrage connu, puis applique la distance annuelle et la durée saisies. Les hybrides incluent les pleins et les recharges. Une variation du prix du carburant s'applique à la part carburant des deux véhicules ; une variation du prix de l'électricité ne s'applique qu'aux recharges. Le graphique de sensibilité au carburant modifie aussi les deux parts carburant.

Les résultats montrent les coûts d'énergie, l'entretien, l'assurance, la dépréciation, le coût total, le coût par distance et le seuil de rentabilité d'achat projeté. Comparez les deux lignes **Énergie** pour l'économie d'énergie ; l'économie totale inclut les autres catégories de coûts. Les coûts enregistrés ne sont jamais modifiés.

## Données et hypothèses

- Enregistrez chaque plein et chaque recharge, y compris les recharges gratuites, et saisissez les relevés kilométriques. Des coûts ou un kilométrage manquants rendent l'estimation peu fiable. Un hybride qui n'a que des pleins ou que des recharges peut être comparé ; le résultat porte alors un avertissement, car le calcul ne peut pas deviner les achats non enregistrés.
- Les coûts représentent des achats du journal, pas l'énergie mesurée en roulant. Une variation du carburant restant dans le réservoir ou de la charge de la batterie peut fausser un historique court.
- Les deux côtés utilisent la même distance annuelle. Avec moins de trois mois d'historique, la distance annuelle vaut 12 000 km par défaut ; modifiez-la selon votre usage.
- Assurance : une prime récurrente en cours est annualisée selon son intervalle de facturation (100 par mois, 300 par trimestre et 1 200 tous les douze mois donnent 1 200 par an) ; une prime ponctuelle avec période de couverture est annualisée sur cette période. Les paiements sans l'un ni l'autre utilisent leur total des douze derniers mois, avec une hypothèse affichée dans le résultat. Les contrats futurs et expirés sont exclus : renseignez une date de fin sur un contrat récurrent remplacé. Les deux côtés appliquent la même inflation de l'assurance, et le kilométrage annuel ne change jamais un total d'assurance.
- Les financements, prêts et locations sont exclus. La dépréciation suit le calcul achat/revente existant ; le seuil de rentabilité compare l'achat et les dépenses courantes sans déduire la revente.
- La **projection** reste un scénario électrique contre thermique saisi à la main. Un comparatif d'hybride suivi demande des données d'hybride enregistrées.

## Coûts de recharge gratuits et inconnus

Saisissez un coût explicite de **0** pour une recharge gratuite, quelle qu'en soit la raison. Zéro est un coût connu ; un coût absent reste inconnu. Les recharges gratuites enregistrées comptent comme des recharges dans les comparatifs d'hybrides suivis et ne déclenchent pas l'avertissement de source d'énergie manquante. Les recharges dont le coût est inconnu reçoivent un rappel distinct pour saisir leur coût réel, ou zéro si elles étaient gratuites.
