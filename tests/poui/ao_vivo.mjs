// Teste de navegador: uma atualização vinda do servidor (timer) precisa
// aparecer na tela sem nenhuma interação. Abre a página via Chrome
// DevTools Protocol, lê o relógio do exemplo fase1.prw duas vezes com 3 s
// de intervalo e falha se ele não mudou. Depois digita um nome e clica em
// "Adicionar" na hora (antes dos 300 ms do envio da digitação): o nome
// precisa chegar ao programa e aparecer na tabela.
// Uso: node ao_vivo.mjs <porta-devtools> <url> [fixture-app]
// Com "fixture-app" (plano onda 2, Task 2.7): abre a SPA, espera o
// FwCallApp abrir a aba /poui/fixture-app, confere a tabela de armazéns,
// clica "Pingar AdvPL" (espera "pong" via AdvPLToJS) e "Fechar".
const [porta, url, cenario] = process.argv.slice(2);

if (cenario === 'fixture-app') {
  const espera = ms => new Promise(r => setTimeout(r, ms));
  const lista = async () => (await (await fetch(`http://127.0.0.1:${porta}/json/list`)).json())
    .filter(t => t.type === 'page');
  const conecta = alvo => new Promise((res, rej) => {
    const ws2 = new WebSocket(alvo.webSocketDebuggerUrl);
    let s = 0; const pend2 = new Map();
    const cmd2 = (m, p = {}) => new Promise(r => { const id = ++s; pend2.set(id, r); ws2.send(JSON.stringify({ id, method: m, params: p })); });
    ws2.onmessage = m => { const d = JSON.parse(m.data); if (d.id && pend2.has(d.id)) { pend2.get(d.id)(d.result); pend2.delete(d.id); } };
    ws2.onopen = () => res({ ws: ws2, cmd: cmd2 });
    ws2.onerror = rej;
  });
  const js2 = (c, e) => c.cmd('Runtime.evaluate', { expression: e, returnByValue: true }).then(r => r.result.value);
  const clica = async (c, texto) => {
    const b = await js2(c, `(()=>{const e=[...document.querySelectorAll('po-button button')].find(x=>x.innerText.trim()==='${texto}');if(!e)return null;const r=e.getBoundingClientRect();return{x:r.x+r.width/2,y:r.y+r.height/2}})()`);
    if (!b) return false;
    for (const type of ['mousePressed', 'mouseReleased']) await c.cmd('Input.dispatchMouseEvent', { type, x: b.x, y: b.y, button: 'left', clickCount: 1 });
    return true;
  };

  // SPA: a sessão web roda FixtureApp -> FwCallApp -> window.open('/poui/...').
  await (await fetch(`http://127.0.0.1:${porta}/json/new?${encodeURIComponent(url)}`, { method: 'PUT' })).json();
  let appAlvo = null;
  for (let i = 0; i < 60 && !appAlvo; i++) {
    await espera(1000);
    appAlvo = (await lista()).find(t => t.url.includes('/poui/fixture-app'));
  }
  if (!appAlvo) { console.log('FALHOU: aba /poui/fixture-app não abriu (FwCallApp)'); process.exit(1); }
  const app = await conecta(appAlvo);

  // Tabela de armazéns via /rest/api/est/v1/armazens (p-service-api).
  let tabela = '';
  for (let i = 0; i < 30; i++) {
    await espera(1000);
    tabela = await js2(app, "(document.querySelector('po-table')||{}).innerText||''");
    if (tabela.includes('Armazem')) break;
  }
  if (!tabela.includes('Armazem')) { console.log('FALHOU: tabela sem armazéns: ' + tabela.slice(0, 200)); process.exit(1); }

  // "Pingar AdvPL" -> JsToAdvpl -> AdvPLToJS -> #pong = "pong".
  if (!await clica(app, 'Pingar AdvPL')) { console.log('FALHOU: botão Pingar AdvPL não achado'); process.exit(1); }
  let pong = '';
  for (let i = 0; i < 15; i++) {
    await espera(1000);
    pong = await js2(app, "(document.querySelector('#pong')||{}).innerText||''");
    if (pong.trim() === 'pong') break;
  }
  if (pong.trim() !== 'pong') { console.log('FALHOU: pong não voltou do AdvPL: "' + pong + '"'); process.exit(1); }

  // "Fechar" -> callAppClose -> jsToAdvpl close -> FwCallApp retorna.
  if (!await clica(app, 'Fechar')) { console.log('FALHOU: botão Fechar não achado'); process.exit(1); }
  let fechou = false;
  for (let i = 0; i < 15; i++) {
    await espera(1000);
    const ainda = (await lista()).some(t => t.url.includes('/poui/fixture-app'));
    if (!ainda) { fechou = true; break; }
  }
  app.ws.close();
  if (!fechou) { console.log('FALHOU: aba do app não fechou após Fechar'); process.exit(1); }
  console.log('AO-VIVO-OK fixture-app (tabela + pong + fechar)');
  process.exit(0);
}

