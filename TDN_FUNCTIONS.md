# Funções TDN do TOTVS Implementadas

## Visão Geral

O AdvPP implementa funções do TOTVS Developer Network (TDN) para manipulação de strings, matemática, datas e arquivos, conforme documentação oficial.

## Funções de String Implementadas

### LEFT
Retorna os primeiros caracteres de uma string.
```advpl
LEFT(s, n)  // Retorna n caracteres à esquerda
```

### RIGHT
Retorna os últimos caracteres de uma string.
```advpl
RIGHT(s, n)  // Retorna n caracteres à direita
```

### REPLICA
Replica uma string n vezes.
```advpl
REPLICA(s, n)  // Retorna s repetida n vezes
```

### CAPSLOCK
Converte primeira letra para maiúscula e restante para minúscula.
```advpl
CAPSLOCK(s)  // Retorna string em Title Case
```

### PROPER
Converte cada palavra para Title Case.
```advpl
PROPER(s)  // Retorna cada palavra com primeira letra maiúscula
```

### ATC
Busca posição de substring (case-insensitive).
```advpl
ATC(search, s)  // Retorna posição (1-based) ou 0
```

### RATC
Busca última posição de substring (case-insensitive).
```advpl
RATC(search, s)  // Retorna última posição (1-based) ou 0
```

### GETWORDNUM
Retorna n-ésima palavra de uma string.
```advpl
GETWORDNUM(s, n, [delim])  // Retorna n-ésima palavra
```

### WORDS
Retorna número de palavras em uma string.
```advpl
WORDS(s, [delim])  // Retorna contagem de palavras
```

### FILENOEXT
Remove extensão de nome de arquivo.
```advpl
FILENOEXT(path)  // Retorna caminho sem extensão
```

### FILEEXT
Retorna extensão de arquivo.
```advpl
FILEEXT(path)  // Retorna extensão apenas
```

### FILENAME
Retorna nome do arquivo com extensão.
```advpl
FILENAME(path)  // Retorna nome do arquivo
```

### FILEPATH
Retorna caminho completo com separador final.
```advpl
FILEPATH(path)  // Retorna caminho do diretório
```

### FILEDIR
Retorna diretório do arquivo.
```advpl
FILEDIR(path)  // Retorna diretório sem separador final
```

## Funções Matemáticas Implementadas

### SIGN
Retorna sinal de número (-1, 0, 1).
```advpl
SIGN(n)  // Retorna -1 se negativo, 0 se zero, 1 se positivo
```

### POWER
Calcula potência.
```advpl
POWER(base, exp)  // Retorna base^exp
```

### PI
Retorna valor de π.
```advpl
PI()  // Retorna 3.141592653589793
```

### SIN
Calcula seno.
```advpl
SIN(angle)  // Retorna seno em radianos
```

### COS
Calcula cosseno.
```advpl
COS(angle)  // Retorna cosseno em radianos
```

### TAN
Calcula tangente.
```advpl
TAN(angle)  // Retorna tangente em radianos
```

### ASIN
Calcula arco seno.
```advpl
ASIN(value)  // Retorna arco seno em radianos
```

### ACOS
Calcula arco cosseno.
```advpl
ACOS(value)  // Retorna arco cosseno em radianos
```

### ATAN
Calcula arco tangente.
```advpl
ATAN(value)  // Retorna arco tangente em radianos
```

### DEG2RAD
Converte graus para radianos.
```advpl
DEG2RAD(degrees)  // Retorna valor em radianos
```

### RAD2DEG
Converte radianos para graus.
```advpl
RAD2DEG(radians)  // Retorna valor em graus
```

## Funções de Data Implementadas

### STOD
Converte string AAAAMMDD para data.
```advpl
STOD(s)  // Retorna data a partir de string
```

### ELAPTIME
Calcula tempo decorrido entre dois momentos.
```advpl
ELAPTIME(t1, t2)  // Retorna diferença em segundos
```

### CTOT
Converte string HH:MM:SS para segundos.
```advpl
CTOT(s)  // Retorna total de segundos
```

### TTOC
Converte segundos para string HH:MM:SS.
```advpl
TTOC(seconds)  // Retorna string formatada
```

## Funções Já Existentes

### String
- ALLTRIM, LTRIM, RTRIM, TRIM
- STR, STRTRAN, STRZERO, SUBSTR, STUFF
- LEN, AT, RAT
- UPPER, LOWER
- PADC, PADL, PADR
- CHR, ASC, VAL, CVALTOCHAR
- CTOD, DTOS, DTOC
- TRANSFORM
- ISDIGIT, ISALPHA, ISLOWER, ISUPPER
- EMPTY, SPACE, REPLICATE
- STRTOKARR

### Matemática
- ABS, INT, ROUND, NOROUND
- CEILING, FLOOR, MOD
- MAX, MIN
- SQRT, EXP, LOG
- RANDOM

### Data
- DATE, DAY, MONTH, YEAR
- CMONTH, CDOW, DOW
- TIME, SECONDS

