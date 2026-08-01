# Recursos TLPP - Orientação a Objeto e REST

## Visão Geral

O AdvPP suporta recursos avançados de TLPP (TOTVS Language Plus Plus) incluindo orientação a objetos completa e definição de serviços REST com annotations.

## Recursos de Orientação a Objeto

### Classes e Herança

**Sintaxe Básica:**
```advpl
Class NomeClasse from ClasseBase
    Data propriedade1
    Data propriedade2
    
    Method Metodo1(parametro1)
    Method Metodo2(parametro1, parametro2) Constructor
EndClass
```

**Exemplo com Herança:**
```advpl
Class Pessoa from BaseObject
    Data cNome
    Data nIdade
    Data cEmail
    
    Method New(cNome, nIdade, cEmail) Constructor
    Method GetNome()
    Method Validar()
EndClass

Class Cliente from Pessoa
    Data cCodigo
    Data nLimiteCredito
    
    Method New(cNome, nIdade, cEmail, cCodigo, nLimite) Constructor
    Method GetCodigo()
    Method AdicionarCompra(nValor)
EndClass
```

### Construtores

- Definidos com a palavra-chave `Constructor`
- Chamados automaticamente ao instanciar a classe
- Podem chamar o construtor da classe base com `Super:New()`

```advpl
Method New(cNome, nIdade, cEmail) Class Pessoa
    ::cNome := cNome
    ::nIdade := nIdade
    ::cEmail := cEmail
Return Self
```

### Métodos

- Métodos podem ser definidos dentro ou fora da classe
- Acesso a propriedades com `::propriedade`
- Retorno com `Return`

```advpl
Method GetNome() Class Pessoa
Return ::cNome

Method Validar() Class Pessoa
    If Empty(::cNome)
        Return .F.
    EndIf
Return .T.
```

### Instanciação

```advpl
Local oPessoa
oPessoa := Pessoa("João Silva", 35, "joao@email.com")
ConOut(oPessoa:ToString())
ConOut(oPessoa:GetNome())
```

## Recursos REST

Há dois caminhos para REST no AdvPP, com status bem diferentes — **não confunda um com o outro**:

### Anotações `@Get`/`@Post`/`@Put`/`@Patch`/`@Delete` — funcional, servidor HTTP real

Este é o caminho **executável**. Uma `User Function` anotada é registrada como rota
real num `WSRestServer` (`net/http` puro, sem framework externo), com path params,
corpo JSON parseado/serializado de verdade e dispatch real para a função AdvPL:

```advpl
@Get("/clientes/{id}")
User Function GetCliente(id)
    Local jResp := { "codigo" : id, "nome" : "Cliente Teste" }
Return jResp

User Function Main()
    Local oRest := WSRestServer():New("API", "1.0")
    oRest:AddRoute("GET", "/status", "GetStatus")
    oRest:Serve(8080)
Return
```

