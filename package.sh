#!/bin/bash
# Packaging vm-monitoring en 1 commande : vide et remplit delivery/ avec
# tout ce qu'il faut pour la prod (images podman taguées + configs + HOWTO).
#
# Usage :
#   ./package.sh [TAG] [--skip-tests]
#   TAG par défaut : date UTC (ex. 20260911-1030). Exemples :
#     ./package.sh            # tag auto
#     ./package.sh v1.2.0     # tag manuel
#
# Contenu généré dans delivery/ :
#   vm-monitoring-backend.tar.gz / vm-monitoring-frontend.tar.gz (podman save)
#   docker-compose.yml (références :TAG), config.yaml, hypervisors.yaml,
#   HOWTO.md, VERSION (tag + date + sha git).
# Sources : Dockerfiles du repo + packaging/*. Les tars sont ignorés par git
# (voir .gitignore) : delivery/ se régénère à volonté avec ce script.
set -euo pipefail

TAG="${1:-$(date -u +%Y%m%d-%H%M)}"
if [[ "${1:-}" == "--skip-tests" ]]; then
  TAG="$(date -u +%Y%m%d-%H%M)"
  SKIP_TESTS=1
else
  SKIP_TESTS=0
fi
if [[ "${2:-}" == "--skip-tests" ]]; then
  SKIP_TESTS=1
fi

cd "$(dirname "$0")"
BACKEND_IMG="localhost/vm-monitoring-backend:${TAG}"
FRONTEND_IMG="localhost/vm-monitoring-frontend:${TAG}"
# Flags requis par le vieux podman rootless de cette machine (crun + format).
BUILD_FLAGS=(--isolation=chroot --format=docker)

command -v podman >/dev/null || { echo "podman introuvable" >&2; exit 1; }
command -v gzip >/dev/null || { echo "gzip introuvable" >&2; exit 1; }

if [[ "$SKIP_TESTS" -eq 0 ]]; then
  echo "==> tests backend"
  export PATH="$HOME/golang_1.22/go/bin:$PATH"
  (cd backend && go test ./... >/dev/null) || { echo "tests backend en échec" >&2; exit 1; }
else
  echo "==> tests sautés (--skip-tests)"
fi

echo "==> build image backend : ${TAG}"
podman build "${BUILD_FLAGS[@]}" -t "${BACKEND_IMG}" . >/dev/null

echo "==> build image frontend : ${TAG}"
podman build "${BUILD_FLAGS[@]}" -t "${FRONTEND_IMG}" ./frontend >/dev/null

echo "==> remplit delivery/"
rm -rf delivery
mkdir -p delivery
podman save "${BACKEND_IMG}" | gzip > delivery/vm-monitoring-backend.tar.gz
podman save "${FRONTEND_IMG}" | gzip > delivery/vm-monitoring-frontend.tar.gz
sed "s/@@TAG@@/${TAG}/g" packaging/docker-compose.yml > delivery/docker-compose.yml
cp packaging/config.yaml packaging/hypervisors.yaml packaging/HOWTO.md delivery/
GIT_SHA="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
{
  echo "TAG=${TAG}"
  echo "DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "GIT=${GIT_SHA}"
} > delivery/VERSION

echo "==> vérifie"
gzip -t delivery/vm-monitoring-backend.tar.gz
gzip -t delivery/vm-monitoring-frontend.tar.gz
grep -q "${TAG}" delivery/docker-compose.yml
ls -la delivery/

echo "OK : delivery/ prêt (tag ${TAG})"
