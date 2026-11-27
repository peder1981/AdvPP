import { Component, ViewChild, computed, inject, signal } from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';
import { take } from 'rxjs';
import {
  PoButtonModule, PoDialogService, PoDynamicFormField, PoDynamicModule, PoLoadingModule, PoMenuItem,
  PoMenuModule, PoNotificationService, PoToasterModule,
  PoModalAction, PoModalComponent, PoModalModule, PoPageModule,
  PoTableAction, PoTableColumn, PoTableModule, PoTagModule,
  PoLanguageService, PoThemeService, PoThemeTypeEnum, PoUserGuideService, PoUserGuideStep,
  poThemeDefault,
} from '@po-ui/ng-components';
import { UiHostComponent, UiNode, descarregaPendentes } from './ui-host';
import { entregaTerm } from './adv-terminal-canal';
import { defineDynPoster, dynAntesResultado } from './dyn-hooks';

// Heurística de ícone pra item de menu (FWMenuSelect não carrega nenhuma
// informação de ícone — só o texto do item) — genérico o bastante pra
// qualquer app AdvPP, não hardcoded pro GesCon: casa por palavra-chave em
// português comum, cai num ícone neutro se nada bater.
const MENU_ICON_RULES: [RegExp, string][] = [
  // Checado primeiro: itens de relatório mencionam palavras (unidade,
  // despesa) que também disparariam as regras de tela CRUD abaixo —
  // "Extrato por Unidade" precisa do ícone de gráfico, não de prédio.
  [/relat[oó]rio|balancete|inadimpl[eê]ncia|extrato|por categoria/i, 'an-chart-bar'],
  [/unidade|apartamento|im[oó]vel/i, 'an-buildings'],
  [/cond[oô]m[ií]?n|cliente|usu[aá]rio|pessoa/i, 'an-users'],
  [/despesa|financeiro|custo/i, 'an-wallet'],
  [/cobran[çc]a|fatura|boleto|conta/i, 'an-receipt'],
  [/fechamento|compet[eê]ncia|m[eê]s/i, 'an-calendar-check'],
  [/mala|e-?mail|correio|mensagem/i, 'an-envelope-simple'],
  [/senha/i, 'an-lock-key'],
  [/voltar/i, 'an-arrow-left'],
  [/sair|encerrar|fechar/i, 'an-sign-out'],
];
// Prefixo explicito de icone: o AdvPL pode mandar "icon:<nome>|Texto"
// (ex.: "icon:wallet|Financeiro") para forcar um icone PO/Animal em vez
// da heuristica por palavra-chave abaixo. Sem prefixo, vale a heuristica
// de sempre (compat: GesCon e demais apps mandam texto puro).
function menuItemIcon(label: string): string {
  const m = /^icon:([a-z0-9-]+)\|/i.exec(label);
  if (m) { return 'an-' + m[1].toLowerCase(); }
  for (const [re, icon] of MENU_ICON_RULES) {
    if (re.test(label)) { return icon; }
  }
  return 'an-caret-right';
}
function menuItemLabel(label: string): string {
  return label.replace(/^icon:[a-z0-9-]+\|/i, '');
}

// Espelho de browseSpec/browseAction do servidor (pkg/vm/browse.go)
interface BrowseColumn { property: string; label: string; type: string; size: number; decimal: number; }
interface BrowseSpec { title: string; alias: string; columns: BrowseColumn[]; items: any[]; readOnly?: boolean; }
// Espelho de dialogSpec do servidor (pkg/vm/dialog.go — MSDIALOG legado)
interface DlgControl { kind: string; x: number; y: number; name?: string; text?: string; value?: string; picture?: string; index: number; }
interface DialogSpec { title: string; rows: DlgControl[][]; buttons: DlgControl[]; }
interface ServerEvent { type: string; id?: number; kind?: string; title?: string; text?: string; data?: any; resumed?: boolean; }
// Espelho de menuSpec/inputSpec do servidor (pkg/webui/server.go)
interface MenuSpec { title: string; items: string[]; }
interface InputSpec { prompt: string; def: string; pw?: boolean; }

