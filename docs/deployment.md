# FoodSea VM Deployment

This repository deploys two isolated Kubernetes environments on one VM:

- `foodsea-dev`, normally deployed from `develop`.
- `foodsea-prod`, normally deployed from `main`.

Only Nginx Ingress is exposed from the VM. Application services, PostgreSQL,
Redis, Kafka, MinIO, gRPC ports, and databases stay as `ClusterIP` services
inside Kubernetes.

## VM bootstrap

Run once on a fresh Ubuntu VM:

```bash
sudo SSH_PORT=22 ./deploy/scripts/bootstrap-vm.sh
```

The bootstrap script installs:

- k3s without Traefik;
- ingress-nginx with host networking on ports `80` and `443`;
- UFW with only SSH, HTTP, and HTTPS open;
- fail2ban with an enabled `sshd` jail.

For temporary development without buying a domain, use `nip.io` hosts:

```text
DEV_HOST=dev.<VM_IP>.nip.io
PROD_HOST=api.<VM_IP>.nip.io
```

## Manual deploy

From the VM, inside the checked-out repository:

```bash
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

FOODSEA_ENV=dev \
FOODSEA_HOST=dev.<VM_IP>.nip.io \
IMAGE_REGISTRY=ghcr.io/<owner>/foodsea-backend \
IMAGE_TAG=<git-sha> \
REGISTRY_USERNAME=<github-user> \
REGISTRY_PASSWORD=<ghcr-read-token> \
./deploy/scripts/deploy-k8s.sh

FOODSEA_ENV=prod \
FOODSEA_HOST=api.<VM_IP>.nip.io \
IMAGE_REGISTRY=ghcr.io/<owner>/foodsea-backend \
IMAGE_TAG=<git-sha> \
REGISTRY_USERNAME=<github-user> \
REGISTRY_PASSWORD=<ghcr-read-token> \
./deploy/scripts/deploy-k8s.sh
```

The deploy script creates per-environment secrets, applies the selected
Kustomize overlay, runs Atlas migrations as Kubernetes Jobs, waits for rollouts,
and smoke-checks `/api/v1/categories` through ingress.

## GitHub Actions secrets

Required repository or environment secrets:

```text
VM_HOST          VM public IP or DNS name
VM_USER          SSH user
VM_SSH_KEY       private SSH key for VM_USER
VM_SSH_PORT      optional, defaults to 22
DEV_HOST         dev API host, for example dev.<VM_IP>.nip.io
PROD_HOST        prod API host, for example api.<VM_IP>.nip.io
GHCR_READ_TOKEN  PAT with read:packages for Kubernetes image pulls
REPO_READ_TOKEN  optional PAT for private repository clone on the VM
```

Workflow behavior:

- `CI` runs Go unit tests, renders both overlays, and builds all service images.
- `CD` builds and pushes images to GHCR.
- push to `develop` deploys `foodsea-dev`;
- push to `main` deploys `foodsea-prod`;
- manual `workflow_dispatch` can deploy either environment.

## Verification

On the VM:

```bash
sudo DEV_HOST=dev.<VM_IP>.nip.io \
  PROD_HOST=api.<VM_IP>.nip.io \
  KUBECONFIG=/etc/rancher/k3s/k3s.yaml \
  ./deploy/scripts/verify-vm.sh
```

Expected checks:

- UFW is active and only SSH/80/443 are open.
- fail2ban `sshd` jail is enabled.
- ingress-nginx pods are ready.
- both `foodsea-dev` and `foodsea-prod` have ready pods, services, and ingress.
- both environments answer through ingress.
