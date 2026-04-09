#!/usr/bin/env bash
# cleanup.sh — remove todos os containers Docker (rodando e parados)
set -euo pipefail

FORCE="0"
if [[ "${1:-}" == "--force" || "${1:-}" == "-f" ]]; then
  FORCE="1"
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "[erro] docker não encontrado no PATH"
  exit 1
fi

if [[ "$FORCE" != "1" ]]; then
  echo "ATENÇÃO: este script vai remover TODOS os containers Docker desta máquina."
  read -r -p "Deseja continuar? (digite: SIM) " CONFIRM
  if [[ "$CONFIRM" != "SIM" ]]; then
    echo "Cancelado."
    exit 0
  fi
fi

ALL_IDS="$(docker ps -aq)"
if [[ -z "$ALL_IDS" ]]; then
  echo "Nenhum container para remover."
  exit 0
fi

echo "Removendo containers..."
# shellcheck disable=SC2086
docker rm -f $ALL_IDS >/dev/null

echo "Containers removidos com sucesso."
