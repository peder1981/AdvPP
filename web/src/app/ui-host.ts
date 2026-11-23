import { Component, Directive, Input, inject, signal, forwardRef, AfterViewInit, OnChanges } from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';
import { HttpClient } from '@angular/common/http';
import { AdvCodeEditorComponent } from './adv-code-editor';
import { AdvTerminalComponent } from './adv-terminal';
import { AdvRotaComponent } from './adv-rota';
import { FormsModule } from '@angular/forms';
import { antesUrl, dynAntes } from './dyn-hooks';
import { from } from 'rxjs';
import {
  PoPageBlockedUserModule, PoPageChangePasswordModule, PoPageDynamicDetailModule, PoPageDynamicEditModule,
  PoPageDynamicSearchModule, PoPageDynamicTableModule, PoPageJobSchedulerModule, PoPageLoginModule,
  PoModalPasswordRecoveryComponent, PoModalPasswordRecoveryModule,
} from '@po-ui/ng-templates';
import {
  PoAccordionModule, PoModalModule, PoModalComponent, PoSlideModule, PoStepperModule,
  PoButtonGroupModule, PoDropdownModule, PoPopupModule, PoPopoverModule, PoContextMenuModule, PoToolbarModule,
  PoBreadcrumbModule, PoLinkModule, PoContextTabsModule,
  PoNavbarModule, PoHeaderModule, PoMenuPanelModule,
  PoAvatarModule, PoBadgeModule, PoImageModule, PoSkeletonModule, PoHelperModule,
  PoCalendarModule, PoDisclaimerGroupModule, PoFilterChipModule,
  PoButtonModule, PoChartModule, PoChartSerie, PoContainerModule, PoDividerModule, PoDynamicModule, PoFieldModule, PoSearchModule,
  PoGaugeModule, PoInfoModule, PoListViewModule, PoPageAction, PoPageModule, PoProgressModule,
  PoTableModule, PoTabsModule, PoTagModule, PoTooltipModule, PoTreeViewItem, PoTreeViewModule, PoWidgetModule,
} from '@po-ui/ng-components';

// Espelho do protocolo de componentes (pkg/vm/poui.go, op "mount"/"batch").
export interface UiNode {
  id: number;
  type: string;
  props: Record<string, any>;
  children: UiNode[];
}

export type UiPost = (id: number, event: string, value: unknown) => void;

// Digitação ainda não enviada (debounce), por componente. Qualquer outro
// evento descarrega antes de sair (descarregaPendentes), para um clique logo
// após digitar não chegar ao programa antes do valor digitado.
const pendentes = new Map<object, () => void>();

export function adiaEnvio(dono: object, ms: number, envia: () => void): ReturnType<typeof setTimeout> {
  const f = () => { pendentes.delete(dono); envia(); };
  pendentes.set(dono, f);
  return setTimeout(f, ms);
}

export function cancelaEnvio(dono: object) {
  pendentes.delete(dono);
}

export function descarregaPendentes() {
  for (const f of [...pendentes.values()]) {
    f();
  }
}

// O po-modal só exibe após open(): a diretiva abre na criação do elemento
// (o @if do template só o monta quando a prop "open" do servidor é true).
// PoModalPasswordRecovery: criado quando o AdvPL chama Open(); abre ao
// nascer e mostra a tela de concluído quando o AdvPL chama Completed().
@Directive({ selector: 'po-modal-password-recovery[appRecOpen]', standalone: true })
export class AppRecOpen implements AfterViewInit, OnChanges {
  private modal = inject(PoModalPasswordRecoveryComponent);
  @Input() appRecCompleted = false;
  private aberto = false;
  // open() lê emailForm.controls: os controles (ngModel) só se registram
  // depois do primeiro ciclo — abrir no mesmo ciclo deixava o controle
  // indefinido e o envio quebrava (markAsPristine de undefined).
  ngAfterViewInit() {
    setTimeout(() => {
      this.modal.open();
      this.aberto = true;
      if (this.appRecCompleted) {
        this.modal.completed();
      }
    });
  }
  ngOnChanges() {
    if (this.aberto && this.appRecCompleted) {
      this.modal.completed();
    }
  }
}

@Directive({ selector: 'po-modal[appModalOpen]', standalone: true })
export class AppModalOpen implements AfterViewInit {
  private modal = inject(PoModalComponent);
  ngAfterViewInit() {
    this.modal.open();
  }
}

// Renderiza um nó da árvore e, recursivamente, os filhos. Genérico: não
// conhece nenhum aplicativo, só o mapeamento tipo -> componente PO-UI.
// Cache de datas convertidas (isoDate/isoDateTime): referência estável.
const datas = new Map<string, Date | null>();
function guardaData(chave: string, f: () => Date | null): Date | null {
  const d = f();
  datas.set(chave, d);
  return d;
}

// Ações que o po-page-dynamic-table executa por função (OnCustomAction).
const ACOES_PADRAO = new Set(['new', 'detail', 'edit', 'duplicate']);

function acoesDe(node: UiNode): Record<string, unknown> {
  const a = node.props['actions'];
  return a && typeof a === 'object' ? (a as Record<string, unknown>) : {};
}

// Registro da linha entre os argumentos da ação (id, resource) do PO-UI.
function linhaDe(args: unknown[]): unknown {
  return [...args].reverse().find(x => x !== null && typeof x === 'object') ?? null;
}

function copiaLista(x: unknown): unknown[] {
  return Array.isArray(x) ? x.map(e => (e && typeof e === 'object' ? { ...e } : e)) : [];
}

function listaTexto(x: unknown): string[] {
  return Array.isArray(x) ? x.map(e => String(e)) : [];
}

