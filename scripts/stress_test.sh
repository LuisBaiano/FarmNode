#!/usr/bin/env bash
# ==============================================================================
# stress_test.sh — Menu interativo para criar sensores por ambiente
#
# Uso:
#   ./stress_test.sh [SERVER_IP] [SERVER_PORT_TCP]
#
# Exemplos:
#   ./stress_test.sh
#   ./stress_test.sh 192.168.101.7
#   ./stress_test.sh 192.168.101.7 6000
# ==============================================================================

set -euo pipefail

SERVER_IP="${1:-localhost}"
SERVER_TCP_PORT="${2:-6000}"
SERVER_ADDR="${SERVER_IP}:${SERVER_TCP_PORT}"
IMAGE="farmnode_client"

TOTAL_SENSORES=0
TOTAL_ATUADORES=0

build_image_if_needed() {
  if docker image inspect "$IMAGE" >/dev/null 2>&1; then
    return
  fi
  echo "Construindo imagem $IMAGE..."
  ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
  docker build -t "$IMAGE" -f "$ROOT_DIR/cmd/client/Dockerfile" "$ROOT_DIR"
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
    echo "Valor inválido. Informe um inteiro >= 1."
  done
}

read_env_ms() {
  local prompt="$1"
  local fallback="$2"
  local min="$3"
  local max="$4"
  local value
  while true; do
    read -r -p "$prompt [$fallback]: " value || true
    value="${value:-$fallback}"
    if [[ "$value" =~ ^[0-9]+$ ]] && [ "$value" -ge "$min" ] && [ "$value" -le "$max" ]; then
      echo "$value"
      return
    fi
    echo "Valor inválido. Informe inteiro entre $min e $max."
  done
}

pick_sensor_type() {
  local ambiente="$1"
  local op
  if [[ "$ambiente" == "Estufa" ]]; then
    echo "Tipos de sensor para Estufa:" >&2
    echo "  1) umidade" >&2
    echo "  2) temperatura" >&2
    echo "  3) luminosidade" >&2
    echo "  4) todos" >&2
    while true; do
      read -r -p "Escolha o tipo [1-4]: " op || true
      case "$op" in
        1) echo "umidade"; return ;;
        2) echo "temperatura"; return ;;
        3) echo "luminosidade"; return ;;
        4) echo "todos"; return ;;
      esac
      echo "Opção inválida." >&2
    done
  else
    echo "Tipos de sensor para Galinheiro:" >&2
    echo "  1) amonia" >&2
    echo "  2) temperatura" >&2
    echo "  3) racao" >&2
    echo "  4) agua" >&2
    echo "  5) todos" >&2
    while true; do
      read -r -p "Escolha o tipo [1-5]: " op || true
      case "$op" in
        1) echo "amonia"; return ;;
        2) echo "temperatura"; return ;;
        3) echo "racao"; return ;;
        4) echo "agua"; return ;;
        5) echo "todos"; return ;;
      esac
      echo "Opção inválida." >&2
    done
  fi
}

pick_atuador_type() {
  local ambiente="$1"
  local op
  if [[ "$ambiente" == "Estufa" ]]; then
    echo "Tipos de atuador para Estufa:" >&2
    echo "  1) bomba" >&2
    echo "  2) ventilador" >&2
    echo "  3) led" >&2
    echo "  4) todos" >&2
    while true; do
      read -r -p "Escolha o tipo [1-4]: " op || true
      case "$op" in
        1) echo "bomba"; return ;;
        2) echo "ventilador"; return ;;
        3) echo "led"; return ;;
        4) echo "todos"; return ;;
      esac
      echo "Opção inválida." >&2
    done
  else
    echo "Tipos de atuador para Galinheiro:" >&2
    echo "  1) exaustor" >&2
    echo "  2) aquecedor" >&2
    echo "  3) motor" >&2
    echo "  4) valvula" >&2
    echo "  5) todos" >&2
    while true; do
      read -r -p "Escolha o tipo [1-5]: " op || true
      case "$op" in
        1) echo "exaustor"; return ;;
        2) echo "aquecedor"; return ;;
        3) echo "motor"; return ;;
        4) echo "valvula"; return ;;
        5) echo "todos"; return ;;
      esac
      echo "Opção inválida." >&2
    done
  fi
}