@Component({
  selector: 'app-root',
  imports: [PoPageModule, PoTableModule, PoButtonModule, PoModalModule, PoDynamicModule, PoTagModule, PoToasterModule,
            UiHostComponent, NgTemplateOutlet, PoMenuModule, PoLoadingModule],
  template: `
    <!-- Componentes Po* (PoApp:Activate): tela montada pelo programa -->
    @if (uiTree(); as t) {
      @if (uiHead(); as h) { <adv-ui-host [node]="h" [post]="postUI" [sid]="sid"></adv-ui-host> }
      @if (uiNav(); as nv) { <adv-ui-host [node]="nv" [post]="postUI" [sid]="sid"></adv-ui-host> }
      @if (uiPanel(); as pn) { <adv-ui-host [node]="pn" [post]="postUI" [sid]="sid"></adv-ui-host> }
      @if (uiMenu()) {
        <!-- PoMenu: layout padrão do PO-UI (menu lateral + conteúdo) -->
        <div class="po-wrapper">
          <po-menu [p-menus]="menuItems()"></po-menu>
          <div>
            <adv-ui-host [node]="t" [post]="postUI" [versao]="uiVersion()" [sid]="sid"></adv-ui-host>
            <div class="po-ml-2 po-mr-2"><ng-container [ngTemplateOutlet]="consoleTpl"></ng-container></div>
          </div>
        </div>
      } @else {
        <adv-ui-host [node]="t" [post]="postUI" [versao]="uiVersion()" [sid]="sid"></adv-ui-host>
        <div class="po-ml-2 po-mr-2"><ng-container [ngTemplateOutlet]="consoleTpl"></ng-container></div>
      }
      @if (uiLoading(); as l) {
        <po-loading-overlay [p-screen-lock]="true" [p-text]="l"></po-loading-overlay>
      }
    } @else {
    <po-page-default [p-title]="pageTitle()" [p-subtitle]="pageSubtitle()">
      <po-tag [p-value]="status()" [p-color]="statusColor()" [p-icon]="statusIcon()"></po-tag>

      @if (browse(); as b) {
        <po-table
          [p-columns]="tableColumns()"
          [p-items]="b.items"
          [p-actions]="b.readOnly ? [] : tableActions"
          [p-striped]="true"
          [p-sort]="true">
        </po-table>
        <div class="po-mt-2">
          @if (!b.readOnly) {
            <po-button p-label="Incluir" p-kind="primary" p-icon="an an-plus" (p-click)="openForm(null)"></po-button>
          }
          <po-button p-label="Voltar ao menu" p-icon="an an-arrow-left" (p-click)="sendAction({ action: 'close' })"></po-button>
        </div>
      }

      @if (finished()) {
        <po-button class="po-mt-2" p-label="Executar novamente" p-icon="an an-arrow-clockwise" (p-click)="reload()"></po-button>
      }

      <ng-container [ngTemplateOutlet]="consoleTpl"></ng-container>
    </po-page-default>
    }

    <ng-template #consoleTpl>
      @if (lines().length > 0) {
      <div class="advpp-console-toggle" (click)="consoleOpen.set(!consoleOpen())">
        <i class="an" [class.an-caret-right]="!consoleOpen()" [class.an-caret-down]="consoleOpen()"></i>
        Log técnico ({{ lines().length }} linha{{ lines().length === 1 ? '' : 's' }})
      </div>
      @if (consoleOpen()) {
        <div class="advpp-console">@for (line of lines(); track $index) {{{ line }}
}</div>
      }
      }
    </ng-template>

    <po-modal #formModal [p-title]="formTitle()" [p-primary-action]="saveAction" [p-secondary-action]="cancelAction" p-size="lg">
      @if (formFields().length) {
        <po-dynamic-form [p-fields]="formFields()" [p-value]="formValue"></po-dynamic-form>
      }
    </po-modal>

    <!-- FWMenuSelect: lista de opções com ícone, navegação entre telas.
         Botão "Fechar" fecha sem escolher opção — mesmo contrato do
         FWMenuSelect real: devolve 0 pro código (ver menuClose()) em vez
         de travar a VM esperando uma resposta que nunca chega. -->
    <po-modal #menuModal [p-title]="menu()?.title || 'Menu'" [p-primary-action]="menuCloseAction" [p-hide-close]="true" p-size="sm">
      @if (menu(); as m) {
        <div class="advpp-menu">
          @for (item of m.items; track $index) {
            <button type="button" class="advpp-menu-item" (click)="menuChoose($index + 1)">
              <i class="an {{ menuIcon(item) }} advpp-menu-item-icon"></i>
              <span class="advpp-menu-item-label">{{ menuLabel(item) }}</span>
              <i class="an an-caret-right advpp-menu-item-chevron"></i>
            </button>
          }
        </div>
      }
    </po-modal>

    <!-- FWGetText: um campo de texto -->
    <po-modal #inputModal [p-title]="inputSpec()?.prompt || 'Informe um valor'" [p-primary-action]="inputConfirmAction" [p-secondary-action]="inputCancelAction" [p-hide-close]="true" p-size="sm">
      @if (inputSpec()) {
        <label class="po-label">Valor</label>
        <input class="po-input" [type]="inputSpec()?.pw ? 'password' : 'text'"
               [value]="inputFormValue.valor"
               (input)="inputFormValue.valor = $any($event.target).value">
      }
    </po-modal>

    <!-- MSDIALOG legado (fase 4): grade heurística de SAY/GET + botões -->
    <po-modal #legacyModal [p-title]="dlg()?.title || 'Diálogo'" [p-primary-action]="dlgPrimaryAction" [p-hide-close]="true" p-size="auto">
      @if (dlg(); as d) {
        @for (row of d.rows; track $index) {
          <div class="advpp-dlg-row">
            @for (ctl of row; track $index) {
              @if (ctl.kind === 'say') {
                <span class="advpp-dlg-say">{{ ctl.text }}</span>
              } @else {
                <input class="po-input advpp-dlg-get" [value]="dlgValues[ctl.name!]"
                       (input)="dlgValues[ctl.name!] = $any($event.target).value">
              }
            }
          </div>
        }
        @if (d.buttons.length) {
          <div class="advpp-dlg-buttons">
            @for (b of d.buttons; track b.index) {
              <po-button [p-label]="b.text || 'Ok'" [p-kind]="b.index === 0 ? 'primary' : 'secondary'"
                         (p-click)="dlgButton(b.index)"></po-button>
            }
          </div>
        }
      }
    </po-modal>

    <po-toaster></po-toaster>
  `,
})
export class App {
  private dialog = inject(PoDialogService);

