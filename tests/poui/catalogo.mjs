// Teste de navegador do catálogo PO-UI (tests/poui/catalogo.prw): cada
// classe Po* tem que virar o elemento PO-UI certo, com conteúdo, sem erro
// no console. Uso: node catalogo.mjs <porta-cdp> <url>
const [porta, url] = process.argv.slice(2);
const espera = ms => new Promise(r => setTimeout(r, ms));

// Página principal: classe -> seletor do PO-UI. "oculto" = existe no DOM
// mas fica fechado até uma ação (modal, popup, popover).
const principal = {
  PoPage: 'po-page-default', PoMenu: 'po-menu', PoNavbar: 'po-navbar',
  PoHeader: 'po-header', PoMenuPanel: 'po-menu-panel',
  PoButton: 'po-button', PoTag: 'po-tag', PoInput: 'po-input',
  PoContainer: 'po-container', PoWidget: 'po-widget', PoDivider: 'po-divider',
  PoTabs: 'po-tabs', PoTab: 'po-tab', PoAccordion: 'po-accordion',
  // o PO-UI renderiza o item como <li class="po-accordion-item"> dentro do accordion
  PoAccordionItem: '.po-accordion-item', PoStepper: 'po-stepper', PoStep: 'po-step',
  PoModal: { sel: 'po-modal', oculto: true }, PoContextTabs: 'po-context-tabs',
  PoInfo: 'po-info', PoProgress: 'po-progress', PoGauge: 'po-gauge',
  // o po-gauge tem um po-chart interno: o seletor pega só o do PoChart
  PoChart: 'po-chart:not(po-gauge po-chart)', PoTable: 'po-table', PoTree: 'po-tree-view',
  PoList: 'po-list-view', PoPassword: 'po-password', PoTextarea: 'po-textarea',
  PoNumber: 'po-number', PoSelect: 'po-select', PoCheckbox: 'po-checkbox',
  PoSwitch: 'po-switch', PoCombo: 'po-combo', PoMultiselect: 'po-multiselect',
  PoRadioGroup: 'po-radio-group', PoCheckboxGroup: 'po-checkbox-group',
  PoLookup: 'po-lookup', PoDatepicker: 'po-datepicker',
  PoDatepickerRange: 'po-datepicker-range', PoDatetimepicker: 'po-datetimepicker',
  PoTimepicker: 'po-timepicker', PoDecimal: 'po-decimal', PoEmail: 'po-email',
  PoUrl: 'po-url', PoRichText: 'po-rich-text', PoSearch: 'po-search',
  PoUpload: 'po-upload', PoButtonGroup: 'po-button-group', PoDropdown: 'po-dropdown',
  PoPopup: { sel: 'po-popup', oculto: true }, PoPopover: { sel: 'po-popover', oculto: true },
  PoContextMenu: 'po-context-menu', PoToolbar: 'po-toolbar',
  PoBreadcrumb: 'po-breadcrumb', PoLink: 'po-link', PoSlide: 'po-slide',
  PoAvatar: 'po-avatar', PoBadge: 'po-badge', PoImage: 'po-image',
  PoSkeleton: 'po-skeleton', PoHelper: 'po-helper', PoCalendar: 'po-calendar',
  PoDynamicView: 'po-dynamic-view', PoDisclaimerGroup: 'po-disclaimer-group',
  PoFilterChip: 'po-filter-chip',
  PoDynamicForm: 'po-dynamic-form', PoLogin: 'po-login',
};
// Páginas especiais, na ordem dos botões "ir:N" do catálogo.
const paginas = [
  ['PoPageList', 'po-page-list'], ['PoPageEdit', 'po-page-edit'],
  ['PoPageDetail', 'po-page-detail'], ['PoPageDynamicTable', 'po-page-dynamic-table'],
  ['PoPageDynamicEdit', 'po-page-dynamic-edit'], ['PoPageDynamicDetail', 'po-page-dynamic-detail'],
  ['PoPageDynamicSearch', 'po-page-dynamic-search'], ['PoPageLogin', 'po-page-login'],
  ['PoPageChangePassword', 'po-page-change-password'], ['PoPageBlockedUser', 'po-page-blocked-user'],
  ['PoPageSlide', { sel: 'po-page-slide', oculto: true }],
  ['PoPageJobScheduler', 'po-page-job-scheduler'],
];

const alvo = await (await fetch(`http://127.0.0.1:${porta}/json/new?about:blank`, { method: 'PUT' })).json();
const ws = new WebSocket(alvo.webSocketDebuggerUrl);
await new Promise(r => { ws.onopen = r; });
let seq = 0;
const pend = new Map();
const erros = [];
ws.onmessage = m => {
  const d = JSON.parse(m.data);
  if (d.id && pend.has(d.id)) { pend.get(d.id)(d.result); pend.delete(d.id); return; }
  if (d.method === 'Runtime.exceptionThrown') {
    erros.push('exceção: ' + (d.params.exceptionDetails.exception?.description || d.params.exceptionDetails.text).split('\n')[0]);
  }
  if (d.method === 'Runtime.consoleAPICalled' && d.params.type === 'error') {
    erros.push('console.error: ' + d.params.args.map(a => a.value ?? a.description ?? '').join(' ').split('\n')[0]);
  }
};
const cmd = (method, params = {}) => new Promise(r => { const id = ++seq; pend.set(id, r); ws.send(JSON.stringify({ id, method, params })); });
const js = async e => (await cmd('Runtime.evaluate', { expression: e, returnByValue: true })).result.value;
await cmd('Runtime.enable');