@Component({
  selector: 'adv-ui-host',
  host: { '[attr.data-poid]': 'node.id' },
  imports: [
    forwardRef(() => UiHostComponent), FormsModule, NgTemplateOutlet, AdvCodeEditorComponent, AdvTerminalComponent, AdvRotaComponent,
    PoPageModule, PoButtonModule, PoFieldModule, PoSearchModule, PoTableModule, PoTagModule,
    PoContainerModule, PoDividerModule, PoWidgetModule, PoTabsModule, PoTreeViewModule,
    PoListViewModule, PoInfoModule, PoProgressModule, PoGaugeModule, PoChartModule, PoDynamicModule,
    PoAccordionModule, PoModalModule, PoSlideModule, PoStepperModule, AppModalOpen,
  PoButtonGroupModule, PoDropdownModule, PoPopupModule, PoPopoverModule, PoContextMenuModule, PoToolbarModule,
  PoTooltipModule,
  PoBreadcrumbModule, PoLinkModule, PoContextTabsModule,
  PoNavbarModule, PoHeaderModule, PoMenuPanelModule,
  PoAvatarModule, PoBadgeModule, PoImageModule, PoSkeletonModule, PoHelperModule,
  PoCalendarModule, PoDisclaimerGroupModule, PoFilterChipModule,
    PoPageDynamicTableModule, PoPageDynamicEditModule, PoPageDynamicDetailModule, PoPageDynamicSearchModule,
    PoPageLoginModule, PoPageChangePasswordModule, PoPageBlockedUserModule, PoPageJobSchedulerModule,
    PoModalPasswordRecoveryModule, AppRecOpen,
  ],
  styles: [`
    .po-login-wrap { display: flex; justify-content: center; padding: 3rem 1rem; }
    .po-login-card { width: 100%; max-width: 360px; display: flex; flex-direction: column; gap: .5rem; }
    .po-login-title { margin: 0 0 .5rem; }
    .po-login-erro { color: var(--color-semantic-negative, #c00); font-size: .875rem; }
    .advpp-ui-grid { display: flex; flex-wrap: wrap; margin: 0 -8px; }
    .advpp-ui-grid > .advpp-ui-item { padding: 0 8px; box-sizing: border-box; }
    .advpp-ui-dropzone { height: 6px; }
    .advpp-ui-grid > .advpp-ui-dropzone { flex: 0 0 100%; padding: 0; }
  `],
  template: `
    <!-- Filhos de page/container/widget/tab: blocos empilhados, ou grade de
         12 colunas quando algum filho usa SetCols. O wrapper carrega o
         tooltip (SetTooltip) e o drag & drop (SetDraggable/OnDrop). -->
    <ng-template #filhos let-pai>
      @if (temCols(pai)) {
        <div class="advpp-ui-grid">
          @for (c of pai.children; track c.id) {
            <div class="advpp-ui-item" [style.flex]="'0 0 ' + largura(c)" [style.max-width]="largura(c)"
                 [p-tooltip]="c.props['tooltip'] || ''" [p-tooltip-position]="c.props['tooltipPosition'] || 'top'"
                 [draggable]="!!c.props['draggable']"
                 (dragstart)="onDragStart($event, c)" (dragover)="onDragOver($event)"
                 (drop)="onDrop($event, pai, c)">
              <adv-ui-host [node]="c" [post]="post"></adv-ui-host>
            </div>
          }
          <div class="advpp-ui-dropzone" (dragover)="onDragOver($event)" (drop)="onDropZone($event, pai)"></div>
        </div>
      } @else {
        @for (c of pai.children; track c.id) {
          <div class="advpp-ui-item"
               [p-tooltip]="c.props['tooltip'] || ''" [p-tooltip-position]="c.props['tooltipPosition'] || 'top'"
               [draggable]="!!c.props['draggable']"
               (dragstart)="onDragStart($event, c)" (dragover)="onDragOver($event)"
               (drop)="onDrop($event, pai, c)">
            <adv-ui-host [node]="c" [post]="post"></adv-ui-host>
          </div>
        }
        <div class="advpp-ui-dropzone" (dragover)="onDragOver($event)" (drop)="onDropZone($event, pai)"></div>
      }
    </ng-template>

    @if (node.props['visible'] !== false) {
      @switch (node.type) {
        @case ('page') {
          <po-page-default [p-title]="node.props['title']" [p-actions]="pageActions()">
            <ng-container *ngTemplateOutlet="filhos; context: { $implicit: node }"></ng-container>
          </po-page-default>
        }
        @case ('pagelist') {
          <po-page-list [p-title]="node.props['title']" [p-actions]="pageActions()"
                        [p-filter]="pageFilter()"
                        (p-change-disclaimers)="post(node.id, 'changedisclaimers', $event)">
            <ng-container *ngTemplateOutlet="filhos; context: { $implicit: node }"></ng-container>
          </po-page-list>
        }
        @case ('pageedit') {
          <po-page-edit [p-title]="node.props['title']"
                        (p-save)="post(node.id, 'save', null)" (p-save-new)="post(node.id, 'savenew', null)"
                        (p-cancel)="post(node.id, 'cancel', null)">
            <ng-container *ngTemplateOutlet="filhos; context: { $implicit: node }"></ng-container>
          </po-page-edit>
        }
        @case ('pagedetail') {
          <po-page-detail [p-title]="node.props['title']"
                          (p-back)="post(node.id, 'back', null)" (p-edit)="post(node.id, 'edit', null)"
                          (p-remove)="post(node.id, 'remove', null)">
            <ng-container *ngTemplateOutlet="filhos; context: { $implicit: node }"></ng-container>
          </po-page-detail>
        }
        @case ('container') {
          <po-container [p-title]="node.props['title']">
            <ng-container *ngTemplateOutlet="filhos; context: { $implicit: node }"></ng-container>
          </po-container>
        }
        @case ('widget') {
          <po-widget [p-title]="node.props['title']">
            <ng-container *ngTemplateOutlet="filhos; context: { $implicit: node }"></ng-container>
          </po-widget>
        }
        @case ('divider') {
          <po-divider [p-label]="node.props['label']"></po-divider>
        }
        @case ('tabs') {
          <!-- po-tab precisa ser filho direto de po-tabs (ContentChildren) -->
          <po-tabs>
            @for (tab of node.children; track tab.id) {
              <po-tab [p-label]="tab.props['label']" [p-active]="!!tab.props['active']">
                <ng-container *ngTemplateOutlet="filhos; context: { $implicit: tab }"></ng-container>
              </po-tab>
            }
          </po-tabs>
        }
        @case ('button') {
          <po-button [p-label]="node.props['label']" [p-disabled]="!!node.props['disabled']"
                     [p-icon]="node.props['icon']" [p-kind]="node.props['kind'] || 'secondary'"
                     (p-click)="post(node.id, 'click', null)"></po-button>
        }
        @case ('input') {
          <po-input [p-label]="node.props['label']" [p-disabled]="node.props['disabled'] ? 'true' : 'false'"
                    [ngModel]="node.props['value']" (ngModelChange)="onInput($event)"></po-input>
        }
        @case ('login') {
          <po-login [p-label]="node.props['label']" [p-disabled]="node.props['disabled'] ? 'true' : 'false'"
                    [ngModel]="node.props['value']" (ngModelChange)="onInput($event)"></po-login>
        }
        @case ('dynamicform') {
          <po-dynamic-form [p-fields]="dynCamposTpl(node)" [p-value]="formValor(node)"
                           [p-validate]="formValida(node)"></po-dynamic-form>
        }
        @case ('passwordrecovery') {
          @if (node.props['open']) {
            <po-modal-password-recovery appRecOpen [appRecCompleted]="!!node.props['completed']"
                                        [p-type]="node.props['type']" [p-contact-email]="node.props['contactEmail'] || ''"
                                        [p-phone-mask]="node.props['phoneMask'] || ''"
                                        (p-submit)="post(node.id, 'recoverysubmit', $event)"></po-modal-password-recovery>
          }
        }
        @case ('password') {
          <po-password [p-label]="node.props['label']" [ngModel]="node.props['value']"
                       (ngModelChange)="onInput($event)"></po-password>
        }
        @case ('textarea') {
          <po-textarea [p-label]="node.props['label']" [ngModel]="node.props['value']"
                       (ngModelChange)="onInput($event)"></po-textarea>
        }
        @case ('number') {
          <po-number [p-label]="node.props['label']" [ngModel]="node.props['value']"
                     (ngModelChange)="onInput($event)"></po-number>
        }
        @case ('select') {
          <po-select [p-label]="node.props['label']" [p-options]="node.props['options']"
                     [ngModel]="node.props['value']" (ngModelChange)="onImediato($event)"></po-select>
        }
        @case ('checkbox') {
          <po-checkbox [p-label]="node.props['label']" [ngModel]="node.props['value']"
                       (ngModelChange)="onImediato($event)"></po-checkbox>
        }
        @case ('switch') {
          <po-switch [p-label]="node.props['label']" [p-hide-label-status]="true" [ngModel]="node.props['value']"
                     (ngModelChange)="onImediato($event)"></po-switch>
        }
        @case ('combo') {
          <po-combo [p-label]="node.props['label']" [p-options]="node.props['options']"
                    [p-filter-service]="node.props['filterUrl'] || undefined"
                    [p-field-label]="node.props['fieldLabel'] || 'label'"
                    [p-field-value]="node.props['fieldValue'] || 'value'"
                    [p-placeholder]="node.props['placeholder']"
                    [p-required]="!!node.props['required']" [p-disabled]="!!node.props['disabled']"
                    [ngModel]="node.props['value']" (p-change)="onImediato($event)"></po-combo>
        }
        @case ('multiselect') {
          <po-multiselect [p-label]="node.props['label']" [p-options]="node.props['options']"
                          [p-filter-service]="node.props['filterUrl'] || undefined"
                          [p-field-label]="node.props['fieldLabel'] || 'label'"
                          [p-field-value]="node.props['fieldValue'] || 'value'"
                          [p-placeholder]="node.props['placeholder']"
                          [p-required]="!!node.props['required']" [p-disabled]="!!node.props['disabled']"
                          [ngModel]="node.props['value']" (p-change)="onImediato($event)"></po-multiselect>
        }
        @case ('radiogroup') {
          <!-- po-radio-group não tem p-label: o rótulo do campo não existe no PO-UI. -->
          <po-radio-group [p-options]="node.props['options']" [p-columns]="node.props['columns'] || 1"
                          [p-required]="!!node.props['required']" [p-disabled]="!!node.props['disabled']"
                          [ngModel]="node.props['value']" (p-change)="onImediato($event)"></po-radio-group>
        }
        @case ('checkboxgroup') {
          <po-checkbox-group [p-label]="node.props['label']" [p-options]="node.props['options']"
                             [p-columns]="node.props['columns'] || 1" [p-indeterminate]="!!node.props['indeterminate']"
                             [p-required]="!!node.props['required']" [p-disabled]="!!node.props['disabled']"
                             [ngModel]="node.props['value']" (p-change)="onImediato($event)"></po-checkbox-group>
        }
        @case ('lookup') {
          <po-lookup [p-label]="node.props['label']" [p-columns]="node.props['columns']"
                     [p-field-label]="node.props['fieldLabel'] || 'label'"
                     [p-field-value]="node.props['fieldValue'] || 'value'"
                     [p-filter-service]="node.props['filterUrl'] || undefined"
                     [p-filter-params]="node.props['filterParams']"
                     [p-required]="!!node.props['required']" [p-disabled]="!!node.props['disabled']"
                     [ngModel]="node.props['value']" (p-change)="onImediato($event)"
                     (p-selected)="onLookupSelected($event)"></po-lookup>
        }
        @case ('datepicker') {
          <po-datepicker [p-label]="node.props['label']" [p-format]="node.props['format']"
                         [p-min-date]="isoDate(node.props['minDate'])" [p-max-date]="isoDate(node.props['maxDate'])"
                         [p-required]="node.props['required'] ? 'true' : 'false'" [p-disabled]="node.props['disabled'] ? 'true' : 'false'"
                         [p-readonly]="node.props['readonly'] ? 'true' : 'false'" [p-clean]="node.props['clean'] === false ? 'false' : 'true'"
                         [ngModel]="isoDate(node.props['value'])" (p-change)="onImediato($event)"></po-datepicker>
        }
        @case ('daterange') {
          <po-datepicker-range [p-label]="node.props['label']"
                               [p-min-date]="isoDate(node.props['minDate'])" [p-max-date]="isoDate(node.props['maxDate'])"
                               [p-required]="!!node.props['required']" [p-disabled]="!!node.props['disabled']"
                               [p-readonly]="!!node.props['readonly']" [p-clean]="node.props['clean'] !== false"
                               [ngModel]="node.props['value']" (p-change)="onImediato($event)"></po-datepicker-range>
        }
        @case ('datetimepicker') {
          <po-datetimepicker [p-label]="node.props['label']"
                             [p-min-date]="isoDate(node.props['minDate'])" [p-max-date]="isoDate(node.props['maxDate'])"
                             [p-min-time]="node.props['minTime']" [p-max-time]="node.props['maxTime']"
                             [p-format-date]="node.props['formatDate']" [p-format-time]="node.props['formatTime']"
                             [p-required]="!!node.props['required']" [p-disabled]="!!node.props['disabled']"
                             [p-readonly]="!!node.props['readonly']" [p-clean]="node.props['clean'] === false ? 'false' : 'true'"
                             [ngModel]="isoDateTime(node.props['value'])" (p-change)="onImediato($event)"></po-datetimepicker>
        }
        @case ('timepicker') {
          <po-timepicker [p-label]="node.props['label']" [p-format]="node.props['format']"
                         [p-min-time]="node.props['minTime']" [p-max-time]="node.props['maxTime']"
                         [p-required]="node.props['required'] ? 'true' : 'false'" [p-disabled]="node.props['disabled'] ? 'true' : 'false'"
                         [p-readonly]="node.props['readonly'] ? 'true' : 'false'" [p-clean]="node.props['clean'] === false ? 'false' : 'true'"
                         [ngModel]="node.props['value']" (p-change)="onImediato($event)"></po-timepicker>
        }
        @case ('email') {
          <po-email [p-label]="node.props['label']" [p-disabled]="node.props['disabled'] ? 'true' : 'false'"
                    [ngModel]="node.props['value']" (ngModelChange)="onInput($event)"></po-email>
        }
        @case ('url') {
          <po-url [p-label]="node.props['label']" [p-disabled]="node.props['disabled'] ? 'true' : 'false'"
                  [ngModel]="node.props['value']" (ngModelChange)="onInput($event)"></po-url>
        }
        @case ('decimal') {
          <po-decimal [p-label]="node.props['label']" [p-disabled]="node.props['disabled'] ? 'true' : 'false'"
                      [p-decimals-length]="node.props['decimalsLength']"
                      [ngModel]="node.props['value']" (ngModelChange)="onInput($event)"></po-decimal>
        }
        @case ('richtext') {
          <po-rich-text [p-label]="node.props['label']" [p-height]="node.props['height']"
                        [ngModel]="node.props['value']" (ngModelChange)="onInput($event)"></po-rich-text>
        }
        @case ('search') {
          <po-search [p-items]="node.props['items']" [p-filter-keys]="node.props['filterKeys']"
                     [p-literals]="node.props['literals']" (p-filter)="onFilter($event)"></po-search>
        }
        @case ('upload') {
          <po-upload [p-label]="node.props['label']" [p-url]="node.props['url']"
                     [p-multiple]="!!node.props['multiple']"
                     [p-restrictions]="node.props['restrictions']"
                     (p-success)="onUploadOk($event)"></po-upload>
        }
        @case ('table') {
          <po-table [p-columns]="node.props['columns']" [p-items]="node.props['items']"
                    [p-selectable]="true" [p-single-select]="true" [p-striped]="true"
                    (p-selected)="onSelect($event)"></po-table>
        }
        @case ('tree') {
          <po-tree-view [p-items]="node.props['items']" [p-selectable]="true" [p-single-select]="true"
                        (p-selected)="onTree($event)"></po-tree-view>
        }
        @case ('list') {
          <!-- O template de conteúdo reserva espaço abaixo do título: só
               existe quando algum item tem detalhe. -->
          @if (temDetalhe()) {
            <po-list-view [p-items]="node.props['items']" p-property-title="title"
                          (p-title-action)="onSelect($event)">
              <ng-template p-list-view-content-template let-item>
                @if (item.detail) { <span>{{ item.detail }}</span> }
              </ng-template>
            </po-list-view>
          } @else {
            <po-list-view [p-items]="node.props['items']" p-property-title="title"
                          (p-title-action)="onSelect($event)"></po-list-view>
          }
        }
        @case ('info') {
          <po-info [p-label]="node.props['label']" [p-value]="node.props['value']"></po-info>
        }
        @case ('progress') {
          <po-progress [p-value]="node.props['value']" [p-text]="node.props['text']"></po-progress>
        }
        @case ('gauge') {
          <po-gauge [p-title]="node.props['title']" [p-value]="node.props['value']"
                    [p-description]="node.props['description']" [p-height]="220"></po-gauge>
        }
        @case ('chart') {
          <po-chart [p-title]="node.props['title']" [p-type]="node.props['type']"
                    [p-categories]="node.props['categories']" [p-series]="chartSeries()" [p-height]="280"></po-chart>
        }
        @case ('editor') {
          <!-- CodeMirror e xterm.js carregam sob demanda: fora do pacote inicial -->
          @defer (on immediate) {
            <adv-code-editor [node]="node" [post]="post"></adv-code-editor>
          } @placeholder {
            <div class="po-p-2">Carregando editor...</div>
          }
        }
        @case ('terminal') {
          @defer (on immediate) {
            <adv-terminal [node]="node" [post]="post"></adv-terminal>
          } @placeholder {
            <div class="po-p-2">Carregando terminal...</div>
          }
        }
        @case ('dyntable') {
          <po-page-dynamic-table [p-title]="node.props['title'] || ''" [p-service-api]="dynUrl(node)"
                                 [p-fields]="dynCamposTpl(node)" [p-actions]="dynAcoesTabela(node)"
                                 [p-table-custom-actions]="dynAcoesCustom(node)"
                                 [p-keep-filters]="!!node.props['keepFilters']"
                                 [p-quick-search-param]="node.props['quickSearchField'] || 'search'"></po-page-dynamic-table>
        }
        @case ('dynsearch') {
          <po-page-dynamic-search [p-title]="node.props['title'] || ''" [p-filters]="dynFiltrosTpl(node)"
                                  [p-quick-search-value]="node.props['quickFilter'] || ''"
                                  (p-quick-search)="post(node.id, 'quicksearch', $event)"
                                  (p-advanced-search)="post(node.id, 'advancedsearch', $event)"
                                  (p-change-disclaimers)="post(node.id, 'changedisclaimers', $event)">
            <ng-container *ngTemplateOutlet="filhos; context: { $implicit: node }"></ng-container>
          </po-page-dynamic-search>
        }
        @case ('dynedit') {
          <!-- Com SetId: edita o registro (GET + PUT); sem: modo inclusão (POST). -->
          @for (rid of [node.props['recordId'] || '']; track rid) {
            <adv-rota [id]="rid || null">
              <po-page-dynamic-edit [p-title]="node.props['title'] || ''" [p-service-api]="dynUrl(node)"
                                    [p-fields]="dynCamposTpl(node)" [p-actions]="dynAcoesEdicao(node)"></po-page-dynamic-edit>
            </adv-rota>
          }
        }
        @case ('dyndetail') {
          <!-- O template lê o id da rota: sem SetId não há registro a mostrar
               (montá-lo buscaria <serviço>/undefined). @for recria ao trocar o id. -->
          @if (node.props['recordId']) {
            @for (rid of [node.props['recordId']]; track rid) {
              <adv-rota [id]="rid">
                <po-page-dynamic-detail [p-title]="node.props['title'] || ''" [p-service-api]="dynUrl(node)"
                                        [p-fields]="dynCamposTpl(node)" [p-actions]="dynAcoesDetalhe(node)"></po-page-dynamic-detail>
              </adv-rota>
            }
          } @else {
            <po-page-default [p-title]="node.props['title'] || ''"></po-page-default>
          }
        }
        @case ('tag') {
          <po-tag [p-value]="node.props['value']" [p-type]="node.props['type']"></po-tag>
        }
        @case ('buttongroup') {
          <po-button-group [p-buttons]="poActions(node, 'buttons')" [p-toggle]="node.props['toggle'] || 'none'"></po-button-group>
        }
        @case ('dropdown') {
          <po-dropdown [p-label]="node.props['label']" [p-actions]="poActions(node, 'actions')"
                       [p-position]="node.props['position'] || 'bottom'"></po-dropdown>
        }
        @case ('popup') {
          @if (node.props['open']) {
            <po-popup [p-target]="poTarget(node)" [p-actions]="poActions(node, 'actions')"
                      (p-close)="post(node.id, 'close', null)"></po-popup>
          }
        }
        @case ('popover') {
          @if (node.props['open'] !== false) {
            <po-popover [p-target]="poTarget(node)" [p-title]="node.props['title']"
                        [p-trigger]="node.props['trigger'] || 'click'" [p-position]="node.props['position'] || 'bottom'"
                        (p-close)="post(node.id, 'close', null)">
              <ng-container *ngTemplateOutlet="filhos; context: { $implicit: node }"></ng-container>
            </po-popover>
          }
        }
        @case ('contextmenu') {
          <po-context-menu [p-title]="node.props['title']" [p-items]="poActions(node, 'items')"
                           (p-item-selected)="post(node.id, 'itemselected', $event)"></po-context-menu>
        }
        @case ('toolbar') {
          <po-toolbar [p-title]="node.props['title']" [p-actions]="poActions(node, 'actions')"
                      [p-profile]="node.props['profile']" [p-notification-number]="node.props['notificationNumber'] || 0"
                      [p-show-notification]="true"></po-toolbar>
        }
        @case ('navbar') {
          <po-navbar [p-items]="chromeItems(node, 'items')" [p-logo]="node.props['logo'] || ''"
                     [p-icon-actions]="chromeItems(node, 'iconActions')"></po-navbar>
        }
        @case ('header') {
          <po-header [p-header-user]="node.props['user']" [p-actions-tools]="chromeItems(node, 'actions')"></po-header>
        }
        @case ('menupanel') {
          <po-menu-panel [p-menus]="chromeItems(node, 'items')" [p-logo]="node.props['logo'] || ''"></po-menu-panel>
        }
        @case ('avatar') {
          <po-avatar [p-src]="node.props['src'] || ''" [p-size]="node.props['size'] || 'medium'"
                     (p-click)="post(node.id, 'click', null)"></po-avatar>
        }
        @case ('badge') {
          <po-badge [p-value]="node.props['value'] || 0" [p-color]="node.props['color'] || ''"
                    [p-status]="node.props['status'] || ''"></po-badge>
        }
        @case ('image') {
          <po-image [p-src]="poAsset(node.props['src'])" [p-alt]="node.props['alt'] || ''"
                    [p-height]="node.props['height'] || undefined"></po-image>
        }
        @case ('skeleton') {
          <po-skeleton [p-type]="node.props['type'] || 'primary'" [p-variant]="node.props['variant'] || 'rect'"
                       [p-size]="node.props['size'] || 'medium'"></po-skeleton>
        }
        @case ('helper') {
          <po-helper [p-helper]="helperOpts(node)"></po-helper>
        }
        @case ('calendar') {
          <po-calendar [p-mode]="node.props['mode'] || 'single'" [ngModel]="node.props['value'] || ''"
                       (ngModelChange)="post(node.id, 'change', $event)"></po-calendar>
        }
        @case ('dynamicview') {
          <po-dynamic-view [p-fields]="node.props['fields'] || []" [p-value]="node.props['value'] || {}"
                           [p-load]="node.props['load'] || ''"></po-dynamic-view>
        }
        @case ('disclaimergroup') {
          <po-disclaimer-group [p-title]="node.props['title'] || ''"
                               [p-disclaimers]="node.props['disclaimers'] || []"
                               (p-change)="post(node.id, 'change', $event)"
                               (p-remove-all)="post(node.id, 'removeall', null)"></po-disclaimer-group>
        }
        @case ('filterchip') {
          <po-filter-chip [p-label]="node.props['label'] || ''" [p-selected]="!!node.props['selected']"
                          (p-selected-change)="post(node.id, 'change', $event?.selected ?? $event)"></po-filter-chip>
        }
        @case ('pagelogin') {
          <po-page-login [p-product-name]="node.props['product'] || ''" [p-literals]="node.props['literals'] || {}"
                         [p-recovery]="loginRecuperacao(node)" [p-login-errors]="loginErrosLista(node.id)"
                         (p-login-submit)="loginSubmitTpl(node, $event)"></po-page-login>
        }
        @case ('pagechangepassword') {
          <po-page-change-password [p-requirements]="senhaRequisitos(node)"
                                   (p-submit)="senhaSubmitTpl(node, $event)"></po-page-change-password>
          @if (loginErro(node.id)) { <div class="po-login-erro">{{ loginErro(node.id) }}</div> }
        }
        @case ('pageblockeduser') {
          <po-page-blocked-user [p-reason]="motivoBloqueio(node)"
                                [p-contact-email]="node.props['contactEmail'] || ''"></po-page-blocked-user>
        }
        @case ('pageslide') {
          @if (node.props['open']) {
            <po-page-slide [p-title]="node.props['title'] || ''" [p-hide-close]="true"
                           (p-close-page-slide)="post(node.id, 'close', null)">
              <ng-container *ngTemplateOutlet="filhos; context: { $implicit: node }"></ng-container>
            </po-page-slide>
          }
        }
        @case ('pagejobscheduler') {
          <po-page-job-scheduler [p-title]="node.props['title'] || ''" [p-service-api]="node.props['url'] || ''"
                                 [p-parameters]="jobParametros(node)"></po-page-job-scheduler>
        }
        @case ('link') {
          <po-link [p-label]="node.props['label']" [p-url]="node.props['url']"
                   [p-open-new-tab]="!!node.props['openNewTab']"
                   (p-action)="post(node.id, 'click', null)"></po-link>
        }
        @case ('breadcrumb') {
          <po-breadcrumb [p-items]="navItems(node)"></po-breadcrumb>
        }
        @case ('contexttabs') {
          <!-- po-context-tabs estende po-tabs: filhos são po-tab; o botão
               de fechar é interno do PO-UI (CloseTab/OnClose ficam para
               quando o output for exposto). -->
          <po-context-tabs>
            @for (t of node.props['tabs'] || []; track $index) {
              <po-tab [p-label]="t['label']"></po-tab>
            }
          </po-context-tabs>
        }
        @case ('accordion') {
          <!-- po-accordion-item precisa ser filho direto (ContentChildren),
               como po-tab em po-tabs -->
          <po-accordion [p-show-manager-accordion]="!!node.props['showManager']"
                        [p-allow-expand-all-items]="!!node.props['allowExpandAll']">
            @for (it of node.children; track it.id) {
              <po-accordion-item [p-label]="it.props['label']"
                                 (p-expand)="post(it.id, 'expand', null)"
                                 (p-collapse)="post(it.id, 'collapse', null)">
                <ng-container *ngTemplateOutlet="filhos; context: { $implicit: it }"></ng-container>
              </po-accordion-item>
            }
          </po-accordion>
        }
        @case ('accordionitem') {
          <po-accordion-item [p-label]="node.props['label']"
                             (p-expand)="post(node.id, 'expand', null)"
                             (p-collapse)="post(node.id, 'collapse', null)">
            <ng-container *ngTemplateOutlet="filhos; context: { $implicit: node }"></ng-container>
          </po-accordion-item>
        }
        @case ('modal') {
          <!-- O po-modal nasce oculto: só é criado quando o servidor abre,
               e a diretiva chama open() na inicialização. Fechar = remover
               o elemento via @if — sem chamada close() que reemitiria p-close. -->
          @if (node.props['open']) {
            <po-modal appModalOpen
                     [p-title]="node.props['title']" [p-size]="node.props['size']"
                     [p-primary-action]="modalAction(node, 'primaryAction')"
                     [p-secondary-action]="modalAction(node, 'secondaryAction')"
                     (p-close)="post(node.id, 'close', null)">
              <ng-container *ngTemplateOutlet="filhos; context: { $implicit: node }"></ng-container>
            </po-modal>
          }
        }
        @case ('stepper') {
          <!-- po-step precisa ser filho direto (ContentChildren) -->
          <po-stepper [p-step]="stepIndex(node) + 1" [p-orientation]="node.props['orientation'] || 'horizontal'"
                      (p-change-step)="post(node.id, 'changestep', $event)">
            @for (st of node.children; track st.id) {
              <po-step [p-label]="st.props['label']" [p-can-active-next-step]="podeAvancar(st)">
                <ng-container *ngTemplateOutlet="filhos; context: { $implicit: st }"></ng-container>
              </po-step>
            }
          </po-stepper>
        }
        @case ('step') {
          <po-step [p-label]="node.props['label']">
            <ng-container *ngTemplateOutlet="filhos; context: { $implicit: node }"></ng-container>
          </po-step>
        }
        @case ('slide') {
          <po-slide [p-slides]="node.props['slides']" [p-interval]="node.props['interval']"
                    [p-height]="node.props['height']"></po-slide>
        }
      }
    }
  `,
})
export class UiHostComponent {
  @Input({ required: true }) node!: UiNode;
  @Input({ required: true }) post!: UiPost;
  // Só existe para o pai forçar nova verificação quando chega um lote.
  @Input() versao = 0;
  @Input() sid = '';

