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
body{font-family:Arial,sans-serif;background:#eef2ee;color:#2c3e50;font-size:13px}
/* ── Header ── */
header{background:#2d6a4f;color:white;padding:10px 20px;display:flex;align-items:center;gap:10px}
header h1{font-size:1rem;font-weight:bold;flex:1}
#status-ws{font-size:.78rem;padding:2px 9px;border-radius:10px}
.ws-on{background:#27ae60}.ws-off{background:#c0392b}.ws-rec{background:#e67e22}
/* ── Nav ── */
nav{background:#1b4332;display:flex;padding:0 16px}
nav button{background:none;border:none;color:#a8c4b0;padding:10px 16px;cursor:pointer;font-size:.85rem;border-bottom:3px solid transparent}
nav button.ativo{color:#fff;border-bottom-color:#52b788}
nav button:hover{color:#fff}
/* ── Tabs ── */
.tab{display:none;padding:14px 16px;max-width:1300px;margin:0 auto}
.tab.ativo{display:block}
/* ── Cards ── */
.card{background:#fff;border-radius:8px;padding:14px;box-shadow:0 1px 5px rgba(0,0,0,.07);margin-bottom:12px}
.card-title{font-size:.9rem;font-weight:bold;color:#1b4332;border-bottom:2px solid #d8f3dc;padding-bottom:5px;margin-bottom:10px}
/* ── Layout grade principal ── */
.grade-principal{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:12px}
@media(max-width:900px){.grade-principal{grid-template-columns:1fr}}
/* ── Sensores no card ── */
.sens-grade{display:grid;grid-template-columns:1fr 1fr 1fr;gap:6px;margin-bottom:10px}
@media(max-width:700px){.sens-grade{grid-template-columns:1fr 1fr}}
.sens-item{background:#f6fbf7;border-radius:6px;padding:8px;text-align:center;border:1px solid #e0ede0}
.sens-label{font-size:.7rem;color:#666;margin-bottom:2px}
.sens-valor{font-size:1.3rem;font-weight:bold;color:#2d6a4f}
.sens-unidade{font-size:.72rem;color:#888}
/* ── Atuadores no card ── */
.atu-row{display:flex;align-items:center;gap:6px;padding:5px 0;border-bottom:1px solid #f0f0f0}
.atu-row:last-child{border-bottom:none}
.atu-label{flex:1;font-size:.82rem;color:#444}
.st{font-size:.75rem;font-weight:bold;padding:2px 7px;border-radius:8px;min-width:68px;text-align:center}
.st-on{background:#d8f3dc;color:#1b4332}.st-off{background:#f0f0f0;color:#999}
.btn{padding:3px 9px;border-radius:4px;border:1px solid;cursor:pointer;font-size:.76rem;background:#fff;transition:.1s}
.btn:hover{filter:brightness(.9)}
.btn-on{border-color:#27ae60;color:#27ae60}
.btn-off{border-color:#e74c3c;color:#e74c3c}
/* ── Grafico ── */
#grafico-wrap{margin-bottom:12px}
.grafico-sel{display:flex;gap:8px;align-items:center;margin-bottom:8px;flex-wrap:wrap}
.grafico-sel select{padding:5px 8px;border:1px solid #ccc;border-radius:5px;font-size:.82rem;background:#fff}
.grafico-sel select:focus{outline:none;border-color:#52b788}
/* ── Blocos de alertas ── */
.alertas-grade{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:12px}
@media(max-width:700px){.alertas-grade{grid-template-columns:1fr}}
.bloco-criticos .card-title{color:#c0392b;border-bottom-color:#f5c6c6}
.bloco-avisos   .card-title{color:#d68910;border-bottom-color:#fde9b0}
.bloco-hist     .card-title{color:#555;border-bottom-color:#e0e0e0}
/* ── Card de alerta individual ── */
.alerta{border-radius:6px;padding:9px 12px;margin-bottom:7px;display:flex;align-items:flex-start;gap:8px}
.al-critico{background:#fdecea;border-left:4px solid #c0392b;animation:pulse 1.4s infinite}
.al-aviso  {background:#fff8e1;border-left:4px solid #e67e22}
.al-ack    {background:#f5f5f5;border-left:4px solid #ccc;opacity:.6}
@keyframes pulse{0%,100%{opacity:1}50%{opacity:.75}}
.al-nivel{font-size:.7rem;font-weight:bold;padding:1px 6px;border-radius:8px;white-space:nowrap;align-self:flex-start}
.nl-critico{background:#c0392b;color:#fff}
.nl-aviso  {background:#e67e22;color:#fff}
.nl-ack    {background:#bbb;color:#fff}
.al-body{flex:1}
.al-msg {font-size:.83rem;font-weight:bold;line-height:1.3}
.al-meta{font-size:.71rem;color:#888;margin-top:2px}
.btn-ack{background:#fff;border:1px solid #ccc;padding:2px 8px;border-radius:4px;cursor:pointer;font-size:.74rem}
.btn-ack:hover{background:#f0f0f0}
.sem-dados{color:#bbb;text-align:center;padding:14px;font-size:.83rem}
/* ── Tabela historico ── */
table{width:100%;border-collapse:collapse;font-size:.78rem}
th{background:#f0f4f0;color:#1b4332;padding:5px 9px;text-align:left;font-weight:bold}
td{padding:4px 9px;border-bottom:1px solid #f0f0f0}
tr:hover td{background:#fafafa}
/* ── Config ── */
.cfg-grid{display:grid;grid-template-columns:1fr 1fr;gap:14px}
@media(max-width:700px){.cfg-grid{grid-template-columns:1fr}}
.cfg-row{display:flex;align-items:center;gap:8px;margin-bottom:8px}
.cfg-lbl{flex:1;font-size:.83rem;color:#444}
.cfg-input{width:80px;padding:5px 7px;border:1px solid #ccc;border-radius:4px;font-size:.83rem}
.cfg-crit{background:#fff5f5;border-radius:4px;padding:3px 6px}
.cfg-crit .cfg-lbl{color:#c0392b}
.cfg-crit .cfg-input{border-color:#e88}
.cfg-h3{font-size:.82rem;font-weight:bold;color:#40916c;margin:10px 0 5px}
.btn-save{background:#2d6a4f;color:#fff;border:none;padding:8px 18px;border-radius:5px;cursor:pointer;font-size:.85rem;margin-top:10px}
.btn-save:hover{background:#1b4332}
</style>
</head>
<body>

<header>
  <h1>FarmNode v3 — Painel Central</h1>
  <span id="status-ws" class="ws-off">Desconectado</span>
</header>

<nav>
  <button class="ativo" onclick="aba('principal')">Painel</button>
  <button onclick="aba('config')">Configuracoes</button>
</nav>

<!-- ═══════════════════════ ABA PAINEL ═══════════════════════ -->
<div id="tab-principal" class="tab ativo">

  <!-- Linha superior: Estufa A | Galinheiro A -->
  <div class="grade-principal">

    <!-- Estufa A -->
    <div class="card">
      <div class="card-title">Estufa A — Sensores e Controles</div>

      <div class="sens-grade">
        <div class="sens-item">
          <div class="sens-label">Umidade Solo</div>
          <div class="sens-valor" id="ea-umidade">--</div>
          <div class="sens-unidade">%</div>
        </div>
        <div class="sens-item">
          <div class="sens-label">Temperatura</div>
          <div class="sens-valor" id="ea-temp">--</div>
          <div class="sens-unidade">C</div>
        </div>
        <div class="sens-item">
          <div class="sens-label">Luminosidade</div>
          <div class="sens-valor" id="ea-luz">--</div>
          <div class="sens-unidade">Lux</div>
        </div>
      </div>

      <div class="atu-row">
        <span class="atu-label">Bomba Irrigacao</span>
        <span class="st st-off" id="st-bomba">DESLIG.</span>
        <button class="btn btn-on"  onclick="cmd('Estufa_A','bomba_irrigacao_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Estufa_A','bomba_irrigacao_01','DESLIGAR')">Desligar</button>
      </div>
      <div class="atu-row">
        <span class="atu-label">Ventilador</span>
        <span class="st st-off" id="st-ventilador">DESLIG.</span>
        <button class="btn btn-on"  onclick="cmd('Estufa_A','ventilador_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Estufa_A','ventilador_01','DESLIGAR')">Desligar</button>
      </div>
      <div class="atu-row">
        <span class="atu-label">Painel LED</span>
        <span class="st st-off" id="st-led">DESLIG.</span>
        <button class="btn btn-on"  onclick="cmd('Estufa_A','painel_led_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Estufa_A','painel_led_01','DESLIGAR')">Desligar</button>
      </div>
    </div>

    <!-- Galinheiro A -->
    <div class="card">
      <div class="card-title">Galinheiro A — Sensores e Controles</div>

      <div class="sens-grade">
        <div class="sens-item">
          <div class="sens-label">Amonia</div>
          <div class="sens-valor" id="ga-amonia">--</div>
          <div class="sens-unidade">ppm</div>
        </div>
        <div class="sens-item">
          <div class="sens-label">Temperatura</div>
          <div class="sens-valor" id="ga-temp">--</div>
          <div class="sens-unidade">C</div>
        </div>
        <div class="sens-item">
          <div class="sens-label">Racao</div>
          <div class="sens-valor" id="ga-racao">--</div>
          <div class="sens-unidade">%</div>
        </div>
        <div class="sens-item">
          <div class="sens-label">Agua</div>
          <div class="sens-valor" id="ga-agua">--</div>
          <div class="sens-unidade">%</div>
        </div>
      </div>

      <div class="atu-row">
        <span class="atu-label">Exaustor de Teto</span>
        <span class="st st-off" id="st-exaustor">DESLIG.</span>
        <button class="btn btn-on"  onclick="cmd('Galinheiro_A','exaustor_teto_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Galinheiro_A','exaustor_teto_01','DESLIGAR')">Desligar</button>
      </div>
      <div class="atu-row">
        <span class="atu-label">Aquecedor</span>
        <span class="st st-off" id="st-aquecedor">DESLIG.</span>
        <button class="btn btn-on"  onclick="cmd('Galinheiro_A','aquecedor_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Galinheiro_A','aquecedor_01','DESLIGAR')">Desligar</button>
      </div>
      <div class="atu-row">
        <span class="atu-label">Motor Comedouro</span>
        <span class="st st-off" id="st-motor">DESLIG.</span>
        <button class="btn btn-on"  onclick="cmd('Galinheiro_A','motor_comedouro_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Galinheiro_A','motor_comedouro_01','DESLIGAR')">Desligar</button>
      </div>
      <div class="atu-row">
        <span class="atu-label">Valvula de Agua</span>
        <span class="st st-off" id="st-valvula">DESLIG.</span>
        <button class="btn btn-on"  onclick="cmd('Galinheiro_A','valvula_agua_01','LIGAR')">Ligar</button>
        <button class="btn btn-off" onclick="cmd('Galinheiro_A','valvula_agua_01','DESLIGAR')">Desligar</button>
      </div>
    </div>
  </div>

  <!-- Grafico historico -->
  <div id="grafico-wrap" class="card">
    <div class="card-title">Historico em Tempo Real</div>
    <div class="grafico-sel">
      <span style="font-size:.82rem;color:#666">Sensor:</span>
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
          <option value="Galinheiro_A|agua|%">Agua</option>
        </optgroup>
      </select>
      <span id="grafico-label" style="font-size:.78rem;color:#888"></span>
    </div>
    <canvas id="grafico" height="70"></canvas>
  </div>

  <!-- Blocos de alertas: criticos | avisos -->
  <div class="alertas-grade">
    <div class="bloco-criticos">
      <div class="card">
        <div class="card-title">Alertas Criticos</div>
        <div id="bloco-criticos-cont"><p class="sem-dados">Nenhum alerta critico.</p></div>
      </div>
    </div>
    <div class="bloco-avisos">
      <div class="card">
        <div class="card-title">Avisos</div>
        <div id="bloco-avisos-cont"><p class="sem-dados">Nenhum aviso ativo.</p></div>
      </div>
    </div>
  </div>

  <!-- Historico de ativacoes e alertas reconhecidos -->
  <div class="bloco-hist">
    <div class="card">
      <div class="card-title">Historico de Ativacoes (24h)</div>
      <div id="hist-atu-cont"><p class="sem-dados">Carregando...</p></div>
    </div>
  </div>

</div>

<!-- ═══════════════════════ ABA CONFIG ═══════════════════════ -->
<div id="tab-config" class="tab">
  <div class="cfg-grid">

    <div class="card">
      <div class="card-title">Estufa A</div>

      <div class="cfg-h3">Bomba de Irrigacao (Umidade)</div>
      <div class="cfg-row">
        <span class="cfg-lbl">Ligar abaixo de (%)</span>
        <input class="cfg-input" type="number" id="cfg-EA-umidade_min" step="1" min="0" max="100">
      </div>
      <div class="cfg-row">
        <span class="cfg-lbl">Desligar acima de (%)</span>
        <input class="cfg-input" type="number" id="cfg-EA-umidade_max" step="1" min="0" max="100">
      </div>
      <div class="cfg-row cfg-crit">
        <span class="cfg-lbl">Critico abaixo de (%)</span>
        <input class="cfg-input" type="number" id="cfg-EA-critico_umidade" step="1" min="0" max="100">
      </div>

      <div class="cfg-h3">Ventilador (Temperatura)</div>
      <div class="cfg-row">
        <span class="cfg-lbl">Ligar acima de (C)</span>
        <input class="cfg-input" type="number" id="cfg-EA-temp_max" step="0.5" min="0" max="60">
      </div>
      <div class="cfg-row cfg-crit">
        <span class="cfg-lbl">Critico acima de (C)</span>
        <input class="cfg-input" type="number" id="cfg-EA-critico_temp" step="0.5" min="0" max="60">
      </div>

      <div class="cfg-h3">Painel LED (Luminosidade)</div>
      <div class="cfg-row">
        <span class="cfg-lbl">Ligar abaixo de (Lux)</span>
        <input class="cfg-input" type="number" id="cfg-EA-luz_min" step="10" min="0" max="2000">
      </div>
      <div class="cfg-row cfg-crit">
        <span class="cfg-lbl">Critico abaixo de (Lux)</span>
        <input class="cfg-input" type="number" id="cfg-EA-critico_luz" step="10" min="0" max="2000">
      </div>

      <button class="btn-save" onclick="salvarConfig('Estufa_A')">Salvar Estufa A</button>
    </div>

    <div class="card">
      <div class="card-title">Galinheiro A</div>

      <div class="cfg-h3">Exaustor (Amonia)</div>
      <div class="cfg-row">
        <span class="cfg-lbl">Ligar acima de (ppm)</span>
        <input class="cfg-input" type="number" id="cfg-GA-amonia_max" step="1" min="0" max="100">
      </div>
      <div class="cfg-row cfg-crit">
        <span class="cfg-lbl">Critico acima de (ppm)</span>
        <input class="cfg-input" type="number" id="cfg-GA-critico_amonia" step="1" min="0" max="100">
      </div>

      <div class="cfg-h3">Aquecedor (Temperatura)</div>
      <div class="cfg-row">
        <span class="cfg-lbl">Ligar abaixo de (C)</span>
        <input class="cfg-input" type="number" id="cfg-GA-temp_min" step="0.5" min="0" max="40">
      </div>
      <div class="cfg-row cfg-crit">
        <span class="cfg-lbl">Critico abaixo de (C)</span>
        <input class="cfg-input" type="number" id="cfg-GA-critico_temp" step="0.5" min="0" max="40">
      </div>

      <div class="cfg-h3">Motor Comedouro (Racao)</div>
      <div class="cfg-row">
        <span class="cfg-lbl">Ligar abaixo de (%)</span>
        <input class="cfg-input" type="number" id="cfg-GA-racao_min" step="1" min="0" max="100">
      </div>
      <div class="cfg-row">
        <span class="cfg-lbl">Desligar acima de (%)</span>
        <input class="cfg-input" type="number" id="cfg-GA-racao_max" step="1" min="0" max="100">
      </div>
      <div class="cfg-row cfg-crit">
        <span class="cfg-lbl">Critico abaixo de (%)</span>
        <input class="cfg-input" type="number" id="cfg-GA-critico_racao" step="1" min="0" max="100">
      </div>

      <div class="cfg-h3">Valvula de Agua</div>
      <div class="cfg-row">
        <span class="cfg-lbl">Ligar abaixo de (%)</span>
        <input class="cfg-input" type="number" id="cfg-GA-agua_min" step="1" min="0" max="100">
      </div>
      <div class="cfg-row">
        <span class="cfg-lbl">Desligar acima de (%)</span>
        <input class="cfg-input" type="number" id="cfg-GA-agua_max" step="1" min="0" max="100">
      </div>
      <div class="cfg-row cfg-crit">
        <span class="cfg-lbl">Critico abaixo de (%)</span>
        <input class="cfg-input" type="number" id="cfg-GA-critico_agua" step="1" min="0" max="100">
      </div>

      <button class="btn-save" onclick="salvarConfig('Galinheiro_A')">Salvar Galinheiro A</button>
    </div>
  </div>
</div>

<script>
// WebSocket
let ws = null, wsConectando = false;
let sensorSel = { node:'Estufa_A', tipo:'umidade', unidade:'%' };
let sensorChart = null;
let todosAlertas = [];

function conectarWS() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  ws = new WebSocket(proto + '://' + location.host + '/ws');
  setWS('rec', 'Conectando...');
  ws.onopen  = () => { setWS('on', 'Conectado'); wsConectando = false; };
  ws.onmessage = ev => {
    try {
      const m = JSON.parse(ev.data);
      if (m.tipo === 'estado') receberEstado(m.dados);
      if (m.tipo === 'alerta') receberAlertas(m.dados);
    } catch(e) {}
  };
  ws.onclose = () => {
    setWS('off', 'Desconectado');
    if (!wsConectando) { wsConectando = true; setTimeout(conectarWS, 3000); }
  };
  ws.onerror = () => setWS('off', 'Erro');
}

function setWS(s, t) {
  const el = document.getElementById('status-ws');
  el.textContent = t; el.className = {on:'ws-on',off:'ws-off',rec:'ws-rec'}[s]||'ws-off';
}

function wsSend(o) {
  if (ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify(o));
}

// Navegacao
function aba(nome) {
  document.querySelectorAll('.tab').forEach(t => t.classList.remove('ativo'));
  document.querySelectorAll('nav button').forEach(b => b.classList.remove('ativo'));
  document.getElementById('tab-' + nome).classList.add('ativo');
  document.querySelectorAll('nav button')[nome==='principal'?0:1].classList.add('ativo');
  if (nome === 'config') carregarConfig();
}

// Atualiza estado de sensores e atuadores.
function receberEstado(d) {
  if (!d) return;

  // Estufa A
  if (d.Estufa_A) {
    setText('ea-umidade', d.Estufa_A.umidade);
    setText('ea-temp',    d.Estufa_A.temperatura);
    setText('ea-luz',     d.Estufa_A.luminosidade);
    setBadge('st-bomba',      d.Estufa_A.bomba_ligada);
    setBadge('st-ventilador', d.Estufa_A.ventilador_ligado);
    setBadge('st-led',        d.Estufa_A.led_ligado);
  }

  // Galinheiro A
  if (d.Galinheiro_A) {
    setText('ga-amonia', d.Galinheiro_A.amonia);
    setText('ga-temp',   d.Galinheiro_A.temperatura);
    setText('ga-racao',  d.Galinheiro_A.racao);
    setText('ga-agua',   d.Galinheiro_A.agua);
    setBadge('st-exaustor',  d.Galinheiro_A.exaustor_ligado);
    setBadge('st-aquecedor', d.Galinheiro_A.aquecedor_ligado);
    setBadge('st-motor',     d.Galinheiro_A.motor_ligado);
    setBadge('st-valvula',   d.Galinheiro_A.valvula_ligada);
  }

  // Grafico — adiciona ponto do sensor selecionado
  const node = d[sensorSel.node];
  if (node) {
    const v = node[sensorSel.tipo];
    if (v !== undefined) addPonto(v);
  }
}

function setText(id, v) {
  const el = document.getElementById(id);
  if (el) el.textContent = (v !== undefined && v !== null) ? parseFloat(v).toFixed(1) : '--';
}

function setBadge(id, on) {
  const el = document.getElementById(id);
  if (!el) return;
  el.textContent = on ? 'LIGADO' : 'DESLIG.';
  el.className = 'st ' + (on ? 'st-on' : 'st-off');
}

// Alertas separados em criticos e avisos.
function receberAlertas(dados) {
  if (!Array.isArray(dados)) return;
  todosAlertas = dados;
  renderAlertas();
}

function renderAlertas() {
  const ativos   = todosAlertas.filter(a => !a.ack);
  const criticos = ativos.filter(a => a.nivel === 'critico').reverse();
  const avisos   = ativos.filter(a => a.nivel !== 'critico').reverse();

  const cC = document.getElementById('bloco-criticos-cont');
  cC.innerHTML = criticos.length === 0
    ? '<p class="sem-dados">Nenhum alerta critico.</p>'
    : criticos.map(a => renderAlerta(a)).join('');

  const cA = document.getElementById('bloco-avisos-cont');
  cA.innerHTML = avisos.length === 0
    ? '<p class="sem-dados">Nenhum aviso ativo.</p>'
    : avisos.map(a => renderAlerta(a)).join('');
}

function renderAlerta(a) {
  const t   = new Date(a.timestamp).toLocaleString('pt-BR');
  const cls = a.ack ? 'al-ack' : (a.nivel==='critico' ? 'al-critico' : 'al-aviso');
  const nlC = a.ack ? 'nl-ack' : (a.nivel==='critico' ? 'nl-critico' : 'nl-aviso');
  const nlT = a.ack ? 'ACK' : a.nivel.toUpperCase();
  const btn = !a.ack ? '<button class="btn-ack" onclick="ackAlerta(\''+a.id+'\')">Reconhecer</button>' : '';
  return '<div class="alerta '+cls+'">'
    +'<span class="al-nivel '+nlC+'">'+nlT+'</span>'
    +'<div class="al-body"><div class="al-msg">'+a.mensagem+'</div>'
    +'<div class="al-meta">'+a.node_id+' / '+a.tipo+' = '+a.valor.toFixed(1)+' — '+t+'</div></div>'
    +btn+'</div>';
}

function ackAlerta(id) {
  wsSend({ tipo:'ack_alerta', id });
}

// Grafico
const cores = {
  umidade:'#3498db', temperatura:'#e74c3c', luminosidade:'#f39c12',
  amonia:'#9b59b6', racao:'#e67e22', agua:'#1abc9c'
};

function criarGrafico() {
  if (sensorChart) { sensorChart.destroy(); sensorChart = null; }
  const ctx = document.getElementById('grafico').getContext('2d');
  const cor = cores[sensorSel.tipo] || '#555';
  document.getElementById('grafico-label').textContent =
    sensorSel.node + ' / ' + sensorSel.tipo + ' (' + sensorSel.unidade + ')';
  sensorChart = new Chart(ctx, {
    type: 'line',
    data: {
      labels: [],
      datasets: [{ label: sensorSel.tipo, borderColor: cor, backgroundColor: cor+'15',
        data:[], fill:true, tension:0.3, pointRadius:1.5 }]
    },
    options: {
      animation: false, responsive: true,
      scales: {
        x: { ticks:{ maxTicksLimit:10, maxRotation:0 } },
        y: { beginAtZero: false }
      },
      plugins: { legend:{ display:false } }
    }
  });
}

function addPonto(v) {
  if (!sensorChart) return;
  const h = new Date().toLocaleTimeString('pt-BR',{hour:'2-digit',minute:'2-digit',second:'2-digit'});
  sensorChart.data.labels.push(h);
  sensorChart.data.datasets[0].data.push(v);
  if (sensorChart.data.labels.length > 60) {
    sensorChart.data.labels.shift();
    sensorChart.data.datasets[0].data.shift();
  }
  sensorChart.update();
}

async function trocarSensor() {
  const v = document.getElementById('sensor-select').value.split('|');
  sensorSel = { node:v[0], tipo:v[1], unidade:v[2] };
  criarGrafico();
  // Carrega historico via HTTP
  try {
    const r = await fetch('/api/sensor/'+sensorSel.tipo+'?horas=1');
    const dados = await r.json();
    if (!Array.isArray(dados) || !sensorChart) return;
    const filtrado = dados.filter(d => d.node_id === sensorSel.node).reverse().slice(-60);
    sensorChart.data.labels = filtrado.map(d =>
      new Date(d.timestamp).toLocaleTimeString('pt-BR',{hour:'2-digit',minute:'2-digit',second:'2-digit'})
    );
    sensorChart.data.datasets[0].data = filtrado.map(d => d.valor);
    sensorChart.update();
  } catch(e) {}
}

// Historico de ativacoes (inicial e a cada 30s).
const nomeAtu = {
  'bomba_irrigacao_01':'Bomba Irrigacao','ventilador_01':'Ventilador',
  'painel_led_01':'Painel LED','exaustor_teto_01':'Exaustor',
  'aquecedor_01':'Aquecedor','motor_comedouro_01':'Motor Comedouro',
  'valvula_agua_01':'Valvula Agua'
};

async function carregarHistAtu() {
  const c = document.getElementById('hist-atu-cont');
  try {
    const r = await fetch('/api/atuador/history?horas=24');
    const d = await r.json();
    if (!Array.isArray(d) || d.length === 0) {
      c.innerHTML = '<p class="sem-dados">Nenhuma ativacao nas ultimas 24h.</p>'; return;
    }
    const rows = d.slice(0, 80).map(x => {
      const t  = new Date(x.timestamp).toLocaleString('pt-BR');
      const nm = nomeAtu[x.atuador] || x.atuador;
      const bg = x.acao === 'LIGAR' ? '#e8f8ed' : '#fdecea';
      return '<tr style="background:'+bg+'"><td>'+t+'</td><td>'+x.node_id+'</td>'
           + '<td>'+nm+'</td><td><b>'+x.acao+'</b></td><td>'+(x.motivo||'-')+'</td></tr>';
    }).join('');
    c.innerHTML = '<table><thead><tr><th>Hora</th><th>Node</th><th>Atuador</th>'
      +'<th>Acao</th><th>Motivo</th></tr></thead><tbody>'+rows+'</tbody></table>';
  } catch(e) { c.innerHTML = '<p class="sem-dados">Erro.</p>'; }
}

// Comandos de atuadores.
function cmd(nodeID, atuadorID, comando) {
  wsSend({ tipo:'comando', node_id:nodeID, atuador_id:atuadorID, comando });
}

// Configuracoes.
async function carregarConfig() {
  try {
    const r = await fetch('/api/config');
    const c = await r.json();
    if (!c) return;
    const ea = c.Estufa_A || {}, ga = c.Galinheiro_A || {};
    si('cfg-EA-umidade_min',    ea.umidade_min);
    si('cfg-EA-umidade_max',    ea.umidade_max);
    si('cfg-EA-temp_max',       ea.temp_max);
    si('cfg-EA-luz_min',        ea.luz_min);
    si('cfg-EA-critico_umidade',ea.critico_umidade);
    si('cfg-EA-critico_temp',   ea.critico_temp);
    si('cfg-EA-critico_luz',    ea.critico_luz);
    si('cfg-GA-amonia_max',     ga.amonia_max);
    si('cfg-GA-critico_amonia', ga.critico_amonia);
    si('cfg-GA-temp_min',       ga.temp_min);
    si('cfg-GA-critico_temp',   ga.critico_temp);
    si('cfg-GA-racao_min',      ga.racao_min);
    si('cfg-GA-racao_max',      ga.racao_max);
    si('cfg-GA-critico_racao',  ga.critico_racao);
    si('cfg-GA-agua_min',       ga.agua_min);
    si('cfg-GA-agua_max',       ga.agua_max);
    si('cfg-GA-critico_agua',   ga.critico_agua);
  } catch(e) {}
}

function si(id, v) {
  const el = document.getElementById(id);
  if (el && v !== undefined) el.value = v;
}

function salvarConfig(nodeID) {
  const p = nodeID === 'Estufa_A' ? 'cfg-EA-' : 'cfg-GA-';
  const campos = nodeID === 'Estufa_A'
    ? ['umidade_min','umidade_max','temp_max','luz_min','critico_umidade','critico_temp','critico_luz']
    : ['amonia_max','critico_amonia','temp_min','critico_temp','racao_min','racao_max','critico_racao','agua_min','agua_max','critico_agua'];
  const dados = {};
  let ok = true;
  campos.forEach(c => {
    const el = document.getElementById(p + c);
    if (el && el.value !== '') {
      const v = parseFloat(el.value);
      if (isNaN(v)) { ok = false; return; }
      dados[c] = v;
    }
  });
  if (!ok) { alert('Valores invalidos.'); return; }
  wsSend({ tipo:'config', node_id:nodeID, dados });
  alert('Configuracoes enviadas!');
}

// Inicializacao da pagina.
criarGrafico();
trocarSensor();
carregarHistAtu();

// Carrega alertas iniciais via HTTP.
fetch('/api/alertas').then(r => r.json()).then(d => {
  if (Array.isArray(d)) { todosAlertas = d; renderAlertas(); }
}).catch(() => {});

// Recarrega historico a cada 30s.
setInterval(carregarHistAtu, 30000);

conectarWS();
</script>
</body>
</html>`
}
