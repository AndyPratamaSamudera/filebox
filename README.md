# FileBox

A lightweight private cloud-storage application (Google Drive / NAS style) for
home-LAN use. Backend in **Go (Fiber)**, frontend as a **vanilla JS SPA**
(HTML/CSS/ES modules) embedded into the single binary, **MariaDB** for metadata,
files on the local filesystem.

Designed to run on low-spec devices (STB, Mini PC, Intel NUC, Raspberry Pi) with
minimal RAM. No Docker, no Kubernetes — just one binary managed by systemd.

## Features

- Register / login with JWT access & refresh tokens.
- Upload files directly or chunked for large files.
- Create folders, rename, move (via path update), favorite, delete.
- Server-side AES-256-GCM disk encryption using `FILEBOX_ENCRYPTION_KEY`.
- Optional per-file password lock (previews/downloads require the password).
- Friends list and explicit file sharing.
- API keys for programmatic access (revealed after account-password verification).
- Optional admin config page (`HAS_ADMIN_PAGE=true`).
- Swagger UI at `/swagger/`.

## Project structure

```
filebox/
├── backend/      Go service (Fiber + SQLX + MariaDB)
├── frontend/     Vanilla JS SPA source (HTML/CSS/ES modules), copied into Go embed
├── scripts/      db setup, schema, systemd unit
├── storage/      file bytes (users/, trash/, temp/, chunks/, thumbnails/)
├── .env.example  single configuration template
├── Makefile
└── README.md
```

## Prerequisites

- Go 1.26+
- MariaDB 10.11+ on `127.0.0.1:3306` (or set `DB_HOST`/`DB_PORT`)
- `make`

## Quick start

### 1. Configure

```bash
cp .env.example .env
```

Edit `.env` and set at least:

- `DB_USER`, `DB_PASS`, `DB_NAME`
- `JWT_SECRET`, `JWT_REFRESH_SECRET` (long random hex strings)
- `FILEBOX_ENCRYPTION_KEY` (32-byte AES key as base64, hex, or raw 32 chars)
- `DEFAULT_STORAGE_QUOTA` (bytes, `0` = unlimited)
- `HAS_ADMIN_PAGE=true|false`

### 2. Create the database, user, and schema

```bash
./scripts/db-setup.sh        # creates DB + user
mariadb -u root -p < scripts/schema.sql
```

### 3. Build & run

```bash
make build
./bin/filebox
```

Open `http://localhost:8000`.

### 4. Develop

```bash
make dev-backend   # Go on :8000 (rebuilds on run)
```

Edit files under `frontend/static/` and copy them to
`backend/internal/web/static/`, then rebuild the backend to see changes.

## API reference

Interactive docs: `http://localhost:8000/swagger/`

All endpoints under `/api/v1` except auth require a `Bearer <access>` token or an
`X-API-Key` header.

| Method | Path | Description |
|--------|------|-------------|
| POST | `/auth/register` | Register a new account |
| POST | `/auth/login` | Login, receive tokens |
| POST | `/auth/refresh` | Refresh access token |
| POST | `/auth/logout` | Revoke current session |
| GET | `/profile` | Current user profile |
| GET | `/item/list` | List folder contents |
| GET | `/item/detail` | File/folder detail + share list |
| POST | `/item/folder` | Create a folder |
| POST | `/item/upload` | Direct file upload |
| PUT | `/item/update` | Rename, favorite, share, password |
| DELETE | `/item/delete` | Delete file/folder |
| GET | `/item/download` | Download file |
| GET | `/item/preview` | Preview file inline |
| GET | `/item/search` | Search by name |
| GET | `/item/shared` | Files shared with me |
| GET | `/item/favorites` | Favorite files |
| POST | `/upload/chunk/:session` | Upload a chunk |
| POST | `/chunk/session` | Create chunk upload session |
| POST | `/chunk/complete` | Finalize chunked upload |
| GET | `/friends` | My friends |
| POST | `/friends` | Add friend by email |
| GET | `/friend-requests` | Incoming friend requests |
| POST | `/friend-requests` | Send friend request |
| POST | `/friend-requests/:id/accept` | Accept request |
| POST | `/friend-requests/:id/reject` | Reject request |
| GET | `/api-keys` | List API keys |
| POST | `/api-keys` | Create API key |
| POST | `/api-keys/:id/reveal` | Reveal plaintext key |
| DELETE | `/api-keys/:id` | Revoke API key |
| GET | `/health` | Health check |

