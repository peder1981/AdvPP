import { AfterViewInit, Component, ElementRef, Input, OnDestroy, ViewChild } from '@angular/core';
import { FitAddon } from '@xterm/addon-fit';
import { Terminal } from '@xterm/xterm';
import { UiNode, UiPost } from './ui-host';
import { registraTerm, removeTerm } from './adv-terminal-canal';


// AdvTerminal: xterm.js. Digitação vai como "input"; mudança de tamanho
// (ajuste ao contêiner) como "resize".
@Component({
  selector: 'adv-terminal',
  template: `<div #host class="advpp-terminal" [style.height]="node.props['height']"></div>`,
  styles: [`.advpp-terminal { background: #1e1e1e; padding: 4px; border-radius: 4px; }`],
})
export class AdvTerminalComponent implements AfterViewInit, OnDestroy {
  @Input({ required: true }) node!: UiNode;
  @Input({ required: true }) post!: UiPost;
  @ViewChild('host') host!: ElementRef<HTMLDivElement>;

  private term?: Terminal;
  private obs?: ResizeObserver;

  ngAfterViewInit() {
    const term = new Terminal({ cursorBlink: true, fontSize: 14, fontFamily: 'monospace', convertEol: false });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(this.host.nativeElement);
    this.term = term;
    term.onData(d => this.post(this.node.id, 'input', d));
    term.onResize(({ cols, rows }) => this.post(this.node.id, 'resize', { cols, rows }));
    fit.fit();
    this.post(this.node.id, 'resize', { cols: term.cols, rows: term.rows });
    this.obs = new ResizeObserver(() => fit.fit());
    this.obs.observe(this.host.nativeElement);
    registraTerm(this.node.id, d => term.write(d));
    term.focus();
  }

  ngOnDestroy() {
    removeTerm(this.node.id);
    this.obs?.disconnect();
    this.term?.dispose();
  }
}
