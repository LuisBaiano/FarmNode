#!/usr/bin/env bash
set -euo pipefail

SERVER_IP="${1:-localhost}"
IMAGE="farmnode_simulador"

build_image_if_needed() {
  if docker image inspect "$IMAGE" >/dev/null 2>&1; then
    return
  fi
  echo "Construindo imagem $IMAGE..."
  ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
  docker build -t "$IMAGE" -f "$ROOT_DIR/cmd/simulador/Dockerfile" "$ROOT_DIR"
}

slugify() {
  echo "$1" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//'
}

starts_with_ci() {
  local v prefix
  v="$(echo "$1" | tr '[:upper:]' '[:lower:]')"
  prefix="$(echo "$2" | tr '[:upper:]' '[:lower:]')"
  [[ "$v" == "$prefix"* ]]
}

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

read_positive_int() {
  local prompt="$1"
  local fallback="$2"
  local value
  while true; do
    read -r -p "$prompt [$fallback]: " value || true
    value="${value:-$fallback}"
    if [[ "$value" =~ ^[0-9]+$ ]] && [ "$value" -ge 1 ]; then
      echo "$value"
      return
    fi
    echo "Valor invalido. Informe um inteiro >= 1."
  done
}

pick_ambiente() {
  local op
  while true; do
    echo "Qual ambiente:"
    echo "[1] Galinheiro"
    echo "[2] Estufa"
    read -r -p "Escolha: " op || true
    case "$op" in
      1) echo "Galinheiro"; return ;;
      2) echo "Estufa"; return ;;
    esac
    echo "Opcao invalida."
  done
}

pick_tipo_sensor() {
  local ambiente="$1"
  local op
  if [[ "$ambiente" == "Estufa" ]]; then
    echo "Qual tipo de sensor:"
    echo "[1] umidade"
    echo "[2] temperatura"
    echo "[3] luminosidade"
    while true; do
      read -r -p "Escolha: " op || true
      case "$op" in
        1) echo "umidade"; return ;;
        2) echo "temperatura"; return ;;
        3) echo "luminosidade"; return ;;
      esac
      echo "Opcao invalida."
    done
  else
    echo "Qual tipo de sensor:"
    echo "[1] amonia"
    echo "[2] temperatura"
    echo "[3] racao"
    echo "[4] agua"
    while true; do
      read -r -p "Escolha: " op || true
      case "$op" in
        1) echo "amonia"; return ;;
        2) echo "temperatura"; return ;;
        3) echo "racao"; return ;;
        4) echo "agua"; return ;;
      esac
      echo "Opcao invalida."
    done
  fi
}

read_node_id() {
  local ambiente="$1"
  local fallback="${ambiente}_$(date +%s)"
  local node_id
  echo "Qual nome do no:"
  read -r -p "Nome [$fallback]: " node_id || true
  node_id="${node_id:-$fallback}"
  if ! starts_with_ci "$node_id" "$ambiente"; then
    node_id="${ambiente}_${node_id}"
    echo "Nome ajustado para '$node_id' para respeitar o ambiente."
  fi
  echo "$node_id"
}

build_image_if_needed

AMBIENTE="$(pick_ambiente)"
TIPO="$(pick_tipo_sensor "$AMBIENTE")"
NODE="$(read_node_id "$AMBIENTE")"
echo "Qual quantidade de sensores:"
QTD="$(read_positive_int "Quantidade" 1)"

NODE_SLUG="$(slugify "$NODE")"
NAME_PREFIX="sensor_${NODE_SLUG}_${TIPO}_"
START_IDX="$(next_index_for_prefix "$NAME_PREFIX")"

echo
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
    ./simulador_exec -sensor "$TIPO" -node "$NODE" -sensor-id "$SENSOR_ID" \
    >/dev/null

  echo "  OK $NOME ($SENSOR_ID)"
done

echo
echo "Containers ativos para '$NODE':"
docker ps --filter "name=sensor_${NODE_SLUG}_${TIPO}_" --format "  {{.Names}}  [{{.Status}}]"