  @ViewChild('formModal') formModal!: PoModalComponent;
  @ViewChild('legacyModal') legacyModal!: PoModalComponent;
  @ViewChild('menuModal') menuModal!: PoModalComponent;
  @ViewChild('inputModal') inputModal!: PoModalComponent;

  protected lines = signal<string[]>([]);
  protected status = signal('conectando');
  protected finished = signal(false);
  private sessaoVista = false;
  protected browse = signal<BrowseSpec | null>(null);
  protected formFields = signal<PoDynamicFormField[]>([]);
  protected formTitle = signal('');
  protected formValue: any = {};
  protected dlg = signal<DialogSpec | null>(null);
  protected dlgValues: Record<string, string> = {};
  protected menu = signal<MenuSpec | null>(null);
  protected inputSpec = signal<InputSpec | null>(null);
  protected inputFormValue: any = {};
  protected consoleOpen = signal(false);
  protected uiTree = signal<UiNode | null>(null);
  protected uiMenu = signal<UiNode | null>(null);
  // Onda 3b.3: chrome do PoApp (navbar/header/menupanel), renderizado
  // acima do conteúdo — nós avulsos do mount, não filhos da página.
  protected uiNav = signal<UiNode | null>(null);
  protected uiHead = signal<UiNode | null>(null);
  protected uiPanel = signal<UiNode | null>(null);
  // Patches mudam objetos comuns, que o Angular não observa: cada lote
  // incrementa esta versão (lida no template) para agendar a renderização.
  protected uiVersion = signal(0);
  protected uiLoading = signal<string | null>(null);
  private notification = inject(PoNotificationService);
  private themeService = inject(PoThemeService);
  private languageService = inject(PoLanguageService);
  private userGuide = inject(PoUserGuideService);
  private menuFor: unknown = null;
  private menuCache: PoMenuItem[] = [];
  private uiNodes = new Map<number, UiNode>();
  private uiAppId = 0; // id do PoApp no protocolo (eventos de app: mídia, rede, guia, storage)
  private mediaAtual = '';
  protected menuIcon = menuItemIcon;
  protected menuLabel = menuItemLabel;

