import { Component, Directive, Input, inject, signal, forwardRef, AfterViewInit } from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';
import { HttpClient } from '@angular/common/http';
import { AdvCodeEditorComponent } from './adv-code-editor';
import { AdvTerminalComponent } from './adv-terminal';
import { FormsModule } from '@angular/forms';
import { dynAntes } from './dyn-hooks';
import {
  PoAccordionModule, PoModalModule, PoModalComponent, PoSlideModule, PoStepperModule,
  PoButtonGroupModule, PoDropdownModule, PoPopupModule, PoPopoverModule, PoContextMenuModule, PoToolbarModule,
  PoButtonModule, PoChartModule, PoChartSerie, PoContainerModule, PoDividerModule, PoDynamicModule, PoDynamicFormField, PoFieldModule, PoSearchModule,
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
@Directive({ selector: 'po-modal[appModalOpen]', standalone: true })
export class AppModalOpen implements AfterViewInit {
  private modal = inject(PoModalComponent);
  ngAfterViewInit() {
    this.modal.open();
  }
}

// Renderiza um nó da árvore e, recursivamente, os filhos. Genérico: não
// conhece nenhum aplicativo, só o mapeamento tipo -> componente PO-UI.
@Component({
  selector: 'adv-ui-host',
  host: { '[attr.data-poid]': 'node.id' },
  imports: [
    forwardRef(() => UiHostComponent), FormsModule, NgTemplateOutlet, AdvCodeEditorComponent, AdvTerminalComponent,
    PoPageModule, PoButtonModule, PoFieldModule, PoSearchModule, PoTableModule, PoTagModule,
    PoContainerModule, PoDividerModule, PoWidgetModule, PoTabsModule, PoTreeViewModule,
    PoListViewModule, PoInfoModule, PoProgressModule, PoGaugeModule, PoChartModule, PoDynamicModule,
    PoAccordionModule, PoModalModule, PoSlideModule, PoStepperModule, AppModalOpen,
  PoButtonGroupModule, PoDropdownModule, PoPopupModule, PoPopoverModule, PoContextMenuModule, PoToolbarModule,
  PoTooltipModule,
  ],
  styles: [`
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
          <adv-code-editor [node]="node" [post]="post"></adv-code-editor>
        }
        @case ('terminal') {
          <adv-terminal [node]="node" [post]="post"></adv-terminal>
        }
        @case ('dyntable') {
          <div class="dyn-page">
            @if (node.props['title']) { <h3 class="dyn-title">{{ node.props['title'] }}</h3> }
            @if (node.props['quickSearchField']) {
              <po-search class="dyn-quick" (p-filter)="dynQuick(node, $event)"></po-search>
            }
            <po-table [p-columns]="dynCols(node)" [p-items]="dynItemsGet(node)"></po-table>
          </div>
        }
        @case ('dynsearch') {
          <div class="dyn-page">
            @if (node.props['title']) { <h3 class="dyn-title">{{ node.props['title'] }}</h3> }
            <po-search class="dyn-quick" (p-filter)="dynSearch(node, $event)"></po-search>
            <po-table [p-columns]="dynCols(node)" [p-items]="dynItemsGet(node)"></po-table>
          </div>
        }
        @case ('dynedit') {
          <div class="dyn-page">
            @if (node.props['title']) { <h3 class="dyn-title">{{ node.props['title'] }}</h3> }
            <po-dynamic-form [p-fields]="dynFormFields(node)" [p-value]="dynVal(node)"
                             (p-form)="dynSetVal(node, $event)"></po-dynamic-form>
            <div class="dyn-actions">
              <po-button p-label="Salvar" p-kind="primary" (p-click)="dynSave(node)"></po-button>
              <po-button p-label="Cancelar" (p-click)="post(node.id, 'cancel', null)"></po-button>
            </div>
          </div>
        }
        @case ('dyndetail') {
          <div class="dyn-page">
            @if (node.props['title']) { <h3 class="dyn-title">{{ node.props['title'] }}</h3> }
            <po-dynamic-form [p-fields]="dynFormFields(node)" [p-value]="dynVal(node)"
                             (p-form)="dynSetVal(node, $event)"></po-dynamic-form>
            <div class="dyn-actions">
              <po-button p-label="Excluir" p-kind="danger" (p-click)="dynRemove(node)"></po-button>
              <po-button p-label="Voltar" (p-click)="post(node.id, 'back', null)"></po-button>
            </div>
          </div>
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
        @case ('accordion') {
          <po-accordion [p-show-manager-accordion]="!!node.props['showManager']"
                        [p-allow-expand-all-items]="!!node.props['allowExpandAll']">
            <ng-container *ngTemplateOutlet="filhos; context: { $implicit: node }"></ng-container>
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
          <po-stepper [p-step]="stepIndex(node) + 1" [p-orientation]="node.props['orientation'] || 'horizontal'"
                      (p-change-step)="post(node.id, 'changestep', $event)">
            <ng-container *ngTemplateOutlet="filhos; context: { $implicit: node }"></ng-container>
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
  protected isoDate(s: unknown): any {
    if (typeof s !== 'string' || s.length < 10) return null;
    const [y, m, d] = s.slice(0, 10).split('-').map(Number);
    if (!y || !m || !d) return null;
    return new Date(y, m - 1, d);
  }

  // ISO 8601 -> Date; vazio/inválido -> null (mesmo motivo do isoDate).
  protected isoDateTime(s: unknown): any {
    if (typeof s !== 'string' || !s) return null;
    const d = new Date(s);
    return isNaN(d.getTime()) ? null : d;
  }

  // PoLookup:p-selected devolve a linha inteira vinda do REST — vai como
  // objeto JSON para o handler OnSelected(oLinha) do AdvPL.
  protected onLookupSelected(row: unknown) {
    this.post(this.node.id, 'selected', row);
  }

  // ---- Onda 3a.6: PoPageDynamic* — CRUD direto no REST (contrato onda 1) ----

  private http = inject(HttpClient);
  private dynTick = signal(0);
  private dynVals = new Map<number, Record<string, unknown>>();
  private dynItemsMap = new Map<number, unknown[]>();
  private dynLoaded = new Set<number>();

  // Campos visíveis do aFields -> colunas do po-table / fields do form.
  private dynCampos(node: UiNode): Record<string, unknown>[] {
    const f = node.props['fields'];
    return Array.isArray(f) ? f.filter((x: Record<string, unknown>) => x['visible'] !== false) : [];
  }

  protected dynCols(node: UiNode): Record<string, unknown>[] {
    this.dynTick();
    return this.dynCampos(node).map(f => ({ property: f['property'], label: f['label'], type: f['type'] }));
  }

  protected dynFormFields(node: UiNode): PoDynamicFormField[] {
    this.dynTick();
    const ro = node.type === 'dyndetail';
    return this.dynCampos(node).map(f => ({ ...f, disabled: ro || !!f['disabled'] })) as PoDynamicFormField[];
  }

  protected dynVal(node: UiNode): Record<string, unknown> {
    this.dynTick();
    return this.dynVals.get(node.id) ?? {};
  }

  protected dynSetVal(node: UiNode, v: unknown) {
    this.dynVals.set(node.id, (v && typeof v === 'object' ? v : {}) as Record<string, unknown>);
  }

  private dynUrl(node: UiNode): string {
    const u = String(node.props['url'] ?? '');
    return u.startsWith('/') ? u : '/' + u;
  }

  // GET <url>?page&pageSize(&filter) -> {items} no po-table.
  protected dynItemsGet(node: UiNode): unknown[] {
    this.dynTick();
    if (!this.dynLoaded.has(node.id)) {
      this.dynLoaded.add(node.id);
      this.dynLoad(node);
    }
    return this.dynItemsMap.get(node.id) ?? [];
  }

  private dynLoad(node: UiNode, filtro = '') {
    const params: Record<string, string> = { page: '1', pageSize: '50' };
    if (filtro) params['filter'] = filtro;
    this.http.get<{ items?: unknown[] }>(this.dynUrl(node), { params }).subscribe({
      next: r => {
        this.dynItemsMap.set(node.id, r?.items ?? []);
        this.dynTick.update(v => v + 1);
      },
      error: () => this.dynItemsMap.set(node.id, []),
    });
  }

  protected dynQuick(node: UiNode, texto: string) {
    this.post(node.id, 'quicksearch', texto);
    this.dynLoad(node, texto);
  }

  protected dynSearch(node: UiNode, texto: string) {
    this.post(node.id, 'advancedsearch', texto);
    this.dynLoad(node, texto);
  }

  // Chave do registro = 1º campo com key=true nos aFields.
  private dynId(node: UiNode, val: Record<string, unknown>): string {
    const key = this.dynCampos(node).find(f => f['key']);
    const k = key ? String(key['property']) : '';
    return k ? String(val[k] ?? '') : '';
  }

  protected async dynSave(node: UiNode) {
    const val = this.dynVal(node);
    if (!(await dynAntes(this.post, node.id, 'beforesave', val))) return;
    const id = this.dynId(node, val);
    const url = this.dynUrl(node) + (id ? '/' + encodeURIComponent(id) : '');
    const req = id
      ? this.http.put(url, val)
      : this.http.post(url, val);
    req.subscribe({ next: () => this.dynLoad(node), error: () => {} });
  }

  protected async dynRemove(node: UiNode) {
    const val = this.dynVal(node);
    if (!(await dynAntes(this.post, node.id, 'beforeremove', val))) return;
    const id = this.dynId(node, val);
    if (id) {
      this.http.delete(this.dynUrl(node) + '/' + encodeURIComponent(id))
        .subscribe({ next: () => { this.dynLoad(node); this.post(node.id, 'back', null); }, error: () => {} });
    }
  }

  // ---- Onda 3b.1: agrupamento ----

  // po-stepper é 1-based no PO-UI; o AdvPL guarda 0-based.
  protected stepIndex(node: UiNode): number {
    const s = node.props['step'];
    return typeof s === 'number' ? s : 0;
  }

  // ---- Onda 3b.2: ações ----

  // {label, icon?, action:"act:N"} -> {label, icon?, action: () => post}.
  protected poActions(node: UiNode, prop: string): any[] {
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
  protected modalAction(node: UiNode, key: string): { label: string; action: () => void } | undefined {
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
