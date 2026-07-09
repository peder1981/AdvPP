import { Component, ViewChild, computed, inject, signal } from '@angular/core';
import {
  PoButtonModule, PoDialogService, PoDynamicFormField, PoDynamicModule,
  PoModalAction, PoModalComponent, PoModalModule, PoPageModule,
  PoTableAction, PoTableColumn, PoTableModule, PoTagModule,
} from '@po-ui/ng-components';

// Espelho de browseSpec/browseAction do servidor (pkg/vm/browse.go)
interface BrowseColumn { property: string; label: string; type: string; size: number; decimal: number; }
interface BrowseSpec { title: string; alias: string; columns: BrowseColumn[]; items: any[]; }
interface ServerEvent { type: string; id?: number; kind?: string; title?: string; text?: string; data?: BrowseSpec; }

@Component({
  selector: 'app-root',
  imports: [PoPageModule, PoTableModule, PoButtonModule, PoModalModule, PoDynamicModule, PoTagModule],
  template: `
    <po-page-default [p-title]="pageTitle()">
      <po-tag [p-value]="status()" [p-color]="statusColor()"></po-tag>

      @if (browse(); as b) {
        <po-table
          [p-columns]="tableColumns()"
          [p-items]="b.items"
          [p-actions]="tableActions"
          [p-striped]="true"
          [p-sort]="true">
        </po-table>
        <div class="po-mt-2">
          <po-button p-label="Incluir" p-kind="primary" p-icon="an an-plus" (p-click)="openForm(null)"></po-button>
          <po-button p-label="Fechar browse" (p-click)="sendAction({ action: 'close' })"></po-button>
        </div>
      }

      <div class="advpp-console">@for (line of lines(); track $index) {{{ line }}
}</div>

      @if (finished()) {
        <po-button class="po-mt-2" p-label="Executar novamente" p-icon="an an-arrow-clockwise" (p-click)="reload()"></po-button>
      }
    </po-page-default>

    <po-modal #formModal [p-title]="formTitle()" [p-primary-action]="saveAction" [p-secondary-action]="cancelAction" p-size="lg">
      @if (formFields().length) {
        <po-dynamic-form [p-fields]="formFields()" [p-value]="formValue"></po-dynamic-form>
      }
    </po-modal>
  `,
})
export class App {
  private dialog = inject(PoDialogService);

  @ViewChild('formModal') formModal!: PoModalComponent;

  protected lines = signal<string[]>([]);
  protected status = signal('conectando');
  protected finished = signal(false);
  protected browse = signal<BrowseSpec | null>(null);
  protected formFields = signal<PoDynamicFormField[]>([]);
  protected formTitle = signal('');
  protected formValue: any = {};

  private sid = Math.random().toString(36).slice(2);
  private browseId = 0;
  private editingRecno = 0;

  protected pageTitle = computed(() => this.browse()?.title ?? 'AdvPP Web');
  protected statusColor = computed(() =>
    this.status() === 'finalizado' ? 'color-11' : this.status() === 'erro' ? 'color-07' : 'color-02');

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

  constructor() {
    const es = new EventSource('/events?s=' + this.sid);
    es.onmessage = (m) => this.onEvent(JSON.parse(m.data) as ServerEvent);
    es.onerror = () => { if (!this.finished()) this.status.set('desconectado'); es.close(); };
  }

  private onEvent(ev: ServerEvent) {
    switch (ev.type) {
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
      case 'error':
        this.status.set('erro');
        this.lines.update(l => [...l, 'ERRO: ' + (ev.text ?? '')]);
        break;
      case 'done':
        this.status.set('finalizado');
        this.finished.set(true);
        this.browse.set(null);
        break;
      case 'reload': // hot reload do --watch: fonte recompilado no servidor
        location.reload();
        break;
    }
  }

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

  private reply(id: number, result: string) {
    fetch('/reply?s=' + this.sid, { method: 'POST', body: JSON.stringify({ id, result }) });
  }

  protected reload() { location.reload(); }
}
