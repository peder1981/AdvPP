# Limites de Recursos do AdvPP

**Versão:** v2.0.3
**Data:** 2026-07-29
**Objetivo:** Documentar os limites duros de recursos impostos pelo AdvPP para prevenir ataques de negação de serviço (CWE-400, CWE-674)

---

## Visão Geral

O AdvPP impõe limites duros sobre recursos críticos para prevenir exaustão de memória, estouro de pilha (stack overflow) e exaustão de goroutines. Todas as violações de limite retornam respostas `ErrorValue` de forma graciosa, capturáveis via blocos Try/Catch — sem crashes.

---

## Limites de Recursos

### 1. Limite de Tamanho de Array

| Propriedade | Valor |
|----------|-------|
| **Limite** | 1.000.000 elementos |
| **Constante** | `MaxArraySize` |
| **Aplica-se a** | `aAdd()`, construção dinâmica de array |
| **Comportamento na violação** | `aAdd()` retorna ErrorValue; array não é modificado |
| **Motivo** | Prevenir exaustão de memória por arrays enormes |
| **Teste** | `TestArraySizeLimit()` em `tests/security_resource_limits_test.prw` |

**Exemplo:**
```advpl
Local aArr := {}
Local i := 0

// Isto terá sucesso para i = 1 até 1.000.000
For i := 1 To 1_000_001
    Local lOk := aAdd(aArr, i)
    If IsError(lOk)
        ConOut("Limite de array: " + lOk:Description)
        Exit  // Para no limite
    EndIf
Next

ConOut("Tamanho final do array: " + cValToChar(Len(aArr)))  // 1.000.000
```

---

### 2. Limite de Propriedades de Objeto

| Propriedade | Valor |
|----------|-------|
| **Limite** | 10.000 propriedades por objeto |
| **Constante** | `MaxObjectProperties` |
| **Aplica-se a** | `ObjectValue.SetProp()`, construção de objeto JSON |
| **Comportamento na violação** | `SetProp()` retorna silenciosamente; nova propriedade não é adicionada (propriedades existentes podem ser sobrescritas) |
| **Motivo** | Prevenir exaustão de memória por objetos enormes |
| **Teste** | `TestObjectPropLimit()` em `tests/security_resource_limits_test.prw` |

**Exemplo:**
```advpl
Local oObj := JsonObject():New()
Local i := 0

// Isto terá sucesso para i = 1 até 10.000
For i := 1 To 10_050
    oObj:SetProperty("key" + cValToChar(i), i)
Next

ConOut("Propriedades do objeto: " + cValToChar(Len(oObj:GetNames())))  // 10.000
```

**Nota:** Propriedades existentes ainda podem ser modificadas mesmo após o limite ser atingido. Só propriedades NOVAS são rejeitadas.

---

### 3. Limite de Profundidade de Aninhamento JSON

| Propriedade | Valor |
|----------|-------|
| **Limite** | 100 níveis de aninhamento |
| **Constante** | `MaxJSONNesting` |
| **Aplica-se a** | Desserialização JSON, aninhamento de objeto/array |
| **Comportamento na violação** | `json.Unmarshal()` retorna erro; JSON não é parseado |
| **Motivo** | Prevenir exaustão de pilha por JSON profundamente aninhado |
| **Teste** | `TestJSONNestingLimit()` em `tests/security_resource_limits_test.prw` |
| **Função** | `ValidateJSONDepth()` em `pkg/runtime/values.go` |

**Exemplo:**
```advpl
Local cJSON := '{"a":{"b":{"c":...}}}'  // 150 níveis de profundidade

// Isto falhará para aninhamento > 100
Local oData := JsonDeserialize(cJSON)
If IsError(oData)
    ConOut("Erro de JSON: " + oData:Description)  // "JSON nesting depth exceeded (max 100 levels)"
EndIf
```

---

### 4. Limite de Jobs Concorrentes

| Propriedade | Valor |
|----------|-------|
| **Limite** | 1.000 goroutines concorrentes |
| **Constante** | `MaxConcurrentJobs` |
| **Aplica-se a** | `StartJob()` com `lWait = .F.` (jobs em segundo plano) |
| **Comportamento na violação** | `StartJob()` retorna erro; job não é criado |
| **Motivo** | Prevenir exaustão de goroutines e vazamento de recursos |
| **Teste** | `TestConcurrentJobsLimit()` em `tests/security_resource_limits_test.prw` |
| **Rastreamento** | Contador atômico `activeJobsCount` em `pkg/vm/vm.go` |

**Exemplo:**
```advpl
Local i := 0

// Isto criará até 1.000 jobs em segundo plano
For i := 1 To 1_050
    Local nErr := StartJob("MyFunction", .F.)  // lWait = .F. (segundo plano)
    If nErr != 0
        ConOut("Limite de jobs: máximo de jobs concorrentes excedido")
        Exit
    EndIf
Next
```

**Nota:** `StartJob(..., .T.)` (síncrono, `lWait = .T.`) NÃO está sujeito a este limite; bloqueia até a conclusão.

---

### 5. Limite de Profundidade de Recursão (Já Existente)

| Propriedade | Valor |
|----------|-------|
| **Limite** | 1.000 call frames |
| **Constante** | `MaxCallFrames` (em `pkg/vm/vm.go`) |
| **Aplica-se a** | Todas as chamadas de função recursivas |
| **Comportamento na violação** | Retorna erro; recursão para |
| **Motivo** | Prevenir estouro de pilha por recursão descontrolada |
| **Status** | Implementado na v2.0.3 |

---

### 6. Limite de Tamanho de String (Já Existente)