  protected sid = Math.random().toString(36).slice(2);
  private browseId = 0;
  private editingRecno = 0;
  private dlgId = 0;
  private menuId = 0;
  private inputId = 0;

  protected pageTitle = computed(() => this.browse()?.title ?? 'AdvPP Web');
  protected pageSubtitle = computed(() => this.menu() ? 'Selecione uma opção no menu' : '');
  protected statusColor = computed(() =>
    this.status() === 'finalizado' ? 'color-11' : this.status() === 'erro' ? 'color-07' : 'color-02');
  protected statusIcon = computed(() =>
    this.status() === 'finalizado' ? 'an an-check-circle' : this.status() === 'erro' ? 'an an-x-circle' : 'an an-circle-notch');

  protected tableColumns = computed<PoTableColumn[]>(() =>
    (this.browse()?.columns ?? []).map(c => ({
      property: c.property,
      label: c.label,
      type: c.type === 'N' ? 'number' : 'string',
    })));

  protected tableActions: PoTableAction[] = [
    { label: 'Editar', icon: 'an an-pencil-simple', action: (row: any) => this.openForm(row) },
    { label: 'Excluir', icon: 'an an-trash', type: 'danger', separator: true, action: (row: any) => this.confirmDelete(row) },
  ];

  protected saveAction: PoModalAction = { label: 'Salvar', action: () => this.save() };
  protected cancelAction: PoModalAction = { label: 'Cancelar', action: () => this.formModal.close() };
  // Fecha o MSDIALOG sem botões próprios (ou como alternativa de saída)
  protected dlgPrimaryAction: PoModalAction = { label: 'Fechar', action: () => this.dlgButton(-1) };

  constructor() {
    // Hooks "before" dos templates PO-UI fazem round-trip pelo POST /ui.
    defineDynPoster(this.postUI);
    // Vários apps (advplc serve --apps): /app/<nome> roda o app <nome>.
    const app = /^\/app\/([A-Za-z0-9_-]{1,64})\/?$/.exec(location.pathname)?.[1] ?? '';
    const es = new EventSource('/events?s=' + this.sid + (app ? '&app=' + app : ''));
    es.onmessage = (m) => this.onEvent(JSON.parse(m.data) as ServerEvent);
    // EventSource reconecta sozinho com o mesmo sid — se o servidor
    // estiver de pé, retoma a mesma sessão sem perder estado. Não fechar:
    // es.close() aqui deixava a página permanentemente morta após
    // qualquer queda (restart do serviço, blip de rede). Se a sessão
    // tiver morrido de verdade, o marcador 'session' na reconexão manda
    // recarregar para o login (ver onEvent).
    es.onerror = () => { if (this.finished()) { es.close(); return; } this.status.set('desconectado'); };

    // Onda 3c.1/3c.2: mídia (breakpoints do grid PO-UI) e rede.
    window.addEventListener('resize', () => this.avisaMidia());
    window.addEventListener('online', () => { if (this.uiAppId) this.postUI(this.uiAppId, 'networkchange', true); });
    window.addEventListener('offline', () => { if (this.uiAppId) this.postUI(this.uiAppId, 'networkchange', false); });
  }

