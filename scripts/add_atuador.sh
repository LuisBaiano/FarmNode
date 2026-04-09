#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════════
# add_atuador.sh — Sobe N instâncias de um tipo de atuador para um nó
#
# Uso:
#   ./add_atuador.sh <tipo> <node_id> <quantidade> [SERVER_ADDR]
#
# Exemplos:
#   ./add_atuador.sh bomba Estufa_A 2
#   ./add_atuador.sh ventilador Estufa_A 1 192.168.1.100:6000
#   ./add_atuador.sh exaustor Galinheiro_A 3 192.168.1.100:6000
#
# Tipos válidos: bomba | ventilador | led | exaustor | aquecedor | motor | valvula
# O atuador_id gerado será: <tipo>_<node>_<sufixo>
# ═══════════════════════════════════════════════════════════════════════════════

set -e

TIPO="${1}"
NODE="${2}"
QTD="${3:-1}"
SERVER_ADDR="${4:-localhost:6000}"

# ── Validações ────────────────────────────────────────────────────────────────
if [[ -z "$TIPO" || -z "$NODE" ]]; then
  echo "Uso: $0 <tipo> <node_id> <quantidade> [SERVER_ADDR]"
  echo "Tipos: bomba | ventilador | led | exaustor | aquecedor | motor | valvula"
  exit 1
fi

TIPOS_VALIDOS="bomba ventilador led exaustor aquecedor motor valvula"
if ! echo "$TIPOS_VALIDOS" | grep -qw "$TIPO"; then
  echo "Erro: tipo '$TIPO' inválido. Válidos: $TIPOS_VALIDOS"
  exit 1
fi

if ! [[ "$QTD" =~ ^[0-9]+$ ]] || [ "$QTD" -lt 1 ]; then
  echo "Erro: quantidade deve ser um número inteiro positivo"
  exit 1
fi

# ── Imagem ────────────────────────────────────────────────────────────────────
IMAGE="farmnode_client"
if ! docker image inspect "$IMAGE" &>/dev/null; then
  echo "Construindo imagem $IMAGE..."
	ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
	docker build -t "$IMAGE" -f "$ROOT_DIR/cmd/client/Dockerfile" "$ROOT_DIR"
fi

# ── Subir containers ──────────────────────────────────────────────────────────
NODE_SLUG=$(echo "$NODE" | tr '[:upper:]' '[:lower:]' | tr '_' '-')
SUFIXO=$(date +%s)

echo "Subindo $QTD atuador(es) '$TIPO' para nó '$NODE' -> servidor $SERVER_ADDR"

for i in $(seq 1 "$QTD"); do
  ATUADOR_ID="${TIPO}_${NODE_SLUG}_${SUFIXO}_${i}"
  NOME="atuador_${NODE_SLUG}_${TIPO}_${SUFIXO}_${i}"

  docker run -d \
    --name "$NOME" \
    --network host \
    --restart on-failure \
    -e "SERVER_ADDR=${SERVER_ADDR}" \
    "$IMAGE" \
    ./client_exec -atuador "$ATUADOR_ID" -node "$NODE"

  echo "  ✓ $NOME ($ATUADOR_ID)"
done

echo ""
echo "Atuadores ativos para '$NODE':"
docker ps --filter "name=atuador_${NODE_SLUG}" --format "  {{.Names}}  [{{.Status}}]"
