import { Component, Input, OnChanges, inject } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { BehaviorSubject } from 'rxjs';

// Rota para os templates dinâmicos do PO-UI (po-page-dynamic-detail/edit),
// que leem o id do registro de ActivatedRoute (snapshot.params['id']). A SPA
// do AdvPP não tem rotas próprias: o AdvPL informa o id com SetId, e o
// template colocado dentro de <adv-rota> enxerga esta rota.
@Component({
  selector: 'adv-rota',
  template: '<ng-content></ng-content>',
  providers: [{ provide: ActivatedRoute, useFactory: novaRota }],
})
export class AdvRotaComponent implements OnChanges {
  @Input() id: string | null = null;
  private rota = inject(ActivatedRoute, { self: true }) as unknown as RotaSimulada;

  // Roda antes do ngOnInit do template projetado, que lê o snapshot.
  ngOnChanges() {
    const params: Record<string, string> = this.id ? { id: this.id } : {};
    this.rota.snapshot.params = params;
    this.rota.params.next(params);
  }
}

interface RotaSimulada {
  snapshot: { params: Record<string, string>; data: Record<string, unknown>; queryParams: Record<string, string> };
  params: BehaviorSubject<Record<string, string>>;
}

// Só os campos que @po-ui/ng-templates 21 lê da rota.
function novaRota(): unknown {
  return {
    snapshot: { params: {}, data: {}, queryParams: {} },
    params: new BehaviorSubject<Record<string, string>>({}),
    queryParams: new BehaviorSubject<Record<string, string>>({}),
    data: new BehaviorSubject<Record<string, unknown>>({}),
  };
}
