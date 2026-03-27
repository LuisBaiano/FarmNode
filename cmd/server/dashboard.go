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
body{font-family:Arial,sans-serif;background:#f0f4f0;color:#2c3e50}
header{background:#2d6a4f;color:white;padding:14px 24px;display:flex;align-items:center;gap:12px;flex-wrap:wrap}
header h1{font-size:1.2rem;flex:1}
#badge-alertas{background:#e74c3c;color:white;padding:3px 10px;border-radius:12px;font-size:.8rem;cursor:pointer;display:none}
#badge-alertas.visivel{display:inline-block}
#status-cn{font-size:.8rem;padding:3px 10px;border-radius:12px}
.online{background:#27ae60}.offline{background:#c0392b}
nav{background:#1b4332;display:flex;gap:2px;padding:0 16px}
nav button{background:none;border:none;color:#adc;padding:12px 18px;cursor:pointer;font-size:.9rem;border-bottom:3px solid transparent;transition:.2s}
nav button.ativo{color:white;border-bottom-color:#52b788}
nav button:hover{color:white}
.tab{display:none;padding:20px;max-width:1200px;margin:0 auto}
.tab.ativo{display:block}
.grid2{display:grid;grid-template-columns:repeat(auto-fit,minmax(340px,1fr));gap:16px;margin-bottom:16px}
.card{background:white;border-radius:10px;padding:18px;box-shadow:0 2px 8px rgba(0,0,0,.08)}
.card h2{font-size:1rem;margin-bottom:12px;color:#1b4332;border-bottom:2px solid #d8f3dc;padding-bottom:6px}
.card h3{font-size:.9rem;margin:10px 0 6px;color:#40916c}
.valor-big{font-size:2.4rem;font-weight:bold;color:#2d6a4f;text-align:center;padding:8px 0}
.unidade{text-align:center;color:#888;font-size:.9rem;margin-bottom:4px}
.ts{text-align:center;color:#aaa;font-size:.75rem}
select,input[type=number]{width:100%;padding:8px 10px;border:1px solid #ccc;border-radius:6px;font-size:.9rem;background:white;margin-bottom:2px}
select:focus,input:focus{outline:none;border-color:#52b788}
.row-atu{display:flex;align-items:center;gap:8px;padding:8px 0;border-bottom:1px solid #f0f0f0}
.row-atu:last-child{border-bottom:none}
.lbl-atu{flex:1;font-size:.88rem;color:#444}
.st-badge{font-size:.8rem;font-weight:bold;padding:2px 8px;border-radius:10px;min-width:76px;text-align:center}
.st-on{background:#d8f3dc;color:#1b4332}.st-off{background:#ffe;color:#888}
.btn{padding:4px 11px;border-radius:5px;border:1px solid;cursor:pointer;font-size:.8rem;background:white;transition:.15s}
.btn:hover{filter:brightness(.9)}
.btn-on{border-color:#27ae60;color:#27ae60}.btn-off{border-color:#e74c3c;color:#e74c3c}
table{width:100%;border-collapse:collapse;font-size:.82rem}
th{background:#d8f3dc;color:#1b4332;padding:7px 10px;text-align:left}
td{padding:6px 10px;border-bottom:1px solid #f0f0f0}
tr:hover td{background:#f9f9f9}
.alerta-card{border-radius:8px;padding:12px 16px;margin-bottom:10px;display:flex;align-items:flex-start;gap:10px}
.av-aviso{background:#fff8e1;border-left:4px solid #f39c12}
.av-critico{background:#fdecea;border-left:4px solid #e74c3c;animation:pulse 1.5s infinite}
@keyframes pulse{0%,100%{opacity:1}50%{opacity:.85}}
.av-ack{background:#f5f5f5;border-left:4px solid #ccc;opacity:.7}
.av-body{flex:1}
.av-msg{font-size:.9rem;font-weight:bold}
.av-meta{font-size:.75rem;color:#888;margin-top:2px}
.av-nivel{font-size:.75rem;font-weight:bold;padding:1px 7px;border-radius:10px}
.nl-critico{background:#e74c3c;color:white}.nl-aviso{background:#f39c12;color:white}.nl-ack{background:#ccc;color:#555}
.btn-ack{background:#fff;border:1px solid #ccc;padding:3px 10px;border-radius:4px;cursor:pointer;font-size:.78rem;white-space:nowrap}
.btn-ack:hover{background:#f0f0f0}
.cfg-row{display:flex;align-items:center;gap:8px;margin-bottom:10px}
.cfg-lbl{flex:1;font-size:.88rem;color:#444}
.cfg-input{width:90px;padding:6px 8px;border:1px solid #ccc;border-radius:5px;font-size:.88rem}
.btn-save{background:#2d6a4f;color:white;border:none;padding:10px 24px;border-radius:6px;cursor:pointer;font-size:.9rem;margin-top:12px}
.btn-save:hover{background:#1b4332}
.sem-dados{color:#aaa;text-align:center;padding:20px;font-size:.9rem}
</style>
</head>
<body>

<header>
  <h1>🌿 FarmNode v3 — Painel Central</h1>
  <span id="badge-alertas" onclick="mostrarAba('alertas')">0 alertas</span>
  <span id="status-cn" class="offline">● Desconectado</span>
</header>

<nav>
  <button class="ativo" onclick="mostrarAba('sensores')">📊 Sensores</button>
  <button onclick="mostrarAba('atuadores')">⚙️ Atuadores</button>
  <button onclick="mostrarAba('alertas')">🚨 Alertas</button>
  <button onclick="mostrarAba('config')">🔧 Configurações</button>
</nav>

<!-- ═══════════════════════════ ABA SENSORES ═══════════════════════════════ -->
<div id="tab-sensores" class="tab ativo">
  <div class="grid2">
    <div class="card">
      <h2>📡 Seleção de Sensor</h2>
      <label style="font-size:.85rem;color:#666;display:block;margin-bottom:4px">Selecione o sensor para monitorar:</label>
      <select id="sensor-select" onchange="trocarSensor()">
        <optgroup label="🌱 Estufa A">
          <option value="Estufa_A|umidade|%">Umidade do Solo</option>
          <option value="Estufa_A|temperatura|°C">Temperatura</option>
          <option value="Estufa_A|luminosidade|Lux">Luminosidade</option>
        </optgroup>
        <optgroup label="🐔 Galinheiro A">
          <option value="Galinheiro_A|amonia|ppm">Amônia</option>
          <option value="Galinheiro_A|temperatura|°C">Temperatura</option>
          <option value="Galinheiro_A|racao|%">Ração</option>
          <option value="Galinheiro_A|agua|%">Nível de Água</option>
        </optgroup>
      </select>
    </div>

    <div class="card" style="text-align:center">
      <h2>📈 Valor Atual</h2>
      <div class="valor-big" id="valor-atual">--</div>
      <div class="unidade" id="unidade-atual"></div>
      <div class="ts" id="ts-atual"></div>
    </div>
  </div>

  <div class="card">
    <h2>📉 Histórico do Sensor (última hora)</h2>
    <canvas id="grafico-sensor" height="80"></canvas>
  </div>
</div>

<!-- ════════════════════════════ ABA ATUADORES ════════════════════════════ -->
<div id="tab-atuadores" class="tab">
  <div class="grid2">

    <div class="card">
      <h2>🌱 Estufa A</h2>
      <div class="row-atu">
        <span class="lbl-atu">💧 Bomba de Irrigação</span>
        <span class="st-badge st-off" id="st-bomba">DESLIGADO</span>
        <button class="btn btn-on" onclick="cmd('Estufa_A','bomba_irrigacao_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Estufa_A','bomba_irrigacao_01','DESLIGAR')">Desligar</button>
      </div>
      <div class="row-atu">
        <span class="lbl-atu">🌀 Ventilador</span>
        <span class="st-badge st-off" id="st-ventilador">DESLIGADO</span>
        <button class="btn btn-on" onclick="cmd('Estufa_A','ventilador_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Estufa_A','ventilador_01','DESLIGAR')">Desligar</button>
      </div>
      <div class="row-atu">
        <span class="lbl-atu">💡 Painel LED</span>
        <span class="st-badge st-off" id="st-led">DESLIGADO</span>
        <button class="btn btn-on" onclick="cmd('Estufa_A','painel_led_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Estufa_A','painel_led_01','DESLIGAR')">Desligar</button>
      </div>
    </div>

    <div class="card">
      <h2>🐔 Galinheiro A</h2>
      <div class="row-atu">
        <span class="lbl-atu">💨 Exaustor de Teto</span>
        <span class="st-badge st-off" id="st-exaustor">DESLIGADO</span>
        <button class="btn btn-on" onclick="cmd('Galinheiro_A','exaustor_teto_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Galinheiro_A','exaustor_teto_01','DESLIGAR')">Desligar</button>
      </div>
      <div class="row-atu">
        <span class="lbl-atu">🔥 Aquecedor</span>
        <span class="st-badge st-off" id="st-aquecedor">DESLIGADO</span>
        <button class="btn btn-on" onclick="cmd('Galinheiro_A','aquecedor_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Galinheiro_A','aquecedor_01','DESLIGAR')">Desligar</button>
      </div>
      <div class="row-atu">
        <span class="lbl-atu">⚙️ Motor Comedouro</span>
        <span class="st-badge st-off" id="st-motor">DESLIGADO</span>
        <button class="btn btn-on" onclick="cmd('Galinheiro_A','motor_comedouro_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Galinheiro_A','motor_comedouro_01','DESLIGAR')">Desligar</button>
      </div>
      <div class="row-atu">
        <span class="lbl-atu">🚰 Válvula de Água</span>
        <span class="st-badge st-off" id="st-valvula">DESLIGADO</span>
        <button class="btn btn-on" onclick="cmd('Galinheiro_A','valvula_agua_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Galinheiro_A','valvula_agua_01','DESLIGAR')">Desligar</button>
      </div>
    </div>

  </div>

  <div class="card">
    <h2>📋 Histórico de Ativações (últimas 24h)</h2>
    <div id="historico-container">
      <p class="sem-dados">Carregando...</p>
    </div>
  </div>
</div>

<!-- ════════════════════════════ ABA ALERTAS ══════════════════════════════ -->
<div id="tab-alertas" class="tab">
  <div class="card" style="margin-bottom:16px">
    <h2>🚨 Alertas Ativos</h2>
    <div id="alertas-ativos-container">
      <p class="sem-dados">Nenhum alerta ativo.</p>
    </div>
  </div>

  <div class="card">
    <h2>📋 Histórico de Alertas</h2>
    <div id="alertas-hist-container">
      <p class="sem-dados">Carregando...</p>
    </div>
  </div>
</div>

<!-- ════════════════════════════ ABA CONFIG ═══════════════════════════════ -->
<div id="tab-config" class="tab">
  <div class="grid2">

    <div class="card">
      <h2>🌱 Estufa A</h2>

      <h3>💧 Bomba de Irrigação (Umidade do Solo)</h3>
      <div class="cfg-row">
        <span class="cfg-lbl">Ligar bomba quando umidade &lt; (%)</span>
        <input class="cfg-input" type="number" id="cfg-EA-umidade_min" step="1" min="0" max="100">
      </div>
      <div class="cfg-row">
        <span class="cfg-lbl">Desligar bomba quando umidade &gt; (%)</span>
        <input class="cfg-input" type="number" id="cfg-EA-umidade_max" step="1" min="0" max="100">
      </div>
      <div class="cfg-row" style="background:#fff0f0;border-radius:6px;padding:4px 8px">
        <span class="cfg-lbl">🔴 Alerta crítico abaixo de (%)</span>
        <input class="cfg-input" type="number" id="cfg-EA-critico_umidade" step="1" min="0" max="100">
      </div>

      <h3>🌀 Ventilador (Temperatura)</h3>
      <div class="cfg-row">
        <span class="cfg-lbl">Ligar ventilador quando temp &gt; (°C)</span>
        <input class="cfg-input" type="number" id="cfg-EA-temp_max" step="0.5" min="0" max="60">
      </div>
      <div class="cfg-row" style="background:#fff0f0;border-radius:6px;padding:4px 8px">
        <span class="cfg-lbl">🔴 Alerta crítico acima de (°C)</span>
        <input class="cfg-input" type="number" id="cfg-EA-critico_temp" step="0.5" min="0" max="60">
      </div>

      <button class="btn-save" onclick="salvarConfig('Estufa_A')">💾 Salvar Estufa A</button>
    </div>

    <div class="card">
      <h2>🐔 Galinheiro A</h2>

      <h3>💨 Exaustor (Amônia)</h3>
      <div class="cfg-row">
        <span class="cfg-lbl">Ligar exaustor quando amônia &gt; (ppm)</span>
        <input class="cfg-input" type="number" id="cfg-GA-amonia_max" step="1" min="0" max="100">
      </div>
      <div class="cfg-row" style="background:#fff0f0;border-radius:6px;padding:4px 8px">
        <span class="cfg-lbl">🔴 Alerta crítico acima de (ppm)</span>
        <input class="cfg-input" type="number" id="cfg-GA-critico_amonia" step="1" min="0" max="100">
      </div>

      <h3>🔥 Aquecedor (Temperatura)</h3>
      <div class="cfg-row">
        <span class="cfg-lbl">Ligar aquecedor quando temp &lt; (°C)</span>
        <input class="cfg-input" type="number" id="cfg-GA-temp_min" step="0.5" min="0" max="40">
      </div>

      <h3>⚙️ Motor Comedouro (Ração)</h3>
      <div class="cfg-row">
        <span class="cfg-lbl">Ligar motor quando ração &lt; (%)</span>
        <input class="cfg-input" type="number" id="cfg-GA-racao_min" step="1" min="0" max="100">
      </div>
      <div class="cfg-row">
        <span class="cfg-lbl">Desligar motor quando ração &gt; (%)</span>
        <input class="cfg-input" type="number" id="cfg-GA-racao_max" step="1" min="0" max="100">
      </div>
      <div class="cfg-row" style="background:#fff0f0;border-radius:6px;padding:4px 8px">
        <span class="cfg-lbl">🔴 Alerta crítico abaixo de (%)</span>
        <input class="cfg-input" type="number" id="cfg-GA-critico_racao" step="1" min="0" max="100">
      </div>

      <h3>🚰 Válvula de Água</h3>
      <div class="cfg-row">
        <span class="cfg-lbl">Ligar válvula quando água &lt; (%)</span>
        <input class="cfg-input" type="number" id="cfg-GA-agua_min" step="1" min="0" max="100">
      </div>
      <div class="cfg-row">
        <span class="cfg-lbl">Desligar válvula quando água &gt; (%)</span>
        <input class="cfg-input" type="number" id="cfg-GA-agua_max" step="1" min="0" max="100">
      </div>
      <div class="cfg-row" style="background:#fff0f0;border-radius:6px;padding:4px 8px">
        <span class="cfg-lbl">🔴 Alerta crítico abaixo de (%)</span>
        <input class="cfg-input" type="number" id="cfg-GA-critico_agua" step="1" min="0" max="100">
      </div>

      <button class="btn-save" onclick="salvarConfig('Galinheiro_A')">💾 Salvar Galinheiro A</button>
    </div>

  </div>
</div>

<script>
// ═══════════════════════════════════════════════════════════════════════════
// Estado global
// ═══════════════════════════════════════════════════════════════════════════
let sensorChart = null;
let sensorSel   = { node: 'Estufa_A', tipo: 'umidade', unidade: '%' };
let estadoAtual = {};

// ═══════════════════════════════════════════════════════════════════════════
// Navegação por abas
// ═══════════════════════════════════════════════════════════════════════════
function mostrarAba(nome) {
  document.querySelectorAll('.tab').forEach(t => t.classList.remove('ativo'));
  document.querySelectorAll('nav button').forEach(b => b.classList.remove('ativo'));
  document.getElementById('tab-' + nome).classList.add('ativo');
  const idx = {'sensores':0,'atuadores':1,'alertas':2,'config':3}[nome];
  document.querySelectorAll('nav button')[idx].classList.add('ativo');

  if (nome === 'atuadores')  carregarHistoricoAtuadores();
  if (nome === 'alertas')    carregarAlertas();
  if (nome === 'config')     carregarConfig();
}

// ═══════════════════════════════════════════════════════════════════════════
// Gráfico do sensor
// ═══════════════════════════════════════════════════════════════════════════
function criarGrafico() {
  if (sensorChart) { sensorChart.destroy(); sensorChart = null; }
  const ctx = document.getElementById('grafico-sensor').getContext('2d');
  sensorChart = new Chart(ctx, {
    type: 'line',
    data: {
      labels: [],
      datasets: [{
        label: sensorSel.node + ' / ' + sensorSel.tipo + ' (' + sensorSel.unidade + ')',
        borderColor: corSensor(sensorSel.tipo),
        backgroundColor: corSensor(sensorSel.tipo) + '18',
        data: [], fill: true, tension: 0.3, pointRadius: 2
      }]
    },
    options: {
      animation: false, responsive: true,
      scales: {
        x: { ticks: { maxTicksLimit: 10, maxRotation: 0 } },
        y: { beginAtZero: false }
      },
      plugins: { legend: { position: 'top' } }
    }
  });
}

function corSensor(tipo) {
  return { umidade:'#3498db', temperatura:'#e74c3c', luminosidade:'#f39c12',
           amonia:'#9b59b6', racao:'#e67e22', agua:'#1abc9c' }[tipo] || '#555';
}

async function trocarSensor() {
  const val = document.getElementById('sensor-select').value.split('|');
  sensorSel = { node: val[0], tipo: val[1], unidade: val[2] };
  criarGrafico();
  await carregarHistoricoSensor();
  atualizarValorAtual();
}

async function carregarHistoricoSensor() {
  try {
    const r = await fetch('/api/sensor/' + sensorSel.tipo + '?horas=1');
    const dados = await r.json();
    if (!Array.isArray(dados) || !sensorChart) return;

    // Filtra pelo node selecionado e ordena por timestamp ascendente
    const filtrado = dados
      .filter(d => d.node_id === sensorSel.node)
      .reverse()
      .slice(-60);

    sensorChart.data.labels = filtrado.map(d => {
      const t = new Date(d.timestamp);
      return t.toLocaleTimeString('pt-BR', {hour:'2-digit',minute:'2-digit',second:'2-digit'});
    });
    sensorChart.data.datasets[0].data = filtrado.map(d => d.valor);
    sensorChart.update();
  } catch(e) { console.warn('Erro histórico sensor:', e); }
}

function atualizarValorAtual() {
  const node = estadoAtual[sensorSel.node];
  if (!node) return;
  const val = node[sensorSel.tipo];
  document.getElementById('valor-atual').textContent = (val !== undefined && val !== 0) ? val.toFixed(1) : '--';
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

// ═══════════════════════════════════════════════════════════════════════════
// Polling de estado (/api/estado) — 1s
// ═══════════════════════════════════════════════════════════════════════════
async function buscarEstado() {
  try {
    const r = await fetch('/api/estado');
    if (!r.ok) throw new Error('HTTP ' + r.status);
    const d = await r.json();
    estadoAtual = d;

    document.getElementById('status-cn').textContent = '● Online';
    document.getElementById('status-cn').className = 'online';

    // Atualiza aba sensores
    atualizarValorAtual();
    const node = d[sensorSel.node];
    if (node) {
      const v = node[sensorSel.tipo];
      if (v !== undefined) adicionarPontoGrafico(v);
    }

    // Atualiza badges de atuadores
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
  } catch(e) {
    document.getElementById('status-cn').textContent = '● Desconectado';
    document.getElementById('status-cn').className = 'offline';
  }
}

function setBadge(id, ligado) {
  const el = document.getElementById(id);
  if (!el) return;
  el.textContent = ligado ? 'LIGADO 🟢' : 'DESLIGADO 🔴';
  el.className = 'st-badge ' + (ligado ? 'st-on' : 'st-off');
}

// ═══════════════════════════════════════════════════════════════════════════
// Controle de atuadores
// ═══════════════════════════════════════════════════════════════════════════
async function cmd(nodeID, atuadorID, comando) {
  try {
    const r = await fetch('/api/comando', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ node_id: nodeID, atuador_id: atuadorID, comando })
    });
    if (!r.ok) { const e = await r.json(); alert('Erro: ' + (e.erro || r.status)); }
  } catch(e) { alert('Falha de conexão: ' + e.message); }
}

// ═══════════════════════════════════════════════════════════════════════════
// Histórico de atuadores
// ═══════════════════════════════════════════════════════════════════════════
async function carregarHistoricoAtuadores() {
  const container = document.getElementById('historico-container');
  try {
    const r = await fetch('/api/atuador/history?horas=24');
    const dados = await r.json();
    if (!Array.isArray(dados) || dados.length === 0) {
      container.innerHTML = '<p class="sem-dados">Nenhuma ativação registrada nas últimas 24h.</p>';
      return;
    }
    const nomeAtu = {
      'bomba_irrigacao_01':'💧 Bomba Irrigação','ventilador_01':'🌀 Ventilador',
      'painel_led_01':'💡 LED','exaustor_teto_01':'💨 Exaustor',
      'aquecedor_01':'🔥 Aquecedor','motor_comedouro_01':'⚙️ Motor Comedouro',
      'valvula_agua_01':'🚰 Válvula Água'
    };
    const rows = dados.slice(0, 100).map(d => {
      const t = new Date(d.timestamp).toLocaleString('pt-BR');
      const nome = nomeAtu[d.atuador] || d.atuador;
      const cor  = d.acao === 'LIGAR' ? '#d8f3dc' : '#fdecea';
      return '<tr style="background:'+cor+'10"><td>'+t+'</td><td>'+d.node_id+'</td><td>'+nome+'</td>'
           + '<td><b>'+d.acao+'</b></td><td>'+(d.motivo||'-')+'</td></tr>';
    }).join('');
    container.innerHTML = '<table><thead><tr><th>Hora</th><th>Node</th><th>Atuador</th><th>Ação</th><th>Motivo</th></tr></thead><tbody>'
      + rows + '</tbody></table>';
  } catch(e) {
    container.innerHTML = '<p class="sem-dados">Erro ao carregar histórico.</p>';
  }
}

// ═══════════════════════════════════════════════════════════════════════════
// Alertas
// ═══════════════════════════════════════════════════════════════════════════
async function carregarAlertas() {
  try {
    const r = await fetch('/api/alertas');
    const todos = await r.json();
    if (!Array.isArray(todos)) return;

    const ativos = todos.filter(a => !a.ack).reverse();
    const hist   = todos.slice().reverse().slice(0, 50);

    // Badge no header
    const badge = document.getElementById('badge-alertas');
    if (ativos.length > 0) {
      badge.textContent = ativos.length + ' alerta' + (ativos.length > 1 ? 's' : '');
      badge.classList.add('visivel');
    } else {
      badge.classList.remove('visivel');
    }

    // Alertas ativos
    const cAtivos = document.getElementById('alertas-ativos-container');
    if (ativos.length === 0) {
      cAtivos.innerHTML = '<p class="sem-dados">✅ Nenhum alerta ativo.</p>';
    } else {
      cAtivos.innerHTML = ativos.map(a => renderAlerta(a, true)).join('');
    }

    // Histórico
    const cHist = document.getElementById('alertas-hist-container');
    if (hist.length === 0) {
      cHist.innerHTML = '<p class="sem-dados">Nenhum alerta registrado.</p>';
    } else {
      cHist.innerHTML = hist.map(a => renderAlerta(a, false)).join('');
    }
  } catch(e) {
    document.getElementById('alertas-ativos-container').innerHTML = '<p class="sem-dados">Erro ao carregar alertas.</p>';
  }
}

function renderAlerta(a, comBotao) {
  const t = new Date(a.timestamp).toLocaleString('pt-BR');
  const cls = a.ack ? 'av-ack' : (a.nivel === 'critico' ? 'av-critico' : 'av-aviso');
  const nlCls = a.ack ? 'nl-ack' : (a.nivel === 'critico' ? 'nl-critico' : 'nl-aviso');
  const nlTxt = a.ack ? 'RECONHECIDO' : a.nivel.toUpperCase();
  const btn   = (comBotao && !a.ack)
    ? '<button class="btn-ack" onclick="ackAlerta(\''+a.id+'\')">✓ Reconhecer</button>'
    : '';
  return '<div class="alerta-card '+cls+'">'
    + '<span class="av-nivel '+nlCls+'">'+nlTxt+'</span>'
    + '<div class="av-body"><div class="av-msg">'+a.mensagem+'</div>'
    + '<div class="av-meta">'+a.node_id+' / '+a.tipo+' = '+a.valor.toFixed(1)+' — '+t+'</div></div>'
    + btn + '</div>';
}

async function ackAlerta(id) {
  try {
    await fetch('/api/alertas/ack', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id })
    });
    carregarAlertas();
  } catch(e) { alert('Erro ao reconhecer alerta: ' + e.message); }
}

// Polling de alertas ativos a cada 5s para atualizar badge
async function pollAlertas() {
  try {
    const r = await fetch('/api/alertas?ativos=true');
    const ativos = await r.json();
    if (!Array.isArray(ativos)) return;
    const badge = document.getElementById('badge-alertas');
    if (ativos.length > 0) {
      badge.textContent = ativos.length + ' alerta' + (ativos.length > 1 ? 's' : '');
      badge.classList.add('visivel');
    } else {
      badge.classList.remove('visivel');
    }
    // Se a aba de alertas estiver ativa, recarrega também
    if (document.getElementById('tab-alertas').classList.contains('ativo')) {
      carregarAlertas();
    }
  } catch(e) {}
}

// ═══════════════════════════════════════════════════════════════════════════
// Configurações
// ═══════════════════════════════════════════════════════════════════════════
async function carregarConfig() {
  try {
    const r = await fetch('/api/config');
    const cfg = await r.json();
    if (!cfg) return;

    const ea = cfg.Estufa_A    || {};
    const ga = cfg.Galinheiro_A || {};

    setInput('cfg-EA-umidade_min',    ea.umidade_min);
    setInput('cfg-EA-umidade_max',    ea.umidade_max);
    setInput('cfg-EA-temp_max',       ea.temp_max);
    setInput('cfg-EA-critico_umidade',ea.critico_umidade);
    setInput('cfg-EA-critico_temp',   ea.critico_temp);

    setInput('cfg-GA-amonia_max',     ga.amonia_max);
    setInput('cfg-GA-critico_amonia', ga.critico_amonia);
    setInput('cfg-GA-temp_min',       ga.temp_min);
    setInput('cfg-GA-racao_min',      ga.racao_min);
    setInput('cfg-GA-racao_max',      ga.racao_max);
    setInput('cfg-GA-critico_racao',  ga.critico_racao);
    setInput('cfg-GA-agua_min',       ga.agua_min);
    setInput('cfg-GA-agua_max',       ga.agua_max);
    setInput('cfg-GA-critico_agua',   ga.critico_agua);
  } catch(e) { console.warn('Erro ao carregar config:', e); }
}

function setInput(id, val) {
  const el = document.getElementById(id);
  if (el && val !== undefined) el.value = val;
}

async function salvarConfig(nodeID) {
  const prefix = nodeID === 'Estufa_A' ? 'cfg-EA-' : 'cfg-GA-';
  const campos = nodeID === 'Estufa_A'
    ? ['umidade_min','umidade_max','temp_max','critico_umidade','critico_temp']
    : ['amonia_max','critico_amonia','temp_min','racao_min','racao_max','critico_racao','agua_min','agua_max','critico_agua'];

  const payload = { [nodeID]: {} };
  let ok = true;
  campos.forEach(c => {
    const el = document.getElementById(prefix + c);
    if (el && el.value !== '') {
      const v = parseFloat(el.value);
      if (isNaN(v)) { ok = false; return; }
      payload[nodeID][c] = v;
    }
  });

  if (!ok) { alert('Valores inválidos — verifique os campos.'); return; }

  try {
    const r = await fetch('/api/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    const res = await r.json();
    if (res.status === 'ok') {
      alert('✅ Configurações de ' + (nodeID === 'Estufa_A' ? 'Estufa A' : 'Galinheiro A') + ' salvas!');
    } else {
      alert('Erro: ' + (res.erro || 'desconhecido'));
    }
  } catch(e) { alert('Falha de conexão: ' + e.message); }
}

// ═══════════════════════════════════════════════════════════════════════════
// Inicialização
// ═══════════════════════════════════════════════════════════════════════════
criarGrafico();
carregarHistoricoSensor();

setInterval(buscarEstado, 1000);
setInterval(pollAlertas,  5000);
buscarEstado();
pollAlertas();
</script>
</body>
</html>`
}