if (cenario === 'servicos-3c') {
  // Onda 3c: storage round-trip (botão "Storage" lê a chave gravada no
  // mount), tema escuro (classe dark no <html>) e guia (driver.js abre e
  // fecha -> OnGuideEnd). Mídia/rede disparam no mount (ConOut no log).
  const espera = ms => new Promise(r => setTimeout(r, ms));
  const lista = async () => (await (await fetch(`http://127.0.0.1:${porta}/json/list`)).json())
    .filter(t => t.type === 'page');
  const alvo = await (await fetch(`http://127.0.0.1:${porta}/json/new?${encodeURIComponent(url)}`, { method: 'PUT' })).json();
  const ws = new WebSocket(alvo.webSocketDebuggerUrl);
  let s = 0; const pend = new Map();
  const cmd = (m, p = {}) => new Promise(r => { const id = ++s; pend.set(id, r); ws.send(JSON.stringify({ id, method: m, params: p })); });
  ws.onmessage = m => { const d = JSON.parse(m.data); if (d.id && pend.has(d.id)) { pend.get(d.id)(d.result); pend.delete(d.id); } };
  await new Promise(r => { ws.onopen = r; });
  const js = async e => (await cmd('Runtime.evaluate', { expression: e, returnByValue: true, awaitPromise: true })).result.value;
  const clica = async texto => {
    const b = await js(`(()=>{const e=[...document.querySelectorAll('po-button button')].find(x=>x.innerText.trim()==='${texto}');if(!e)return null;const r=e.getBoundingClientRect();return{x:r.x+r.width/2,y:r.y+r.height/2}})()`);
    if (!b) return false;
    for (const type of ['mousePressed', 'mouseReleased']) await cmd('Input.dispatchMouseEvent', { type, x: b.x, y: b.y, button: 'left', clickCount: 1 });
    return true;
  };
  const log = () => js("(document.querySelector('.advpp-console')||document.body).innerText||''");

  await espera(3000);
  // abre o "Log técnico" (recolhido por padrão) para ler os ConOut
  await js("document.querySelector('.advpp-console-toggle')?.click()");
  await espera(500);
  // mount já disparou mediachange/networkchange -> ConOut no log técnico
  let txt = await log();
  if (!txt.includes('[3c] media=')) { console.log('FALHOU: mediachange não chegou: ' + txt.slice(-300)); process.exit(1); }

  // Storage: grava no mount (StorageSet), lê no clique (StorageGet sync)
  if (!await clica('Storage')) { console.log('FALHOU: botão Storage não achado'); process.exit(1); }
  let ok = false;
  for (let i = 0; i < 15; i++) { await espera(1000); txt = await log(); if (txt.includes('[3c] storage=')) { ok = true; break; } }
  if (!ok) { console.log('FALHOU: StorageGet não voltou: ' + txt.slice(-300)); process.exit(1); }

  // Tema: clique troca para dark -> classe no <html>
  if (!await clica('Tema escuro')) { console.log('FALHOU: botão Tema não achado'); process.exit(1); }
  await espera(1500);
  const html = await js('document.documentElement.className');
  if (!/dark/i.test(html)) { console.log('FALHOU: tema dark não aplicado: ' + html); process.exit(1); }

  // Guia: abre o driver.js e fecha -> OnGuideEnd loga
  if (!await clica('Guia')) { console.log('FALHOU: botão Guia não achado'); process.exit(1); }
  let guia = false;
  for (let i = 0; i < 15; i++) {
    await espera(1000);
    const pop = await js("!!document.querySelector('.driver-popover')");
    if (pop) {
      await js("document.querySelector('.driver-popover-close-btn, .driver-popover-footer .driver-popover-close-btn')?.click(); document.querySelector('.driver-popover')?.dispatchEvent(new Event('close'))");
      // fecha pelo botão de fechar do driver
      await js("[...document.querySelectorAll('.driver-popover button, .driver-popover a')].find(b=>/close|fechar/i.test(b.innerText||b.getAttribute('aria-label')||''))?.click()");
      break;
    }
  }
  for (let i = 0; i < 15; i++) { await espera(1000); txt = await log(); if (txt.includes('[3c] guia fim=')) { guia = true; break; } }
  if (!guia) { console.log('FALHOU: OnGuideEnd não disparou'); process.exit(1); }

  ws.close();
  console.log('AO-VIVO-OK servicos-3c (midia + storage + tema + guia)');
  process.exit(0);
}

