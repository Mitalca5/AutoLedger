# Usure mesurée et attendue des pneus

Un pneu avec des mesures d’usure exploitables affiche côte à côte le rythme observé et le rythme attendu, avec la durée de vie totale impliquée par chacun. Le rythme attendu suit la durée de vie saisie pour le pneu, lorsqu’elle existe ; sinon, il utilise le rythme moyen d’usure. Sans mesure exploitable, cette comparaison est masquée.

Le rythme observé rapporte la perte de profondeur à la distance parcourue pendant le montage du pneu. Il s’agit du rythme mesuré brut, pas du rythme pondéré utilisé par les prévisions de remplacement. Le poids de la mesure dans la prévision est affiché à titre de contexte ; ce n’est pas un intervalle de confiance statistique. Les durées de vie totales calculées supposent que ces rythmes restent constants et ne constituent pas des garanties de remplacement. La profondeur enregistrée et le calcul des prévisions restent inchangés. La durée de vie ajustée par télémétrie reste séparée.

L’écart compare les rythmes d’usure, pas la distance restante : un écart positif signifie une usure plus rapide. Un écart strictement supérieur à 25 % est ambre, strictement inférieur à -25 % est vert, et la plage intermédiaire reste neutre. Ces couleurs décrivent la comparaison sans modifier l’état GOOD/WARNING/CRITICAL du pneu. Les distances et les dénominateurs des rythmes d’usure suivent l’unité du compte.
