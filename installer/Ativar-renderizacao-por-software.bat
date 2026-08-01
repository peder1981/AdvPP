@echo off
rem Sem acentos: o console do Windows abre em codepage 850/437.
rem
rem Use SO se o advpp-ide ou o adveditor fecharem com a mensagem:
rem   "Fyne error: window creation error"
rem   "Cause: APIUnavailable: WGL: The driver does not appear to support OpenGL"
rem
rem Isso quer dizer que o Windows nao oferece OpenGL 2.0+ -- maquina sem driver
rem de video, ou VM sem aceleracao. Este script copia o Mesa3D para junto dos
rem executaveis, e eles passam a desenhar por software na CPU.
rem
rem POR QUE ISTO NAO E AUTOMATICO: opengl32.dll e import estatico dos binarios,
rem entao o Windows o carrega na criacao do processo, antes de qualquer codigo
rem nosso. Se o Mesa nao inicializar nesta maquina -- acontece em algumas VMs,
rem por instrucoes de CPU que o libgallium_wgl.dll usa e a vCPU nao tem -- o
rem processo morre no carregador, sem janela e sem mensagem nenhuma. Trocar um
rem erro legivel por silencio total e pior, entao o Mesa so entra quando
rem alguem decide que precisa dele.
rem
rem O advplc nao e afetado: ele nao abre janela.
rem Para desfazer, use Desativar-renderizacao-por-software.bat.

setlocal
set "ORIGEM=%~dp0mesa"

if not exist "%ORIGEM%\opengl32.dll" goto :semmesa

echo Copiando o Mesa3D para %~dp0 ...
copy /y "%ORIGEM%\opengl32.dll" "%~dp0" >nul || goto :erro
copy /y "%ORIGEM%\libgallium_wgl.dll" "%~dp0" >nul || goto :erro

echo.
echo Pronto. Abra a IDE pelo atalho do menu Iniciar.
echo Se ela passar a nao abrir mais NADA, o Mesa nao funciona nesta maquina:
echo rode Desativar-renderizacao-por-software.bat para voltar atras.
echo.
pause
exit /b 0

:semmesa
echo Nao encontrei os arquivos do Mesa em %ORIGEM%.
pause
exit /b 1

:erro
echo.
echo Falha ao copiar. Execute este arquivo como administrador
echo (clique direito, "Executar como administrador").
echo.
pause
exit /b 1
