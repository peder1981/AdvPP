// Fixture usada pelo smoke test de CI (Windows) do executável standalone
// (advplc build): confirma que o stub gerado (janela Fyne + console +
// banco) compila e RODA de verdade num binário nativo, não só que o VM
// headless funciona. Programa 100% console: deve imprimir as duas linhas
// abaixo e encerrar sozinho sem exigir interação do usuário.
User Function Main()
    ConOut("standalone smoke test: linha 1")
    ConOut("standalone smoke test: linha 2")
Return .T.