  private actionsFor: unknown = null;
  private actionsCache: PoPageAction[] = [];
  private seriesFor: unknown = null;
  private seriesCache: PoChartSerie[] = [];
  private debounce?: ReturnType<typeof setTimeout>;

  protected temDetalhe(): boolean {
    return (this.node.props['items'] ?? []).some((it: { detail?: string }) => !!it.detail);
  }

  protected temCols(pai: UiNode): boolean {
    return pai.children.some(c => c.props['cols']);
  }

  protected largura(c: UiNode): string {
    return ((c.props['cols'] || 12) / 12 * 100).toFixed(4) + '%';
  }

  // Recria as ações só quando a lista do servidor muda (mesma referência =
  // mesmo array para o po-page-default, sem re-render a cada ciclo).
  protected pageActions(): PoPageAction[] {
    const acts = this.node.props['actions'];
    if (acts !== this.actionsFor) {
      this.actionsFor = acts;
      this.actionsCache = (acts ?? []).map((a: { label: string; index: number }) => ({
        label: a.label,
        action: () => this.post(this.node.id, 'action', a.index),
      }));
    }
    return this.actionsCache;
  }

  // Pizza/rosca: uma fatia por categoria, com o 1º valor de cada uma.
  protected chartSeries(): PoChartSerie[] {
    const series = this.node.props['series'];
    if (series !== this.seriesFor) {
      this.seriesFor = series;
      const tipo = this.node.props['type'];
      if (tipo === 'pie' || tipo === 'donut') {
        const cats: string[] = this.node.props['categories'] ?? [];
        const dados: number[] = series?.[0]?.data ?? [];
        this.seriesCache = cats.map((c, i) => ({ label: String(c), data: dados[i] ?? 0 }));
      } else {
        this.seriesCache = (series ?? []).map((s: { label: string; data: number[] }) => ({ label: s.label, data: s.data }));
      }
    }
    return this.seriesCache;
  }

