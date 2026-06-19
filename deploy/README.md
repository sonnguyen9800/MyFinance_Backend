# Deployment

## Local / self-host (one command)

```bash
cp .env.example .env        # then edit JWT_SECRET, ALLOWED_ORIGINS, DATABASE_NAME
docker compose up --build
```

- API: http://localhost:8080
- Mongo admin (dev only): http://localhost:8081 (admin / admin)

## Production (Hetzner VPS)

1. Provision a CX22 (€4.35/mo), point `api.yourdomain` DNS to it.
2. Server setup:
   ```bash
   apt update && apt install -y docker.io docker-compose-plugin nginx certbot python3-certbot-nginx ufw
   ufw allow 22,80,443/tcp && ufw enable
   ```
3. Copy `deploy/nginx.conf` to `/etc/nginx/sites-available/myfinance-backend`, symlink, then:
   ```bash
   certbot --nginx -d api.yourdomain
   ```
4. Add the rate-limit zones (see top of nginx.conf) to `/etc/nginx/nginx.conf`.
5. Set env in `/opt/myfinance/.env` and run:
   ```bash
   docker compose -f docker-compose.prod.yml up -d
   ```
6. Schedule backups: `crontab -e` → `0 2 * * * /opt/myfinance/backup.sh`

## Purge the leaked debug binary from git history

The 18MB `__debug_bin*.exe` was committed previously. Remove it from history:

```bash
# Using BFG (recommended)
bfg --delete-files '__debug_bin*.exe'
git reflog expire --expire=now --all && git gc --prune=now --aggressive
git push --force
```
