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

echo "==> Installing Docker Engine + Compose plugin"
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
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

cat <<EOF

==> Server bootstrap complete.

Next steps (run as $DEPLOY_USER, or via scp from your machine):
  1. Copy docker-compose.prod.yml and Caddyfile to $APP_DIR
  2. Copy .env.production.example to $APP_DIR/.env and fill in real secrets
     chmod 600 $APP_DIR/.env
  3. If GHCR packages are private:
       docker login ghcr.io -u <github-user> -p <PAT with read:packages>
  4. First deploy:
       cd $APP_DIR
       docker compose -f docker-compose.prod.yml pull
       docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/migrate backend up
       docker compose -f docker-compose.prod.yml up -d
  5. Point DNS A-records for WEB_DOMAIN/ADMIN_DOMAIN/API_DOMAIN at this server's IP
     before step 4, so Caddy can issue Let's Encrypt certificates on first boot.
EOF
