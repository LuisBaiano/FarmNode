#!/usr/bin/env bash
set -euo pipefail

TIPO="${1:-}"
NODE="${2:-}"
QTD="${3:-}"
SERVER_IP="${4:-localhost}"
IMAGE="farmnode_simulador"

TIPOS_ESTUFA=("umidade" "temperatura" "luminosidade")
TIPOS_GALINHEIRO=("amonia" "temperatura" "racao" "agua")
TODOS_TIPOS=("umidade" "temperatura" "luminosidade" "amonia" "racao" "agua")

contains() {
  local needle="$1"
  shift
  local item
  for item in "$@"; do
    [[ "$item" == "$needle" ]] && return 0
  done
  return 1
}

node_env() {
  local node="$1"
  local low
  low="$(echo "$node" | tr '[:upper:]' '[:lower:]')"
  if [[ "$low" == estufa* ]]; then
    echo "Estufa"
    return
  fi
  if [[ "$low" == galinheiro* ]]; then
    echo "Galinheiro"
    return
  fi
  echo ""
}

envs_for_tipo() {
  local tipo="$1"
  local out=()
  contains "$tipo" "${TIPOS_ESTUFA[@]}" && out+=("Estufa")
  contains "$tipo" "${TIPOS_GALINHEIRO[@]}" && out+=("Galinheiro")
  printf '%s\n' "${out[@]}"
}

pick_ambiente_from_tipo() {
  local tipo="$1"
  local ambientes=()
  local op
  mapfile -t ambientes < <(envs_for_tipo "$tipo")
  if [[ "${#ambientes[@]}" -eq 1 ]]; then
    echo "${ambientes[0]}"
    return
  fi
  while true; do
    echo "O tipo '$tipo' existe em mais de um ambiente."
    echo "Escolha o ambiente do no:"
    echo "  1) Estufa"
    echo "  2) Galinheiro"
    read -r -p "Escolha [1-2]: " op || true
    case "$op" in
      1) echo "Estufa"; return ;;
      2) echo "Galinheiro"; return ;;
    esac
    echo "Opcao invalida."
  done
}

tipo_valido() {
  contains "$1" "${TODOS_TIPOS[@]}"
}

tipo_pertence_ao_ambiente() {
  local tipo="$1"
  local ambiente="$2"
  if [[ "$ambiente" == "Estufa" ]]; then
    contains "$tipo" "${TIPOS_ESTUFA[@]}"
    return
  fi
  contains "$tipo" "${TIPOS_GALINHEIRO[@]}"
}

pick_ambiente() {
  local op
  while true; do
    echo "Ambiente do no:"
    echo "  1) Estufa"
    echo "  2) Galinheiro"
    read -r -p "Escolha [1-2]: " op || true
    case "$op" in
      1) echo "Estufa"; return ;;
      2) echo "Galinheiro"; return ;;
    esac
    echo "Opcao invalida."
  done
}

pick_tipo_sensor() {
  local ambiente="$1"
  local op
  while true; do
    echo "Tipos de sensor para $ambiente:"
    if [[ "$ambiente" == "Estufa" ]]; then
      echo "  1) umidade"
      echo "  2) temperatura"
      echo "  3) luminosidade"
    else
      echo "  1) amonia"
      echo "  2) temperatura"
      echo "  3) racao"
      echo "  4) agua"
    fi
    read -r -p "Escolha: " op || true
    if [[ "$ambiente" == "Estufa" ]]; then
      case "$op" in
        1) echo "umidade"; return ;;
        2) echo "temperatura"; return ;;
        3) echo "luminosidade"; return ;;
      esac
    else
      case "$op" in
        1) echo "amonia"; return ;;
        2) echo "temperatura"; return ;;
        3) echo "racao"; return ;;
        4) echo "agua"; return ;;
      esac
    fi
    echo "Opcao invalida."
  done
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

read_node_id() {
  local ambiente="$1"
  local fallback="${ambiente}_$(date +%s)"
  local value
  read -r -p "Nome do no [$fallback]: " value || true
  value="${value:-$fallback}"
  if [[ -z "$(node_env "$value")" ]]; then
    value="${ambiente}_${value}"
    echo "Nome ajustado para '$value' para respeitar o ambiente." >&2
  fi
  echo "$value"
}

menu_mode() {
  local ambiente
  ambiente="$(pick_ambiente)"
  TIPO="$(pick_tipo_sensor "$ambiente")"
  NODE="$(read_node_id "$ambiente")"
  QTD="$(read_positive_int "Quantidade de sensores" 1)"
  read -r -p "IP do servidor [localhost]: " SERVER_IP || true
  SERVER_IP="${SERVER_IP:-localhost}"
}

if [[ -z "$TIPO" || -z "$NODE" || -z "$QTD" ]]; then
  echo "Modo menu: faltaram argumentos obrigatorios."
  menu_mode
fi

if ! tipo_valido "$TIPO"; then
  echo "Erro: tipo '$TIPO' invalido. Validos: ${TODOS_TIPOS[*]}"
  exit 1
fi

if ! [[ "$QTD" =~ ^[0-9]+$ ]] || [ "$QTD" -lt 1 ]; then
  echo "Erro: quantidade deve ser um numero inteiro positivo"
  exit 1
fi

AMBIENTE_NO="$(node_env "$NODE")"
if [[ -z "$AMBIENTE_NO" ]]; then
  AMBIENTE_NO="$(pick_ambiente_from_tipo "$TIPO")"
  NODE="${AMBIENTE_NO}_${NODE}"
  echo "Aviso: node_id sem prefixo. Ajustado para '$NODE'."
fi

if ! tipo_pertence_ao_ambiente "$TIPO" "$AMBIENTE_NO"; then
  echo "Erro: o sensor '$TIPO' nao pertence ao ambiente '$AMBIENTE_NO'."
  if [[ "$AMBIENTE_NO" == "Estufa" ]]; then
    echo "Tipos validos para Estufa: ${TIPOS_ESTUFA[*]}"
  else
    echo "Tipos validos para Galinheiro: ${TIPOS_GALINHEIRO[*]}"
  fi
  exit 1
fi

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

NODE_SLUG="$(echo "$NODE" | tr '[:upper:]' '[:lower:]' | tr '_' '-')"
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
