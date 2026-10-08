# AutoLedger 🚗

[English](README.md) · **Français**

> **Le registre auto-hébergé de ce que votre voiture coûte vraiment, au kilomètre.**  
> Électrique, hybride rechargeable, à prolongateur d'autonomie ou thermique, de toute marque : énergie ou carburant, financement (comptant, crédit, location), pneus, entretien, assurance, péages et covoiturage au même endroit, alimentés à la main, par CSV, par Home Assistant ou un script, ou synchronisés depuis [TeslaMate](https://github.com/teslamate-org/teslamate).

[![CI](https://github.com/Rem7474/AutoLedger/actions/workflows/ci.yml/badge.svg)](https://github.com/Rem7474/AutoLedger/actions/workflows/ci.yml)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=Rem7474_TeslaCost&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=Rem7474_TeslaCost)
[![Docker Image](https://img.shields.io/badge/docker-ghcr.io%2Frem7474%2Fautoledger-blue?logo=docker)](https://github.com/Rem7474/AutoLedger/pkgs/container/autoledger)

<p align="center">
  <img src="docs/screenshots/dashboard-ev.fr.png" alt="Tableau de bord des coûts d'une voiture électrique" width="62%">
  <img src="docs/screenshots/mobile-quickadd.fr.png" alt="Saisie rapide d'un plein sur téléphone" width="22%">
</p>
<p align="center">
  <img src="docs/screenshots/fleet.fr.png" alt="Flotte du foyer comparant une voiture électrique et une essence" width="86%">
</p>

---

## ✨ Ce que fait AutoLedger

1. **Ajoutez un véhicule** : nom, motorisation (électrique, hybride, à prolongateur d'autonomie ou thermique), marque et modèle facultatifs, devise.
2. **Enregistrez vos dépenses** : saisissez pleins et recharges depuis le téléphone en quelques appuis, importez un CSV ([format](docs/csv-import.fr.md)), laissez Home Assistant ou un script envoyer les sessions de recharge, ou connectez une instance TeslaMate.
3. **Voyez le vrai coût au km** : énergie, péages, entretien, pneus, assurance et financement se cumulent en un seul chiffre, avec un score de complétude qui indique ce qui manque encore.

Tout reste sur votre serveur : aucun compte cloud, aucune connexion constructeur, aucune télémétrie envoyée à l'extérieur.

## 🔌 D'où viennent les données

Choisissez la source qui convient à chaque véhicule ; elles se combinent dans un même compte.

| Source | Ce qu'elle apporte | Mise en place |
| :--- | :--- | :--- |
| **Saisie manuelle / PWA** | Saisie rapide des pleins, recharges, dépenses et relevés d'odomètre, file d'attente hors ligne, photos de justificatifs | aucune |
| **Import et export CSV** | Recharges, trajets, pleins, odomètre ; profils de colonnes enregistrés ; export réimportable ; recette pour un enregistreur OBD2 | [docs/csv-import.fr.md](docs/csv-import.fr.md) |
| **Home Assistant** | Sessions de recharge d'une wallbox ou d'un compteur d'énergie envoyées automatiquement | [intégration HACS](https://github.com/Rem7474/autoledger-homeassistant) |
| **API d'ingestion** | N'importe quel script, Node-RED ou n8n qui envoie recharges, trajets, pleins et relevés d'odomètre avec un jeton | [docs/ingestion-api.fr.md](docs/ingestion-api.fr.md) |
| **Synchronisation TeslaMate** (facultative) | Import continu de l'odomètre, de l'historique de recharges et de trajets d'une Tesla déjà suivie par [TeslaMate](https://github.com/teslamate-org/teslamate) | par véhicule, dans les réglages du véhicule |

```mermaid
flowchart LR
    A["📱 Saisie manuelle / PWA"] --> Core
    B["📄 Import CSV"] --> Core
    C["🏠 Home Assistant, scripts, n8n"] --> Core
    D["🔄 Synchronisation TeslaMate (facultative)"] --> Core
    Core["Registre AutoLedger<br/>coût au km • financement • pneus • rappels • documents"]
```

- **Saisie manuelle / PWA** : application web progressive hors ligne d'abord, avec une saisie rapide pour trajets, recharges, carburant, péages et entretien, une progression intelligente de l'odomètre, des photos de justificatifs et une file d'attente IndexedDB.
- **CSV** : détection du séparateur, association des colonnes avec aperçu en direct, dédoublonnage sur les horodatages et les coordonnées.
- **Sources qui envoient** : voir l'[API d'ingestion](docs/ingestion-api.fr.md) ([EN](docs/ingestion-api.md)), les [recettes wallbox](docs/wallbox-recipes.fr.md) ([EN](docs/wallbox-recipes.md)) et l'[import et export CSV](docs/csv-import.fr.md) ([EN](docs/csv-import.md)).
- **TeslaMate** : import complet reprenable, relecture glissante sur 30 jours, rapprochement des trajets et recharges supprimés, panneaux batterie et température. Rien d'autre dans AutoLedger n'en dépend.

### Compatibilité

| Motorisation | Recharges | Pleins | Efficacité | Synchronisation TeslaMate |
| :--- | :---: | :---: | :---: | :---: |
| Électrique | ✅ | - | kWh/100 km | ✅ |
| Hybride rechargeable / prolongateur d'autonomie | ✅ | ✅ | kWh/100 km et L/100 km | - |
| Thermique | - | ✅ | L/100 km | - |

TeslaMate suit des Tesla, toutes électriques : la synchronisation est donc proposée pour les véhicules électriques. Toutes les autres sources (saisie manuelle, CSV, Home Assistant, API d'ingestion) fonctionnent pour toutes les motorisations.

### Comparatif de véhicules

Comparez un véhicule suivi (électrique, hybride rechargeable ou à prolongateur d'autonomie) à un véhicule thermique configurable, ou l'inverse. Le comparatif s'appuie sur les coûts au kilomètre enregistrés ; les hybrides incluent carburant et électricité, et les variations de prix du carburant et de l'électricité s'appliquent séparément. Voir [Comparatif de véhicules](docs/vehicle-comparison.fr.md) ([EN](docs/vehicle-comparison.md)) pour les hypothèses et les limites, dont l'annualisation des primes d'assurance.

### Tarifs d'électricité

Un véhicule peut porter une [grille tarifaire](docs/tariffs.fr.md) ([EN](docs/tariffs.md)) à prix unique, heures pleines / heures creuses ou à plusieurs tranches. Une recharge sans coût, venue de Home Assistant, d'un script ou du formulaire de recharge, est chiffrée d'après ses heures de début et de fin dans le fuseau horaire de l'instance, au-delà de minuit et entre les tranches.

### Captures d'écran

<p align="center">
  <img src="docs/screenshots/energy.fr.png" alt="Page Énergie d'une voiture électrique : efficacité, recharges et estimation" width="48%">
  <img src="docs/screenshots/energy-fuel.fr.png" alt="Pleins d'une voiture essence sur la page Énergie" width="48%">
</p>
<p align="center">
  <img src="docs/screenshots/expenses.fr.png" alt="Dépenses d'un véhicule" width="48%">
  <img src="docs/screenshots/maintenance.fr.png" alt="Rappels et historique d'entretien" width="48%">
</p>
<p align="center">
  <img src="docs/screenshots/tires.fr.png" alt="Jeux de pneus et usure" width="48%">
  <img src="docs/screenshots/drives.fr.png" alt="Trajets et déplacements" width="48%">
</p>

<p align="center">
  <img src="docs/screenshots/comparison.fr.png" alt="Comparatif sur cinq ans d'une voiture électrique et d'une essence" width="48%">
</p>

Les captures anglaises se trouvent à côté des françaises (`*.en.png`) ; `scripts/screenshots/capture.mjs` les régénère toutes depuis une instance de démonstration alimentée par des données de test.

---

## 🌟 Points forts et fonctionnalités

### 📊 Calcul du coût total de possession et financement flexible
- **Registre de coûts unifié (`cost_ledger`)** : centralise toutes les dépenses (énergie électrique ou carburant, péages, factures d'entretien, primes d'assurance, amortissement des pneus et acquisition du véhicule).
- **Modèles de financement complets** :
  - *Achat comptant* : dépréciation linéaire d'après une estimation de valeur résiduelle ou le prix de revente réel.
  - *Crédit classique* : suivi du tableau d'amortissement avec la part de capital et d'intérêts, les frais de dossier et l'assurance emprunteur.
  - *Location (LOA / LLD)* : apport, mensualités, dépôt de garantie, kilométrage contractuel et provision pour les kilomètres excédentaires.
- **Indicateurs avancés** : coût réel au km (énergie + péages ou coût total de possession complet), coût net tenant compte des revenus de covoiturage et score adaptatif de complétude du coût total.
- **Analyse énergétique** (véhicules qui se rechargent) : consommation réelle en kWh/100 km, efficacité des recharges à domicile, AC et DC, corrélation avec la température de la batterie et impact du froid.
- **Santé de la batterie et valeur résiduelle** : état de santé indépendant de la source (relevés OBD2 ou garage, TeslaMate, ou estimé à partir de recharges complètes) et projection de la valeur de revente construite sur votre prix d'achat et votre revente prévue, ajustée selon l'âge, la distance et la santé de la batterie.

### 🛞 Gestion du cycle de vie des pneus
- **Suivi par essieu** : montez, permutez et démontez les pneus entre les positions (`FL`, `FR`, `RL`, `RR`, `STORAGE`, `DISPOSED`) avec un journal chronologique des sessions.
- **Usure et projections de kilométrage** : historique des mesures de profondeur de sculpture et projection automatique du kilométrage sûr restant (les périodes de stockage sont exclues automatiquement).
- **Caractéristiques universelles** : marque, modèle, dimension ISO, indices de charge et de vitesse, saison (été, hiver, 4 saisons), code DOT de fabrication et prix d'achat.

### 👥 Module de covoiturage équitable
- **Répartition des trajets** : rattachez les tronçons à des trajets de télémétrie réels, à des trajets CSV importés ou à des saisies manuelles.
- **Calcul de la part équitable** : prix réel de l'électricité pondéré sur les recharges récentes, combiné à une répartition consolidée de l'assurance au kilomètre.

### 🔔 Rappels d'entretien et notifications homelab
- **Double déclencheur** : notifications anticipées fondées sur des échéances de date et/ou de kilométrage calculées d'après l'odomètre réel.
- **Dates fixes** : rattachez un rappel à un jour du calendrier (pneus hiver le 1er novembre), une fois ou chaque année, au lieu d'un intervalle ; le marquer fait dans sa fenêtre d'anticipation reporte un rappel annuel à l'année suivante.
- **À partir d'une intervention passée** : liez un rappel à un entretien déjà enregistré ; sa date et son odomètre deviennent le point de départ et suivent l'enregistrement s'il est modifié. Terminer un rappel avec une dépense lie le nouvel enregistrement.
- **Vos propres modèles** : aucun plan constructeur n'est intégré. Enregistrez les rappels d'un véhicule comme modèle, appliquez-le à un autre véhicule et voyez l'intervalle que vous suivez réellement une fois un rappel terminé deux fois.
- **Notifications multicanal** : intégrations natives pour **Discord** (embeds enrichis), **Telegram** (API bot en Markdown), **Gotify** (notifications push) et **webhooks JSON génériques** (Home Assistant, Node-RED, n8n).

### 🔐 Authentification hybride et sécurité homelab
- **Authentification locale** : mots de passe hachés avec bcrypt, jetons d'accès JWT HS256 de 15 minutes et cookies de rafraîchissement `HttpOnly` de 30 jours avec rotation.
- **Intégration OIDC / SSO** : authentification unique avec Authentik, Keycloak, Authelia ou Kanidm par le flux Authorization Code avec PKCE standard.
- **Sécurité et durcissement** : chiffrement AES-256-GCM des identifiants stockés, limitation de débit contre la force brute, en-têtes de sécurité (CSP, HSTS, X-Frame-Options) et reverse proxies de confiance configurables.

### 🏠 Intégration Home Assistant
- **Intégration HACS officielle** : [Rem7474/autoledger-homeassistant](https://github.com/Rem7474/autoledger-homeassistant) détecte les sessions de recharge de votre wallbox ou compteur d'énergie et les envoie à AutoLedger, avec un délai antirebond configurable pour la recharge solaire qui se met en pause puis reprend.
- **Multi-véhicule** : un même chargeur peut servir plusieurs voitures, attribuées à un véhicule fixe, à un `input_select`, par corrélation automatique, ou laissées non attribuées pour être qualifiées plus tard dans l'interface web.
- **Capteurs et services** : coût de la dernière recharge et coût aux 100 km par véhicule, ainsi que les services `autoledger.sync` et `autoledger.submit_charge`.
- Elle dialogue avec l'[API d'ingestion](#-doù-viennent-les-données) au moyen d'un jeton `al_live_` ; tout autre système peut utiliser la même API.

### 📁 Archivage des documents et factures
- Les pièces jointes (factures PDF, justificatifs, cartes grises) sont stockées de façon sécurisée sur un volume dédié (`/data/documents`), avec isolation sans droits root et autorisation JWT stricte.

---

## 📁 Architecture et structure du projet

```text
AutoLedger/
├── cmd/
│   └── server/
│       └── main.go                 # Point d'entrée, routage chi et arrêt propre
├── internal/
│   ├── apierror/                   # Erreurs et messages d'API codés et traduisibles
│   ├── auth/                       # Hachage bcrypt, rotation des jetons JWT et client OIDC SSO
│   ├── config/                     # Chargement de la configuration par variables d'environnement, avec repli sur les anciens noms
│   ├── crypto/                     # Chiffrement symétrique AES-256-GCM
│   ├── database/                   # Pool de connexions pgx, exécution des migrations et dépôts
│   ├── demodata/                   # Données de la démo en lecture seule
│   ├── geocode/                    # Géocodage inverse facultatif des coordonnées envoyées
│   ├── handlers/                   # Contrôleurs de l'API REST et pipeline d'import CSV
│   ├── middleware/                 # En-têtes de sécurité, limitation de débit, proxies de confiance, CORS
│   ├── models/                     # Modèles de données typés (véhicules, trajets, coût total, pneus)
│   ├── money/                      # Montants en centimes entiers
│   ├── servertext/                 # Textes anglais / français des webhooks et des notes enregistrées
│   ├── services/                   # Moteur de coût total, projection d'usure des pneus, calcul du covoiturage, webhooks
│   ├── storage/                    # Stockage des pièces jointes sur des volumes Docker
│   ├── teslamate/                  # Client de la synchronisation TeslaMate facultative
│   └── tolldata/                   # Données de référence des péages d'autoroute français
├── migrations/                     # Migrations versionnées du schéma PostgreSQL
├── web/                            # SPA et PWA Vue 3 + TypeScript + Vite + Tailwind CSS
├── backup/                         # Image du conteneur de sauvegarde (dump de la base et archive des documents)
├── docs/                           # Guides (anglais et français) et captures d'écran
├── docker-compose.yml              # Définition de la pile de production
├── docker-compose.dev.yml          # Surcharge de développement local avec rechargement à chaud
├── Dockerfile                      # Build de production multi-étapes
└── .env.example                    # Modèle complet des variables d'environnement
```

---

## 🛠️ Déploiement et démarrage rapide

### Option 1 : Docker Compose (recommandé)

1. **Téléchargez le fichier compose :**
   ```bash
   mkdir autoledger && cd autoledger
   curl -O https://raw.githubusercontent.com/Rem7474/AutoLedger/main/docker-compose.yml
   ```

2. **Démarrez la pile :**
   ```bash
   docker compose up -d
   ```

Aucune configuration n'est requise. Le mot de passe de la base, le secret de signature des sessions et la clé de chiffrement des identifiants sont générés au premier démarrage et conservés dans le volume `autoledger_secrets` (le service de sauvegarde l'archive à côté du dump de la base, sous le nom `autoledger-secrets-<horodatage>.tar.gz`). Conservez-le avec vos sauvegardes de la base : la clé de chiffrement est nécessaire pour relire les identifiants stockés. Hors Docker Compose (`docker run` simple), l'image génère elle-même le secret de session et la clé de chiffrement dans `.autoledger-secrets.json` sur le volume des documents. Pour gérer une valeur vous-même, définissez `AUTOLEDGER_DB_PASSWORD`, `AUTOLEDGER_JWT_SECRET` ou `AUTOLEDGER_ENCRYPTION_KEY` dans un fichier `.env` (modèle : `.env.example`) ; une valeur explicite l'emporte toujours. Les réglages facultatifs, comme OIDC / SSO, se placent aussi dans `.env`.

L'application est servie sur **`http://localhost:8080`**.

---

### Option 2 : Docker Run autonome

Des images multi-architectures (`linux/amd64`, `linux/arm64`) sont publiées sur le GitHub Container Registry :

```bash
docker pull ghcr.io/rem7474/autoledger:latest
```

Exemple d'exécution autonome vers une base PostgreSQL existante :

```bash
docker run -d \
  --name autoledger-app \
  -p 8080:8080 \
  -v autoledger_documents:/data/documents \
  -e ENVIRONMENT="production" \
  -e AUTOLEDGER_DATABASE_URL="postgres://autoledger:secret@postgres-host:5432/autoledger?sslmode=disable" \
  -e AUTOLEDGER_JWT_SECRET="your_strong_jwt_secret" \
  -e AUTOLEDGER_ENCRYPTION_KEY="hex_key_of_exactly_64_characters" \
  -e APP_TIMEZONE="Europe/Paris" \
  ghcr.io/rem7474/autoledger:latest
```

---

## 🔄 Migrer depuis TeslaCost

Un déploiement TeslaCost existant se met à jour sur place : aucune migration de données et aucune copie manuelle de volume. L'image est `ghcr.io/rem7474/autoledger`, et toutes les variables TeslaCost (voir la colonne « ancien nom » de la référence des variables d'environnement ci-dessous) sont toujours lues, y compris `TESLACOST_VERSION`.

Les nouvelles installations créent des volumes `autoledger_*`. Les installations existantes gardent leurs données en faisant pointer le fichier compose vers les volumes qu'elles ont déjà :

| Données | Nouveau volume par défaut | Volume TeslaCost | Variable de surcharge |
|---|---|---|---|
| PostgreSQL | `autoledger_db_data` | `postgres_data` | `DB_VOLUME_NAME` |
| Documents | `autoledger_documents` | `teslacost_documents` | `DOCUMENTS_VOLUME_NAME` |
| Sauvegardes | `autoledger_backups` | `teslacost_backups` | `BACKUPS_VOLUME_NAME` |
| Secrets générés (mot de passe de la base, secret de session, clé de chiffrement) | `autoledger_secrets` | - | `SECRETS_VOLUME_NAME` |

```dotenv
DB_VOLUME_NAME=postgres_data
DOCUMENTS_VOLUME_NAME=teslacost_documents
BACKUPS_VOLUME_NAME=teslacost_backups

# Le rôle et le nom de base stockés dans le volume existant :
AUTOLEDGER_DB_USER=teslacost
AUTOLEDGER_DB_NAME=teslacost
```

Puis récupérez l'image et redémarrez :
```bash
docker compose pull && docker compose up -d
```
Véhicules, recharges, dépenses, factures et réglages restent inchangés.

---

## 🛡️ Reverse proxy et durcissement en production

AutoLedger ne termine pas le TLS : placez-le derrière un reverse proxy moderne (Caddy, Traefik, Nginx) avec un certificat SSL.

Exemple de configuration avec **Caddy** (`Caddyfile`) :

```caddyfile
autoledger.homelab.local {
    reverse_proxy localhost:8080
}
```

- **Proxies de confiance** : `TRUSTED_PROXIES` garantit que les en-têtes `X-Forwarded-*` ne sont pris en compte que depuis votre reverse proxy (par défaut, la boucle locale et les réseaux privés LAN/Docker).
- **En-têtes de sécurité** : CSP stricte intégrée (`Content-Security-Policy`), HSTS sur HTTPS et `X-Frame-Options: DENY`.
- **Contrôle de sécurité en production** : avec `ENVIRONMENT=production`, l'application refuse de démarrer si elle détecte des mots de passe ou secrets par défaut du dépôt. Un secret JWT ou une clé de chiffrement non définis sont générés puis conservés.

---

## 🛟 Exploitation : sauvegardes automatiques et restauration

### Sauvegardes automatiques
Le conteneur annexe `backup` tourne en continu à côté de la base et de l'application. Toutes les `BACKUP_INTERVAL_HOURS` heures (24 h par défaut), il produit un dump PostgreSQL compressé et une archive du volume des pièces jointes :

```bash
# Lister les archives de sauvegarde
docker compose exec backup ls -lh /backups

# Consulter les journaux de sauvegarde
docker compose logs -f backup
```

### Restaurer une sauvegarde
```bash
# 1. Extraire le dump de la base depuis le volume de sauvegarde
docker compose cp backup:/backups/autoledger-db-<horodatage>.sql.gz .

# 2. Restaurer la base PostgreSQL
gunzip -c autoledger-db-<horodatage>.sql.gz | docker compose exec -T postgres psql -U "${AUTOLEDGER_DB_USER:-autoledger}" -d "${AUTOLEDGER_DB_NAME:-autoledger}"

# 3. Restaurer les pièces jointes
docker compose cp backup:/backups/autoledger-documents-<horodatage>.tar.gz .
docker run --rm \
  -v autoledger_documents:/data \
  -v "$(pwd)":/backup \
  alpine sh -c "cd /data && tar -xzf /backup/autoledger-documents-<horodatage>.tar.gz --strip-components=1"
```

---

## ⚙️ Référence des variables d'environnement

| Variable (principale) | Ancien nom | Description | Valeur par défaut |
|---|---|---|---|
| `AUTOLEDGER_PORT` | `PORT` | Port d'écoute du serveur HTTP | `8080` |
| `ENVIRONMENT` | - | Environnement d'exécution (`production`, `development`) | `production` dans le fichier Compose, `development` si non défini ailleurs |
| `AUTOLEDGER_VERSION` | `TESLACOST_VERSION` | Étiquette de l'image Docker à déployer | `latest` |
| `AUTOLEDGER_BASE_URL` | `APP_BASE_URL` | URL publique canonique de l'application | `http://localhost:8080` |
| `AUTOLEDGER_DATABASE_URL`| `DATABASE_URL` | URL de connexion PostgreSQL complète | *Dérivée de DB_\** |
| `AUTOLEDGER_DB_HOST` | `DB_HOST` | Hôte PostgreSQL (lorsqu'il est défini, l'URL est construite à partir des variables `DB_*`) | *Non défini* |
| `AUTOLEDGER_DB_PORT` | `DB_PORT` | Port PostgreSQL | `5432` |
| `AUTOLEDGER_DB_USER` | `DB_USER` | Utilisateur PostgreSQL | `autoledger` |
| `AUTOLEDGER_DB_PASSWORD` | `DB_PASSWORD` | Mot de passe PostgreSQL (`DB_PASSWORD_FILE` le lit depuis un fichier) | *Généré* |
| `AUTOLEDGER_DB_NAME` | `DB_NAME` | Nom de la base PostgreSQL | `autoledger` |
| `AUTOLEDGER_ENCRYPTION_KEY`| `APP_ENCRYPTION_KEY` | Clé AES-256 de 32 octets pour les identifiants sensibles | *Générée* |
| `AUTOLEDGER_JWT_SECRET` | `JWT_SECRET` | Clé secrète de signature des sessions utilisateur | *Généré* |
| `AUTOLEDGER_STORAGE_DIR` | `STORAGE_DIR` | Chemin du système de fichiers pour les pièces jointes | `/data/documents` |
| `APP_TIMEZONE` | - | Fuseau horaire IANA pour les rapports et agrégations | `Europe/Paris` |
| `GEOCODING_ENABLED` | - | Résoudre en adresses, par OpenStreetMap Nominatim, les coordonnées envoyées par les intégrations (trajets Home Assistant). Désactivé par défaut : les coordonnées sont transmises au service de géocodage | `false` |
| `GEOCODING_URL` / `GEOCODING_USER_AGENT` | - | Instance Nominatim à utiliser (une instance auto-hébergée évite d'envoyer les coordonnées à un tiers) et User-Agent qui lui est envoyé | Instance OSM publique / `AutoLedger (+https://github.com/Rem7474/AutoLedger)` |
| `DISABLE_REGISTRATION` | - | Définir à `true` pour désactiver l'inscription publique | `false` |
| `AUTOLEDGER_DEMO` | - | Démo publique en lecture seule : toute écriture sauf la connexion est refusée, l'inscription est fermée | `false` |
| `AUTOLEDGER_DEMO_EMAIL` / `AUTOLEDGER_DEMO_PASSWORD` | - | Compte de démo proposé par le bouton « Essayer la démo » de la page de connexion (public par conception) | - |
| `INITIAL_ADMIN_EMAIL` | - | E-mail de l'administrateur préconfiguré | *Facultatif* |
| `INITIAL_ADMIN_PASSWORD` | - | Mot de passe de l'administrateur préconfiguré | *Facultatif* |
| `CORS_ALLOWED_ORIGINS` | - | Origines, séparées par des virgules, autorisées à appeler l'API depuis un navigateur, en plus de l'URL publique | Dérivées de `AUTOLEDGER_BASE_URL` |
| `TRUSTED_PROXIES` | - | Liste blanche CIDR des reverse proxies pour déterminer l'IP du client | Plages privées |
| `SECURITY_HEADERS` | - | Activer les en-têtes de sécurité HTTP intégrés | `true` |
| `CONTENT_SECURITY_POLICY`| - | Remplacer la politique CSP (`off` pour la désactiver) | Stricte intégrée |
| `BACKUP_INTERVAL_HOURS` | - | Intervalle entre deux archives de sauvegarde automatiques | `24` |
| `BACKUP_RETENTION_DAYS` | - | Nombre de jours de conservation des archives avant suppression | `14` |

Chaque variable principale l'emporte lorsque les deux sont définies ; l'ancien nom n'est lu que si la variable principale est vide. Les anciens noms sont ceux des déploiements TeslaCost : un `.env` existant continue donc de fonctionner sans changement.

### Configuration OIDC / SSO (facultative)

| Variable | Description | Exemple |
|---|---|---|
| `OIDC_ISSUER_URL` | URL de découverte OpenID Connect du fournisseur d'identité | `https://auth.homelab.local/application/o/autoledger/` |
| `OIDC_CLIENT_ID` | Identifiant du client OAuth2 | `autoledger` |
| `OIDC_CLIENT_SECRET` | Secret du client OAuth2 | `your_secret_from_idp` |
| `OIDC_REDIRECT_URL` | URI de redirection enregistrée | `https://autoledger.homelab.local/api/auth/oidc/callback` |
| `OIDC_PROVIDER_NAME` | Nom affiché sur l'écran de connexion | `Authentik` / `Keycloak` |
| `OIDC_SCOPES` | Scopes OAuth2 demandés | `openid email profile` |
| `OIDC_ALLOWED_EMAILS` | Liste blanche d'e-mails autorisés, séparés par des virgules | `user@example.com` |
| `OIDC_DISABLE_LOCAL_AUTH` | Désactiver la connexion locale par e-mail et mot de passe | `false` |

---

## 🧪 Développement et tests

```bash
# Lancer les tests unitaires du backend
go test -v ./...

# Lancer les tests d'intégration du backend avec PostgreSQL
docker run -d --name autoledger-test-pg -e POSTGRES_USER=autoledger -e POSTGRES_PASSWORD=test -e POSTGRES_DB=autoledger_test -p 55432:5432 postgres:16-alpine
TEST_DATABASE_URL="postgres://autoledger:test@localhost:55432/autoledger_test?sslmode=disable" go test -v ./...

# Lancer les tests unitaires du frontend et la vérification des types
cd web && npm test && npm run typecheck

# Construire le bundle de production du frontend
cd web && npm run build
```

Voir [CONTRIBUTING.md](CONTRIBUTING.md) pour signaler un bug, proposer une fonctionnalité et ouvrir une pull request.

---

## 📄 Licence

Distribué sous licence [MIT](LICENSE).

Les données de calcul des péages d'autoroute (`internal/tolldata`) proviennent d'[OpenTollData](https://github.com/louis2038/OpenTollData), sous licence [ODbL-1.0](https://opendatacommons.org/licenses/odbl/1-0/).
