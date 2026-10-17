# Renouvellement de session

English : [session-refresh.md](session-refresh.md).

Le navigateur conserve les jetons d’accès et de renouvellement dans des cookies HttpOnly. Une requête qui reçoit une erreur d’authentification peut renouveler la session et être réessayée une fois, en conservant son corps et sa clé d’idempotence.

Les onglets d’une même origine coordonnent le renouvellement via Web Locks dans les contextes sécurisés compatibles (HTTPS ou localhost). Un onglet qui a attendu un renouvellement réussi réutilise les cookies actualisés. Seul un marqueur aléatoire de coordination est enregistré dans le stockage local ; il ne contient ni jeton ni données de compte. Si le stockage local est indisponible, le verrou sérialise encore les renouvellements.

Les navigateurs sans Web Locks conservent uniquement la coordination au sein de chaque onglet. Utilisez HTTPS pour les installations accessibles par un nom d’hôte ou une adresse du réseau local. La détection côté serveur de la réutilisation des jetons reste active.