  // Digitação: guarda localmente e manda um "change" 300 ms após a última tecla.
  protected onInput(value: unknown) {
    this.node.props['value'] = value;
    clearTimeout(this.debounce);
    this.debounce = adiaEnvio(this, 300, () => this.post(this.node.id, 'change', value));
  }

  // Seleção/marcação: manda na hora.
  protected onImediato(value: unknown) {
    this.node.props['value'] = value;
    this.post(this.node.id, 'change', value);
  }

  // PoPageList:p-filter é um PoPageFilter {placeholder, action,
  // advancedAction} — as ações chamam o AdvPL (OnFilter/OnAdvancedSearch).
  protected pageFilter(): any {
    if (this.node.type !== 'pagelist' || !this.node.props['filterPlaceholder']) return undefined;
    return {
      placeholder: this.node.props['filterPlaceholder'],
      action: (texto: string) => this.post(this.node.id, 'filter', texto),
      advancedAction: () => this.post(this.node.id, 'advancedsearch', null),
    };
  }

  // PoUpload:p-success traz a resposta do POST /poui-upload
  // ([{cNome,cCaminhoGravado,nBytes}]) — vira o evento "upload".
  protected onUploadOk(value: unknown) {
    this.post(this.node.id, 'upload', value);
  }

  // PoSearch:p-filter emite o texto digitado — OnFilter(cTexto) no AdvPL.
  protected onFilter(value: unknown) {
    this.post(this.node.id, 'filter', value);
  }

