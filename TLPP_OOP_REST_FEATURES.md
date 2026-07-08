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

### WSRESTFUL

Define um serviço REST completo com metadata e endpoints.

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

Define campos de dados para o serviço REST.

```advpl
WSDATA cCodigo as String
WSDATA cNome as String
WSDATA nIdade as Integer
WSDATA nLimiteCredito as Decimal
```

### WSMETHOD

Define métodos/endpoints do serviço REST com verbos HTTP.

```advpl
WSMETHOD GET ListarClientes Description "Lista todos os clientes"
WSMETHOD GET ObterCliente Description "Obtém cliente por código"
WSMETHOD POST CriarCliente Description "Cria novo cliente"
WSMETHOD PUT AtualizarCliente Description "Atualiza cliente existente"
WSMETHOD DELETE ExcluirCliente Description "Exclui cliente por código"
```

**Verbos HTTP Suportados:**
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
| WSRESTFUL | ✅ Parsing | Sintaxe parseada, não executada |
| WSDATA | ✅ Parsing | Campos definidos corretamente |
| WSMETHOD | ✅ Parsing | Verbos HTTP reconhecidos |
| JSON Inline | ✅ Completo | Sintaxe `{ "key" : "value" }` funciona |
| Try/Catch | ⚠️ Parcial | Parsing funciona, execução limitada |
| Tipagem Estática | ⚠️ Parcial | `as` parseado, não validado |

## Limitações Atuais

1. **REST Execution**: WSRESTFUL é parseado mas não executado (requer servidor HTTP)
2. **Try/Catch**: Parsing funciona mas tratamento de exceções é limitado
3. **Tipagem**: Declarações de tipos são parseadas mas não validadas em runtime
4. **Modificadores de Acesso**: PUBLIC/PRIVATE/PROTECTED são parseados mas não enforcement
5. **Interfaces**: Sintaxe `implements` é parseada mas não validada

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

1. **Para REST**: Adicionar servidor HTTP (net/http) para executar endpoints WSRESTFUL
2. **Para Try/Catch**: Melhorar tratamento de exceções na VM
3. **Para Tipagem**: Implementar validação de tipos em runtime
4. **Para Modificadores**: Implementar enforcement de PUBLIC/PRIVATE/PROTECTED
5. **Para Interfaces**: Implementar validação de implementação de interface
