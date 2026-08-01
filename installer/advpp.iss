; installer/advpp.iss -- instalador Windows do AdvPP (Inno Setup 6).
;
; Compilar:  ISCC.exe /DAppVersion=2.0.9 installer\advpp.iss
; Espera, ao lado deste .iss:
;   advplc.exe  adveditor.exe  advpp-ide.exe  e  mesa\*.dll
;
; O AdvPP e uma toolchain, nao um app: o que o zip solto nao entrega e
; justamente o que faz uma toolchain ser usavel --
;
;  1. PATH. Sem isso `advplc` so funciona com caminho completo.
;  2. Atalhos para as duas GUIs (advpp-ide, adveditor).
;  3. Mesa3D FORA do caminho de carregamento. opengl32.dll e import ESTATICO
;     dos binarios Fyne: o Windows o carrega na criacao do processo, antes de
;     qualquer codigo nosso. Um Mesa que nao inicializa nesta maquina --
;     visto em VM QEMU/QXL, provavelmente instrucoes de CPU que o
;     libgallium_wgl.dll usa e a vCPU nao tem -- mata o processo no
;     carregador: sem janela, sem saida, sem log de aplicacao. Instalar o
;     Mesa ao lado dos executaveis trocava um erro visivel ("WGL: driver
;     does not support OpenGL") por silencio total.
;
;     Entao ele vai para {app}\mesa\, fora do caminho de busca de DLL, e so
;     entra em jogo por acao explicita no .bat de ativacao.
;
;  4. Toolchain de compilacao sob demanda. `advplc build` embute o bytecode
;     num stub Go e chama `go build`, o que exige o Go e -- porque o stub
;     importa o Fyne -- um compilador C, ja que CGO e obrigatorio. Embutir os
;     dois no pacote levaria o instalador de 43 MB para ~760 MB por causa de
;     um subcomando, entao eles sao BAIXADOS durante a instalacao, so quando
;     a opcao e marcada, e vao para {app}\toolchain\. O advplc procura essa
;     pasta ao lado dele mesmo (pkg/compiler/toolchain.go) -- nada precisa
;     entrar no PATH, e quem tem Go proprio nao e afetado.
;
;     Os fontes do modulo Go do AdvPP vao para {app}\advpp-src, que o
;     findModuleRoot consulta como ultimo recurso -- assim ADVPP_SRC e o
;     checkout do desenvolvedor continuam com precedencia.

; Exige Inno Setup 6.1+ pelo CreateDownloadPage. O choco instala o 6.x atual.
#ifndef AppVersion
  #define AppVersion "0.0.0-dev"
#endif

; Versoes fixas, como qualquer dependencia: atualizar e um commit, nao uma
; surpresa em quem instalar amanha.
#define GoVersao "1.24.2"
#define GoURL "https://go.dev/dl/go" + GoVersao + ".windows-amd64.zip"
; Publicado pelo go.dev. O download ja vem por HTTPS, mas o hash tambem
; protege contra o arquivo mudar no servidor sob a mesma URL -- e o que sera
; executado como compilador na maquina de quem instala.
#define GoSHA256 "29c553aabee0743e2ffa3e9fa0cda00ef3b3cc4ff0bc92007f31f80fd69892e1"
#define MinGWTag "16.1.0posix-14.0.0-msvcrt-r4"
#define MinGWURL "https://github.com/brechtsanders/winlibs_mingw/releases/download/" + MinGWTag + "/winlibs-x86_64-posix-seh-gcc-16.1.0-mingw-w64msvcrt-14.0.0-r4.zip"

[Setup]
; AppId fixo: e o que faz uma versao nova atualizar a instalada em vez de
; instalar do lado. Nao mude entre releases.
AppId={{A17D9E42-6B03-4F58-8C2E-D5194A7B3F60}
AppName=AdvPP
AppVersion={#AppVersion}
AppVerName=AdvPP {#AppVersion}
AppPublisher=AdvPP
DefaultDirName={autopf}\AdvPP
DefaultGroupName=AdvPP
OutputDir=.
OutputBaseFilename=AdvPP-Setup-{#AppVersion}
Compression=lzma2/max
SolidCompression=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
PrivilegesRequired=admin
WizardStyle=modern
DisableProgramGroupPage=yes
UninstallDisplayName=AdvPP {#AppVersion}
UninstallDisplayIcon={app}\advpp-ide.exe
ChangesEnvironment=yes

[Languages]
Name: "brazilianportuguese"; MessagesFile: "compiler:Languages\BrazilianPortuguese.isl"

[Tasks]
Name: "path"; Description: "Adicionar o AdvPP ao PATH (permite chamar advplc de qualquer pasta)"
Name: "desktopicon"; Description: "Criar atalho da IDE na Area de Trabalho"; Flags: unchecked
; ~370 MB de download. Marcada por InitializeWizard quando nao ha Go na
; maquina -- quem ja tem o proprio nao precisa de outro.
Name: "toolchain"; Description: "Baixar o toolchain de compilacao: Go {#GoVersao} + compilador C, ~350 MB (necessario apenas para o comando advplc build)"; GroupDescription: "Compilacao:"; Flags: unchecked

[Files]
Source: "advplc.exe";     DestDir: "{app}"; Flags: ignoreversion
Source: "adveditor.exe";  DestDir: "{app}"; Flags: ignoreversion
Source: "advpp-ide.exe";  DestDir: "{app}"; Flags: ignoreversion
; Fontes do modulo Go: o stub gerado por `advplc build` importa este modulo,
; entao ele precisa existir em disco. Sao ~7 MB, entao vao no pacote mesmo
; sem a opcao de toolchain -- assim quem ja tem Go proprio nao precisa
; clonar o repositorio.
Source: "advpp-src\*"; DestDir: "{app}\advpp-src"; Flags: ignoreversion recursesubdirs createallsubdirs
; Fora do caminho de busca de DLL. So o .bat abaixo os poe junto dos
; executaveis, e so quando o OpenGL do sistema nao serve.
Source: "mesa\opengl32.dll";        DestDir: "{app}\mesa"; Flags: ignoreversion
Source: "mesa\libgallium_wgl.dll";  DestDir: "{app}\mesa"; Flags: ignoreversion
Source: "installer\Ativar-renderizacao-por-software.bat";    DestDir: "{app}"; Flags: ignoreversion
Source: "installer\Desativar-renderizacao-por-software.bat"; DestDir: "{app}"; Flags: ignoreversion


[Dirs]
; Destino dos zips baixados, extraidos pelo [Run] logo abaixo.
Name: "{app}\toolchain"; Tasks: toolchain
; As tres ferramentas compartilham o mesmo banco (~/.advpp/ADVPP.db); a pasta
; e por usuario, entao nao precisa de permissao especial.
Name: "{userappdata}\..\.advpp"

[Icons]
Name: "{group}\AdvPP IDE"; Filename: "{app}\advpp-ide.exe"
Name: "{group}\AdvEditor (banco e dicionario)"; Filename: "{app}\adveditor.exe"
Name: "{group}\Ativar renderizacao por software"; Filename: "{app}\Ativar-renderizacao-por-software.bat"; \
    IconFilename: "{app}\advpp-ide.exe"
Name: "{group}\Desinstalar o AdvPP"; Filename: "{uninstallexe}"
Name: "{autodesktop}\AdvPP IDE"; Filename: "{app}\advpp-ide.exe"; Tasks: desktopicon

[Registry]
; Append no PATH da maquina. O check em NecessitaPath evita duplicar a entrada
; em reinstalacao -- PATH duplicado nao quebra nada, mas cresce a cada release.
Root: HKLM; Subkey: "SYSTEM\CurrentControlSet\Control\Session Manager\Environment"; \
    ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}"; \
    Tasks: path; Check: NecessitaPath(ExpandConstant('{app}'))

[Run]
; tar.exe do proprio Windows (bsdtar, presente desde o Windows 10 1803) em vez
; do "extractarchive" do Inno: com ele o instalador abortava em "O formato do
; arquivo compactado nao e compativel" para um zip deflate perfeitamente
; valido -- SHA-256 conferido, o 7-Zip le sem reclamar. Os dois arquivos
; trazem a propria pasta raiz (go\, mingw64\), e o advplc procura o gcc
; dentro de toolchain\ sem depender desse nome.
Filename: "{sys}\tar.exe"; Parameters: "-xf ""{tmp}\go.zip"" -C ""{app}\toolchain"""; \
    StatusMsg: "Extraindo o toolchain Go..."; Flags: runhidden waituntilterminated; Tasks: toolchain
Filename: "{sys}\tar.exe"; Parameters: "-xf ""{tmp}\mingw.zip"" -C ""{app}\toolchain"""; \
    StatusMsg: "Extraindo o compilador C..."; Flags: runhidden waituntilterminated; Tasks: toolchain

Filename: "{app}\advpp-ide.exe"; Description: "Abrir a IDE agora"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
; Extraido em tempo de instalacao, entao o desinstalador nao o conhece pelo
; [Files] e precisa ser dito.
Type: filesandordirs; Name: "{app}\toolchain"

[Code]
// A heuristica de driver OpenGL que existia aqui foi removida junto com a
// caixa que ela marcava. Ela adivinhava, pelo registro, se a maquina tinha
// driver de verdade, e errou numa VM QXL -- e o preco do erro nao era "roda
// mais devagar", era "nao abre e nao diz nada".

function NecessitaPath(Pasta: String): Boolean;
var
  Atual: String;
begin
  if not RegQueryStringValue(HKEY_LOCAL_MACHINE,
      'SYSTEM\CurrentControlSet\Control\Session Manager\Environment', 'Path', Atual) then
  begin
    Result := True;
    Exit;
  end;
  // Delimitado por ';' dos dois lados para nao casar com um prefixo de outra
  // pasta (C:\AdvPP dando falso positivo dentro de C:\AdvPP-antigo).
  Result := Pos(';' + Uppercase(Pasta) + ';', ';' + Uppercase(Atual) + ';') = 0;
end;

function TemGo(): Boolean;
var
  Codigo: Integer;
begin
  Result := ShellExec('', 'cmd.exe', '/c go version', '', SW_HIDE, ewWaitUntilTerminated, Codigo)
            and (Codigo = 0);
end;

var
  PaginaDownload: TDownloadWizardPage;

function AoBaixar(const Url, Arquivo: String; const Feito, Total: Int64): Boolean;
begin
  Result := True;
end;

procedure InitializeWizard();
begin
  PaginaDownload := CreateDownloadPage(
    'Baixando o toolchain de compilacao',
    'Go e compilador C, necessarios para o comando advplc build.',
    @AoBaixar);

  // Quem ja tem Go proprio nao precisa de outro: a caixa so vem marcada
  // quando nao ha nenhum na maquina.
  if not TemGo() then
    WizardSelectTasks('toolchain');
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  if (CurPageID <> wpReady) or (not WizardIsTaskSelected('toolchain')) then
    Exit;

  if not FileExists(ExpandConstant('{sys}\tar.exe')) then
  begin
    MsgBox('Esta versao do Windows nao tem o tar.exe, usado para extrair o '
      + 'toolchain de compilacao. A instalacao continua sem ele: funcionam '
      + '"advplc run", "advplc check" e "advplc serve", mas nao "advplc build".',
      mbInformation, MB_OK);
    WizardSelectTasks('!toolchain');
    Exit;
  end;

  PaginaDownload.Clear;
  PaginaDownload.Add('{#GoURL}', 'go.zip', '{#GoSHA256}');
  // Sem hash: o winlibs nao publica checksum num lugar estavel. Fica a
  // integridade do HTTPS mais a tag fixa do release.
  PaginaDownload.Add('{#MinGWURL}', 'mingw.zip', '');
  PaginaDownload.Show;
  try
    try
      PaginaDownload.Download;
    except
      // Falha de rede nao pode derrubar a instalacao inteira: o AdvPP e util
      // sem o toolchain (run, check, serve). Desmarca a opcao e segue.
      MsgBox('Nao foi possivel baixar o toolchain de compilacao:' #13#10 #13#10
        + GetExceptionMessage + #13#10 #13#10
        + 'A instalacao continua sem ele. Para tentar de novo, rode este '
        + 'instalador outra vez com a opcao marcada.',
        mbError, MB_OK);
      WizardSelectTasks('!toolchain');
    end;
  finally
    PaginaDownload.Hide;
  end;
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if (CurStep = ssPostInstall) and (not TemGo()) and (not WizardIsTaskSelected('toolchain')) then
    MsgBox('O AdvPP foi instalado.' #13#10 #13#10
      + 'Nao ha compilador Go nesta maquina e o toolchain nao foi baixado. '
      + 'Funcionam "advplc run", "advplc check" e "advplc serve"; para '
      + '"advplc build", que gera executavel, rode este instalador de novo '
      + 'e marque a opcao de baixar o toolchain.',
      mbInformation, MB_OK);
end;
