#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root: sudo $0"
  exit 1
fi

SSH_PORT="${SSH_PORT:-22}"
K3S_CHANNEL="${K3S_CHANNEL:-stable}"

apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y \
  ca-certificates curl gnupg git openssl ufw fail2ban jq

install -d -m 0755 /etc/apt/keyrings
if ! command -v helm >/dev/null 2>&1; then
  curl -fsSL https://packages.buildkite.com/helm-linux/helm-debian/gpgkey | gpg --dearmor -o /etc/apt/keyrings/helm.gpg
  echo "deb [signed-by=/etc/apt/keyrings/helm.gpg] https://packages.buildkite.com/helm-linux/helm-debian/any/ any main" > /etc/apt/sources.list.d/helm-stable-debian.list
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y helm
fi

cat >/etc/fail2ban/jail.d/sshd.local <<EOF
[sshd]
enabled = true
port = ${SSH_PORT}
maxretry = 5
findtime = 10m
bantime = 1h
EOF
systemctl enable --now fail2ban
systemctl restart fail2ban

ufw --force reset
ufw default deny incoming
ufw default allow outgoing
ufw default allow routed
ufw allow "${SSH_PORT}"/tcp comment "SSH"
ufw allow 80/tcp comment "HTTP ingress"
ufw allow 443/tcp comment "HTTPS ingress"
ufw allow in on cni0 comment "k3s pods"
ufw allow in on flannel.1 comment "k3s flannel"
ufw route allow in on cni0 comment "k3s routed pods"
ufw route allow in on flannel.1 comment "k3s routed flannel"
ufw --force enable

if ! command -v k3s >/dev/null 2>&1; then
  curl -sfL https://get.k3s.io | INSTALL_K3S_CHANNEL="${K3S_CHANNEL}" INSTALL_K3S_EXEC="server --disable traefik --write-kubeconfig-mode 644" sh -
fi

export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
kubectl wait --for=condition=Ready node --all --timeout=180s

helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx >/dev/null
helm repo update >/dev/null
kubectl create namespace ingress-nginx --dry-run=client -o yaml | kubectl apply -f -
kubectl label namespace ingress-nginx app.kubernetes.io/name=ingress-nginx --overwrite
helm upgrade --install ingress-nginx ingress-nginx/ingress-nginx \
  --namespace ingress-nginx \
  --set controller.kind=DaemonSet \
  --set controller.hostNetwork=true \
  --set controller.dnsPolicy=ClusterFirstWithHostNet \
  --set controller.service.enabled=false \
  --set controller.ingressClassResource.default=true \
  --set controller.watchIngressWithoutClass=true

kubectl -n ingress-nginx rollout status daemonset/ingress-nginx-controller --timeout=180s

install -d -m 0755 /opt/foodsea/backend

echo "VM bootstrap complete."
echo "Open host ports:"
ufw status numbered
echo "Cluster:"
kubectl get nodes -o wide
