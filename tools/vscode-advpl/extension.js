const vscode = require('vscode');
const { spawn } = require('child_process');
const path = require('path');
const fs = require('fs');

let output;
let extensionContext;

function binaryName() {
	return process.platform === 'win32' ? 'advplc.exe' : 'advplc';
}

function findCompilerUpward(startDir) {
	let dir = startDir;
	for (let i = 0; i < 25; i++) {
		const candidate = path.join(dir, binaryName());
		if (fs.existsSync(candidate)) return candidate;
		const parent = path.dirname(dir);
		if (parent === dir) break;
		dir = parent;
	}
	return null;
}

// Binário embutido no próprio .vsix (tools/vscode-advpl/bin/<platform>-<arch>/),
// gerado por build-vsix.sh a partir do cross-compile do compilador — a
// extensão funciona sozinha sem exigir instalar o advplc à parte. Chave é
// process.platform+process.arch do Node, não GOOS/GOARCH do Go.
function bundledCompiler(context) {
	const dir = `${process.platform}-${process.arch}`;
	const candidate = path.join(context.extensionPath, 'bin', dir, binaryName());
	return fs.existsSync(candidate) ? candidate : null;
}

function resolveCompiler(fileDir) {
	const configured = vscode.workspace.getConfiguration('advpl').get('compilerPath');
	if (configured) return configured;
	const found = findCompilerUpward(fileDir);
	if (found) return found;
	const bundled = bundledCompiler(extensionContext);
	if (bundled) return bundled;
	return binaryName(); // fall back to PATH resolution
}

function runCompiler(args, cwd) {
	output.clear();
	output.show(true);
	const compiler = resolveCompiler(cwd);
	output.appendLine(`$ ${compiler} ${args.join(' ')}`);
	const proc = spawn(compiler, args, { cwd });

	proc.on('error', err => {
		if (err.code === 'ENOENT') {
			output.appendLine(
				`advplc not found ("${compiler}"). This extension bundles the compiler for ` +
				`linux-x64, linux-arm64, win32-x64 and darwin-arm64 — if you're on a different ` +
				`platform (e.g. Intel Mac), install it separately (curl -fsSL ` +
				`https://raw.githubusercontent.com/peder1981/AdvPP/master/install.sh | sh) or ` +
				`build it with "go build -o advplc ./cmd/advplc" from the AdvPP repo root, then ` +
				`set "advpl.compilerPath" in Settings.`
			);
		} else {
			output.appendLine(`Failed to launch advplc: ${err.message}`);
		}
	});
	proc.stdout.on('data', d => output.append(d.toString()));
	proc.stderr.on('data', d => output.append(d.toString()));
	proc.on('close', code => {
		if (code !== null) output.appendLine(`\n[exit code ${code}]`);
	});
}

function withActiveFile(callback) {
	const editor = vscode.window.activeTextEditor;
	if (!editor || editor.document.languageId !== 'advpl') {
		vscode.window.showWarningMessage('AdvPL: open a .prw/.prg/.tlpp file first.');
		return;
	}
	editor.document.save().then(() => {
		const file = editor.document.uri.fsPath;
		callback(file, path.dirname(file));
	});
}

function registerCommand(context, name, buildArgs) {
	const disposable = vscode.commands.registerCommand(name, () => {
		withActiveFile((file, dir) => runCompiler(buildArgs(file), dir));
	});
	context.subscriptions.push(disposable);
}

function activate(context) {
	extensionContext = context;
	output = vscode.window.createOutputChannel('AdvPL');
	context.subscriptions.push(output);

	registerCommand(context, 'advpl.check', f => ['check', f]);
	registerCommand(context, 'advpl.run', f => ['run', f]);
	registerCommand(context, 'advpl.compile', f => ['compile', f]);
	registerCommand(context, 'advpl.build', f => [
		'build', f, '-o', f.replace(/\.(prw|prg|prx|tlpp)$/i, '')
	]);
	registerCommand(context, 'advpl.serve', f => [
		'serve', f, '--debug-port', String(debugPort())
	]);

	context.subscriptions.push(
		vscode.commands.registerCommand('advpl.attachServeDebug', () => {
			const folder = vscode.workspace.workspaceFolders?.[0];
			vscode.debug.startDebugging(folder, {
				type: 'advpl',
				request: 'attach',
				name: 'Attach to AdvPL serve session (web / PO-UI)',
				port: debugPort(),
				host: 'localhost'
			});
		})
	);

	context.subscriptions.push(
		vscode.debug.registerDebugAdapterDescriptorFactory('advpl', {
			createDebugAdapterDescriptor(session) {
				if (session.configuration.request === 'attach') {
					// advplc serve --debug-port já está rodando e escutando;
					// o adaptador aqui é só um cliente TCP, não um processo novo.
					return new vscode.DebugAdapterServer(
						session.configuration.port || debugPort(),
						session.configuration.host || 'localhost'
					);
				}
				const program = session.configuration.program;
				const dir = program ? path.dirname(program) : process.cwd();
				const compiler = resolveCompiler(dir);
				return new vscode.DebugAdapterExecutable(compiler, ['debug']);
			}
		}),
		// Sem essa provider, F5 sem launch.json pergunta qual debugger usar
		// mesmo havendo só um contribuído. Preenche um config "launch" pro
		// arquivo ativo automaticamente quando o usuário não escreveu um —
		// só se aplica a launch: um "attach" sem type/request/name também
		// bateria nessa condição, mas na prática vem sempre com os três.
		vscode.debug.registerDebugConfigurationProvider('advpl', {
			async resolveDebugConfiguration(_folder, config) {
				const editor = vscode.window.activeTextEditor;
				if (!config.type && !config.request && !config.name) {
					config.type = 'advpl';
					config.name = 'Debug current AdvPL file';
					config.request = 'launch';
					config.program = editor ? editor.document.uri.fsPath : '${file}';
					config.stopOnEntry = false;
				}
				if (config.request === 'attach') {
					return config;
				}
				if (!config.program) {
					config.program = editor ? editor.document.uri.fsPath : '${file}';
				}
				const target = vscode.workspace.textDocuments.find(
					d => d.uri.fsPath === config.program && d.isDirty
				);
				if (target) await target.save();
				return config;
			}
		})
	);
}

function debugPort() {
	return vscode.workspace.getConfiguration('advpl').get('debugPort') || 4711;
}

function deactivate() {}

module.exports = { activate, deactivate };