### Array
- AADD, ASIZE, ASCAN, ADEL, AINS
- ALEN, ACLONE, AFILL, ASORT, AEVAL

### Lógica/Tipo
- IIF, IF, VALTYPE, TYPE, ISNIL

### Banco de Dados
- DBSELECTAREA, DBSEEK, DBSKIP
- DBGOTOP, DBGOBOTTOM, EOF, BOF
- RECLOCK, MSUNLOCK, RECCOUNT, RECNO
- DBCLOSEAREA, DBSETORDER, DBSETFILTER
- DBCLEARFILTER, DBAPPEND, DBDELETE, DBCOMMIT
- SELECT, ALIAS, USED
- FIELDGET, FIELDPUT, FIELDPOS, FIELDNAME
- XFILIAL

### MVC
- FWFORMMODEL, FWFORMVIEW, FWFORMBROWSE
- FWFORMSTRUCT, FWMBROWSE, VIEWDEF, AXCADASTRO

### JSON
- JSONOBJECT

### Outras
- HELP, SETDATE, SETCENT, SET
- FREEOBJ, SLEEP
- PROCNAME, PROCLINE
- GETMV, GETNEWPAR, GETENV
- FILE, MAKEDIR, CURDIR, GETSRVNAME

## Teste de Funções TDN

O arquivo `tests/tdn_functions_test.prw` contém testes abrangentes das funções TDN implementadas.

**Resultado do Teste:**
```
=========================================
Teste de Funcoes TDN do TOTVS
=========================================

--- Teste 1: LEFT ---
LEFT('TOTVS Protheus', 5) = TOTVS

--- Teste 2: RIGHT ---
RIGHT('TOTVS Protheus', 7) = rotheus

--- Teste 3: REPLICA ---
REPLICA('AB', 3) = ABABAB

--- Teste 4: CAPSLOCK ---
CAPSLOCK('totvs') = Totvs

--- Teste 5: PROPER ---
PROPER('totvs protheus') = Totvs Protheus

--- Teste 6: ATC ---
ATC('totvs', 'TOTVS PROTHEUS') = 1

--- Teste 7: RATC ---
RATC('s', 'TOTVS PROTHEUS') = 14

--- Teste 8: GETWORDNUM ---
GETWORDNUM('TOTVS PROTHEUS', 2) = PROTHEUS

--- Teste 9: WORDS ---
WORDS('TOTVS PROTHEUS') = 2

--- Teste 10: FILENOEXT ---
FILENOEXT('/path/to/file.txt') = /path/to/file

--- Teste 11: FILEEXT ---
FILEEXT('/path/to/file.txt') = txt

--- Teste 12: FILENAME ---
FILENAME('/path/to/file.txt') = file.txt

--- Teste 13: FILEPATH ---
FILEPATH('/path/to/file.txt') = /path/to/

--- Teste 14: FILEDIR ---
FILEDIR('/path/to/file.txt') = /path/to

--- Teste 15: SIGN ---
SIGN(10) = 1
SIGN(-5) = -1
SIGN(0) = 0

--- Teste 16: POWER ---
POWER(2, 3) = 8

--- Teste 17: PI ---
PI() = 3.141592653589793

--- Teste 18: SIN ---
SIN(0) = 0

--- Teste 19: COS ---
COS(0) = 1

--- Teste 20: STOD ---
STOD('20240101') = 01/01/2024

--- Teste 21: ELAPTIME ---
ELAPTIME(100, 150) = 50

--- Teste 22: CTOT ---
CTOT('12:30:45') = 45045

--- Teste 23: TTOC ---
TTOC(45000) = 12:30:00

=========================================
Teste de funcoes TDN concluido!
Todas as funcoes TDN funcionam
=========================================
```

## Compatibilidade

| Categoria | Funções Implementadas | Status |
|-----------|---------------------|--------|
| String (TDN) | 14 | ✅ 100% |
| Matemática (TDN) | 11 | ✅ 100% |
| Data (TDN) | 4 | ✅ 100% |
| String (Geral) | 30+ | ✅ 100% |
| Matemática (Geral) | 12+ | ✅ 100% |
| Data (Geral) | 10+ | ✅ 100% |
| Array | 10+ | ✅ 100% |
| Banco de Dados | 20+ | ✅ Stubs |
| MVC | 8+ | ✅ Estruturas |
| JSON | 1 | ✅ Estrutura |

## Limitações

1. **Banco de Dados**: Funções de banco de dados são stubs e requerem implementação real
2. **MVC**: Classes MVC são estruturas de dados, renderização visual requer integração
3. **JSON**: Suporte básico, métodos avançados requerem implementação adicional
4. **Eventos**: Manipuladores de eventos não estão conectados automaticamente

## Próximos Passos

1. Implementar funções de banco de dados reais
2. Adicionar renderização visual para classes MVC
3. Implementar métodos JSON avançados
4. Conectar manipuladores de eventos
5. Adicionar mais funções matemáticas e de string conforme necessário
