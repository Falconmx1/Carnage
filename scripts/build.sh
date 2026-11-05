#!/usr/bin/env bash
# build.sh - Compila Carnage para múltiples plataformas en un solo binario estático.
# Uso: ./scripts/build.sh [version]

set -euo pipefail

VERSION="${1:-dev}"
OUTDIR="dist"
LDFLAGS="-s -w -X main.Version=${VERSION}"

PLATFORMS=(
  "linux/amd64"
  "linux/arm64"
  "linux/arm/7"
  "linux/mips"
  "linux/mipsle"
  "windows/amd64"
  "windows/arm64"
  "darwin/amd64"
  "darwin/arm64"
)

rm -rf "${OUTDIR}"
mkdir -p "${OUTDIR}"

echo "[*] Compilando Carnage ${VERSION}..."
for p in "${PLATFORMS[@]}"; do
  IFS="/" read -r GOOS GOARCH GOARM <<< "${p}"
  NAME="carnage-${GOOS}-${GOARCH}"
  if [ -n "${GOARM:-}" ]; then
    NAME="${NAME}v${GOARM}"
    export GOARM
  else
    unset GOARM || true
  fi
  if [ "${GOOS}" = "windows" ]; then
    NAME="${NAME}.exe"
  fi
  echo "    -> ${NAME}"
  CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" \
    go build -trimpath -ldflags="${LDFLAGS}" -o "${OUTDIR}/${NAME}" ./cmd/carnage
done

echo "[+] Listo. Binarios en ${OUTDIR}/"
ls -lh "${OUTDIR}/"