  protected onSelect(row: object) {
    const i = (this.node.props['items'] as object[]).indexOf(row);
    if (i >= 0) {
      this.post(this.node.id, 'select', i);
    }
  }

  protected onTree(item: PoTreeViewItem) {
    this.post(this.node.id, 'select', item.value);
  }

  // "AAAA-MM-DD" -> Date local (meia-noite); vazio/inválido -> null.
  // Retorna any: p-min-date/p-max-date do PO-UI são string|Date e null
  // não é atribuível — o componente trata null como "sem limite".
  // Mesma string -> mesmo objeto Date: um Date novo a cada ciclo fazia o
  // ngModel reescrever o po-datepicker, que disparava outro ciclo — laço
  // infinito com a CPU do navegador em 100%.
  protected isoDate(s: unknown): any {
    if (typeof s !== 'string' || s.length < 10) return null;
    return datas.get('d:' + s) ?? guardaData('d:' + s, () => {
      const [y, m, d] = s.slice(0, 10).split('-').map(Number);
      return y && m && d ? new Date(y, m - 1, d) : null;
    });
  }

  // ISO 8601 -> Date; vazio/inválido -> null (mesmo motivo do isoDate).
  protected isoDateTime(s: unknown): any {
    if (typeof s !== 'string' || !s) return null;
    return datas.get('t:' + s) ?? guardaData('t:' + s, () => {
      const d = new Date(s);
      return isNaN(d.getTime()) ? null : d;
    });
  }

