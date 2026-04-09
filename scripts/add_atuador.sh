#!/usr/bin/env bash
set -euo pipefail

TIPO="${1:-}"
NODE="${2:-}"
QTD="${3:-}"
SERVER_ADDR="${4:-localhost}:6000"
IMAGE="farmnode_simulador"

TIPOS_ESTUFA=("bomba" "ventilador" "led")
TIPOS_GALINHEIRO=("exaustor" "aquecedor" "motor" "valvula")
TODOS_TIPOS=("bomba" "ventilador" "led" "exaustor" "aquecedor" "motor" "valvula")

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

env_for_tipo() {
  local tipo="$1"
  if contains "$tipo" "${TIPOS_ESTUFA[@]}"; then
    echo "Estufa"
    return
  fi
  echo "Galinheiro"
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

pick_tipo_atuador() {
  local ambiente="$1"
  local op
  while true; do
    echo "Tipos de atuador para $ambiente:"
    if [[ "$ambiente" == "Estufa" ]]; then
      echo "  1) bomba"
      echo "  2) ventilador"
      echo "  3) led"
    else
      echo "  1) exaustor"
      echo "  2) aquecedor"
      echo "  3) motor"
      echo "  4) valvula"
    fi
    read -r -p "Escolha: " op || true
    if [[ "$ambiente" == "Estufa" ]]; then
      case "$op" in
        1) echo "bomba"; return ;;
        2) echo "ventilador"; return ;;
        3) echo "led"; return ;;
      esac
    else
      case "$op" in
        1) echo "exaustor"; return ;;
        2) echo "aquecedor"; return ;;
        3) echo "motor"; return ;;
        4) echo "valvula"; return ;;
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
  TIPO="$(pick_tipo_atuador "$ambiente")"
  NODE="$(read_node_id "$ambiente")"
  QTD="$(read_positive_int "Quantidade de atuadores" 1)"
  read -r -p "Endereco do servidor [localhost:6000]: " SERVER_ADDR || true
  SERVER_ADDR="${SERVER_ADDR:-localhost:6000}"
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
  AMBIENTE_NO="$(env_for_tipo "$TIPO")"
  NODE="${AMBIENTE_NO}_${NODE}"
  echo "Aviso: node_id sem prefixo. Ajustado para '$NODE'."
fi

if ! tipo_pertence_ao_ambiente "$TIPO" "$AMBIENTE_NO"; then
  echo "Erro: o atuador '$TIPO' nao pertence ao ambiente '$AMBIENTE_NO'."
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
NAME_PREFIX="atuador_${NODE_SLUG}_${TIPO}_"
START_IDX="$(next_index_for_prefix "$NAME_PREFIX")"

echo "Subindo $QTD atuador(es) '$TIPO' para no '$NODE' -> servidor $SERVER_ADDR"
echo "Indice inicial: $START_IDX"

for offset in $(seq 0 $((QTD - 1))); do
  IDX=$((START_IDX + offset))
  ATUADOR_ID="${TIPO}_${NODE_SLUG}_${IDX}"
  NOME="${NAME_PREFIX}${IDX}"

  docker run -d \
    --name "$NOME" \
    --network host \
    --restart on-failure \
    -e "SERVER_ADDR=${SERVER_ADDR}" \
    "$IMAGE" \
    ./simulador_exec -atuador "$ATUADOR_ID" -node "$NODE"

  echo "  OK $NOME ($ATUADOR_ID)"
done

echo
echo "Atuadores ativos para '$NODE':"
docker ps --filter "name=atuador_${NODE_SLUG}_${TIPO}_" --format "  {{.Names}}  [{{.Status}}]"
