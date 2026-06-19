#!/bin/bash
# /opt/myfinance/backup.sh — run via cron at 2am daily:
#   0 2 * * * /opt/myfinance/backup.sh >> /opt/myfinance/logs/backup.log 2>&1
set -euo pipefail

DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR=/opt/myfinance/backups
BACKUP_FILE="myfinance_backup_$DATE.gz"
mkdir -p "$BACKUP_DIR"

# Dump MongoDB from the running container
docker exec myfinance-mongo mongodump \
  --archive --gzip \
  --uri="mongodb://localhost:27017" \
  > "$BACKUP_DIR/$BACKUP_FILE"

# Optional: upload to S3-compatible object storage (Hetzner)
# aws s3 cp "$BACKUP_DIR/$BACKUP_FILE" \
#   "s3://myfinance-backups/$BACKUP_FILE" \
#   --endpoint-url https://fsn1.your-objectstorage.com

# Keep only the last 30 days locally
find "$BACKUP_DIR" -name "*.gz" -mtime +30 -delete

echo "Backup complete: $BACKUP_FILE"