  // PoLookup:p-selected devolve a linha inteira vinda do REST — vai como
  // objeto JSON para o handler OnSelected(oLinha) do AdvPL.
  protected onLookupSelected(row: unknown) {
    this.post(this.node.id, 'selected', row);
  }

  // ---- Onda 3a.6: templates reais do PO-UI (@po-ui/ng-templates) ----
  // po-page-dynamic-table/edit/detail/search falam direto com o REST do
  // AdvPP (contrato {items, hasNext} da onda 1). Os hooks "before" usam a
  // URL reservada de dyn-hooks.ts, respondida pelo round-trip ao AdvPL.

  private http = inject(HttpClient);

  // Referências estáveis: o PO-UI reexecuta os setters a cada entrada nova,
  // então cada valor só é recriado quando a prop de origem muda.
  private memo = new Map<string, { k: string; v: unknown }>();
  private estavel<T>(chave: string, base: unknown, f: () => T): T {
    const k = JSON.stringify(base ?? null);
    const m = this.memo.get(chave);
    if (m && m.k === k) {
      return m.v as T;
    }
    const v = f();
    this.memo.set(chave, { k, v });
    return v;
  }

  protected dynUrl(node: UiNode): string {
    const u = String(node.props['url'] ?? '');
    return u.startsWith('/') ? u : '/' + u;
  }

  // aFields do AdvPL já vêm no formato do PO-UI ({property, label, type,
  // key, filter, visible}).
  protected dynCamposTpl(node: UiNode): any[] {
    return this.estavel(`campos:${node.id}`, node.props['fields'], () => copiaLista(node.props['fields']));
  }

