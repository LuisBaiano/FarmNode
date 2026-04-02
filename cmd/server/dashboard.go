package main

func getDashboardHTML() string {
	return `<!DOCTYPE html>
<html lang="pt-BR">
<head>
<meta charset="UTF-8">
<title>FarmNode v3</title>
<script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:Arial,sans-serif;background:#f0f4f0;color:#2c3e50;font-size:14px}
header{background:#2d6a4f;color:white;padding:12px 24px;display:flex;align-items:center;gap:12px;flex-wrap:wrap}
header h1{font-size:1.1rem;font-weight:bold;flex:1}
#badge-alertas{background:#c0392b;color:white;padding:2px 10px;border-radius:12px;font-size:.8rem;cursor:pointer;display:none}
#badge-alertas.visivel{display:inline-block}
#status-ws{font-size:.8rem;padding:2px 10px;border-radius:12px}
.ws-on{background:#27ae60}.ws-off{background:#c0392b}.ws-rec{background:#f39c12}
nav{background:#1b4332;display:flex;gap:2px;padding:0 16px}
nav button{background:none;border:none;color:#a8c4b0;padding:11px 16px;cursor:pointer;font-size:.88rem;border-bottom:3px solid transparent;transition:.15s}
nav button.ativo{color:white;border-bottom-color:#52b788}
nav button:hover{color:white}
.tab{display:none;padding:18px;max-width:1200px;margin:0 auto}
.tab.ativo{display:block}
.grid2{display:grid;grid-template-columns:repeat(auto-fit,minmax(340px,1fr));gap:14px;margin-bottom:14px}
.card{background:white;border-radius:8px;padding:16px;box-shadow:0 1px 6px rgba(0,0,0,.08)}
.card h2{font-size:.95rem;margin-bottom:10px;color:#1b4332;border-bottom:2px solid #d8f3dc;padding-bottom:5px;font-weight:bold}
.card h3{font-size:.85rem;margin:10px 0 5px;color:#40916c;font-weight:bold}
.valor-big{font-size:2.2rem;font-weight:bold;color:#2d6a4f;text-align:center;padding:8px 0}
.unidade{text-align:center;color:#888;font-size:.85rem;margin-bottom:2px}
.ts{text-align:center;color:#aaa;font-size:.72rem}
select{width:100%;padding:7px 9px;border:1px solid #ccc;border-radius:5px;font-size:.88rem;background:white;margin-bottom:2px}
select:focus{outline:none;border-color:#52b788}
.row-atu{display:flex;align-items:center;gap:6px;padding:7px 0;border-bottom:1px solid #f0f0f0}
.row-atu:last-child{border-bottom:none}
.lbl-atu{flex:1;font-size:.85rem;color:#444}
.st-badge{font-size:.78rem;font-weight:bold;padding:2px 8px;border-radius:9px;min-width:72px;text-align:center}
.st-on{background:#d8f3dc;color:#1b4332}.st-off{background:#f5f5f5;color:#888}
.btn{padding:3px 10px;border-radius:4px;border:1px solid;cursor:pointer;font-size:.78rem;background:white;transition:.12s}
.btn:hover{filter:brightness(.9)}
.btn-on{border-color:#27ae60;color:#27ae60}.btn-off{border-color:#e74c3c;color:#e74c3c}
table{width:100%;border-collapse:collapse;font-size:.8rem}
th{background:#d8f3dc;color:#1b4332;padding:6px 10px;text-align:left;font-weight:bold}
td{padding:5px 10px;border-bottom:1px solid #f0f0f0}
tr:hover td{background:#fafafa}
.alerta-card{border-radius:6px;padding:10px 14px;margin-bottom:8px;display:flex;align-items:flex-start;gap:10px}
.av-aviso{background:#fff8e1;border-left:4px solid #f39c12}
.av-critico{background:#fdecea;border-left:4px solid #c0392b;animation:pulse 1.5s infinite}
@keyframes pulse{0%,100%{opacity:1}50%{opacity:.82}}
.av-ack{background:#f5f5f5;border-left:4px solid #ccc;opacity:.65}
.av-body{flex:1}
.av-msg{font-size:.88rem;font-weight:bold}
.av-meta{font-size:.72rem;color:#888;margin-top:2px}
.av-nivel{font-size:.72rem;font-weight:bold;padding:1px 7px;border-radius:9px;white-space:nowrap}
.nl-critico{background:#c0392b;color:white}.nl-aviso{background:#f39c12;color:white}.nl-ack{background:#ccc;color:#555}
.btn-ack{background:white;border:1px solid #ccc;padding:3px 9px;border-radius:4px;cursor:pointer;font-size:.76rem;white-space:nowrap}
.btn-ack:hover{background:#f0f0f0}
.cfg-row{display:flex;align-items:center;gap:8px;margin-bottom:8px}
.cfg-lbl{flex:1;font-size:.85rem;color:#444}
.cfg-input{width:88px;padding:5px 8px;border:1px solid #ccc;border-radius:4px;font-size:.85rem}
.cfg-crit{background:#fff5f5}
.cfg-crit .cfg-lbl{color:#c0392b}
.cfg-crit .cfg-input{border-color:#e88}
.btn-save{background:#2d6a4f;color:white;border:none;padding:8px 20px;border-radius:5px;cursor:pointer;font-size:.88rem;margin-top:10px}
.btn-save:hover{background:#1b4332}
.sem-dados{color:#aaa;text-align:center;padding:18px;font-size:.88rem}
</style>
</head>
<body>

<header>
  <h1>FarmNode v3 — Painel Central</h1>
  <span id="badge-alertas" onclick="mostrarAba('alertas')">0 alertas</span>
  <span id="status-ws" class="ws-off">WebSocket: Desconectado</span>
</header>

<nav>
  <button class="ativo" onclick="mostrarAba('sensores')">Sensores</button>
  <button onclick="mostrarAba('atuadores')">Atuadores</button>
  <button onclick="mostrarAba('alertas')">Alertas</button>
  <button onclick="mostrarAba('config')">Configuracoes</button>
</nav>

<!-- ==================== ABA SENSORES ==================== -->
<div id="tab-sensores" class="tab ativo">
  <div class="grid2">
    <div class="card">
      <h2>Selecao de Sensor</h2>
      <label style="font-size:.83rem;color:#666;display:block;margin-bottom:4px">Sensor monitorado:</label>
      <select id="sensor-select" onchange="trocarSensor()">
        <optgroup label="Estufa A">
          <option value="Estufa_A|umidade|%">Umidade do Solo</option>
          <option value="Estufa_A|temperatura|C">Temperatura</option>
          <option value="Estufa_A|luminosidade|Lux">Luminosidade</option>
        </optgroup>
        <optgroup label="Galinheiro A">
          <option value="Galinheiro_A|amonia|ppm">Amonia</option>
          <option value="Galinheiro_A|temperatura|C">Temperatura</option>
          <option value="Galinheiro_A|racao|%">Racao</option>
          <option value="Galinheiro_A|agua|%">Nivel de Agua</option>
        </optgroup>
      </select>
    </div>
    <div class="card" style="text-align:center">
      <h2>Valor Atual</h2>
      <div class="valor-big" id="valor-atual">--</div>
      <div class="unidade" id="unidade-atual"></div>
      <div class="ts" id="ts-atual"></div>
    </div>
  </div>
  <div class="card">
    <h2>Historico do Sensor (ultima hora)</h2>
    <canvas id="grafico-sensor" height="80"></canvas>
  </div>
</div>

<!-- ==================== ABA ATUADORES ==================== -->
<div id="tab-atuadores" class="tab">
  <div class="grid2">
    <div class="card">
      <h2>Estufa A</h2>
      <div class="row-atu">
        <span class="lbl-atu">Bomba de Irrigacao</span>
        <span class="st-badge st-off" id="st-bomba">DESLIGADO</span>
        <button class="btn btn-on"  onclick="cmd('Estufa_A','bomba_irrigacao_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Estufa_A','bomba_irrigacao_01','DESLIGAR')">Desligar</button>
      </div>
      <div class="row-atu">
        <span class="lbl-atu">Ventilador</span>
        <span class="st-badge st-off" id="st-ventilador">DESLIGADO</span>
        <button class="btn btn-on"  onclick="cmd('Estufa_A','ventilador_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Estufa_A','ventilador_01','DESLIGAR')">Desligar</button>
      </div>
      <div class="row-atu">
        <span class="lbl-atu">Painel LED</span>
        <span class="st-badge st-off" id="st-led">DESLIGADO</span>
        <button class="btn btn-on"  onclick="cmd('Estufa_A','painel_led_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Estufa_A','painel_led_01','DESLIGAR')">Desligar</button>
      </div>
    </div>
    <div class="card">
      <h2>Galinheiro A</h2>
      <div class="row-atu">
        <span class="lbl-atu">Exaustor de Teto</span>
        <span class="st-badge st-off" id="st-exaustor">DESLIGADO</span>
        <button class="btn btn-on"  onclick="cmd('Galinheiro_A','exaustor_teto_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Galinheiro_A','exaustor_teto_01','DESLIGAR')">Desligar</button>
      </div>
      <div class="row-atu">
        <span class="lbl-atu">Aquecedor</span>
        <span class="st-badge st-off" id="st-aquecedor">DESLIGADO</span>
        <button class="btn btn-on"  onclick="cmd('Galinheiro_A','aquecedor_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Galinheiro_A','aquecedor_01','DESLIGAR')">Desligar</button>
      </div>
      <div class="row-atu">
        <span class="lbl-atu">Motor Comedouro</span>
        <span class="st-badge st-off" id="st-motor">DESLIGADO</span>
        <button class="btn btn-on"  onclick="cmd('Galinheiro_A','motor_comedouro_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Galinheiro_A','motor_comedouro_01','DESLIGAR')">Desligar</button>
      </div>
      <div class="row-atu">
        <span class="lbl-atu">Valvula de Agua</span>
        <span class="st-badge st-off" id="st-valvula">DESLIGADO</span>
        <button class="btn btn-on"  onclick="cmd('Galinheiro_A','valvula_agua_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Galinheiro_A','valvula_agua_01','DESLIGAR')">Desligar</button>
      </div>
    </div>
  </div>
  <div class="card">
    <h2>Historico de Ativacoes (ultimas 24h)</h2>
    <div id="historico-container"><p class="sem-dados">Carregando...</p></div>
  </div>
</div>

<!-- ==================== ABA ALERTAS ==================== -->
<div id="tab-alertas" class="tab">
  <div class="card" style="margin-bottom:14px">
    <h2>Alertas Ativos</h2>
    <div id="alertas-ativos-container"><p class="sem-dados">Nenhum alerta ativo.</p></div>
  </div>
  <div class="card">
    <h2>Historico de Alertas</h2>
    <div id="alertas-hist-container"><p class="sem-dados">Carregando...</p></div>
  </div>
</div>

<!-- ==================== ABA CONFIGURACOES ==================== -->
<div id="tab-config" class="tab">
  <div class="grid2">
    <div class="card">
      <h2>Estufa A</h2>
      <h3>Bomba de Irrigacao (Umidade)</h3>
      <div class="cfg-row">
        <span class="cfg-lbl">Ligar bomba abaixo de (%)</span>
        <input class="cfg-input" type="number" id="cfg-EA-umidade_min" step="1" min="0" max="100">
      </div>
      <div class="cfg-row">
        <span class="cfg-lbl">Desligar bomba acima de (%)</span>
        <input class="cfg-input" type="number" id="cfg-EA-umidade_max" step="1" min="0" max="100">
      </div>
      <div class="cfg-row cfg-crit">
        <span class="cfg-lbl">Alerta critico abaixo de (%)</span>
        <input class="cfg-input" type="number" id="cfg-EA-critico_umidade" step="1" min="0" max="100">
      </div>
      <h3>Ventilador (Temperatura)</h3>
      <div class="cfg-row">
        <span class="cfg-lbl">Ligar ventilador acima de (C)</span>
        <input class="cfg-input" type="number" id="cfg-EA-temp_max" step="0.5" min="0" max="60">
      </div>
      <div class="cfg-row cfg-crit">
        <span class="cfg-lbl">Alerta critico acima de (C)</span>
        <input class="cfg-input" type="number" id="cfg-EA-critico_temp" step="0.5" min="0" max="60">
      </div>
      <h3>Painel LED (Luminosidade)</h3>
      <div class="cfg-row">
        <span class="cfg-lbl">Ligar LED abaixo de (Lux)</span>
        <input class="cfg-input" type="number" id="cfg-EA-luz_min" step="10" min="0" max="2000">
      </div>
      <div class="cfg-row cfg-crit">
        <span class="cfg-lbl">Alerta critico abaixo de (Lux)</span>
        <input class="cfg-input" type="number" id="cfg-EA-critico_luz" step="10" min="0" max="2000">
      </div>
      <button class="btn-save" onclick="salvarConfig('Estufa_A')">Salvar Estufa A</button>
    </div>

    <div class="card">
      <h2>Galinheiro A</h2>
      <h3>Exaustor (Amonia)</h3>
      <div class="cfg-row">
        <span class="cfg-lbl">Ligar exaustor acima de (ppm)</span>
        <input class="cfg-input" type="number" id="cfg-GA-amonia_max" step="1" min="0" max="100">
      </div>
      <div class="cfg-row cfg-crit">
        <span class="cfg-lbl">Alerta critico acima de (ppm)</span>
        <input class="cfg-input" type="number" id="cfg-GA-critico_amonia" step="1" min="0" max="100">
      </div>
      <h3>Aquecedor (Temperatura)</h3>
      <div class="cfg-row">
        <span class="cfg-lbl">Ligar aquecedor abaixo de (C)</span>
        <input class="cfg-input" type="number" id="cfg-GA-temp_min" step="0.5" min="0" max="40">
      </div>
      <div class="cfg-row cfg-crit">
        <span class="cfg-lbl">Alerta critico abaixo de (C)</span>
        <input class="cfg-input" type="number" id="cfg-GA-critico_temp" step="0.5" min="0" max="40">
      </div>
      <h3>Motor Comedouro (Racao)</h3>
      <div class="cfg-row">
        <span class="cfg-lbl">Ligar motor abaixo de (%)</span>
        <input class="cfg-input" type="number" id="cfg-GA-racao_min" step="1" min="0" max="100">
      </div>
      <div class="cfg-row">
        <span class="cfg-lbl">Desligar motor acima de (%)</span>
        <input class="cfg-input" type="number" id="cfg-GA-racao_max" step="1" min="0" max="100">
      </div>
      <div class="cfg-row cfg-crit">
        <span class="cfg-lbl">Alerta critico abaixo de (%)</span>
        <input class="cfg-input" type="number" id="cfg-GA-critico_racao" step="1" min="0" max="100">
      </div>
      <h3>Valvula de Agua</h3>
      <div class="cfg-row">
        <span class="cfg-lbl">Ligar valvula abaixo de (%)</span>
        <input class="cfg-input" type="number" id="cfg-GA-agua_min" step="1" min="0" max="100">
      </div>
      <div class="cfg-row">
        <span class="cfg-lbl">Desligar valvula acima de (%)</span>
        <input class="cfg-input" type="number" id="cfg-GA-agua_max" step="1" min="0" max="100">
      </div>
      <div class="cfg-row cfg-crit">
        <span class="cfg-lbl">Alerta critico abaixo de (%)</span>
        <input class="cfg-input" type="number" id="cfg-GA-critico_agua" step="1" min="0" max="100">
      </div>
      <button class="btn-save" onclick="salvarConfig('Galinheiro_A')">Salvar Galinheiro A</button>
    </div>
  </div>
</div>

<script>
// ================================================================
// WebSocket — comunicacao principal cliente <-> servidor
//
// O dashboard NAO faz polling HTTP para estado ou comandos.
// Toda comunicacao em tempo real usa a conexao WebSocket (TCP).
//
// Mensagens enviadas pelo SERVIDOR (push):
//   {"tipo":"estado", "dados":{...}}   — a cada 1s
//   {"tipo":"alerta", "dados":[...]}   — imediato ao gerar alerta
//
// Mensagens enviadas pelo BROWSER (pull):
//   {"tipo":"comando",    "node_id":"...", "atuador_id":"...", "comando":"LIGAR"}
//   {"tipo":"ack_alerta", "id":"..."}
//   {"tipo":"config",     "node_id":"...", "dados":{...}}
// ================================================================

let ws = null;
let wsReconectando = false;
let sensorChart = null;
let sensorSel = { node: 'Estufa_A', tipo: 'umidade', unidade: '%' };
let estadoAtual = {};
let todosAlertas = [];

// ── Conexao WebSocket ──────────────────────────────────────────────────────

function conectarWS() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  const url   = proto + '://' + location.host + '/ws';

  setStatusWS('rec', 'WebSocket: Conectando...');
  ws = new WebSocket(url);

  ws.onopen = () => {
    setStatusWS('on', 'WebSocket: Conectado');
    wsReconectando = false;
  };

  ws.onmessage = (ev) => {
    try {
      const msg = JSON.parse(ev.data);
      switch (msg.tipo) {
        case 'estado': receberEstado(msg.dados); break;
        case 'alerta': receberAlertas(msg.dados); break;
      }
    } catch(e) { console.warn('[WS] Erro ao processar mensagem:', e); }
  };

  ws.onclose = () => {
    setStatusWS('off', 'WebSocket: Desconectado');
    if (!wsReconectando) {
      wsReconectando = true;
      setTimeout(conectarWS, 3000); // tenta reconectar em 3s
    }
  };

  ws.onerror = () => setStatusWS('off', 'WebSocket: Erro');
}

// Envia mensagem JSON pelo WebSocket
function wsSend(obj) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(obj));
  } else {
    console.warn('[WS] Nao conectado — mensagem descartada:', obj);
  }
}

function setStatusWS(estado, texto) {
  const el = document.getElementById('status-ws');
  el.textContent = texto;
  el.className = { on:'ws-on', off:'ws-off', rec:'ws-rec' }[estado] || 'ws-off';
}

// ── Receber estado do servidor ─────────────────────────────────────────────

function receberEstado(d) {
  if (!d) return;
  estadoAtual = d;

  // Aba sensores — valor atual e grafico
  atualizarValorAtual();
  const node = d[sensorSel.node];
  if (node) {
    const v = node[sensorSel.tipo];
    if (v !== undefined) adicionarPontoGrafico(v);
  }

  // Aba atuadores — badges de estado
  if (d.Estufa_A) {
    setBadge('st-bomba',      d.Estufa_A.bomba_ligada);
    setBadge('st-ventilador', d.Estufa_A.ventilador_ligado);
    setBadge('st-led',        d.Estufa_A.led_ligado);
  }
  if (d.Galinheiro_A) {
    setBadge('st-exaustor',  d.Galinheiro_A.exaustor_ligado);
    setBadge('st-aquecedor', d.Galinheiro_A.aquecedor_ligado);
    setBadge('st-motor',     d.Galinheiro_A.motor_ligado);
    setBadge('st-valvula',   d.Galinheiro_A.valvula_ligada);
  }
}

// ── Receber alertas do servidor ────────────────────────────────────────────

function receberAlertas(dados) {
  if (!Array.isArray(dados)) return;
  todosAlertas = dados;
  atualizarBadgeAlertas();
  if (document.getElementById('tab-alertas').classList.contains('ativo')) {
    renderizarAlertas();
  }
}

function atualizarBadgeAlertas() {
  const ativos = todosAlertas.filter(a => !a.ack).length;
  const badge  = document.getElementById('badge-alertas');
  if (ativos > 0) {
    badge.textContent = ativos + ' alerta' + (ativos > 1 ? 's' : '');
    badge.classList.add('visivel');
  } else {
    badge.classList.remove('visivel');
  }
}

// ── Grafico do sensor ──────────────────────────────────────────────────────

const cores = {
  umidade:'#3498db', temperatura:'#e74c3c', luminosidade:'#f39c12',
  amonia:'#9b59b6', racao:'#e67e22', agua:'#1abc9c'
};

function criarGrafico() {
  if (sensorChart) { sensorChart.destroy(); sensorChart = null; }
  const ctx = document.getElementById('grafico-sensor').getContext('2d');
  const cor = cores[sensorSel.tipo] || '#555';
  sensorChart = new Chart(ctx, {
    type: 'line',
    data: {
      labels: [],
      datasets: [{
        label: sensorSel.node + ' / ' + sensorSel.tipo + ' (' + sensorSel.unidade + ')',
        borderColor: cor, backgroundColor: cor + '18',
        data: [], fill: true, tension: 0.3, pointRadius: 2
      }]
    },
    options: {
      animation: false, responsive: true,
      scales: { x: { ticks: { maxTicksLimit:10, maxRotation:0 } }, y: { beginAtZero: false } },
      plugins: { legend: { position:'top' } }
    }
  });
}

async function trocarSensor() {
  const val = document.getElementById('sensor-select').value.split('|');
  sensorSel = { node: val[0], tipo: val[1], unidade: val[2] };
  criarGrafico();
  await carregarHistoricoSensor(); // HTTP — dados historicos do JSON salvo
  atualizarValorAtual();
}

// HTTP GET para historico — unico uso de HTTP no dashboard
async function carregarHistoricoSensor() {
  try {
    const r = await fetch('/api/sensor/' + sensorSel.tipo + '?horas=1');
    const dados = await r.json();
    if (!Array.isArray(dados) || !sensorChart) return;
    const filtrado = dados.filter(d => d.node_id === sensorSel.node).reverse().slice(-60);
    sensorChart.data.labels = filtrado.map(d =>
      new Date(d.timestamp).toLocaleTimeString('pt-BR', {hour:'2-digit',minute:'2-digit',second:'2-digit'})
    );
    sensorChart.data.datasets[0].data = filtrado.map(d => d.valor);
    sensorChart.update();
  } catch(e) { console.warn('Erro historico sensor:', e); }
}

function atualizarValorAtual() {
  const node = estadoAtual[sensorSel.node];
  if (!node) return;
  const val = node[sensorSel.tipo];
  document.getElementById('valor-atual').textContent = (val !== undefined && val !== null) ? parseFloat(val).toFixed(1) : '--';
  document.getElementById('unidade-atual').textContent = sensorSel.unidade;
  document.getElementById('ts-atual').textContent = 'Atualizado: ' + new Date().toLocaleTimeString('pt-BR');
}

function adicionarPontoGrafico(valor) {
  if (!sensorChart) return;
  const hora = new Date().toLocaleTimeString('pt-BR',{hour:'2-digit',minute:'2-digit',second:'2-digit'});
  sensorChart.data.labels.push(hora);
  sensorChart.data.datasets[0].data.push(valor);
  if (sensorChart.data.labels.length > 60) {
    sensorChart.data.labels.shift();
    sensorChart.data.datasets[0].data.shift();
  }
  sensorChart.update();
}

function setBadge(id, ligado) {
  const el = document.getElementById(id);
  if (!el) return;
  el.textContent = ligado ? 'LIGADO' : 'DESLIGADO';
  el.className = 'st-badge ' + (ligado ? 'st-on' : 'st-off');
}

// ── Navegacao ──────────────────────────────────────────────────────────────

function mostrarAba(nome) {
  document.querySelectorAll('.tab').forEach(t => t.classList.remove('ativo'));
  document.querySelectorAll('nav button').forEach(b => b.classList.remove('ativo'));
  document.getElementById('tab-' + nome).classList.add('ativo');
  const idx = {sensores:0, atuadores:1, alertas:2, config:3}[nome];
  document.querySelectorAll('nav button')[idx].classList.add('ativo');
  if (nome === 'atuadores') carregarHistoricoAtuadores();
  if (nome === 'alertas')   renderizarAlertas();
  if (nome === 'config')    carregarConfig();
}

// ── Comandos de atuadores — via WebSocket ──────────────────────────────────

function cmd(nodeID, atuadorID, comando) {
  wsSend({ tipo: 'comando', node_id: nodeID, atuador_id: atuadorID, comando });
}

// ── Historico de atuadores — HTTP GET (leitura historica) ──────────────────

const nomeAtu = {
  'bomba_irrigacao_01':'Bomba Irrigacao', 'ventilador_01':'Ventilador',
  'painel_led_01':'Painel LED', 'exaustor_teto_01':'Exaustor',
  'aquecedor_01':'Aquecedor', 'motor_comedouro_01':'Motor Comedouro',
  'valvula_agua_01':'Valvula Agua'
};

async function carregarHistoricoAtuadores() {
  const c = document.getElementById('historico-container');
  try {
    const r = await fetch('/api/atuador/history?horas=24');
    const dados = await r.json();
    if (!Array.isArray(dados) || dados.length === 0) {
      c.innerHTML = '<p class="sem-dados">Nenhuma ativacao nas ultimas 24h.</p>'; return;
    }
    const rows = dados.slice(0,100).map(d => {
      const t  = new Date(d.timestamp).toLocaleString('pt-BR');
      const nm = nomeAtu[d.atuador] || d.atuador;
      const bg = d.acao === 'LIGAR' ? '#e8f8ed' : '#fdecea';
      return '<tr style="background:'+bg+'"><td>'+t+'</td><td>'+d.node_id+'</td><td>'+nm+'</td>'
           + '<td><b>'+d.acao+'</b></td><td>'+(d.motivo||'-')+'</td></tr>';
    }).join('');
    c.innerHTML = '<table><thead><tr><th>Hora</th><th>Node</th><th>Atuador</th><th>Acao</th><th>Motivo</th></tr></thead><tbody>'
      + rows + '</tbody></table>';
  } catch(e) { c.innerHTML = '<p class="sem-dados">Erro ao carregar historico.</p>'; }
}

// ── Alertas ────────────────────────────────────────────────────────────────

function renderizarAlertas() {
  const ativos = todosAlertas.filter(a => !a.ack).reverse();
  const hist   = todosAlertas.slice().reverse().slice(0, 50);

  const cA = document.getElementById('alertas-ativos-container');
  cA.innerHTML = ativos.length === 0
    ? '<p class="sem-dados">Nenhum alerta ativo.</p>'
    : ativos.map(a => renderAlerta(a, true)).join('');

  const cH = document.getElementById('alertas-hist-container');
  cH.innerHTML = hist.length === 0
    ? '<p class="sem-dados">Nenhum alerta registrado.</p>'
    : hist.map(a => renderAlerta(a, false)).join('');
}

function renderAlerta(a, comBotao) {
  const t   = new Date(a.timestamp).toLocaleString('pt-BR');
  const cls = a.ack ? 'av-ack' : (a.nivel==='critico' ? 'av-critico' : 'av-aviso');
  const nlC = a.ack ? 'nl-ack' : (a.nivel==='critico' ? 'nl-critico' : 'nl-aviso');
  const nlT = a.ack ? 'RECONHECIDO' : a.nivel.toUpperCase();
  const btn = (comBotao && !a.ack)
    ? '<button class="btn-ack" onclick="ackAlerta(\''+a.id+'\')">Reconhecer</button>'
    : '';
  return '<div class="alerta-card '+cls+'">'
    + '<span class="av-nivel '+nlC+'">'+nlT+'</span>'
    + '<div class="av-body"><div class="av-msg">'+a.mensagem+'</div>'
    + '<div class="av-meta">'+a.node_id+' / '+a.tipo+' = '+a.valor.toFixed(1)+' — '+t+'</div></div>'
    + btn + '</div>';
}

// Envia ack pelo WebSocket
function ackAlerta(id) {
  wsSend({ tipo: 'ack_alerta', id });
}

// ── Configuracoes — leitura HTTP GET, escrita via WebSocket ────────────────

async function carregarConfig() {
  try {
    const r   = await fetch('/api/config');
    const cfg = await r.json();
    if (!cfg) return;
    const ea = cfg.Estufa_A     || {};
    const ga = cfg.Galinheiro_A || {};
    setInput('cfg-EA-umidade_min',    ea.umidade_min);
    setInput('cfg-EA-umidade_max',    ea.umidade_max);
    setInput('cfg-EA-temp_max',       ea.temp_max);
    setInput('cfg-EA-luz_min',        ea.luz_min);
    setInput('cfg-EA-critico_umidade',ea.critico_umidade);
    setInput('cfg-EA-critico_temp',   ea.critico_temp);
    setInput('cfg-EA-critico_luz',    ea.critico_luz);
    setInput('cfg-GA-amonia_max',     ga.amonia_max);
    setInput('cfg-GA-critico_amonia', ga.critico_amonia);
    setInput('cfg-GA-temp_min',       ga.temp_min);
    setInput('cfg-GA-critico_temp',   ga.critico_temp);
    setInput('cfg-GA-racao_min',      ga.racao_min);
    setInput('cfg-GA-racao_max',      ga.racao_max);
    setInput('cfg-GA-critico_racao',  ga.critico_racao);
    setInput('cfg-GA-agua_min',       ga.agua_min);
    setInput('cfg-GA-agua_max',       ga.agua_max);
    setInput('cfg-GA-critico_agua',   ga.critico_agua);
  } catch(e) { console.warn('Erro config:', e); }
}

function setInput(id, val) {
  const el = document.getElementById(id);
  if (el && val !== undefined) el.value = val;
}

// Salvar config envia mensagem WebSocket (nao usa HTTP POST)
function salvarConfig(nodeID) {
  const prefix = nodeID === 'Estufa_A' ? 'cfg-EA-' : 'cfg-GA-';
  const campos = nodeID === 'Estufa_A'
    ? ['umidade_min','umidade_max','temp_max','luz_min','critico_umidade','critico_temp','critico_luz']
    : ['amonia_max','critico_amonia','temp_min','critico_temp','racao_min','racao_max','critico_racao','agua_min','agua_max','critico_agua'];

  const dados = {};
  let ok = true;
  campos.forEach(c => {
    const el = document.getElementById(prefix + c);
    if (el && el.value !== '') {
      const v = parseFloat(el.value);
      if (isNaN(v)) { ok = false; return; }
      dados[c] = v;
    }
  });

  if (!ok) { alert('Valores invalidos — verifique os campos.'); return; }

  wsSend({ tipo: 'config', node_id: nodeID, dados });
  alert('Configuracoes enviadas para o servidor.');
}

// ── Inicializacao ──────────────────────────────────────────────────────────

// Carrega historico de alertas via HTTP na abertura (dados persistidos no JSON)
async function carregarAlertasIniciais() {
  try {
    const r = await fetch('/api/alertas');
    const dados = await r.json();
    if (Array.isArray(dados)) {
      todosAlertas = dados;
      atualizarBadgeAlertas();
    }
  } catch(e) {}
}

criarGrafico();
carregarHistoricoSensor();
carregarAlertasIniciais();
conectarWS(); // inicia conexao WebSocket
</script>
</body>
</html>`
}
