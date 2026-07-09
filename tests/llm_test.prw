#include "totvs.ch"

// Teste da classe nativa LLM (pkg/llm): motor de inferência GGUF/I2_S em
// Go puro embutido no compilador. Executar com:
//   advplc run tests/llm_test.prw -- /caminho/modelo-i2_s.gguf
User Function LlmTst()
    Local oLLM
    Local cModelo := "/media/peder/DATA/BitNet/models/Falcon3-3B-Instruct-1.58bit/ggml-model-i2_s.gguf"
    Local aTokens
    Local cTexto

    ConOut("Carregando modelo: " + cModelo)
    oLLM := LLM():New(cModelo)

    aTokens := oLLM:Tokenize("The capital of France is")
    ConOut("Tokens do prompt: " + cValToChar(Len(aTokens)))

    cTexto := oLLM:Decode(aTokens)
    ConOut("Decodificado de volta: " + cTexto)

    ConOut("Gerando (greedy, 6 tokens)...")
    cTexto := oLLM:Generate("The capital of France is", 6, 0)
    ConOut("Geração: " + cTexto)

    oLLM:Close()
    ConOut("Concluído.")
Return