  protected dynFiltrosTpl(node: UiNode): any[] {
    return this.estavel(`filtros:${node.id}`, node.props['filters'], () => copiaLista(node.props['filters']));
  }

  // SetActions do AdvPL ({{"edit","Editar"}, ...}): new/detail/edit/
  // duplicate chamam OnCustomAction(oSelf, cAcao, oLinha); remove/removeAll
  // ficam com o PO-UI (DELETE no serviço, depois do OnBeforeRemove). Chaves
  // que o PO-UI não conhece viram ações de linha (p-table-custom-actions).
  protected dynAcoesTabela(node: UiNode): Record<string, unknown> {
    return this.estavel(`acoesT:${node.id}`, node.props['actions'], () => {
      const out: Record<string, unknown> = {};
      for (const k of Object.keys(acoesDe(node))) {
        if (k === 'remove' || k === 'removeall') {
          out[k === 'remove' ? 'remove' : 'removeAll'] = true;
          out['beforeRemove'] = antesUrl(node.id, 'beforeremove');
        } else if (ACOES_PADRAO.has(k)) {
          out[k] = (...args: unknown[]) => this.post(node.id, 'customaction', { acao: k, linha: linhaDe(args) });
        }
      }
      return out;
    });
  }

  protected dynAcoesCustom(node: UiNode): any[] {
    return this.estavel(`acoesC:${node.id}`, node.props['actions'], () =>
      Object.entries(acoesDe(node))
        .filter(([k]) => !ACOES_PADRAO.has(k) && k !== 'remove' && k !== 'removeall')
        .map(([k, label]) => ({
          label: String(label || k),
          action: (linha: unknown) => this.post(node.id, 'customaction', { acao: k, linha }),
        })));
  }

  protected dynAcoesEdicao(node: UiNode): Record<string, unknown> {
    return this.estavel(`acoesE:${node.id}`, node.id, () => ({
      beforeSave: antesUrl(node.id, 'beforesave'),
      beforeSaveNew: antesUrl(node.id, 'beforesave'),
      cancel: () => this.post(node.id, 'cancel', null),
    }));
  }

  protected dynAcoesDetalhe(node: UiNode): Record<string, unknown> {
    return this.estavel(`acoesD:${node.id}`, node.id, () => ({
      back: () => this.post(node.id, 'back', null),
      beforeRemove: antesUrl(node.id, 'beforeremove'),
      remove: (id: unknown) => {
        if (id === undefined || id === null || id === '') {
          return;
        }
        this.http.delete(this.dynUrl(node) + '/' + encodeURIComponent(String(id)))
          .subscribe({ next: () => this.post(node.id, 'back', null), error: () => {} });
      },
    }));
  }

  // ---- Onda 3b.1: agrupamento ----

  // po-stepper é 1-based no PO-UI; o AdvPL guarda 0-based.
  protected stepIndex(node: UiNode): number {
    const s = node.props['step'];
    return typeof s === 'number' ? s : 0;
  }

  // PoStep:OnCanActivate() -> lOk: o po-step aceita Observable<boolean>;
  // a resposta vem pelo round-trip síncrono (sem handler, a VM libera).
  protected podeAvancar(st: UiNode): any {
    return this.estavel(`can:${st.id}`, st.id, () => () => from(dynAntes(this.post, st.id, 'canactivate', null)));
  }

  // PoDynamicForm: o objeto do p-value é o modelo (o PO-UI grava nele o que
  // é digitado); só é recriado quando o AdvPL troca o valor (SetValue).
  protected formValor(node: UiNode): any {
    return this.estavel(`fv:${node.id}`, node.props['value'], () => ({ ...(node.props['value'] ?? {}) }));
  }

  // p-validate recebe {property, value: valores do formulário}: o campo
  // alterado vai ao AdvPL (OnChange(oForm, cCampo, xValor)).
  protected formValida(node: UiNode): any {
    return this.estavel(`fval:${node.id}`, node.id, () => (mudou: { property?: string; value?: Record<string, unknown> }) => {
      const p = mudou?.property ?? '';
      if (p) {
        this.post(node.id, 'fieldchange', { property: p, value: mudou?.value?.[p] ?? null });
      }
      return {};
    });
  }

  // ---- Onda 3b.5: páginas especiais (templates reais do PO-UI) ----

  // Erros de login/troca de senha por componente (limpos a cada submit).
  protected loginErros = signal<Record<number, string>>({});
  protected loginErro(id: number): string { return this.loginErros()[id] ?? ''; }

  protected loginErrosLista(id: number): string[] {
    const e = this.loginErro(id);
    return this.estavel(`lerr:${id}`, e, () => (e ? [e] : []));
  }

  // Credenciais vão só no payload do evento (round-trip síncrono): nunca
  // em props, console ou Get().
  private async credSubmit(node: UiNode, ev: 'login' | 'submit', data: { user?: string; pass?: string; new?: string }) {
    const ok = await dynAntes(this.post, node.id, ev, data);
    this.loginErros.update(m => ({ ...m, [node.id]: ok ? '' : 'Falha na autenticação' }));
  }

  protected loginSubmitTpl(node: UiNode, ev: { login?: string; password?: string }) {
    void this.credSubmit(node, 'login', { user: ev?.login ?? '', pass: ev?.password ?? '' });
  }

  protected senhaSubmitTpl(node: UiNode, ev: { currentPassword?: string; newPassword?: string }) {
    void this.credSubmit(node, 'submit', { pass: ev?.currentPassword ?? '', new: ev?.newPassword ?? '' });
  }

  // SetRecovery(.T.): o link "esqueci a senha" chama OnRecovery no AdvPL.
  protected loginRecuperacao(node: UiNode): any {
    if (!node.props['recovery']) {
      return undefined;
    }
    return this.estavel(`rec:${node.id}`, node.id, () => () => this.post(node.id, 'recovery', null));
  }

  // Requisitos em texto: o AdvPP não avalia regras de senha no navegador;
  // status true mantém o envio habilitado (a validação é do OnSubmit).
  protected senhaRequisitos(node: UiNode): any[] {
    return this.estavel(`req:${node.id}`, node.props['requirements'], () =>
      listaTexto(node.props['requirements']).map(requirement => ({ requirement, status: true })));
  }

  // Motivo livre do AdvPL -> PoPageBlockedUserReason.
  protected motivoBloqueio(node: UiNode): any {
    const r = String(node.props['reason'] ?? '').toLowerCase();
    if (/tentativa|attempt/.test(r)) {
      return 'exceededAttempts';
    }
    if (/expir/.test(r)) {
      return 'expiredPassword';
    }
    return 'none';
  }

  protected jobParametros(node: UiNode): any[] {
    return this.estavel(`job:${node.id}`, node.props['parameters'], () =>
      listaTexto(node.props['parameters']).map(p => ({ property: p, label: p })));
  }

  // ---- Onda 3b.4: exibição ----