  // Breakpoints do grid PO-UI: sm<=1024, md 1025-1366, lg 1367-1465, xl>=1466.
  private poMediaAtual(): string {
    const w = window.innerWidth;
    if (w <= 1024) return 'sm';
    if (w <= 1366) return 'md';
    if (w <= 1465) return 'lg';
    return 'xl';
  }

  private avisaMidia() {
    const m = this.poMediaAtual();
    if (m !== this.mediaAtual && this.uiAppId) {
      this.mediaAtual = m;
      this.postUI(this.uiAppId, 'mediachange', m);
    }
  }

  // PoApp:SetTheme — "light"|"dark" usa o tema default do PO-UI; JSON é
  // PoThemeTokens (ou PoTheme completo, se tiver name+type) embrulhado.
  private aplicarTema(value: unknown) {
    if (value === 'light' || value === 'dark') {
      this.themeService.setTheme(poThemeDefault,
        value === 'dark' ? PoThemeTypeEnum.dark : PoThemeTypeEnum.light);
      return;
    }
    if (value && typeof value === 'object') {
      const v = value as Record<string, unknown>;
      const tema = (v['name'] && v['type']) ? v : { name: 'advpp', type: { light: v, dark: v } };
      this.themeService.setTheme(tema as any);
    }
  }

  // PoApp:SetLanguage — idiomas do PoLanguageDefault (pt, en, es, ru).
  private aplicarIdioma(lang: string) {
    if (!['pt', 'en', 'es', 'ru'].includes(lang)) return;
    this.languageService.setLanguage(lang);
  }

  // Namespace do PoStorageService do AdvPP: um por sessão de browser.
  private storageKey(chave: string): string {
    return 'advpp:poui:' + this.sid + ':' + chave;
  }

  // PoApp:StartGuide — resolve o componente alvo pelo data-poid e roda o
  // PoUserGuideService; o fim (completed|closed) volta como OnGuideEnd.
  private iniciarGuia(stepsRaw: unknown) {
    const steps = ((Array.isArray(stepsRaw) ? stepsRaw : []) as { target?: number; title?: string; content?: string }[])
      .map(s => {
        const el = s.target ? document.querySelector('adv-ui-host[data-poid="' + s.target + '"]') : null;
        return el ? ({ element: el, title: s.title, content: s.content ?? '' } as PoUserGuideStep) : null;
      })
      .filter((s): s is PoUserGuideStep => s !== null);
    if (!steps.length) return;
    this.userGuide.setSteps(steps);
    this.userGuide.tourEnd$.pipe(take(1)).subscribe(e => {
      if (this.uiAppId) this.postUI(this.uiAppId, 'guideend', e.reason === 'completed');
    });
    void this.userGuide.start();
  }

