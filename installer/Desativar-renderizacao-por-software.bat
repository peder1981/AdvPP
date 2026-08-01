@echo off
rem Sem acentos: o console do Windows abre em codepage 850/437.
rem
rem O AdvPP instalado usa o Mesa3D -- renderizacao por software na CPU --
rem porque em maquina virtual, e em qualquer Windows sem driver de video real,
rem nao existe OpenGL e o Fyne nao consegue criar janela nenhuma.
rem
rem Use este script SO se a IDE parar de abrir sem dar mensagem alguma.
rem Isso acontece quando o Mesa nao inicializa nesta maquina especifica:
rem opengl32.dll e import estatico do executavel, entao o Windows o carrega
rem na criacao do processo e a falha mata tudo antes de qualquer mensagem.
rem
rem Para voltar atras, use Ativar-renderizacao-por-software.bat.

setlocal
cd /d "%~dp0"

if not exist "opengl32.dll" goto :jadesativado

ren "opengl32.dll" "opengl32.dll.off"       || goto :erro
ren "libgallium_wgl.dll" "libgallium_wgl.dll.off" || goto :erro

echo.
echo Mesa desativado. As ferramentas voltam a usar o OpenGL do sistema.
echo Se agora ele reclamar que o driver nao suporta OpenGL, o Mesa era
echo necessario: rode Ativar-renderizacao-por-software.bat.
echo.
pause
exit /b 0

:jadesativado
echo O Mesa ja esta desativado.
pause
exit /b 0

:erro
echo.
echo Falha ao renomear. Execute como administrador
echo (clique direito, "Executar como administrador").
echo.
pause
exit /b 1
