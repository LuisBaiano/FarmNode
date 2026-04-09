#!/usr/bin/env bash
set -e

TIPO="${1}"
NODE="${2}"
QTD="${3:-1}"
SERVER_IP="${4:-localhost}"

if [[ -z "$TIPO" || -z "$NODE" ]]; then
  echo "Uso: $0 <tipo> <node_id> <quantidade> [SERVER_IP]"
  echo "Tipos: umidade | temperatura | luminosidade | amonia | racao | agua"
  exit 1
fi

TIPOS_VALIDOS="umidade temperatura luminosidade amonia racao agua"
if ! echo "$TIPOS_VALIDOS" | grep -qw "$TIPO"; then
  echo "Erro: tipo '$TIPO' invalido. Validos: $TIPOS_VALIDOS"
  exit 1
fi

if ! [[ "$QTD" =~ ^[0-9]+$ ]] || [ "$QTD" -lt 1 ]; then
  echo "Erro: quantidade deve ser um numero inteiro positivo"
  exit 1
fi

IMAGE="farmnode_simulador"
if ! docker image inspect "$IMAGE" &>/dev/null; then
  echo "Construindo imagem $IMAGE..."
  ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
  docker build -t "$IMAGE" -f "$ROOT_DIR/cmd/simulador/Dockerfile" "$ROOT_DIR"
fi

next_index_for_prefix() {
  local prefix="$1"
  local max=0
  local name num
  while IFS= read -r name; do
    if [[ "$name" =~ ^${prefix}([0-9]+)$ ]]; then
      num="${BASH_REMATCH[1]}"
      if (( num > max )); then
        max="$num"
      fi
    fi
  done < <(docker ps -a --format '{{.Names}}')
  echo $((max + 1))
}

NODE_SLUG=$(echo "$NODE" | tr '[:upper:]' '[:lower:]' | tr '_' '-')
NAME_PREFIX="sensor_${NODE_SLUG}_${TIPO}_"
START_IDX="$(next_index_for_prefix "$NAME_PREFIX")"

echo "Subindo $QTD sensor(es) '$TIPO' para no '$NODE' -> servidor $SERVER_IP:8080"
echo "Indice inicial: $START_IDX"

for offset in $(seq 0 $((QTD - 1))); do
  IDX=$((START_IDX + offset))
  SENSOR_ID="sensor_${TIPO}_${NODE_SLUG}_${IDX}"
  NOME="${NAME_PREFIX}${IDX}"

  docker run -d \
    --name "$NOME" \
    --network host \
    --restart on-failure \
    -e "SERVER_IP=${SERVER_IP}:8080" \
    "$IMAGE" \
    ./simulador_exec -sensor "$TIPO" -node "$NODE" -sensor-id "$SENSOR_ID"

  echo "  OK $NOME ($SENSOR_ID)"
done

echo
echo "Containers ativos para '$NODE':"
docker ps --filter "name=sensor_${NODE_SLUG}_${TIPO}_" --format "  {{.Names}}  [{{.Status}}]"