  private onEvent(ev: ServerEvent) {
    switch (ev.type) {
      case 'session':
        // Marcador emitido pelo servidor a cada conexão SSE (primeiro
        // evento do stream). resumed=false com sessão já vista = a sessão
        // anterior morreu (restart do serviço etc.) e um programa NOVO
        // reexecuta com o sid antigo — o DOM existente não reflete esse
        // programa, então recarrega para recomeçar limpo (login). Blip de
        // rede com servidor vivo reconecta na mesma sessão (resumed=true)
        // e segue de onde parou.
        if (!ev.resumed && this.sessaoVista) { location.reload(); break; }
        this.sessaoVista = true;
        this.status.set('conectado');
        break;
      case 'output':
        this.status.set('executando');
        this.lines.update(l => [...l, ev.text ?? '']);
        break;
      case 'dialog':
        this.showDialog(ev);
        break;
      case 'browse':
        this.browseId = ev.id ?? 0;
        this.browse.set(ev.data ?? null);
        break;
      case 'msdialog': {
        this.dlgId = ev.id ?? 0;
        const spec = ev.data as DialogSpec;
        this.dlgValues = {};
        for (const row of spec.rows) {
          for (const ctl of row) {
            if (ctl.kind === 'get' && ctl.name) { this.dlgValues[ctl.name] = ctl.value ?? ''; }
          }
        }
        this.dlg.set(spec);
        this.legacyModal.open();
        break;
      }
      case 'menu': {
        this.menuId = ev.id ?? 0;
        this.menu.set(ev.data ?? null);
        this.menuModal.open();
        break;
      }
      case 'input': {
        this.inputId = ev.id ?? 0;
        const spec = ev.data as InputSpec;
        this.inputSpec.set(spec);
        this.inputFormValue = { valor: spec.def ?? '' };
        this.inputModal.open();
        break;
      }
      case 'error':
        this.status.set('erro');
        this.lines.update(l => [...l, 'ERRO: ' + (ev.text ?? '')]);
        break;
      case 'done':
        this.status.set('finalizado');
        this.finished.set(true);
        this.browse.set(null);
        this.uiTree.set(null);
        break;
      case 'ui':
        this.onUi(ev.data);
        break;
      case 'reload': // hot reload do --watch: fonte recompilado no servidor
        location.reload();
        break;
      case 'open-tab': { // FwCallApp: aba nova com o app PO-UI (url vem do servidor)
        const d = ev.data as { url?: string } | undefined;
        if (typeof d?.url === 'string' && d.url.startsWith('/poui/')) {
          // Popup bloqueado: window.open devolve null — avisa com o link
          // para o usuário abrir manualmente (review focus onda 2, item 1).
          if (window.open(d.url, '_blank') === null) {
            (this.notification as any).warning('Pop-up bloqueado — abra o app manualmente: ' + d.url);
          }
        }
        break;
      }
    }
  }

  // Operações dos componentes Po* (pkg/vm/poui.go): mount monta a árvore,
  // batch aplica patches de propriedades, close desmonta.
  private onUi(op: any) {
    switch (op?.op) {
      case 'mount': {
        this.uiNodes.clear();
        const indexa = (n: UiNode) => { this.uiNodes.set(n.id, n); n.children.forEach(indexa); };
        if (op.tree) { indexa(op.tree); }
        if (op.menu) { indexa(op.menu); }
        if (op.nav) { indexa(op.nav); }
        if (op.head) { indexa(op.head); }
        if (op.panel) { indexa(op.panel); }
        this.uiMenu.set(op.menu ?? null);
        this.uiNav.set(op.nav ?? null);
        this.uiHead.set(op.head ?? null);
        this.uiPanel.set(op.panel ?? null);
        document.title = op.title || 'AdvPP Web';
        this.uiAppId = Number(op.appId ?? 0);
        this.uiTree.set(op.tree ?? null);
        // Estado inicial de mídia/rede: o OnMediaChange/OnNetworkChange
        // do AdvPL recebe o valor atual assim que o laço começa.
        if (this.uiAppId) {
          this.postUI(this.uiAppId, 'mediachange', this.poMediaAtual());
          this.postUI(this.uiAppId, 'networkchange', navigator.onLine);
        }
        break;
      }
      case 'batch':
        for (const p of op.ops ?? []) {
          this.onUiOp(p);
        }
        this.uiVersion.update(v => v + 1);
        break;
      case 'term': { // saída do AdvTerminal (base64 de bytes crus)
        const bin = atob(op.data ?? '');
        const bytes = new Uint8Array(bin.length);
        for (let i = 0; i < bin.length; i++) { bytes[i] = bin.charCodeAt(i); }
        entregaTerm(op.id, bytes);
        break;
      }
      case 'close':
        this.uiTree.set(null);
        this.uiMenu.set(null);
        this.uiNav.set(null);
        this.uiHead.set(null);
        this.uiPanel.set(null);
        this.uiLoading.set(null);
        this.uiNodes.clear();
        this.uiAppId = 0;
        break;
    }
  }

