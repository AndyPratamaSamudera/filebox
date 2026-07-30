#!/usr/bin/env bash
#
# Create the FileBox database and application user. Reads DB credentials from
# the project-root .env so no secrets are stored in this script. Run once with
# a MariaDB admin account.
#
# This script only creates the database and grants privileges. After running it,
# apply the schema manually with:
#   mariadb -u root -p < scripts/schema.sql
#
#   sudo ./scripts/db-setup.sh        # Debian/MariaDB unix_socket auth (default)
#   ./scripts/db-setup.sh             # if your root can access the local socket
#   DBA_HOST=127.0.0.1 ./scripts/db-setup.sh   # TCP root + password prompt
#   ./scripts/db-setup.sh admin       # use a different admin user than "root"
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$SCRIPT_DIR/../.env"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "ERROR: $ENV_FILE not found." >&2
  echo "Copy .env.example to .env and fill in DB_USER/DB_PASS/DB_NAME first." >&2
  exit 1
fi

# Source .env without exposing values on the command line.
set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

DB_NAME="${DB_NAME:?DB_NAME is required in .env}"
DB_USER="${DB_USER:?DB_USER is required in .env}"
DB_PASS="${DB_PASS:?DB_PASS is required in .env}"
ADMIN_USER="${1:-root}"

# Admin connection. Default: local socket (works with `sudo` on Debian/MariaDB
# unix_socket auth). Set DBA_HOST to authenticate over TCP with a password.
if [[ -n "${DBA_HOST:-}" ]]; then
  MYSQL_CMD=(mariadb -h "$DBA_HOST" -P "${DBA_PORT:-3306}" -u "$ADMIN_USER" -p)
else
  MYSQL_CMD=(mariadb -u "$ADMIN_USER")
fi

echo "Creating database '$DB_NAME' and user '$DB_USER' (authenticating as '$ADMIN_USER')..."

"${MYSQL_CMD[@]}" <<SQL
CREATE DATABASE IF NOT EXISTS \`$DB_NAME\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS '$DB_USER'@'localhost' IDENTIFIED BY '$DB_PASS';
CREATE USER IF NOT EXISTS '$DB_USER'@'127.0.0.1' IDENTIFIED BY '$DB_PASS';
GRANT ALL PRIVILEGES ON \`$DB_NAME\`.* TO '$DB_USER'@'localhost';
GRANT ALL PRIVILEGES ON \`$DB_NAME\`.* TO '$DB_USER'@'127.0.0.1';
FLUSH PRIVILEGES;
SQL

echo "Done. Apply scripts/schema.sql manually to create the tables."
