import { AfterViewInit, Component, DoCheck, ElementRef, Input, OnDestroy, ViewChild } from '@angular/core';
import { json } from '@codemirror/lang-json';
import { StreamLanguage } from '@codemirror/language';
import { shell } from '@codemirror/legacy-modes/mode/shell';
import { Diagnostic, lintGutter, setDiagnostics } from '@codemirror/lint';
import { Compartment, EditorState, Extension, Prec } from '@codemirror/state';
import { EditorView, keymap } from '@codemirror/view';
import { basicSetup } from 'codemirror';
import { advpl } from './advpl-lang';
import { UiNode, UiPost } from './ui-host';

interface Marcador { line: number; col: number; message: string; severity: 'error' | 'warning' | 'info' }

function linguagem(l: string): Extension {
  switch (l) {
    case 'advpl': case 'tlpp': return advpl;
    case 'json': return json();
    case 'sh': return StreamLanguage.define(shell);
    default: return [];
  }
}

// AdvCodeEditor: CodeMirror 6. Ctrl+S manda "save" com o texto; digitar
// manda "change" 500 ms após a última tecla. Marcações vêm do servidor.
@Component({
  selector: 'adv-code-editor',
  template: `<div #host class="advpp-editor" [style.height]="node.props['height']"></div>`,
  styles: [`
    .advpp-editor { border: 1px solid #c4c9cc; border-radius: 4px; overflow: hidden; }
    .advpp-editor ::ng-deep .cm-editor { height: 100%; font-size: 14px; }
  `],
})
export class AdvCodeEditorComponent implements AfterViewInit, DoCheck, OnDestroy {
  @Input({ required: true }) node!: UiNode;
  @Input({ required: true }) post!: UiPost;
  @ViewChild('host') host!: ElementRef<HTMLDivElement>;

  private view?: EditorView;
  private lang = new Compartment();
  private ro = new Compartment();
  private visto: Record<string, unknown> = {};
  private aplicando = false;
  private debounce?: ReturnType<typeof setTimeout>;

  ngAfterViewInit() {
    const p = this.node.props;
    this.view = new EditorView({
      parent: this.host.nativeElement,
      state: EditorState.create({
        doc: String(p['value'] ?? ''),
        extensions: [
          Prec.highest(keymap.of([{ key: 'Mod-s', preventDefault: true, run: () => { this.salvar(); return true; } }])),
          basicSetup,
          lintGutter(),
          this.lang.of(linguagem(p['language'])),
          this.ro.of(EditorState.readOnly.of(!!p['readonly'])),
          EditorView.updateListener.of(u => { if (u.docChanged && !this.aplicando) { this.mudou(); } }),
        ],
      }),
    });
    this.visto = { value: p['value'], language: p['language'], readonly: p['readonly'] };
    this.aplicaMarcadores(p['markers']);
  }

  // Patches do servidor: aplica só o que mudou de referência.
  ngDoCheck() {
    const v = this.view;
    if (!v) { return; }
    const p = this.node.props;
    if (p['value'] !== this.visto['value']) {
      this.visto['value'] = p['value'];
      const novo = String(p['value'] ?? '');
      if (novo !== v.state.doc.toString()) {
        this.aplicando = true;
        v.dispatch({ changes: { from: 0, to: v.state.doc.length, insert: novo } });
        this.aplicando = false;
      }
    }
    if (p['language'] !== this.visto['language']) {
      this.visto['language'] = p['language'];
      v.dispatch({ effects: this.lang.reconfigure(linguagem(p['language'])) });
    }
    if (p['readonly'] !== this.visto['readonly']) {
      this.visto['readonly'] = p['readonly'];
      v.dispatch({ effects: this.ro.reconfigure(EditorState.readOnly.of(!!p['readonly'])) });
    }
    if (p['markers'] !== this.visto['markers']) {
      this.aplicaMarcadores(p['markers']);
    }
  }

  private aplicaMarcadores(ms: Marcador[] | undefined) {
    const v = this.view!;
    this.visto['markers'] = ms;
    const doc = v.state.doc;
    const diags: Diagnostic[] = (ms ?? []).map(m => {
      const linha = doc.line(Math.min(Math.max(1, m.line || 1), doc.lines));
      const from = Math.min(linha.from + Math.max(0, (m.col || 1) - 1), linha.to);
      return { from, to: Math.max(from, linha.to), severity: m.severity || 'error', message: m.message };
    });
    v.dispatch(setDiagnostics(v.state, diags));
  }

  private salvar() {
    const t = this.view!.state.doc.toString();
    clearTimeout(this.debounce);
    this.node.props['value'] = t;
    this.visto['value'] = t;
    this.post(this.node.id, 'save', t);
  }

  private mudou() {
    clearTimeout(this.debounce);
    this.debounce = setTimeout(() => {
      const t = this.view!.state.doc.toString();
      this.node.props['value'] = t;
      this.visto['value'] = t;
      this.post(this.node.id, 'change', t);
    }, 500);
  }

  ngOnDestroy() {
    clearTimeout(this.debounce);
    this.view?.destroy();
  }
}
