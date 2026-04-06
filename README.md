# FarmNode v3 - TEC502 - Problema 1 (Rota das Coisas)

Sistema IoT distribuído para integração entre sensores, atuadores e aplicação cliente, desenvolvido sem framework de mensageria, usando apenas comunicação nativa da arquitetura da Internet (UDP/TCP/HTTP/WebSocket).

## 1. Objetivo do Projeto

Este projeto resolve o problema de alto acoplamento entre dispositivos e aplicações por meio de um **serviço de integração central (broker)**.

No cenário original, cada sensor precisaria abrir conexões diretas para várias aplicações. Nesta solução:

- sensores enviam telemetria para o servidor de integração;
- atuadores mantêm conexão TCP com o servidor;
- clientes (dashboard) consomem dados em tempo real e enviam comandos via WebSocket;
- o servidor centraliza regras, roteamento e persistência.

## 2. Arquitetura e Componentes

### 2.1 Componentes principais

1. **Dispositivos virtuais simulados (cmd/client + internal/simulador)**
   - Sensores: geram dados contínuos (1ms) e enviam via UDP.
   - Atuadores: conectam via TCP e recebem comandos do servidor.

2. **Serviço de integração (cmd/server)**
   - Recebe telemetria UDP (sensores).
   - Mantém conexões TCP persistentes com atuadores.
   - Executa regras automáticas de acionamento.
   - Expõe dashboard HTTP + WebSocket.
   - Persiste histórico e alertas em JSON.

3. **Aplicação cliente (dashboard Web)**
   - Visualiza dados em tempo real.
   - Envia comandos de controle.
   - Reconhece alertas.
   - Consulta histórico e configurações.

### 2.2 Fluxo resumido

1. Sensor simulado envia JSON via UDP para `:8080`.
2. Servidor processa, atualiza estado e regras.
3. Se necessário, servidor envia comando TCP para atuador conectado em `:6000`.
4. Servidor publica estado/alertas via WebSocket (`/ws`) para dashboard.
5. Dashboard pode enviar comandos manuais e ajustes de configuração.

## 3. Perfis de Tráfego e QoS

A solução separa os perfis de tráfego conforme o problema:

- **Telemetria contínua (alta frequência): UDP**
  - prioridade para baixa latência;
  - perdas pontuais toleráveis;
  - servidor usa worker pool e fila para alto volume.

- **Comandos críticos: TCP**
  - conexão persistente com atuadores;
  - confiabilidade maior para comandos de controle;
  - verificação de disponibilidade do atuador antes de acionar.

## 4. Protocolo (API remota e mensagens)

### 4.1 Sensor -> Servidor (UDP :8080)

Formato `MensagemSensor` (JSON):

```json
{
  "node_id": "Estufa_A",
  "sensor_id": "sensor_umidade_01",
  "tipo": "umidade",
  "valor": 42.7,
  "unidade": "%",
  "timestamp": "2026-04-06T12:00:00Z",
  "status_leitura": "normal"
}
```

### 4.2 Atuador -> Servidor (TCP :6000)

Handshake inicial `RegistroAtuador` (JSON em linha):

```json
{
  "node_id": "Estufa_A",
  "atuador_id": "bomba_irrigacao_01"
}
```

Comando enviado pelo servidor `ComandoAtuador`:

```json
{
  "node_id": "Estufa_A",
  "atuador_id": "bomba_irrigacao_01",
  "comando": "LIGAR",
  "motivo_acionamento": "umidade_baixa",
  "timestamp_ordem": "2026-04-06T12:00:01Z"
}
```

### 4.3 Regra de Framing TCP

No canal TCP, as mensagens são enviadas em **JSON delimitado por linha** (`\n`):

- o atuador abre conexão e envia 1 linha JSON de registro;
- o servidor responde com comandos, também em JSON por linha;
- o atuador processa linha a linha com scanner.

Isso evita ambiguidade de leitura em socket contínuo.

### 4.4 Dashboard <-> Servidor (WebSocket `/ws`)

Mensagens servidor -> cliente:

- `{"tipo":"estado","dados":{...}}`
- `{"tipo":"alerta","dados":[...]}`

Mensagens cliente -> servidor:

- Comando manual:
  - `{"tipo":"comando","node_id":"...","atuador_id":"...","comando":"LIGAR|DESLIGAR"}`
- Reconhecimento de alerta:
  - `{"tipo":"ack_alerta","id":"..."}`
- Atualização de configuração:
  - `{"tipo":"config","node_id":"...","dados":{...}}`

### 4.5 Endpoints HTTP

- `GET /dashboard` - interface web
- `GET /api/estado` - estado atual
- `GET /api/sensor/{tipo}?horas=...` - histórico por tipo
- `GET /api/atuador/history?horas=...` - histórico de atuadores
- `GET /api/alertas?ativos=true|false` - alertas
- `GET /api/config` - configuração atual

## 5. Concorrência e Desempenho

- Worker pool UDP (`NumWorkers = 64`) com fila (`UDPQueueSize = 8192`).
- Broadcast WebSocket para múltiplos clientes simultâneos.
- Filtro de persistência de sensores para reduzir escrita em disco:
  - salva por variação mínima (`LogMinVariacao = 0.5`) ou por intervalo (`LogMinIntervalo = 2s`).
- Throttle de alertas para evitar repetição excessiva.

## 6. Confiabilidade Básica

