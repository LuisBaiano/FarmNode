package main

func getDashboardHTML() string {
	return `<!DOCTYPE html>
<html lang="pt-BR">
<head>
<meta charset="UTF-8">
<title>FarmNode</title>
<script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:Arial,sans-serif;background:#eef4ef;color:#183028;display:flex;flex-direction:column;min-height:100vh}

/* ── Header ── */
header{background:#1f5c48;color:#fff;padding:9px 16px;display:flex;gap:10px;align-items:center;flex-shrink:0;position:sticky;top:0;z-index:100}
header h1{font-size:.95rem;flex:1}
#ws-badge{font-size:.75rem;padding:2px 8px;border-radius:10px}
.ws-on{background:#2ecc71}.ws-off{background:#e74c3c}.ws-rec{background:#f39c12}
#alerta-badge{background:#e74c3c;color:#fff;border-radius:10px;font-size:.72rem;padding:2px 7px;display:none}

/* ── Alertas — faixa acima do painel ── */
#faixa-alertas{background:#fff;border-bottom:2px solid #e2ede6;padding:0;overflow:hidden;transition:max-height .3s;max-height:0}
#faixa-alertas.visivel{max-height:320px;padding:8px 16px}
#faixa-alertas h3{font-size:.85rem;color:#c0392b;margin-bottom:6px;display:flex;justify-content:space-between;align-items:center}
#faixa-alertas h3 span{font-size:.72rem;color:#83968c;cursor:pointer;font-weight:normal}
#alertas-lista{max-height:250px;overflow-y:auto;padding-right:2px}
#alertas-lista::-webkit-scrollbar{width:4px}
#alertas-lista::-webkit-scrollbar-thumb{background:#e0b3ad;border-radius:2px}
.alerta-item{padding:6px 8px;border-left:3px solid #e67e22;background:#fff8e6;border-radius:5px;margin-bottom:5px;font-size:.77rem}
.alerta-item.crit{border-left-color:#c0392b;background:#fdecea}
.alerta-meta{color:#83968c;font-size:.72rem;margin-top:1px}

/* ── Tabs ── */
.tabs-wrap{background:#fff;border-bottom:2px solid #1f5c48;padding:0 16px;display:flex;gap:2px;position:sticky;top:40px;z-index:90}
.tab{padding:7px 18px;font-size:.83rem;cursor:pointer;border-bottom:2px solid transparent;margin-bottom:-2px;color:#456b55;transition:color .15s,border-color .15s}
.tab:hover{color:#1f5c48}
.tab.ativo{color:#1f5c48;font-weight:bold;border-bottom-color:#1f5c48}

/* ── Conteúdo principal ── */
.main{padding:14px;max-width:1600px;margin:0 auto;width:100%;flex:1}

/* ── Layout painel 3 colunas ── */
.painel-grid{display:grid;grid-template-columns:1fr 1fr 1fr;gap:12px;align-items:start}
@media(max-width:1200px){.painel-grid{grid-template-columns:1fr 1fr}}
@media(max-width:700px){.painel-grid{grid-template-columns:1fr}}

/* ── Cards ── */
.card{background:#fff;border-radius:10px;padding:12px;box-shadow:0 1px 4px rgba(0,0,0,.08)}
.card h2{font-size:.88rem;color:#1f5c48;padding-bottom:5px;border-bottom:1px solid #e2ede6;margin-bottom:8px;display:flex;justify-content:space-between;align-items:center}
.card h2 .badge-cnt{font-size:.72rem;color:#83968c;font-weight:normal}

/* ── Scroll interno para cards grandes ── */
.scroll-inner{max-height:420px;overflow-y:auto;padding-right:2px}
.scroll-inner::-webkit-scrollbar{width:4px}
.scroll-inner::-webkit-scrollbar-thumb{background:#c8ddd1;border-radius:2px}

/* ── Nós/Sensores ── */
.no{border:1px solid #e2ede6;border-radius:8px;padding:8px;margin-bottom:8px;background:#fafdfb}
.no:last-child{margin-bottom:0}
.no-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:5px}
.no-id{font-size:.82rem;font-weight:bold;color:#1f5c48}
.sensor-row{display:flex;justify-content:space-between;align-items:center;padding:3px 0;border-bottom:1px dashed #e8f0ea;font-size:.76rem}
.sensor-row:last-child{border-bottom:none}
.s-alias{font-weight:500}
.s-id{font-size:.7rem;color:#83968c}
.s-val{font-weight:bold;color:#183028}
.s-unit{color:#6b7f75;font-size:.72rem;margin-left:2px}

/* ── Atuadores ── */
.atu-no{border:1px solid #e2ede6;border-radius:8px;padding:7px;margin-bottom:7px;background:#fafdfb}
.atu-no:last-child{margin-bottom:0}
.atu-no-title{font-size:.78rem;font-weight:bold;color:#1f5c48;margin-bottom:5px}
.atu-row{display:flex;justify-content:space-between;align-items:center;padding:4px 0;border-bottom:1px dashed #e8f0ea;gap:4px}
.atu-row:last-child{border-bottom:none}
.atu-info{flex:1;min-width:0}
.atu-name{font-size:.77rem;font-weight:600;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.atu-tipo{font-size:.69rem;color:#6b7f75}
.atu-ctrl{display:flex;gap:3px;align-items:center;flex-shrink:0}
.st{font-size:.68rem;padding:1px 6px;border-radius:7px;font-weight:bold;white-space:nowrap}
.st-online{background:#d4f4df;color:#1f5c48}
.st-offline{background:#f2f2f2;color:#999}
.st-on{background:#1f5c48;color:#fff}
.st-desl{background:#fdecea;color:#c0392b}
.btn{border:1px solid #ccc;background:#fff;border-radius:4px;padding:2px 7px;font-size:.72rem;cursor:pointer}
.btn:disabled{opacity:.3;cursor:default}
.btn-on{border-color:#27ae60;color:#27ae60}
.btn-off{border-color:#e74c3c;color:#e74c3c}

/* ── Gráfico ── */
.sel-row{display:flex;gap:6px;align-items:center;flex-wrap:wrap;margin-bottom:7px}
select{padding:4px 7px;border-radius:4px;border:1px solid #ccc;font-size:.8rem;flex:1;min-width:0}
.grafico-label{font-size:.74rem;color:#83968c}
canvas{width:100%!important}

/* ── Velocidade ── */
.vel-mono{font-family:monospace;font-size:.77rem;background:#f4f8f5;padding:7px;border-radius:6px;line-height:1.7}

/* ── Config ── */
.cfg-node{background:#f4f8f5;border-radius:8px;padding:10px;margin-bottom:10px}
.cfg-node:last-child{margin-bottom:0}
.cfg-node-title{font-weight:bold;font-size:.85rem;color:#1f5c48;margin-bottom:8px;display:flex;align-items:center;gap:5px}
.cfg-section{margin-bottom:9px}
.cfg-sec-title{font-size:.71rem;font-weight:bold;text-transform:uppercase;letter-spacing:.05em;color:#456b55;border-bottom:1px solid #d5e8dc;padding-bottom:2px;margin-bottom:6px}
.cfg-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(160px,1fr));gap:6px}
.cfg-field{display:flex;flex-direction:column;gap:2px}
.cfg-field label{font-size:.7rem;color:#456b55}
.cfg-field input{border:1px solid #c8ddd1;border-radius:4px;padding:4px 6px;font-size:.81rem}
.cfg-save{background:#1f5c48;color:#fff;border:none;border-radius:5px;padding:5px 14px;font-size:.79rem;cursor:pointer;margin-top:5px}
.cfg-save:hover{background:#27ae60}

.muted{color:#83968c;font-size:.77rem}
.tab-content{display:none}
.tab-content.ativo{display:block}
</style>
</head>
<body>

<header>
  <h1>🌱 FarmNode — Painel Dinâmico</h1>
  <span id="alerta-badge" onclick="toggleAlertas()">0 alertas</span>
  <span id="ws-badge" class="ws-off">Desconectado</span>
</header>

<!-- Faixa de alertas — fica abaixo do header, sempre acessível -->
<div id="faixa-alertas">
  <h3>🔴 Alertas Ativos <span onclick="toggleAlertas()">fechar ✕</span></h3>
  <div id="alertas-lista"></div>
</div>

<div class="tabs-wrap">
  <div class="tab ativo" id="tab-btn-painel"  onclick="mudarTab('painel')">📊 Painel</div>
  <div class="tab"       id="tab-btn-config"  onclick="mudarTab('config')">⚙️ Configurações</div>
</div>

<div class="main">

  <!-- ── Painel ── -->
  <div id="tab-painel" class="tab-content ativo">
    <div class="painel-grid">

      <!-- Coluna 1: Sensores -->
      <div>
        <div class="card">
          <h2>Nós e Sensores <span class="badge-cnt" id="nos-count"></span></h2>
          <div class="scroll-inner" id="nos"><div class="muted">Aguardando dados...</div></div>
        </div>
      </div>

      <!-- Coluna 2: Atuadores -->
      <div>
        <div class="card">
          <h2>Atuadores <span class="badge-cnt" id="atu-count"></span></h2>
          <div class="scroll-inner" id="atuadores"><div class="muted">Aguardando conexões...</div></div>
        </div>
      </div>

      <!-- Coluna 3: Gráfico + Velocidade -->
      <div>
        <div class="card" style="margin-bottom:12px">
          <h2>Gráfico</h2>
          <div class="sel-row">
            <select id="sensor-select" onchange="trocarSensor()"></select>
          </div>
          <div class="grafico-label" id="grafico-label" style="margin-bottom:6px"></div>
          <canvas id="grafico" height="160"></canvas>
        </div>
        <div class="card">
          <h2>Velocidade UDP</h2>
          <div id="velocidade" class="vel-mono muted">Carregando...</div>
        </div>
      </div>

    </div>
  </div>

  <!-- ── Configurações ── -->
  <div id="tab-config" class="tab-content">
    <p class="muted" style="margin-bottom:12px">Limites por nó. Campos exibidos conforme os tipos de sensor detectados.</p>
    <div id="cfg-nos"><div class="muted">Aguardando nós...</div></div>
  </div>

</div><!-- /main -->

<script>
// ── Estado global ─────────────────────────────────────────────────────────────
let ws = null, wsConectando = false;
let estadoAtual = {};
let todosAlertas = [];
let configAtual  = {};
let sensorChart  = null;
let sensorSel    = {node:'', alias:'', tipo:''};
let alertasFaixaAberta = false;

// Ordem de descoberta (estável — nunca re-ordenada pelo sort)
let nosOrdem = [];   // nodeIDs na ordem que apareceram
let atuOrdem = {};   // nodeID -> [atuador_ids] na ordem que apareceram

// ── Tabs ──────────────────────────────────────────────────────────────────────
function mudarTab(id) {
  ['painel','config'].forEach(t => {
    document.getElementById('tab-' + t).classList.toggle('ativo', t === id);
    document.getElementById('tab-btn-' + t).classList.toggle('ativo', t === id);
  });
  if (id === 'config') renderConfig();
}

// ── WebSocket ─────────────────────────────────────────────────────────────────
function setWS(s, t) {
  const el = document.getElementById('ws-badge');
  el.textContent = t;
  el.className = {on:'ws-on',off:'ws-off',rec:'ws-rec'}[s] || 'ws-off';
}
function wsSend(o) {
  if (ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify(o));
}
function conectarWS() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  ws = new WebSocket(proto + '://' + location.host + '/ws');
  setWS('rec','Conectando...');
  ws.onopen  = () => { wsConectando = false; setWS('on','Conectado'); };
  ws.onmessage = ev => {
    try {
      const m = JSON.parse(ev.data);
      if (m.tipo === 'estado') receberEstado(m.dados || {});
      if (m.tipo === 'alerta') { todosAlertas = Array.isArray(m.dados) ? m.dados : []; renderAlertas(); }
      if (m.tipo === 'comando_resultado') tratarResultadoComando(m.dados || {});
    } catch(e){}
  };
  ws.onclose = () => {
    setWS('off','Desconectado');
    if (!wsConectando) { wsConectando = true; setTimeout(conectarWS, 2500); }
  };
  ws.onerror = () => setWS('off','Erro');
}

// ── Ordem de descoberta ───────────────────────────────────────────────────────
function registrarOrdem(d) {
  Object.keys(d).forEach(nid => {
    if (!nosOrdem.includes(nid)) nosOrdem.push(nid);
    const atus = Array.isArray((d[nid]||{})._atuadores) ? d[nid]._atuadores : [];
    if (!atuOrdem[nid]) atuOrdem[nid] = [];
    atus.forEach(a => {
      if (!atuOrdem[nid].includes(a.atuador_id)) atuOrdem[nid].push(a.atuador_id);
    });
  });
}

// ── Receber estado ────────────────────────────────────────────────────────────
function receberEstado(d) {
  estadoAtual = d || {};
  registrarOrdem(estadoAtual);
  renderNos();
  renderAtuadores();
  atualizarSelectSensores();
  const tabCfg = document.getElementById('tab-config');
  if (tabCfg && tabCfg.classList.contains('ativo')) renderConfig();
  // Atualiza gráfico do sensor selecionado
  const node = estadoAtual[sensorSel.node] || {};
  const s = (Array.isArray(node._sensores) ? node._sensores : []).find(x => x.alias === sensorSel.alias);
  if (s) addPonto(s.valor);
}

// ── Sensores (ordem de descoberta) ───────────────────────────────────────────
function renderNos() {
  const cont = document.getElementById('nos');
  // Filtra os nós que estão na ordem de descoberta E que possuem dados no estado atual
  const nodes = nosOrdem.filter(nid => estadoAtual[nid]);
  
  document.getElementById('nos-count').textContent = nodes.length ? '(' + nodes.length + ' nós)' : '';
  
  if (!nodes.length) { 
    cont.innerHTML = '<div class="muted">Aguardando dados...</div>'; 
    return; 
  }

  cont.innerHTML = nodes.map(nid => {
    const nodeData = estadoAtual[nid] || {};
    const sens = Array.isArray(nodeData._sensores) ? nodeData._sensores : [];
    
    // IMPORTANTE: Ordenar os sensores internamente pelo ID para que não pulem dentro do card
    const sensOrdenados = [...sens].sort((a, b) => a.sensor_id.localeCompare(b.sensor_id));

    const rows = sensOrdenados.length
      ? sensOrdenados.map(s =>
          '<div class="sensor-row">'
          + '<div><span class="s-alias">' + s.alias + '</span> <span class="s-id">(' + s.sensor_id + ')</span></div>'
          + '<div><span class="s-val">' + Number(s.valor||0).toFixed(2) + '</span><span class="s-unit">' + (s.unidade||'') + '</span></div>'
          + '</div>').join('')
      : '<div class="muted" style="font-size:.73rem;padding:3px 0">Sem sensores ativos</div>';

    return '<div class="no"><div class="no-head"><div class="no-id">📍 ' + nid + '</div>'
         + '<span class="muted">' + sens.length + ' sensor(es)</span></div>' + rows + '</div>';
  }).join('');
}

// ── Atuadores (ordem de descoberta, agrupados por nó) ────────────────────────
function renderAtuadores() {
  const cont = document.getElementById('atuadores');
  const nodes = nosOrdem.filter(nid => estadoAtual[nid] && atuOrdem[nid] && atuOrdem[nid].length);
  let total = 0;

  if (!nodes.length) {
    document.getElementById('atu-count').textContent = '';
    cont.innerHTML = '<div class="muted">Nenhum atuador detectado.</div>';
    return;
  }

  cont.innerHTML = nodes.map(nid => {
    const atuMap = {};
    ((estadoAtual[nid]||{})._atuadores || []).forEach(a => atuMap[a.atuador_id] = a);

    const rows = atuOrdem[nid].map(aid => {
      const a = atuMap[aid];
      if (!a) return '';
      total++;
      const online = !!a.conectado;
      const ligado  = !!a.ligado;
      const dis     = online ? '' : 'disabled';
      const stConn  = online
        ? '<span class="st st-online">ON</span>'
        : '<span class="st st-offline">OFF</span>';
      const stEst   = ligado
        ? '<span class="st st-on">LIGADO</span>'
        : '<span class="st st-desl">DESL.</span>';
      return '<div class="atu-row">'
        + '<div class="atu-info"><div class="atu-name">' + a.atuador_id + '</div>'
        + '<div class="atu-tipo">' + (a.tipo||'') + '</div></div>'
        + '<div class="atu-ctrl">' + stConn + stEst
        + '<button class="btn btn-on" ' + dis + ' onclick="cmd(\'' + nid + '\',\'' + a.atuador_id + '\',\'LIGAR\')">▲</button>'
        + '<button class="btn btn-off" ' + dis + ' onclick="cmd(\'' + nid + '\',\'' + a.atuador_id + '\',\'DESLIGAR\')">▼</button>'
        + '</div></div>';
    }).join('');

    return '<div class="atu-no"><div class="atu-no-title">📍 ' + nid + '</div>' + rows + '</div>';
  }).join('');

  document.getElementById('atu-count').textContent = total ? '(' + total + ')' : '';
}

function cmd(nodeID, atuadorID, comando) {
  wsSend({tipo:'comando', node_id:nodeID, atuador_id:atuadorID, comando});
}

function tratarResultadoComando(dados) {
  if (!dados || dados.ok !== false) return;
  const alvo = (dados.node_id || '?') + '/' + (dados.atuador_id || '?');
  const cmd  = dados.comando || 'COMANDO';
  const erro = dados.erro || 'falha_desconhecida';
  alert('Falha ao enviar ' + cmd + ' para ' + alvo + ': ' + erro);
}

// ── Gráfico ───────────────────────────────────────────────────────────────────
function atualizarSelectSensores() {
  const sel = document.getElementById('sensor-select');
  const current = sensorSel.node + '|' + sensorSel.alias;
  const opts = [];
  nosOrdem.filter(nid => estadoAtual[nid]).forEach(nid => {
    const sens = Array.isArray((estadoAtual[nid]||{})._sensores) ? estadoAtual[nid]._sensores : [];
    sens.forEach(s => opts.push({value:nid+'|'+s.alias+'|'+s.tipo, label:nid+' / '+s.alias, node:nid, alias:s.alias}));
  });
  const prevLen = sel.options.length;
  sel.innerHTML = opts.map(o => '<option value="'+o.value+'">'+o.label+'</option>').join('');
  if (!opts.length) return;
  const idx = opts.findIndex(o => (o.node+'|'+o.alias) === current);
  sel.selectedIndex = idx >= 0 ? idx : 0;
  // Só troca o gráfico se o sensor mudou (preserva histórico)
  if (idx < 0 || prevLen === 0) trocarSensor();
}

function trocarSensor() {
  const sel = document.getElementById('sensor-select');
  if (!sel || !sel.value) return;
  const p = sel.value.split('|');
  const novoSel = {node:p[0], alias:p[1], tipo:p[2]};
  if (novoSel.node === sensorSel.node && novoSel.alias === sensorSel.alias) return;
  sensorSel = novoSel;
  document.getElementById('grafico-label').textContent = sensorSel.node + ' › ' + sensorSel.alias;
  criarGrafico();
}

function criarGrafico() {
  if (sensorChart) sensorChart.destroy();
  const ctx = document.getElementById('grafico').getContext('2d');
  sensorChart = new Chart(ctx, {
    type:'line',
    data:{labels:[], datasets:[{
      data:[], borderColor:'#2d8f6a', backgroundColor:'rgba(45,143,106,.12)',
      fill:true, tension:0.3, pointRadius:1.2, borderWidth:1.8
    }]},
    options:{
      animation:false,
      plugins:{legend:{display:false}},
      scales:{
        x:{ticks:{maxTicksLimit:6,maxRotation:0,font:{size:10}}},
        y:{ticks:{font:{size:10}}}
      }
    }
  });
}

function addPonto(v) {
  if (!sensorChart) return;
  sensorChart.data.labels.push(new Date().toLocaleTimeString('pt-BR'));
  sensorChart.data.datasets[0].data.push(v);
  if (sensorChart.data.labels.length > 90) {
    sensorChart.data.labels.shift();
    sensorChart.data.datasets[0].data.shift();
  }
  sensorChart.update('none');
}

// ── Alertas ───────────────────────────────────────────────────────────────────
function toggleAlertas() {
  alertasFaixaAberta = !alertasFaixaAberta;
  document.getElementById('faixa-alertas').classList.toggle('visivel', alertasFaixaAberta);
}

function renderAlertas() {
  const ativos = (todosAlertas||[]).filter(a => !a.ack).slice(-15).reverse();
  const badge  = document.getElementById('alerta-badge');
  const lista  = document.getElementById('alertas-lista');

  // Badge no header
  if (ativos.length) {
    badge.textContent = ativos.length + ' alerta' + (ativos.length > 1 ? 's' : '');
    badge.style.display = 'inline';
    // Abre automaticamente se há alertas críticos
    const temCrit = ativos.some(a => a.nivel === 'critico');
    if (temCrit && !alertasFaixaAberta) {
      alertasFaixaAberta = true;
      document.getElementById('faixa-alertas').classList.add('visivel');
    }
  } else {
    badge.style.display = 'none';
    alertasFaixaAberta = false;
    document.getElementById('faixa-alertas').classList.remove('visivel');
  }

  lista.innerHTML = ativos.map(a =>
    '<div class="alerta-item' + (a.nivel === 'critico' ? ' crit' : '') + '">'
    + '<b>' + a.nivel.toUpperCase() + ':</b> ' + a.mensagem
    + '<div class="alerta-meta">' + a.node_id + ' / ' + a.tipo
    + ' — ' + new Date(a.timestamp).toLocaleString('pt-BR') + '</div>'
    + '</div>'
  ).join('');
}

async function sincronizarEstado() {
  try {
    const d = await fetch('/api/estado', {cache:'no-store'}).then(r => r.json());
    receberEstado(d || {});
  } catch(e){}
}

// ── Velocidade ────────────────────────────────────────────────────────────────
async function atualizarVelocidade() {
  try {
    const v = await fetch('/api/velocidade').then(r => r.json());
    document.getElementById('velocidade').innerHTML =
      'Total datagramas: <b>' + (v.total_datagramas||0) + '</b><br>'
      + 'JSON inválido: <b>' + (v.total_invalidos_json||0) + '</b><br>'
      + 'Último pacote: ' + (v.ultimo_datagrama||'—') + '<br>'
      + 'Últ. inválido: ' + (v.ultimo_invalido_json||'—') + '<br>'
      + 'Trace terminal: ' + (v.trace_terminal ? '🟢 ON' : '⚫ OFF');
  } catch(e){}
}

// ── Configurações ─────────────────────────────────────────────────────────────
// Campos por tipo de sensor — cada tipo tem sua própria seção
const CFG = {
  umidade:     [{key:'umidade_min',     label:'Umidade mín (%)'},
                {key:'umidade_max',     label:'Umidade máx (%)'},
                {key:'critico_umidade', label:'Crítico mín (%)'}],
  luminosidade:[{key:'luz_min',         label:'Luz mín (Lux)'},
                {key:'critico_luz',     label:'Crítico luz (Lux)'}],
  amonia:      [{key:'amonia_max',      label:'Amônia máx (ppm)'},
                {key:'critico_amonia',  label:'Crítico (ppm)'}],
  racao:       [{key:'racao_min',       label:'Ração mín (%)'},
                {key:'racao_max',       label:'Ração máx (%)'},
                {key:'critico_racao',   label:'Crítico (%)'}],
  agua:        [{key:'agua_min',        label:'Água mín (%)'},
                {key:'agua_max',        label:'Água máx (%)'},
                {key:'critico_agua',    label:'Crítico (%)'}],
};

// Temperatura: campos dependem dos atuadores presentes no nó
function camposTemp(nodeID) {
  const atus  = ((estadoAtual[nodeID]||{})._atuadores)||[];
  const tipos = atus.map(a => (a.tipo||'').toLowerCase());
  const temV  = tipos.some(t => t.includes('ventilador'));
  const temA  = tipos.some(t => t.includes('aquecedor'));
  const campos = [];
  if (temV || !temA) campos.push({key:'temp_max', label:'Temp máx (°C)'});
  if (temA || !temV) campos.push({key:'temp_min', label:'Temp mín (°C)'});
  if (campos.length) campos.push({key:'critico_temp', label:'Crítico (°C)'});
  return campos;
}

function tiposDoNo(nodeID) {
  const sens = ((estadoAtual[nodeID]||{})._sensores)||[];
  const visto = new Set();
  return sens.map(s => s.tipo).filter(t => { if(visto.has(t)) return false; visto.add(t); return true; });
}

async function carregarConfig() {
  try { configAtual = await fetch('/api/config').then(r => r.json()) || {}; } catch(e){}
}

function renderConfig() {
  const cont  = document.getElementById('cfg-nos');
  const nodes = nosOrdem.filter(nid => estadoAtual[nid]);
  if (!nodes.length) { cont.innerHTML = '<div class="muted">Nenhum nó descoberto ainda.</div>'; return; }

  cont.innerHTML = nodes.map(nodeID => {
    const cfg   = configAtual[nodeID] || {};
    const tipos = tiposDoNo(nodeID);
    if (!tipos.length) {
      return '<div class="cfg-node"><div class="cfg-node-title">📍 ' + nodeID + '</div>'
        + '<div class="muted">Nenhum sensor ativo neste nó ainda.</div></div>';
    }

    const secoes = tipos.map(tipo => {
      const campos = tipo === 'temperatura' ? camposTemp(nodeID) : (CFG[tipo]||[]);
      if (!campos.length) return '';
      const fields = '<div class="cfg-grid">'
        + campos.map(c => {
            const val = cfg[c.key] !== undefined ? cfg[c.key] : '';
            return '<div class="cfg-field"><label>' + c.label + '</label>'
              + '<input type="number" step="0.1" id="cfg_'+nodeID+'_'+c.key+'" value="'+val+'"></div>';
          }).join('')
        + '</div>';
      return '<div class="cfg-section"><div class="cfg-sec-title">📌 ' + tipo + '</div>' + fields + '</div>';
    }).join('');

    return '<div class="cfg-node">'
      + '<div class="cfg-node-title">📍 ' + nodeID + '</div>'
      + secoes
      + '<button class="cfg-save" onclick="salvarConfig(\''+nodeID+'\')">💾 Salvar ' + nodeID + '</button>'
      + '</div>';
  }).join('');
}

function salvarConfig(nodeID) {
  const tipos = tiposDoNo(nodeID);
  const dados = {};
  tipos.forEach(tipo => {
    const campos = tipo === 'temperatura' ? camposTemp(nodeID) : (CFG[tipo]||[]);
    campos.forEach(c => {
      const el = document.getElementById('cfg_'+nodeID+'_'+c.key);
      if (el && el.value !== '') dados[c.key] = parseFloat(el.value);
    });
  });
  if (!Object.keys(dados).length) return;
  wsSend({tipo:'config', node_id:nodeID, dados});
  configAtual[nodeID] = Object.assign(configAtual[nodeID]||{}, dados);
  const btn = document.querySelector('[onclick="salvarConfig(\''+nodeID+'\')"]');
  if (btn) { const o = btn.textContent; btn.textContent = '✓ Salvo!'; setTimeout(() => btn.textContent = o, 1800); }
}

// ── Init ──────────────────────────────────────────────────────────────────────
async function init() {
  await sincronizarEstado();
  try {
    todosAlertas = await fetch('/api/alertas?ativos=true').then(r => r.json());
    if (!Array.isArray(todosAlertas)) todosAlertas = [];
    renderAlertas();
  } catch(e){}
  await carregarConfig();
  await atualizarVelocidade();
}

criarGrafico();
init();
setInterval(atualizarVelocidade, 2000);
setInterval(sincronizarEstado, 1500);
setInterval(carregarConfig, 15000);
conectarWS();
</script>
</body>
</html>`
}
