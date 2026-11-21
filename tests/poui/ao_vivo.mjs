// Teste de navegador: uma atualização vinda do servidor (timer) precisa
// aparecer na tela sem nenhuma interação. Abre a página via Chrome
// DevTools Protocol, lê o relógio do exemplo fase1.prw duas vezes com 3 s
// de intervalo e falha se ele não mudou.
// Uso: node ao_vivo.mjs <porta-devtools> <url>   (Node 22+: WebSocket nativo)
const [porta, url] = process.argv.slice(2);
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
ws.close();
if (!antes || antes === depois) {
  console.log(`FALHOU: relógio não atualizou na tela (${antes} -> ${depois})`);
  process.exit(1);
}
console.log(`AO-VIVO-OK (${antes} -> ${depois})`);
process.exit(0);