Cada requisição roda numa instância de VM isolada (não reentra a VM que está
servindo), o que torna o dispatch seguro sob concorrência. Ver seção
"Servidor REST" do [`README.md`](README.md#servidor-rest-wsrestserver) para
o exemplo completo.

### WSRESTFUL (DSL clássico) — apenas parsing, não executa

Define a *sintaxe* de um serviço REST completo com metadata e endpoints — mas
o AdvPP **reconhece a sintaxe e descarta verbo/PATH no parser**; não há
dispatch nem servidor HTTP por trás deste DSL. Para expor os mesmos endpoints
de verdade, reescreva como `User Function` com anotações (acima).

```advpl
WSRESTFUL ClienteService
    DESCRIPTION "Serviço REST para gerenciamento de clientes"
    NAMESPACE "http://localhost:8080/api"
    
    WSDATA cCodigo as String
    WSDATA cNome as String
    WSDATA nIdade as Integer
    
    WSMETHOD GET ListarClientes Description "Lista todos os clientes"
    WSMETHOD POST CriarCliente Description "Cria novo cliente"
    WSMETHOD PUT AtualizarCliente Description "Atualiza cliente existente"
    WSMETHOD DELETE ExcluirCliente Description "Exclui cliente por código"
EndWSRESTFUL
```

### WSDATA

Define campos de dados para o serviço REST (DSL clássico). Parseado, mas —
como o `WSRESTFUL` que o contém — sem efeito em runtime.

```advpl
WSDATA cCodigo as String
WSDATA cNome as String
WSDATA nIdade as Integer
WSDATA nLimiteCredito as Decimal
```

### WSMETHOD

Define métodos/endpoints do serviço REST com verbos HTTP (DSL clássico —
mesma ressalva de execução acima).

```advpl
WSMETHOD GET ListarClientes Description "Lista todos os clientes"
WSMETHOD GET ObterCliente Description "Obtém cliente por código"
WSMETHOD POST CriarCliente Description "Cria novo cliente"
WSMETHOD PUT AtualizarCliente Description "Atualiza cliente existente"
WSMETHOD DELETE ExcluirCliente Description "Exclui cliente por código"
```

**Verbos HTTP Suportados (ambos os caminhos):**
- GET
- POST
- PUT
- DELETE
- PATCH

## Recursos Adicionais TLPP

### JSON Inline

Sintaxe compacta para criação de objetos JSON.

```advpl
Local jDados
jDados := { "codigo" : "CLI002", "nome" : "Pedro Costa", "idade" : 42, "ativo" : .T. }
ConOut(jDados:codigo)
ConOut(jDados:nome)
ConOut(Str(jDados:idade))
```

### Try/Catch

Tratamento de exceções estruturado.

```advpl
Try
    Local nDiv := 10
    Local nResult := nDiv / 0
    ConOut("Resultado: " + Str(nResult))
Catch eError
    ConOut("Erro capturado: " + eError)
EndTry
```

### Tipagem Estática

TLPP suporta declaração de tipos (embora opcional no AdvPP).

```advpl
Data cNome as Character
Data nIdade as Integer
Data nValor as Decimal

Method New(cNome as Character, nIdade as Integer) Constructor
Method GetNome() as Character
Method Validar() as Logical
```

## Status de Implementação

| Recurso | Status | Notas |
|---------|--------|-------|
| Classes | ✅ Completo | Parsing e execução funcionam |
| Herança | ✅ Completo | Suporte a `from` e `Super:New()` |
| Construtores | ✅ Completo | `Constructor` suportado |
| Métodos | ✅ Completo | Definição dentro/fora da classe |
| Anotações `@Get`/`@Post`/`@Put`/`@Patch`/`@Delete` | ✅ Completo | Servidor HTTP real (`net/http`) via `WSRestServer`, dispatch de verdade |
| WSRESTFUL (DSL clássico) | ⚠️ Apenas Parsing | Sintaxe parseada; verbo/PATH descartados, sem servidor por trás — use anotações acima |
| WSDATA | ⚠️ Apenas Parsing | Campos parseados, sem efeito em runtime (DSL clássico) |
| WSMETHOD | ⚠️ Apenas Parsing | Verbos HTTP reconhecidos, sem dispatch (DSL clássico) |
| JSON Inline | ✅ Completo | Sintaxe `{ "key" : "value" }` funciona |
| Try/Catch | ✅ Completo | Try/Catch real na VM: pilha de handlers por frame, desenrolamento entre chamadas aninhadas, `Throw`/`UserException` propagam erro capturável |
| Tipagem Estática | ⚠️ Parcial | `as` parseado, tipo é documentário — não há checagem estática nem validação em runtime |

## Limitações Atuais

1. **REST Execution (DSL clássico)**: `WSRESTFUL`/`WSMETHOD` são parseados mas não executados — use anotações `@Get`/`@Post`/etc para endpoints reais (ver seção acima)
2. **Tipagem**: Declarações de tipos são parseadas mas não validadas em runtime
3. **Modificadores de Acesso**: PUBLIC/PRIVATE/PROTECTED são parseados mas sem enforcement
4. **Interfaces**: bloco `Interface ... EndInterface` é parseado (métodos declarados viram um `InterfaceDecl` real); não há, porém, cláusula `implements` em `Class` nem checagem de que uma classe realmente implementa os métodos declarados

## Exemplo Completo

Veja `tests/tlpp_oop_rest_test.tlpp` para um exemplo completo demonstrando:
- Classes base e derivadas
- Herança e polimorfismo
- Construtores com Super:New()
- Métodos com acesso a propriedades
- WSRESTFUL com múltiplos endpoints
- JSON inline
- Try/Catch

## Recomendações

1. **Para REST**: já existe servidor HTTP real (`net/http` via `WSRestServer` + anotações `@Get`/`@Post`/etc) — se seu código usa `WSRESTFUL`/`WSMETHOD`, reescreva como `User Function` anotada para ter dispatch de verdade
2. **Para Tipagem**: Implementar validação de tipos em runtime (hoje é só documentário)
3. **Para Modificadores**: Implementar enforcement de PUBLIC/PRIVATE/PROTECTED
4. **Para Interfaces**: Implementar cláusula `implements` em `Class` + checagem de que os métodos da interface foram de fato implementados