  // Src relativa de imagem -> /poui-asset (resolvida no servidor contra o
  // diretório do fonte, mesma checagem de caminho do upload).
  protected poAsset(src: unknown): string {
    const s = String(src ?? '');
    if (!s || /^https?:|^data:|^blob:|^\//.test(s)) { return s; }
    return '/poui-asset?sid=' + encodeURIComponent(this.sid) + '&p=' + encodeURIComponent(s);
  }

  // po-helper: string ou {title, content, type} (PoHelperOptions).
  // Referência estável por nó (ver estavel): só recria quando a prop muda.
  protected helperOpts(node: UiNode): any {
    return this.estavel(`ho:${node.id}`, [node.props['helper'], node.props['title'], node.props['type']], () => this.helperOptsNovo(node));
  }

  private helperOptsNovo(node: UiNode): any {
    const content = String(node.props['helper'] ?? '');
    const title = String(node.props['title'] ?? '');
    const type = String(node.props['type'] ?? '');
    if (!title && !type) { return content; }
    const o: any = { content };
    if (title) { o.title = title; }
    if (type) { o.type = type; }
    return o;
  }

  // ---- Onda 3b.3: chrome do app ----

  // Itens do chrome (navbar/header/menupanel): {label, icon?, action:"act:N"}
  // -> {label, icon?, action: () => post} — mesmo contrato do po-menu.
  // Referência estável por nó (ver estavel): só recria quando a prop muda.
  protected chromeItems(node: UiNode, prop: string): any[] {
    return this.estavel(`ci:${node.id}:${prop}`, node.props[prop], () => this.chromeItemsNovo(node, prop));
  }

  private chromeItemsNovo(node: UiNode, prop: string): any[] {
    const lst = (node.props[prop] ?? []) as { label?: string; icon?: string; action?: string }[];
    return lst.map((a): any => ({
      label: a.label ?? '',
      icon: a.icon || undefined,
      shortLabel: a.label ?? '',
      action: a.action ? () => this.post(node.id, a.action!, null) : undefined,
    }));
  }

  // ---- Onda 3b.3: navegação ----

  // Itens do breadcrumb: {label, link} passa direto; {label, page:id}
  // vira action que posta "navigate" para o AdvPL resolver a página.
  // Referência estável por nó (ver estavel): só recria quando a prop muda.
  protected navItems(node: UiNode): any[] {
    return this.estavel(`ni:${node.id}`, node.props['items'], () => this.navItemsNovo(node));
  }

  private navItemsNovo(node: UiNode): any[] {
    const lst = (node.props['items'] ?? []) as Record<string, unknown>[];
    return lst.map(it => {
      if (it['page'] != null) {
        const pg = it['page'];
        const lb = it['label'];
        return { label: lb, action: () => this.post(node.id, 'navigate', { page: pg, label: lb }) };
      }
      return { label: it['label'], link: it['link'] };
    });
  }

  // ---- Onda 3b.2: ações ----

  // {label, icon?, action:"act:N"} -> {label, icon?, action: () => post}.
  // Referência estável por nó (ver estavel): só recria quando a prop muda.
  protected poActions(node: UiNode, prop: string): any[] {
    return this.estavel(`pa:${node.id}:${prop}`, node.props[prop], () => this.poActionsNovo(node, prop));
  }

  private poActionsNovo(node: UiNode, prop: string): any[] {
    const lst = (node.props[prop] ?? []) as { label?: string; icon?: string; action?: string }[];
    return lst.map((a): any => ({
      label: a.label ?? '',
      icon: a.icon,
      action: a.action ? () => this.post(node.id, a.action!, null) : undefined,
    }));
  }

  // Alvo de PoPopup/PoPopover: elemento adv-ui-host do componente alvo
  // (o servidor guarda o id do nó em props.target).
  protected poTarget(node: UiNode): any {
    const id = node.props['target'];
    if (id == null) return undefined;
    return document.querySelector(`adv-ui-host[data-poid="${id}"]`);
  }

  // p-primary-action/p-secondary-action do po-modal esperam {label, action}.
  // O backend guarda {label, action:"click:primaryaction"} — aqui viram
  // função que posta o evento de volta ao AdvPL.
  // Referência estável por nó (ver estavel): só recria quando a prop muda.
  protected modalAction(node: UiNode, key: string): { label: string; action: () => void } | undefined {
    return this.estavel(`ma:${node.id}:${key}`, node.props[key], () => this.modalActionNovo(node, key));
  }

  private modalActionNovo(node: UiNode, key: string): { label: string; action: () => void } | undefined {
    const a = node.props[key] as { label?: string; action?: string } | undefined;
    if (!a?.action) return undefined;
    const ev = a.action;
    return { label: a.label ?? '', action: () => this.post(node.id, ev, null) };
  }

  // ---- Onda 3c.3: drag & drop entre filhos de um contêiner ----
  // HTML5 DnD nativo: o servidor reordena c.children e dispara OnDrop
  // (oArrastado, nPos); aqui a ordem visual muda na hora.

  private dragNode: UiNode | null = null;

  protected onDragStart(ev: DragEvent, c: UiNode) {
    if (!c.props['draggable']) { ev.preventDefault(); return; }
    this.dragNode = c;
    ev.dataTransfer?.setData('text/plain', String(c.id));
    if (ev.dataTransfer) ev.dataTransfer.effectAllowed = 'move';
  }

  protected onDragOver(ev: DragEvent) {
    ev.preventDefault(); // sem isto o drop não dispara
    if (ev.dataTransfer) ev.dataTransfer.dropEffect = 'move';
  }

  // Posição de inserção no array de filhos SEM o arrastado (mesma regra
  // do poReorder no servidor): índice do filho sob o cursor, ou o fim.
  private posicaoDrop(pai: UiNode, alvo: UiNode | null): number | null {
    const drag = this.dragNode;
    if (!drag || !pai.children.includes(drag)) return null;
    const resto = pai.children.filter(c => c !== drag);
    if (!alvo) return resto.length;
    const i = resto.indexOf(alvo);
    return i < 0 ? resto.length : i;
  }

  private aplicarDrop(pai: UiNode, alvo: UiNode | null) {
    const pos = this.posicaoDrop(pai, alvo);
    const drag = this.dragNode;
    if (pos === null || !drag) return;
    const resto = pai.children.filter(c => c !== drag);
    resto.splice(pos, 0, drag);
    pai.children = resto;
    this.post(pai.id, 'drop', { drag: drag.id, pos });
  }

  protected onDrop(ev: DragEvent, pai: UiNode, alvo: UiNode) {
    ev.preventDefault();
    if (!this.dragNode) {
      const id = Number(ev.dataTransfer?.getData('text/plain'));
      this.dragNode = pai.children.find(c => c.id === id) ?? null;
    }
    this.aplicarDrop(pai, alvo);
    this.dragNode = null;
  }

  // Drop na zona extra após o último filho = posição final.
  protected onDropZone(ev: DragEvent, pai: UiNode) {
    ev.preventDefault();
    if (!this.dragNode) return;
    this.aplicarDrop(pai, null);
    this.dragNode = null;
  }
}
