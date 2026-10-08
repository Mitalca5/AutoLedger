# Migrer depuis TeslaCost

English: [upgrading-from-teslacost.md](upgrading-from-teslacost.md).

Un déploiement TeslaCost existant se met à jour sur place : les données restent dans leurs volumes actuels. L'image est `ghcr.io/rem7474/autoledger`, et toutes les variables TeslaCost (voir la colonne « ancien nom » de la [référence des variables d'environnement](../README.fr.md)) sont toujours lues, y compris `TESLACOST_VERSION`.

Les nouvelles installations créent des volumes `autoledger_*`. Une installation existante doit nommer les volumes qu'elle possède déjà, sinon la pile démarre sur une base vide et affiche la page d'onboarding.

1. **Lister les volumes existants.** Compose préfixe le nom des volumes avec le nom du projet (le répertoire) :
   ```bash
   docker volume ls | grep -Ei "postgres_data|documents|backups"
   ```
   Pour une pile démarrée depuis un répertoire `teslacost` avec le compose TeslaCost, les volumes sont typiquement `teslacost_postgres_data`, `teslacost_teslacost_documents` et `teslacost_teslacost_backups`.

2. **Les déclarer dans `.env`**, avec les identifiants et la clé de chiffrement de la pile TeslaCost :

   | Données | Nouveau volume par défaut | Variable de surcharge |
   |---|---|---|
   | PostgreSQL | `autoledger_db_data` | `DB_VOLUME_NAME` |
   | Documents | `autoledger_documents` | `DOCUMENTS_VOLUME_NAME` |
   | Sauvegardes | `autoledger_backups` | `BACKUPS_VOLUME_NAME` |
   | Secrets générés (mot de passe de la base, secret de session, clé de chiffrement) | `autoledger_secrets` | `SECRETS_VOLUME_NAME` |

   ```dotenv
   DB_VOLUME_NAME=teslacost_postgres_data
   DOCUMENTS_VOLUME_NAME=teslacost_teslacost_documents
   BACKUPS_VOLUME_NAME=teslacost_teslacost_backups

   # Le rôle, le nom et le mot de passe de la base stockés dans le volume existant :
   DB_USER=teslacost
   DB_NAME=teslacost
   DB_PASSWORD=<votre mot de passe de base TeslaCost>

   # Les secrets stockés sont chiffrés avec cette clé : gardez la valeur utilisée par TeslaCost
   # (le défaut du compose, si elle n'a jamais été définie, est change-this-to-a-secure-32-byte-key-in-prod!).
   APP_ENCRYPTION_KEY=<votre clé TeslaCost>
   JWT_SECRET=<votre secret TeslaCost>
   ```

3. **Récupérer l'image et redémarrer.** Un volume PostgreSQL 16 est alors mis à niveau en 18 automatiquement, après une sauvegarde vérifiée (voir [Passer de PostgreSQL 16 à 18](../README.fr.md#passer-de-postgresql-16-à-18)) :
   ```bash
   docker compose pull && docker compose up -d
   docker compose logs db-upgrade db-restore
   ```

Véhicules, recharges, dépenses, factures et réglages restent inchangés.

Ne lancez jamais `docker compose down -v` ni `docker volume rm` sur ces volumes : ils contiennent les données.

## Passer aux volumes `autoledger_*` (facultatif)

Au lieu de garder les noms de volumes TeslaCost, les données peuvent être copiées dans les volumes `autoledger_*` par défaut. La copie lit les anciens volumes en lecture seule et ne les modifie pas : ils restent une solution de repli. Arrêtez d'abord la pile (sans `-v`), puis, avec les vrais noms issus de `docker volume ls` :

```bash
docker compose down
for pair in teslacost_postgres_data:autoledger_db_data \
            teslacost_teslacost_documents:autoledger_documents \
            teslacost_teslacost_backups:autoledger_backups; do
  docker run --rm -v "${pair%%:*}":/from:ro -v "${pair##*:}":/to alpine cp -a /from/. /to/
done
```

Supprimez ensuite les trois lignes `*_VOLUME_NAME` du `.env` et démarrez la pile. Le rôle et le nom de la base gardent les valeurs TeslaCost tant qu'ils ne sont pas renommés dans la dernière section, et la valeur de la clé de chiffrement reste la même ; la clé peut être définie sous `AUTOLEDGER_ENCRYPTION_KEY` au lieu de `APP_ENCRYPTION_KEY`.

## Renommer le rôle et la base (facultatif)

Le rôle et la base d'une pile TeslaCost s'appellent `teslacost`. Ils peuvent être renommés en `autoledger` une fois la pile démarrée sur les données migrées (données, propriétaires et mot de passe sont conservés). PostgreSQL ne peut pas renommer le rôle de la session courante ni une base utilisée : les commandes passent donc par un super-utilisateur temporaire :

```bash
docker compose stop api backup
export OLD=teslacost NEW=autoledger PW='<votre mot de passe de base>'
docker compose exec -T postgres psql -U "$OLD" -d postgres -v ON_ERROR_STOP=1 \
  -c "CREATE ROLE tmp_admin SUPERUSER LOGIN"
docker compose exec -T postgres psql -h 127.0.0.1 -U tmp_admin -d postgres -v ON_ERROR_STOP=1 -v pw="$PW" <<SQL
ALTER DATABASE $OLD RENAME TO $NEW;
ALTER ROLE $OLD RENAME TO $NEW;
ALTER ROLE $NEW PASSWORD :'pw';
SQL
docker compose exec -T postgres psql -h 127.0.0.1 -U "$NEW" -d postgres -c "DROP ROLE tmp_admin"
```

Supprimez ensuite `DB_USER` et `DB_NAME` (ou `AUTOLEDGER_DB_USER` et `AUTOLEDGER_DB_NAME`) du `.env`, gardez `DB_PASSWORD` avec le même mot de passe, puis lancez `docker compose up -d`.
