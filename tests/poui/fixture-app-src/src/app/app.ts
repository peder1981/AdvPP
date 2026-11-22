import { Component, OnInit, inject, signal } from '@angular/core';
import { PoButtonModule, PoTableColumn, PoTableModule } from '@po-ui/ng-components';
import { ProAppConfigService, ProJsToAdvplService } from '@totvs/protheus-lib-core';

@Component({
  selector: 'app-root',
  imports: [PoTableModule, PoButtonModule],
  template: `
    <div class="po-wrapper">
      <h1>Fixture App — Armazéns</h1>
      @if (apiUrl()) {
        <po-table
          [p-service-api]="apiUrl()"
          [p-columns]="columns"
          p-container="light"
          [p-hide-select-all]="true"
        ></po-table>
      }
      <div class="po-row">
        <po-button p-label="Pingar AdvPL" (p-click)="ping()"></po-button>
        <po-button p-label="Fechar" (p-click)="fechar()"></po-button>
      </div>
      @if (pong()) {
        <p id="pong">{{ pong() }}</p>
      }
    </div>
  `
})
export class App implements OnInit {
  private readonly advpl = inject(ProJsToAdvplService);
  private readonly appConfig = inject(ProAppConfigService);

  readonly apiUrl = signal('');
  readonly pong = signal('');
  readonly columns: PoTableColumn[] = [
    { property: 'codigo', label: 'Código', width: '100px' },
    { property: 'descri', label: 'Descrição' },
    { property: 'estoque', label: 'Estoque', width: '100px', type: 'number' }
  ];

  async ngOnInit(): Promise<void> {
    await this.appConfig.loadAppConfig();
    const base = this.appConfig.proAppConfig?.api_baseUrl ?? '';
    this.apiUrl.set(base + '/api/est/v1/armazens');
  }

  ping(): void {
    this.advpl
      .buildObservable<{ protheusResponse?: string }>(
        (res: { protheusResponse?: string }) => this.pong.set(res.protheusResponse ?? ''),
        { sendInfo: { type: 'ping' }, autoDestruct: true }
      )
      .subscribe();
  }

  fechar(): void {
    this.appConfig.callAppClose(false);
  }
}
