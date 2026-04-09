#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════════
# add_sensor.sh — Sobe N instâncias de um tipo de sensor para um nó
#
# Uso:
#   ./add_sensor.sh <tipo> <node_id> <quantidade> [SERVER_IP]
#
# Exemplos:
#   ./add_sensor.sh temperatura Estufa_A 3
#   ./add_sensor.sh amonia Galinheiro_A 2 192.168.1.100
#   ./add_sensor.sh umidade Estufa_B 5 192.168.1.100
#
# Tipos válidos: umidade | temperatura | luminosidade | amonia | racao | agua
# Node: qualquer string — Estufa_A, Galinheiro_A, Estufa_B, MeuNo, etc.
# ═══════════════════════════════════════════════════════════════════════════════

set -e

TIPO="${1}"
NODE="${2}"
QTD="${3:-1}"
SERVER_IP="${4:-localhost}"

# ── Validações ────────────────────────────────────────────────────────────────
if [[ -z "$TIPO" || -z "$NODE" ]]; then
  echo "Uso: $0 <tipo> <node_id> <quantidade> [SERVER_IP]"
  echo "Tipos: umidade | temperatura | luminosidade | amonia | racao | agua"
  exit 1
fi

TIPOS_VALIDOS="umidade temperatura luminosidade amonia racao agua"
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

echo "Subindo $QTD sensor(es) '$TIPO' para nó '$NODE' -> servidor $SERVER_IP:8080"

for i in $(seq 1 "$QTD"); do
  SENSOR_ID="sensor_${TIPO}_${SUFIXO}_${i}"
  NOME="sensor_${NODE_SLUG}_${TIPO}_${SUFIXO}_${i}"

  docker run -d \
    --name "$NOME" \
    --network host \
    --restart on-failure \
    -e "SERVER_IP=${SERVER_IP}:8080" \
    "$IMAGE" \
    ./client_exec -sensor "$TIPO" -node "$NODE" -sensor-id "$SENSOR_ID"

  echo "  ✓ $NOME ($SENSOR_ID)"
done

echo ""
echo "Containers ativos para '$NODE':"
docker ps --filter "name=sensor_${NODE_SLUG}" --format "  {{.Names}}  [{{.Status}}]"
