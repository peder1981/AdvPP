import { Component, Input, forwardRef } from '@angular/core';
import { FormsModule } from '@angular/forms';
import {
  PoButtonModule, PoFieldModule, PoPageAction, PoPageModule, PoTableModule, PoTagModule,
} from '@po-ui/ng-components';

// Espelho do protocolo de componentes (pkg/vm/poui.go, op "mount"/"batch").
export interface UiNode {
  id: number;
  type: string; // page | button | input | table | tag
  props: Record<string, any>;
  children: UiNode[];
}

export type UiPost = (id: number, event: string, value: unknown) => void;

// Renderiza um nó da árvore e, recursivamente, os filhos. Genérico: não
// conhece nenhum aplicativo, só o mapeamento tipo -> componente PO-UI.
@Component({
  selector: 'adv-ui-host',
  imports: [
    forwardRef(() => UiHostComponent), FormsModule,
    PoPageModule, PoButtonModule, PoFieldModule, PoTableModule, PoTagModule,
  ],
  template: `
    @if (node.props['visible'] !== false) {
      @switch (node.type) {
        @case ('page') {
          <po-page-default [p-title]="node.props['title']" [p-actions]="pageActions()">
            @for (c of node.children; track c.id) {
              <div class="advpp-ui-item"><adv-ui-host [node]="c" [post]="post"></adv-ui-host></div>
            }
          </po-page-default>
        }
        @case ('button') {
          <po-button [p-label]="node.props['label']" [p-disabled]="!!node.props['disabled']"
                     (p-click)="post(node.id, 'click', null)"></po-button>
        }
        @case ('input') {
          <po-input [p-label]="node.props['label']" [p-disabled]="node.props['disabled'] ? 'true' : 'false'"
                    [ngModel]="node.props['value']" (ngModelChange)="onInput($event)"></po-input>
        }
        @case ('table') {
          <po-table [p-columns]="node.props['columns']" [p-items]="node.props['items']"
                    [p-selectable]="true" [p-single-select]="true" [p-striped]="true"
                    (p-selected)="onSelect($event)"></po-table>
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

  private actionsFor: unknown = null;
  private actionsCache: PoPageAction[] = [];
  private debounce?: ReturnType<typeof setTimeout>;

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

  // Digitação: guarda localmente e manda um "change" 300 ms após a última tecla.
  protected onInput(value: string) {
    this.node.props['value'] = value;
    clearTimeout(this.debounce);
    this.debounce = setTimeout(() => this.post(this.node.id, 'change', value), 300);
  }

  protected onSelect(row: object) {
    const i = (this.node.props['items'] as object[]).indexOf(row);
    if (i >= 0) {
      this.post(this.node.id, 'select', i);
    }
  }
}
