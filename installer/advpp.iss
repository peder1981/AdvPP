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
;  3. Mesa3D opcional: as duas GUIs sao Fyne e morrem em maquina sem driver
;     de video com "WGL: The driver does not appear to support OpenGL".
;
; O que este instalador NAO resolve, de proposito: `advplc build` compila o
; bytecode dentro de um stub Go, entao exige o toolchain Go e o repositorio
; do AdvPP (ADVPP_SRC) na maquina do usuario. Empacotar o Go aqui trocaria um
; download de 25 MB por um de 200 MB para um subcomando so. O instalador
; apenas avisa quando nao encontra o Go; `advplc run|check|serve` funcionam
; sem nada disso.

#ifndef AppVersion
  #define AppVersion "0.0.0-dev"
#endif

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
Name: "mesa"; Description: "Renderizacao por software (maquina virtual ou sem driver de video)"; GroupDescription: "Compatibilidade:"; Flags: unchecked

[Files]
Source: "advplc.exe";     DestDir: "{app}"; Flags: ignoreversion
Source: "adveditor.exe";  DestDir: "{app}"; Flags: ignoreversion
Source: "advpp-ide.exe";  DestDir: "{app}"; Flags: ignoreversion
; Os DLLs vao para a pasta dos executaveis porque e la que o Windows procura
; DLL antes do System32 -- o diretorio de trabalho nao entra nessa busca.
Source: "mesa\opengl32.dll";        DestDir: "{app}"; Tasks: mesa; Flags: ignoreversion
Source: "mesa\libgallium_wgl.dll";  DestDir: "{app}"; Tasks: mesa; Flags: ignoreversion

[Dirs]
; As tres ferramentas compartilham o mesmo banco (~/.advpp/ADVPP.db); a pasta
; e por usuario, entao nao precisa de permissao especial.
Name: "{userappdata}\..\.advpp"

[Icons]
Name: "{group}\AdvPP IDE"; Filename: "{app}\advpp-ide.exe"
Name: "{group}\AdvEditor (banco e dicionario)"; Filename: "{app}\adveditor.exe"
Name: "{group}\Desinstalar o AdvPP"; Filename: "{uninstallexe}"
Name: "{autodesktop}\AdvPP IDE"; Filename: "{app}\advpp-ide.exe"; Tasks: desktopicon

[Registry]
; Append no PATH da maquina. O check em NecessitaPath evita duplicar a entrada
; em reinstalacao -- PATH duplicado nao quebra nada, mas cresce a cada release.
Root: HKLM; Subkey: "SYSTEM\CurrentControlSet\Control\Session Manager\Environment"; \
    ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}"; \
    Tasks: path; Check: NecessitaPath(ExpandConstant('{app}'))

[Run]
Filename: "{app}\advpp-ide.exe"; Description: "Abrir a IDE agora"; Flags: nowait postinstall skipifsilent

[Code]
// Heuristica para o estado inicial da caixa "Renderizacao por software".
// Um driver de video WDDM real registra OpenGLDriverName na chave da classe
// Display; o Microsoft Basic Display Adapter, nao. So define o default -- o
// usuario marca ou desmarca por cima, entao um palpite errado nao custa nada.
function TemDriverOpenGL(): Boolean;
var
  I: Integer;
  Chave, Valor: String;
begin
  Result := False;
  for I := 0 to 7 do
  begin
    Chave := Format('SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}\%.4d', [I]);
    if RegQueryStringValue(HKEY_LOCAL_MACHINE, Chave, 'OpenGLDriverName', Valor) and (Valor <> '') then
    begin
      Result := True;
      Exit;
    end;
  end;
end;

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

procedure InitializeWizard();
begin
  if not TemDriverOpenGL() then
    WizardSelectTasks('mesa');
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if (CurStep = ssPostInstall) and (not TemGo()) then
    MsgBox('O AdvPP foi instalado.' #13#10 #13#10
      + 'Nao encontrei o compilador Go nesta maquina. Sem ele funcionam '
      + '"advplc run", "advplc check" e "advplc serve", mas nao "advplc build", '
      + 'que gera executavel e precisa do Go mais o repositorio do AdvPP '
      + 'apontado por ADVPP_SRC.' #13#10 #13#10
      + 'Go: https://go.dev/dl/',
      mbInformation, MB_OK);
end;
