#include "totvs.ch"

// Teste da classe nativa MCPServer (pkg/mcp): expõe funções AdvPL/TLPP
// como "tools" de um servidor MCP (Model Context Protocol) real, rodando
// sobre stdio. Executar com: advplc run tests/mcp_test.prw
// (o processo bloqueia lendo mensagens JSON-RPC do stdin).
User Function McpTst()
    Local oMCP := MCPServer():New("advpp-demo", "1.0.0")

    oMCP:AddTool("soma", "Soma dois números", ;
        '{"type":"object","properties":{"a":{"type":"number"},"b":{"type":"number"}},"required":["a","b"]}', ;
        "ToolSoma")

    oMCP:AddTool("saudacao", "Cumprimenta alguém pelo nome", ;
        '{"type":"object","properties":{"nome":{"type":"string"}},"required":["nome"]}', ;
        "ToolSaudacao")

    oMCP:Serve()
Return

User Function ToolSoma(oArgs)
Return cValToChar(oArgs:A + oArgs:B)

User Function ToolSaudacao(oArgs)
Return "Ola, " + oArgs:NOME + "!"