- Monitoramento de sensores por timeout (`SensorTimeoutMs = 5000`).
- Monitoramento de atuadores esperados com detecção de desconexão.
- Comando só é efetivado quando o atuador está disponível.
- Atuadores reconectam automaticamente em caso de falha.
- Tratamento de erros de parse/validação em mensagens JSON.

## 7. Estrutura de Diretórios

```text
.
├── cmd/
│   ├── server/
│   │   ├── server_main.go
│   │   ├── dashboard.go
│   │   └── Dockerfile
│   └── client/
│       ├── client_main.go
│       └── Dockerfile
├── internal/
│   ├── logger/
│   ├── models/
│   ├── network/
│   ├── simulador/
│   ├── state/
│   └── storage/
├── docker-compose.yml
├── docker-compose.server.yml
├── docker-compose.sensors.yml
├── docker-compose.actuators.yml
├── go.mod
└── go.sum
```

## 8. Pacotes e Tecnologias

- Go 1.23
- Biblioteca padrão Go (`net`, `net/http`, `encoding/json`, `sync`, `time`, etc.)
- Docker / Docker Compose
- Chart.js no dashboard (CDN)

## 9. Pré-requisitos

- Docker Engine 24+ (ou equivalente compatível)
- Docker Compose v2+
- Go 1.23+ (apenas para execução sem Docker)

## 10. Variáveis de Ambiente

| Variável | Onde usar | Exemplo | Descrição |
|---|---|---|---|
| `SERVER_ADDR` | `docker-compose.sensors.yml` e `docker-compose.actuators.yml` | `SERVER_ADDR=172.16.103.2` | IP/host do servidor |
| `SERVER_IP` | sensores/client direto | `SERVER_IP=172.16.103.2:8080` | endereço UDP completo do servidor |

Observação: em `docker-compose.actuators.yml`, o `:6000` é anexado no próprio compose (`${SERVER_ADDR}:6000`).

## 11. Dockerfiles (análise)

### 11.1 `cmd/server/Dockerfile`

- Base: `golang:1.23`
- Copia `go.mod` e `go.sum`, roda `go mod download`
- Compila `./cmd/server` para `server_exec`
- Expõe `8080/udp`
- Executa `./server_exec`

### 11.2 `cmd/client/Dockerfile`

- Base: `golang:1.23`
- Copia dependências e código
- Compila `./cmd/client` para `client_exec`
- Executa `./client_exec`

## 12. Como Executar

### 12.1 Execução local completa (1 máquina)

```bash
docker compose up --build
```

Acessos:

- Dashboard: `http://localhost:8082/dashboard`
- UDP sensores: `localhost:8080/udp`
- TCP atuadores: `localhost:6000`

### 12.2 Execução distribuída (máquinas separadas)

#### Servidor

```bash
docker compose -f docker-compose.server.yml up --build
```

#### Sensores (em outra máquina)

```bash
SERVER_ADDR=<IP_DO_SERVIDOR> docker compose -f docker-compose.sensors.yml up --build
```

#### Atuadores (em outra máquina)

```bash
SERVER_ADDR=<IP_DO_SERVIDOR> docker compose -f docker-compose.actuators.yml up --build
```

## 13. Comandos Úteis (Ciclo de Vida)

Subir serviços:

```bash
docker compose up --build -d
```

Ver status:

```bash
docker compose ps
```

Ver logs:

```bash
docker compose logs -f server
```

Parar e remover containers/rede:

```bash
docker compose down
```

Parar e remover também volumes:

```bash
docker compose down -v
```

## 14. Como Usar

1. Suba os containers.
2. Abra `http://<host>:8082/dashboard`.
3. Acompanhe sensores em tempo real.
4. Acione atuadores manualmente pelos botões.
5. Verifique alertas críticos/avisos e histórico.
6. Ajuste limites de configuração pela aba de configurações.

## 15. Persistência e Logs

Os dados ficam em `./logs` (volume Docker):

- `sensor_logs.json`
- `atuador_logs.json`
- `alertas.json`

## 16. Testes

Validação de build do projeto:

```bash
go test ./...
```

Para teste de carga funcional, executar múltiplas instâncias de sensores/atuadores pelos compose separados.

## 17. Troubleshooting

- **Dashboard não abre em `:8082`**
  - Verifique se a porta está livre: `ss -lntup | rg 8082`.

- **Atuador desconectado / comando não aplicado**
  - Confira `SERVER_ADDR` e conectividade com o host do servidor.
  - Veja logs: `docker compose logs -f atuador_bomba` (ou atuador correspondente).

- **Sensores não aparecem no dashboard**
  - Confirme `SERVER_IP`/`SERVER_ADDR` e rota de rede até o servidor.
  - Valide se o servidor recebeu UDP em `:8080`.

- **IP correto, mas sem comunicação em máquinas diferentes**
  - Verifique firewall liberando `8080/udp`, `6000/tcp`, `8082/tcp`.

## 18. Limitações Conhecidas

- Telemetria em 1ms gera volume muito alto; o sistema reduz gravação em disco por filtro de persistência.
- O dashboard foi projetado para monitoramento operacional, não para BI histórico de longo prazo.

## 19. Requisitos para Entrega (GitHub)

Para submissão individual, incluir no repositório:

- Código-fonte completo
- Este `README` com arquitetura, execução e uso
- Dockerfiles e docker-compose utilizados
- (Opcional, mas recomendado) seção de evidências/testes no relatório PDF
