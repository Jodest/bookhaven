#!/usr/bin/env bash
# One-time bootstrap for a fresh Ubuntu 22.04/24.04 VPS.
# Run as root: curl -fsSL <raw-url-of-this-file> | bash
# or: scp this file to the server and run it there.
set -euo pipefail

DEPLOY_USER="${DEPLOY_USER:-deploy}"
APP_DIR="/opt/bookhaven"

echo "==> Updating system packages"
apt-get update -y
apt-get upgrade -y

echo "==> Installing base tools"
apt-get install -y ca-certificates curl ufw fail2ban

echo "==> Installing backup tools (age encryption, rclone off-site upload)"
apt-get install -y age rclone

echo "==> Installing Docker Engine + Compose plugin"
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
# shellcheck disable=SC1091 # /etc/os-release exists only on the target host
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
  | tee /etc/apt/sources.list.d/docker.list > /dev/null
apt-get update -y
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

echo "==> Creating deploy user (SSH key auth only, docker group)"
if ! id "$DEPLOY_USER" &>/dev/null; then
  adduser --disabled-password --gecos "" "$DEPLOY_USER"
  usermod -aG docker "$DEPLOY_USER"
  rsync --archive --chown="$DEPLOY_USER:$DEPLOY_USER" ~/.ssh/ "/home/$DEPLOY_USER/.ssh/" 2>/dev/null || true
fi

echo "==> Hardening SSH (key auth only, no root login)"
sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
sed -i 's/^#\?PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config
systemctl reload sshd

echo "==> Configuring firewall (22, 80, 443 only)"
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable

echo "==> Enabling fail2ban"
systemctl enable --now fail2ban

echo "==> Creating app directory: $APP_DIR"
mkdir -p "$APP_DIR"
chown "$DEPLOY_USER:$DEPLOY_USER" "$APP_DIR"

echo "==> Creating backup directory: $APP_DIR/backups"
install -d -m 700 -o "$DEPLOY_USER" -g "$DEPLOY_USER" "$APP_DIR/backups"

cat <<EOF

==> Server bootstrap complete.

Next steps (run as $DEPLOY_USER, or via scp from your machine):
  1. Copy docker-compose.prod.yml, Caddyfile and infrastructure/deploy/ to $APP_DIR
     (keep the infrastructure/deploy/ path), then:
       chmod +x $APP_DIR/infrastructure/deploy/*.sh
  2. Copy .env.production.example to $APP_DIR/.env and fill in real secrets
     chmod 600 $APP_DIR/.env
     Keep an encrypted copy of .env in your password manager (see docs/DEPLOYMENT.md "Backups").
  3. If GHCR packages are private:
       docker login ghcr.io -u <github-user> -p <PAT with read:packages>
  4. First deploy:
       cd $APP_DIR
       docker compose -f docker-compose.prod.yml pull
       docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/migrate backend up
       docker compose -f docker-compose.prod.yml up -d
  5. Point DNS A-records for WEB_DOMAIN/ADMIN_DOMAIN/API_DOMAIN at this server's IP
     before step 4, so Caddy can issue Let's Encrypt certificates on first boot.
  6. Backups (docs/DEPLOYMENT.md "Backups"):
     a. Configure the off-site rclone remote (S3-compatible bucket):
          rclone config        # creates ~/.config/rclone/rclone.conf
          chmod 600 ~/.config/rclone/rclone.conf
     b. Generate the age keypair OFF this server (age-keygen -o bookhaven-backup.key),
        store the identity in your password manager, and put only the public
        age1... recipient into $APP_DIR/.env.
     c. Set the BACKUP_* keys in $APP_DIR/.env: BACKUP_RCLONE_REMOTE,
        BACKUP_AGE_RECIPIENT, BACKUP_HEARTBEAT_URL (BACKUP_LOCAL_RETENTION_DAYS optional).
     d. Run once by hand and check $APP_DIR/backups/backup.log:
          $APP_DIR/infrastructure/deploy/backup.sh
     e. Install the cron line (crontab -e), no output redirect:
          0 3 * * * $APP_DIR/infrastructure/deploy/backup.sh
EOF