// Estado de um seletor: existe? o componente PO-UI renderizou conteúdo?
// (Elementos customizados com filhos em bloco medem 0x0, então o critério
// é o HTML interno: um componente sem dados fica só com "<!---->".)
const estado = sel => js(`(()=>{const e=document.querySelector('${sel}');if(!e)return 'ausente';
  const h=e.innerHTML.replace(/<!--[^]*?-->/g,'').trim();
  return (e.childElementCount>0&&h.length>20)?'ok':'vazio'})()`);
const confere = async (mapa, falhas) => {
  for (const [classe, v] of Object.entries(mapa)) {
    const { sel, oculto } = typeof v === 'string' ? { sel: v, oculto: false } : v;
    const st = await estado(sel);
    if (st === 'ausente' || (!oculto && st !== 'ok')) falhas.push(`${classe} (${sel}): ${st}`);
  }
};
// Clique pelo próprio botão (o PO-UI escuta o evento click do <button>):
// coordenadas de mouse falham numa página longa com cabeçalho fixo.
const clica = texto => js(`(()=>{const e=[...document.querySelectorAll('po-button button')].find(x=>x.innerText.trim()==='${texto}');if(!e)return false;e.click();return true})()`);
const carrega = async () => {
  await cmd('Page.navigate', { url });
  for (let i = 0; i < 30; i++) {
    await espera(1000);
    if (await js("!!document.querySelector('po-page-default po-button')")) return true;
  }
  return false;
};

const falhas = [];
if (!await carrega()) { console.log('FALHOU: catálogo não montou'); process.exit(1); }
await espera(2000);
await confere(principal, falhas);

// Interação ao vivo: clique e digitação voltam ao AdvPL e atualizam a tela.
await clica('Clicar');
await espera(1500);
if (!(await js("[...document.querySelectorAll('po-tag')].some(t=>t.innerText.includes('clicado'))"))) falhas.push('evento click não voltou do AdvPL');
await js(`(()=>{const i=document.querySelector('po-input input');i.focus();i.value='abc';i.dispatchEvent(new Event('input',{bubbles:true}));i.dispatchEvent(new Event('change',{bubbles:true}));i.blur()})()`);
await espera(1500);
if (!(await js("[...document.querySelectorAll('po-tag')].some(t=>t.innerText.includes('eco:abc'))"))) falhas.push('evento change do PoInput não voltou do AdvPL');

// PoDynamicForm: digitar num campo chama OnChange(oForm, cCampo, xValor).
// Digitação real + Tab: o po-dynamic-form só valida com o form "dirty".
await js("(()=>{const i=document.querySelector('po-dynamic-form input');i.focus();i.select()})()");
await cmd('Input.insertText', { text: 'Zeca' });
for (const type of ['keyDown', 'keyUp']) await cmd('Input.dispatchKeyEvent', { type, key: 'Tab', code: 'Tab', windowsVirtualKeyCode: 9 });
await espera(2000);
if (!(await js("[...document.querySelectorAll('po-tag')].some(t=>t.innerText.includes('form:nome=Zeca'))"))) falhas.push('evento do PoDynamicForm não voltou do AdvPL');

// PoModalPasswordRecovery: só existe quando aberto (Open() pelo botão).
await clica('recuperar');
await espera(2000);
await confere({ PoModalPasswordRecovery: 'po-modal-password-recovery' }, falhas);

// Cada página especial: recarrega (sessão nova), navega e confere.
for (let n = 0; n < paginas.length; n++) {
  const [classe, v] = paginas[n];
  if (!await carrega()) { falhas.push(`${classe}: catálogo não remontou`); continue; }
  if (!await clica(`ir:${n + 1}`)) { falhas.push(`${classe}: botão ir:${n + 1} não achado`); continue; }
  await espera(2500);
  await confere({ [classe]: v }, falhas);
}
// Dados reais do REST na tabela dinâmica (onda 1 -> 3a.6).
if (await carrega() && await clica('ir:4')) {
  let txt = '';
  for (let i = 0; i < 15 && !txt.includes('Armazem'); i++) { await espera(1000); txt = await js("(document.querySelector('po-page-dynamic-table')||{}).innerText||''"); }
  if (!txt.includes('Armazem')) falhas.push('PoPageDynamicTable: sem dados do REST /rest/api/est/v1/armazens');
}

const total = Object.keys(principal).length + paginas.length + 1; // + PoModalPasswordRecovery
for (const f of falhas) console.log('FALHA: ' + f);
for (const e of [...new Set(erros)]) console.log('ERRO-CONSOLE: ' + e);
if (falhas.length || erros.length) {
  console.log(`FALHOU catalogo: ${falhas.length} falhas, ${new Set(erros).size} erros de console, ${total} classes verificadas`);
  process.exit(1);
}
console.log(`AO-VIVO-OK catalogo: ${total} classes renderizadas + click/change + dados REST, console limpo`);
process.exit(0);