spawn_sensor() {
  local node_id="$1"
  local tipo="$2"
  local sensor_interval_ms="$3"
  local atuador_poll_ms="$4"
  local batch="$5"
  local idx="$6"
  local node_slug sensor_id container_name

  node_slug="$(slugify "$node_id")"
  sensor_id="s_${tipo}_${batch}_${idx}"
  container_name="stress_s_${node_slug}_${tipo}_${batch}_${idx}"

  docker run -d \
    --name "$container_name" \
    --network host \
    --restart on-failure \
    -e "SERVER_IP=${SERVER_IP}:8080" \
    -e "SENSOR_INTERVAL_MS=${sensor_interval_ms}" \
    -e "ATUADOR_POLL_MS=${atuador_poll_ms}" \
    "$IMAGE" \
    ./client_exec -sensor "$tipo" -node "$node_id" -sensor-id "$sensor_id" \
    >/dev/null

  TOTAL_SENSORES=$((TOTAL_SENSORES + 1))
  echo "  ✓ $container_name (tipo=$tipo, node=$node_id)"
}

spawn_atuador() {
  local node_id="$1"
  local tipo="$2"
  local batch="$3"
  local idx="$4"
  local node_slug atuador_id container_name

  node_slug="$(slugify "$node_id")"
  atuador_id="${tipo}_${node_slug}_${idx}"
  container_name="stress_a_${node_slug}_${tipo}_${batch}_${idx}"

  docker run -d \
    --name "$container_name" \
    --network host \
    --restart on-failure \
    -e "SERVER_ADDR=${SERVER_ADDR}" \
    "$IMAGE" \
    ./client_exec -atuador "$atuador_id" -node "$node_id" \
    >/dev/null

  TOTAL_ATUADORES=$((TOTAL_ATUADORES + 1))
  echo "  ✓ $container_name (tipo=$tipo, node=$node_id)"
}

create_menu_sensores() {
  local ambiente="$1"
  local default_node="${ambiente}_$(date +%s)"
  local node_id sensor_type qtd sensor_interval_ms atuador_poll_ms batch i
  local tipos=()

  read -r -p "Nome do nó [$default_node]: " node_id || true
  node_id="${node_id:-$default_node}"

  if ! starts_with_ci "$node_id" "$ambiente"; then
    node_id="${ambiente}_${node_id}"
    echo "Nome ajustado para '$node_id' para respeitar o ambiente."
  fi

  sensor_type="$(pick_sensor_type "$ambiente")"
  if [[ "$sensor_type" == "todos" ]]; then
    if [[ "$ambiente" == "Estufa" ]]; then
      tipos=("umidade" "temperatura" "luminosidade")
    else
      tipos=("amonia" "temperatura" "racao" "agua")
    fi
  else
    tipos=("$sensor_type")
  fi

  qtd="$(read_positive_int "Quantidade de sensores por tipo" 5)"
  sensor_interval_ms="$(read_env_ms "Intervalo de envio do sensor (ms)" 1 1 1000)"
  atuador_poll_ms="$(read_env_ms "Intervalo de polling dos atuadores (ms)" 200 1 10000)"
  batch="$(date +%s%N)"

  echo
  echo "Criando sensores..."
  echo "  Ambiente: $ambiente"
  echo "  Nó      : $node_id"
  echo "  Tipo(s) : ${tipos[*]}"
  echo "  Qtd/tipo: $qtd"
  echo "  Server  : UDP ${SERVER_IP}:8080 | TCP ${SERVER_ADDR}"
  echo

  for tipo in "${tipos[@]}"; do
    for i in $(seq 1 "$qtd"); do
      spawn_sensor "$node_id" "$tipo" "$sensor_interval_ms" "$atuador_poll_ms" "$batch" "$i"
    done
  done

  echo
  echo "Nó '$node_id' criado com sucesso."
}