  // Uma operação do lote, na ordem em que o programa a gerou.
  private onUiOp(p: any) {
    switch (p.op) {
      case 'patch': {
        const n = this.uiNodes.get(p.id);
        if (n) { n.props = { ...n.props, ...p.props }; }
        break;
      }
      case 'notify': {
        // PoApp:Notify estendido — PoNotification completo (duração,
        // orientação, ação). A ação volta ao AdvPL via POST /ui com o
        // actionId registrado no servidor.
        const tipo = ['success', 'warning', 'error', 'information'].includes(p.type) ? p.type : 'information';
        const notif: any = { message: String(p.text ?? '') };
        if (typeof p.duration === 'number' && p.duration > 0) { notif.duration = p.duration; }
        if (p.orientation === 'top') { notif.orientation = 1; } // PoToasterOrientation.Top
        if (p.actionLabel && typeof p.actionId === 'number') {
          const aid = p.actionId;
          notif.actionLabel = String(p.actionLabel);
          notif.action = () => this.postUI(aid, 'notifyact', null);
        }
        (this.notification as any)[tipo](notif);
        break;
      }
      case 'confirm':
        this.dialog.confirm({
          title: p.title || 'Confirmação', message: p.text ?? '',
          confirm: () => this.postUI(p.id, 'confirm', true),
          cancel: () => this.postUI(p.id, 'confirm', false),
        });
        break;
      case 'loading':
        this.uiLoading.set(p.on ? (p.text || 'Carregando') : null);
        break;
      case 'beforeResult': // hook síncrono OnBefore* dos PoPageDynamic*
        dynAntesResultado(Number(p.reqId ?? 0), !!p.ok);
        break;
      case 'open': // PoApp:Open: outro app numa janela nova
        if (/^[A-Za-z0-9_-]{1,64}$/.test(p.app)) { window.open('/app/' + p.app, '_blank'); }
        break;
      // ---- Onda 3c: serviços e diretivas ----
      case 'theme': // PoApp:SetTheme — "light"|"dark" ou tokens JSON
        this.aplicarTema(p.value);
        break;
      case 'language': // PoApp:SetLanguage — pt|en|es|ru
        this.aplicarIdioma(String(p.value ?? ''));
        break;
      case 'storageset': // PoApp:StorageSet — localStorage namespaced por sessão
        try { localStorage.setItem(this.storageKey(String(p.key ?? '')), JSON.stringify(p.value)); } catch {}
        break;
      case 'storageremove':
        try { localStorage.removeItem(this.storageKey(String(p.key ?? ''))); } catch {}
        break;
      case 'storageget': { // PoApp:StorageGet — resposta síncrona ao VM
        const raw = (() => { try { return localStorage.getItem(this.storageKey(String(p.key ?? ''))); } catch { return null; } })();
        let value: unknown = null;
        if (raw != null) {
          try { value = JSON.parse(raw); } catch { value = raw; }
        }
        this.postUI(this.uiAppId, 'storageresult', { reqId: p.reqId, value });
        break;
      }
      case 'guide': // PoApp:StartGuide — passos {target,title,content}
        this.iniciarGuia(p.steps);
        break;
    }
  }

  // Itens do PoMenu; recriados só quando a lista do servidor muda.
  protected menuItems(): PoMenuItem[] {
    const m = this.uiMenu();
    const items = m?.props['items'];
    if (items !== this.menuFor) {
      this.menuFor = items;
      this.menuCache = (items ?? []).map((it: { label: string; icon: string; index: number }) => ({
        label: it.label, icon: it.icon || undefined, shortLabel: it.label,
        action: () => this.postUI(m!.id, 'action', it.index),
      }));
    }
    return this.menuCache;
  }

  // Evento de componente -> laço de PoApp:Activate (POST /ui).
  // Os POSTs saem em fila (um por vez), na ordem dos eventos: um "change"
  // não pode chegar ao servidor depois do "click" que veio em seguida.
  private filaUI: Promise<unknown> = Promise.resolve();
  protected postUI = (id: number, event: string, value: unknown) => {
    descarregaPendentes();
    const body = JSON.stringify({ id, event, value });
    this.filaUI = this.filaUI.then(() => fetch('/ui?s=' + this.sid, { method: 'POST', body })).catch(() => undefined);
  };

