@echo off
rem Sem acentos: o console do Windows abre em codepage 850/437.
rem
rem Desfaz o Ativar-renderizacao-por-software.bat: tira o Mesa3D de junto do
rem programa e devolve o OpenGL do sistema.
rem
rem Existe porque o passo de ativacao tem um modo de falha ruim. opengl32.dll
rem e import estatico do executavel: se o Mesa nao inicializar nesta maquina,
rem o processo morre no carregador e as ferramentas param de abrir, sem
rem mensagem. Sem este script, sair dessa situacao exigiria saber quais
rem arquivos apagar.

setlocal
set "DESTINO=%~dp0."

if exist "%DESTINO%\opengl32.dll"       del /q "%DESTINO%\opengl32.dll"
if exist "%DESTINO%\libgallium_wgl.dll" del /q "%DESTINO%\libgallium_wgl.dll"

echo.
echo Mesa removido de %DESTINO%.
echo As ferramentas voltam a usar o OpenGL do sistema.
echo.
pause
exit /b 0