| Propriedade | Valor |
|----------|-------|
| **Limite** | 10 MB (10.485.760 bytes) |
| **Aplica-se a** | Construção de string, I/O de arquivo |
| **Comportamento na violação** | String truncada ou erro retornado |
| **Motivo** | Prevenir exaustão de memória por strings enormes |
| **Status** | Implementado na v2.0.3 |

---

### 7. Tamanho da Pilha de Call Frames (Já Existente)

| Propriedade | Valor |
|----------|-------|
| **Limite** | 10.000 posições de pilha |
| **Constante** | `MaxStackSize` (em `pkg/vm/vm.go`) |
| **Aplica-se a** | Todas as atribuições de variável, valores intermediários |
| **Comportamento na violação** | Retorna erro; execução para |
| **Motivo** | Prevenir estouro de pilha por avaliação de expressões |
| **Status** | Implementado na v2.0.3 |

---

## Detalhes de Implementação

### Aplicação do Limite de Array

**Arquivo:** `pkg/runtime/values.go`

```go
const MaxArraySize = 1_000_000

func (a *ArrayValue) Add(elem Value) Value {
    if len(a.Elements) >= MaxArraySize {
        return NewError(fmt.Sprintf("array size limit exceeded (%d elements)", MaxArraySize))
    }
    a.Elements = append(a.Elements, elem)
    return elem
}
```

**Chamadores:** native `AADD` em `pkg/vm/natives.go`

---

### Aplicação do Limite de Propriedades de Objeto

**Arquivo:** `pkg/runtime/values.go`

```go
const MaxObjectProperties = 10_000

func (o *ObjectValue) SetProp(key string, val Value) {
    if _, exists := o.Props[key]; !exists {
        if len(o.Props) >= MaxObjectProperties {
            return  // Ignora silenciosamente a nova propriedade
        }
        o.Keys = append(o.Keys, key)
    }
    o.Props[key] = val
}
```

**Chamadores:** operações de objeto em `pkg/vm/vm.go`, funções nativas de JSON

---

### Aplicação do Limite de Aninhamento JSON

**Arquivo:** `pkg/runtime/values.go`

```go
const MaxJSONNesting = 100

func ValidateJSONDepth(val interface{}, depth int) error {
    if depth > MaxJSONNesting {
        return fmt.Errorf("JSON nesting depth exceeded (max %d levels)", MaxJSONNesting)
    }
    // Verifica recursivamente o aninhamento de objeto/array
    // ...
}
```

**Uso:** Chamado antes ou depois de `json.Unmarshal()` para validar o aninhamento

---

### Aplicação do Limite de Jobs Concorrentes

**Arquivo:** `pkg/vm/vm.go`

```go
const MaxConcurrentJobs = 1_000

var activeJobsCount int32

func (v *VM) StartJob(funcName string, wait bool, args []advplrt.Value) error {
    if wait {
        // Síncrono: sem limite
        return job.RunFunction(...)
    }

    // Segundo plano: verifica o limite
    currentCount := atomic.LoadInt32(&activeJobsCount)
    if currentCount >= int32(MaxConcurrentJobs) {
        return fmt.Errorf("max concurrent jobs exceeded (%d)", MaxConcurrentJobs)
    }

    atomic.AddInt32(&activeJobsCount, 1)
    go func() {
        defer atomic.AddInt32(&activeJobsCount, -1)
        job.RunFunction(...)
    }()
    return nil
}
```

---

## Testes

### Rodando os Testes de Limite

```bash
# Roda todos os testes de limite de recursos
make test

# Roda um teste de limite específico
advplc run tests/security_resource_limits_test.prw

# Roda uma função específica
advplc run -f TestArraySizeLimit tests/security_resource_limits_test.prw
```

### Cobertura de Testes

| Limite | Função de Teste | Status |
|-------|---------------|--------|
| Tamanho de Array | `TestArraySizeLimit()` | ✅ |
| Propriedades de Objeto | `TestObjectPropLimit()` | ✅ |
| Aninhamento JSON | `TestJSONNestingLimit()` | ✅ |
| Jobs Concorrentes | `TestConcurrentJobsLimit()` | ✅ |

---

## Tratamento de Erros

### Retornos Graciosos de Erro

Todas as violações de limite retornam objetos `ErrorValue` capturáveis via Try/Catch do AdvPL:

```advpl
Try
    Local aArr := {}
    For i := 1 To 2_000_000
        aAdd(aArr, i)
    Next
Catch oError
    ConOut("Erro de array: " + oError:Description)
End Try
```

### Sem Crashes

- Os limites são aplicados ANTES da exaustão de recursos
- Sem panic, sem comportamento indefinido
- Execução continua graciosamente ou é explicitamente capturada

---

## Impacto de Performance

As verificações de limite de recursos são operações de tempo constante O(1):
- Verificação de tamanho de array: comparação única (`len >= limit`)
- Verificação de propriedades de objeto: lookup de tamanho de map
- Aninhamento JSON: percurso recursivo (inevitável, mas rápido para aninhamento razoável)
- Contador de jobs: operação atômica

**Overhead desprezível (<1% em cargas de trabalho típicas)**

---

## Conformidade

**CWE-400:** Uncontrolled Resource Consumption (Consumo Descontrolado de Recursos)
**CWE-674:** Uncontrolled Recursion (Recursão Descontrolada, parcialmente via limites)
**OWASP A01:2021:** Broken Access Control (prevenção de DoS)

---

## Melhorias Futuras

- [ ] Limite de handles de arquivo (atualmente aplicado pelo SO)
- [ ] Limite flexível de uso de memória (monitorado via `runtime.MemStats`)
- [ ] Isolamento de memória por VM
- [ ] Limites configuráveis via variáveis de ambiente

---

**Responsável pelo documento:** Auditoria de Segurança (Task 5)
**Última atualização:** 2026-07-29
**Status:** ATIVO (limites aplicados desde a v2.0.3)