  private showDialog(ev: ServerEvent) {
    const title = ev.title || 'Atenção';
    if (ev.kind === 'yesno') {
      this.dialog.confirm({
        title, message: ev.text ?? '',
        confirm: () => this.reply(ev.id!, 'yes'),
        cancel: () => this.reply(ev.id!, 'no'),
      });
    } else {
      this.dialog.alert({ title, message: ev.text ?? '', ok: () => this.reply(ev.id!, 'ok') });
    }
  }

  protected openForm(row: any | null) {
    const b = this.browse();
    if (!b) { return; }
    this.editingRecno = row?.recno ?? 0;
    this.formTitle.set((row ? 'Alterar — ' : 'Incluir — ') + b.title);
    this.formValue = {};
    for (const c of b.columns) {
      this.formValue[c.property] = row ? row[c.property] : (c.type === 'N' ? 0 : '');
    }
    this.formFields.set(b.columns.map(c => ({
      property: c.property,
      label: c.label,
      gridColumns: c.size > 20 ? 12 : 6,
      maxLength: c.type === 'C' && c.size > 0 ? c.size : undefined,
      type: c.type === 'N' ? 'number' : 'string',
    })));
    this.formModal.open();
  }

  private save() {
    const data: Record<string, string> = {};
    for (const f of this.formFields()) {
      data[f.property] = String(this.formValue[f.property] ?? '');
    }
    this.formModal.close();
    this.sendAction({ action: 'save', recno: this.editingRecno, data });
  }

  private confirmDelete(row: any) {
    this.dialog.confirm({
      title: 'Excluir',
      message: 'Confirma a exclusão do registro?',
      confirm: () => this.sendAction({ action: 'delete', recno: row.recno }),
    });
  }

  protected sendAction(action: object) {
    this.reply(this.browseId, JSON.stringify(action));
  }

  protected menuChoose(index: number) {
    this.menuModal.close();
    this.menu.set(null);
    this.reply(this.menuId, String(index));
  }

  // Fecha o menu sem escolher opção (botão "Fechar") — mesmo contrato do
  // FWMenuSelect real (ex.: PADRAO_GUI.md do GesCon): devolve 0 pro
  // código chamador em vez de deixar a VM bloqueada pra sempre esperando
  // um /reply que nunca chega (era exatamente esse bloqueio, combinado
  // com o EventSource reconectando sozinho, que fazia o programa inteiro
  // "reiniciar" do nada na 2ª chamada de FWMenuSelect — ver server.go).
  protected menuCloseAction: PoModalAction = {
    label: 'Fechar',
    action: () => {
      this.menuModal.close();
      this.menu.set(null);
      this.reply(this.menuId, '0');
    },
  };

  protected inputConfirmAction: PoModalAction = { label: 'OK', action: () => this.inputConfirm() };
  protected inputCancelAction: PoModalAction = { label: 'Cancelar', action: () => this.inputCancel() };

  private inputConfirm() {
    const value = String(this.inputFormValue.valor ?? '');
    this.inputModal.close();
    this.inputSpec.set(null);
    this.reply(this.inputId, value);
  }

  private inputCancel() {
    this.inputModal.close();
    this.inputSpec.set(null);
    this.reply(this.inputId, '');
  }

  protected dlgButton(index: number) {
    this.legacyModal.close();
    this.dlg.set(null);
    this.reply(this.dlgId, JSON.stringify({
      action: index >= 0 ? 'button' : 'close',
      index,
      data: this.dlgValues,
    }));
  }

  private reply(id: number, result: string) {
    fetch('/reply?s=' + this.sid, { method: 'POST', body: JSON.stringify({ id, result }) });
  }

  protected reload() { location.reload(); }
}
