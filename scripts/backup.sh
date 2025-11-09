#!/bin/bash
# 备份数据库和配置

BACKUP_DIR="/root/singbox-manager-backups"
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="singbox-manager-backup-${DATE}.tar.gz"

echo "Creating backup directory..."
mkdir -p $BACKUP_DIR

echo "Backing up database and configs..."
tar -czf "${BACKUP_DIR}/${BACKUP_FILE}" \
    -C /root/github.com/firadio/golang-singbox-manager \
    data/manager.db \
    configs/config.yaml \
    configs/singbox/

if [ $? -eq 0 ]; then
    echo "Backup created successfully: ${BACKUP_DIR}/${BACKUP_FILE}"
    ls -lh "${BACKUP_DIR}/${BACKUP_FILE}"

    # 只保留最近 10 个备份
    echo ""
    echo "Cleaning old backups (keeping last 10)..."
    cd $BACKUP_DIR
    ls -t singbox-manager-backup-*.tar.gz | tail -n +11 | xargs -r rm -f

    echo ""
    echo "Current backups:"
    ls -lht singbox-manager-backup-*.tar.gz 2>/dev/null || echo "No backups found"
else
    echo "Backup failed!"
    exit 1
fi
