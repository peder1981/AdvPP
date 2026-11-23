import { Component, Input, forwardRef } from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';
import { AdvCodeEditorComponent } from './adv-code-editor';
import { AdvTerminalComponent } from './adv-terminal';
import { FormsModule } from '@angular/forms';
import {
  PoButtonModule, PoChartModule, PoChartSerie, PoContainerModule, PoDividerModule, PoFieldModule,
  PoGaugeModule, PoInfoModule, PoListViewModule, PoPageAction, PoPageModule, PoProgressModule,
  PoTableModule, PoTabsModule, PoTagModule, PoTreeViewItem, PoTreeViewModule, PoWidgetModule,
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

// Renderiza um nó da árvore e, recursivamente, os filhos. Genérico: não
// conhece nenhum aplicativo, só o mapeamento tipo -> componente PO-UI.
@Component({
  selector: 'adv-ui-host',
  imports: [
    forwardRef(() => UiHostComponent), FormsModule, NgTemplateOutlet, AdvCodeEditorComponent, AdvTerminalComponent,
    PoPageModule, PoButtonModule, PoFieldModule, PoTableModule, PoTagModule,
    PoContainerModule, PoDividerModule, PoWidgetModule, PoTabsModule, PoTreeViewModule,
    PoListViewModule, PoInfoModule, PoProgressModule, PoGaugeModule, PoChartModule,
  ],
  styles: [`
    .advpp-ui-grid { display: flex; flex-wrap: wrap; margin: 0 -8px; }
    .advpp-ui-grid > .advpp-ui-item { padding: 0 8px; box-sizing: border-box; }
  `],
  template: `
    <!-- Filhos de page/container/widget/tab: blocos empilhados, ou grade de
         12 colunas quando algum filho usa SetCols. -->
    <ng-template #filhos let-pai>
      @if (temCols(pai)) {
        <div class="advpp-ui-grid">
          @for (c of pai.children; track c.id) {
            <div class="advpp-ui-item" [style.flex]="'0 0 ' + largura(c)" [style.max-width]="largura(c)">
              <adv-ui-host [node]="c" [post]="post"></adv-ui-host>
            </div>
          }
        </div>
      } @else {
        @for (c of pai.children; track c.id) {
          <div class="advpp-ui-item"><adv-ui-host [node]="c" [post]="post"></adv-ui-host></div>
        }
      }
    </ng-template>

    @if (node.props['visible'] !== false) {
      @switch (node.type) {
        @case ('page') {
          <po-page-default [p-title]="node.props['title']" [p-actions]="pageActions()">
            <ng-container *ngTemplateOutlet="filhos; context: { $implicit: node }"></ng-container>
          </po-page-default>
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
        @case ('tag') {
          <po-tag [p-value]="node.props['value']" [p-type]="node.props['type']"></po-tag>
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

  protected onSelect(row: object) {
    const i = (this.node.props['items'] as object[]).indexOf(row);
    if (i >= 0) {
      this.post(this.node.id, 'select', i);
    }
  }

  protected onTree(item: PoTreeViewItem) {
    this.post(this.node.id, 'select', item.value);
  }

  // PoLookup:p-selected devolve a linha inteira vinda do REST — vai como
  // objeto JSON para o handler OnSelected(oLinha) do AdvPL.
  protected onLookupSelected(row: unknown) {
    this.post(this.node.id, 'selected', row);
  }
}