const alvo = await (await fetch(`http://127.0.0.1:${porta}/json/new?${encodeURIComponent(url)}`, { method: 'PUT' })).json();
const ws = new WebSocket(alvo.webSocketDebuggerUrl);
let seq = 0;
const pend = new Map();
const cmd = (method, params = {}) => new Promise(r => { const id = ++seq; pend.set(id, r); ws.send(JSON.stringify({ id, method, params })); });
ws.onmessage = m => { const d = JSON.parse(m.data); if (d.id && pend.has(d.id)) { pend.get(d.id)(d.result); pend.delete(d.id); } };
await new Promise(r => { ws.onopen = r; });
const espera = ms => new Promise(r => setTimeout(r, ms));
const relogio = async () => (await cmd('Runtime.evaluate', { expression: "(document.querySelector('po-tag')||{}).innerText", returnByValue: true })).result.value;
await espera(2500);
const antes = await relogio();
await espera(3000);
const depois = await relogio();
const js = async e => (await cmd('Runtime.evaluate', { expression: e, returnByValue: true })).result.value;
// Botão com Set("kind", "primary") e Set("icon", ...) (fase1.prw)
const botao = await js("(()=>{const e=[...document.querySelectorAll('po-button')].find(x=>x.innerText.trim()==='Adicionar');const b=e&&e.querySelector('button');return b?{kind:b.getAttribute('p-kind')||b.className,icone:!!e.querySelector('.an-plus')}:null})()");
await js("document.querySelector('po-input input').focus()");
await cmd('Input.insertText', { text: 'Digitado na pressa' });
const b = await js("(()=>{const e=[...document.querySelectorAll('po-button button')].find(x=>x.innerText.trim()==='Adicionar');const r=e.getBoundingClientRect();return{x:r.x+r.width/2,y:r.y+r.height/2}})()");
for (const type of ['mousePressed', 'mouseReleased']) await cmd('Input.dispatchMouseEvent', { type, x: b.x, y: b.y, button: 'left', clickCount: 1 });
await espera(1500);
const tabela = await js("(document.querySelector('po-table')||{}).innerText||''");
ws.close();
if (!antes || antes === depois) {
  console.log(`FALHOU: relógio não atualizou na tela (${antes} -> ${depois})`);
  process.exit(1);
}
if (!botao || !/primary/.test(botao.kind) || !botao.icone) {
  console.log('FALHOU: botão sem destaque/ícone: ' + JSON.stringify(botao));
  process.exit(1);
}
if (!tabela.includes('Digitado na pressa')) {
  console.log('FALHOU: texto digitado e clicado em seguida não chegou ao programa');
  process.exit(1);
}
console.log(`AO-VIVO-OK (${antes} -> ${depois})`);
process.exit(0);