## Database schema

Single `filebox` database. Final tables:

- `users` — accounts, quotas, auth secrets.
- `sessions` — JWT refresh sessions.
- `friends` — one-way contact list.
- `friend_requests` — pending friend invitations.
- `items` — unified files + folders (path-based tree, `type` = `file`/`folder`).
- `item_shares` — explicit share records between users.
- `chunk_uploads` — in-progress chunked uploads.
- `api_keys` — API keys with re-revealable plaintext.
- `settings` — admin page runtime overrides.

See `scripts/schema.sql` for the full DDL.

## Upload & download flow

**Upload:**
1. Files `<= UPLOAD_MAX_DIRECT` are uploaded in one `multipart/form-data` POST to `/item/upload`.
2. Larger files create a chunk session, upload chunks to `/upload/chunk/:session`, then call `/chunk/complete`. The server assembles, encrypts, and stores as `<uuid>.item` under `<STORAGE_PATH>/users/<uid>/`.
3. `STORAGE_PATH` defaults to `storage/` relative to the working directory. You can point it to an external disk, e.g. `STORAGE_PATH=/mnt/external/filebox`.
4. Original name, extension, MIME, and size are kept in the `items` table.

**Download / preview:**
1. Server decrypts the `.item` file to a temp file using `FILEBOX_ENCRYPTION_KEY`.
2. If the file has a password hash, the supplied password is verified first.
3. The file is sent with the original filename and MIME type.

## Deployment (systemd)

Build the binary:

```bash
make build
```

Install layout:

```bash
sudo mkdir -p /opt/filebox
sudo cp bin/filebox /opt/filebox/
sudo cp .env /opt/filebox/.env
sudo chown -R filebox:filebox /opt/filebox
```

If `STORAGE_PATH` is the default `storage/`, the binary will create
`/opt/filebox/storage/` and its subdirectories on first start.  If you want
file bytes on an external disk, set `STORAGE_PATH=/mnt/external/filebox` in
`.env` and create that directory with the correct ownership before starting the
service.

Create a dedicated user:

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin filebox
```

Create the DB and apply the schema:

```bash
./scripts/db-setup.sh
mariadb -u root -p < scripts/schema.sql
```

Install and start the systemd unit:

```bash
sudo cp scripts/filebox.service /etc/systemd/system/filebox.service
sudo systemctl daemon-reload
sudo systemctl enable --now filebox
sudo systemctl status filebox
```

Unit (`scripts/filebox.service`):

```ini
[Unit]
Description=FileBox private cloud storage
After=network-online.target mariadb.service
Wants=network-online.target

[Service]
Type=simple
User=filebox
Group=filebox
WorkingDirectory=/opt/filebox
EnvironmentFile=/opt/filebox/.env
ExecStart=/opt/filebox/filebox
Restart=on-failure
RestartSec=5
LimitNOFILE=65536
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/filebox/storage   # change if STORAGE_PATH points elsewhere
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

## Reverse proxy (optional)

FileBox listens on `:8000`. For TLS / a friendly hostname, put Caddy or nginx
in front:

```
# Caddyfile
filebox.home {
    reverse_proxy localhost:8000
}
```

## Backup & restore

**Backup:**

```bash
# Database
mariadb-dump -u filebox -p filebox > filebox_$(date +%F).sql

# File bytes (replace /opt/filebox/storage with your STORAGE_PATH)
rsync -a /opt/filebox/storage/ /backup/filebox/storage/
```

**Restore:**

```bash
# Database
mariadb -u filebox -p filebox < filebox_YYYY-MM-DD.sql

# File bytes
rsync -a /backup/filebox/storage/ /opt/filebox/storage/
```

A simple cron backup:

```cron
0 3 * * * mariadb-dump -u filebox -p'YOUR_PASS' filebox > /backup/filebox/db_$(date +\%F).sql && rsync -a /opt/filebox/storage/ /backup/filebox/storage/
```

## Upgrading

```bash
make build
sudo systemctl stop filebox
sudo cp bin/filebox /opt/filebox/filebox
# Apply any schema changes manually (scripts/schema.sql or ALTER statements)
sudo systemctl start filebox
```

## License

Personal project.
