# Changelog

Todas as mudanças relevantes no projeto AdvPP.

## [4.3.1] - 2026-09-30

### Added
- ReplaceAPO dinâmico com suporte a mudanças de tamanho
- Recriptografia automática após modificação
- Heurística aprimorada de identificação de ciphers
- Testes unitários para injector (7 testes)
- Makefile com targets RPO
- Scripts de automação (capture_and_inject.sh)

### Changed
- Parser APO com validação melhorada
- Documentation atualizada
- Error handling aprimorado

### Fixed
- Bug de compilação `-compile` (falta RPODB no INI)
- Problemas de slice bounds no parser
- Tratamento de edge cases na reconstrução do body

### Performance
- Parse RPO 22KB: 40µs
- Parse RPO 9.5MB: 9.6ms
- Memória otimizada

## [4.3.0] - 2026-09-30

### Added
- Parser completo de estrutura RPO
- Parser de registros APO
- CLI `advplc rpo inject`
- Support para 24+ ciphers OpenSSL
- Sistema de captura LD_PRELOAD

### Changed
- Refatoração do código para modularidade
- Melhorias na documentação

## [4.2.0] - 2026-09-29

### Added
- Heurística de identificação de cipher
- Mapeamento key/IV size → cipher
- Testes de heurística

## [4.1.0] - 2026-09-28

### Added
- Serializer JSON→binário
- Geração de bytecodes
- Comandos CLI básicos

## [4.0.0] - 2026-09-26

### Added
- Infraestrutura inicial
- Parser RPO básico
- Hook LD_PRELOAD
- Testes unitários

---

## Notas de Versão

### 4.3.1
- **Destaque:** ReplaceAPO dinâmico
- **Testes:** 17/17 passing
- **Performance:** <10ms para RPOs até 10MB

### 4.3.0
- **Destaque:** Parser completo
- **Novo:** Suporte a múltiplos ciphers
- **Testes:** Infraestrutura de testes

---

*Changelog do projeto AdvPP*