create_menu_atuadores() {
  local ambiente="$1"
  local default_node="${ambiente}_$(date +%s)"
  local node_id atuador_type qtd batch i
  local tipos=()

  read -r -p "Nome do nó [$default_node]: " node_id || true
  node_id="${node_id:-$default_node}"

  if ! starts_with_ci "$node_id" "$ambiente"; then
    node_id="${ambiente}_${node_id}"
    echo "Nome ajustado para '$node_id' para respeitar o ambiente."
  fi

  atuador_type="$(pick_atuador_type "$ambiente")"
  if [[ "$atuador_type" == "todos" ]]; then
    if [[ "$ambiente" == "Estufa" ]]; then
      tipos=("bomba" "ventilador" "led")
    else
      tipos=("exaustor" "aquecedor" "motor" "valvula")
    fi
  else
    tipos=("$atuador_type")
  fi

  qtd="$(read_positive_int "Quantidade de atuadores por tipo" 3)"
  batch="$(date +%s%N)"

  echo
  echo "Criando atuadores..."
  echo "  Ambiente: $ambiente"
  echo "  Nó      : $node_id"
  echo "  Tipo(s) : ${tipos[*]}"
  echo "  Qtd/tipo: $qtd"
  echo "  Server  : TCP ${SERVER_ADDR}"
  echo

  for tipo in "${tipos[@]}"; do
    for i in $(seq 1 "$qtd"); do
      spawn_atuador "$node_id" "$tipo" "$batch" "$i"
    done
  done

  echo
  echo "Atuadores do nó '$node_id' criados com sucesso."
}

list_stress_containers() {
  echo
  echo "Containers stress em execução:"
  docker ps --filter "name=stress_" --format "  {{.Names}}  [{{.Status}}]" || true
  echo
}

cleanup_stress_containers() {
  echo
  echo "Removendo containers 'stress_'..."
  docker ps -a --filter "name=stress_" -q | xargs -r docker rm -f >/dev/null
  echo "Limpeza concluída."
  echo
}

main_menu() {
  while true; do
    echo "============================================================"
    echo " FarmNode - Menu de Stress (sensores por ambiente)"
    echo " Servidor: UDP ${SERVER_IP}:8080 | TCP ${SERVER_ADDR}"
    echo " Sensores criados nesta sessão: $TOTAL_SENSORES"
    echo " Atuadores criados nesta sessão: $TOTAL_ATUADORES"
    echo "============================================================"
    echo "1) Criar sensores de Estufa"
    echo "2) Criar sensores de Galinheiro"
    echo "3) Criar atuadores de Estufa"
    echo "4) Criar atuadores de Galinheiro"
    echo "5) Listar containers stress"
    echo "6) Limpar containers stress"
    echo "7) Sair"
    echo
    read -r -p "Escolha uma opção [1-7]: " op || true
    case "$op" in
      1) create_menu_sensores "Estufa" ;;
      2) create_menu_sensores "Galinheiro" ;;
      3) create_menu_atuadores "Estufa" ;;
      4) create_menu_atuadores "Galinheiro" ;;
      5) list_stress_containers ;;
      6) cleanup_stress_containers ;;
      7)
        echo "Saindo. Total sensores: $TOTAL_SENSORES | Total atuadores: $TOTAL_ATUADORES"
        echo "Monitor de velocidade: curl http://${SERVER_IP}:8082/api/velocidade"
        exit 0
        ;;
      *) echo "Opção inválida."; echo ;;
    esac
  done
}

build_image_if_needed
main_menu
