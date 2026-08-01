@echo off
rem Sem acentos: o console do Windows abre em codepage 850/437.
rem
rem Desfaz o Desativar-renderizacao-por-software.bat: devolve o Mesa3D ao
rem lugar e as ferramentas voltam a desenhar por software na CPU.
rem
rem O Mesa ja vem ativo na instalacao. Este script so e necessario para quem
rem desativou e quer voltar -- tipicamente porque a IDE passou a reclamar
rem de "WGL: The driver does not appear to support OpenGL", sinal de que a
rem maquina nao tem OpenGL proprio e precisa mesmo do Mesa.

setlocal
cd /d "%~dp0"

if not exist "opengl32.dll.off" goto :jaativo

ren "opengl32.dll.off" "opengl32.dll"       || goto :erro
ren "libgallium_wgl.dll.off" "libgallium_wgl.dll" || goto :erro

echo.
echo Mesa reativado. Abra a IDE pelo atalho.
echo.
pause
exit /b 0

:jaativo
echo O Mesa ja esta ativo (e o padrao da instalacao).
pause
exit /b 0

:erro
echo.
echo Falha ao renomear. Execute como administrador
echo (clique direito, "Executar como administrador").
echo.
pause
exit /b 1
