# Limitações arquiteturais conhecidas (VM AdvPP)

Descobertas durante a implementação da série de cobertura íntegra do TDN
AdvPL (`docs/superpowers/plans/2026-08-09-advpp-tdn-integral-plan.md`).
Funções cujo comportamento documentado no TDN depende de uma dessas
limitações devem implementar o que for possível e documentar o gap
explicitamente no código — não inventar workarounds ad-hoc por função.

## Parâmetros por referência (`@var`) em natives

Native functions recebem `[]advplrt.Value` (cópias) e retornam um único
`advplrt.Value`. Não existe, em nenhum lugar do VM/compilador, um
mecanismo para uma native mutar diretamente uma variável do chamador
passada com `@`. Descoberto na Task 5 (`IPCWaitEx`, que segundo o TDN
recupera dados via parâmetros por referência).

Funções afetadas até agora: `IPCWaitEx` (pkg/vm/execucaoprocessos_native.go);
`SFTPDirLs`, `SFTPDwld1`, `SFTPDwld2`, `SFTPUpld1`, `SFTPUpld2`
(pkg/vm/sftp_native.go) — o parâmetro final `@sError`/`@cError` de todas as cinco
não é populado; o valor de retorno principal (array/código de status) permanece
correto e é a forma suportada de checar sucesso/falha.

`SFTPDirLs`/`SFTPDwld1`/`SFTPDwld2`/`SFTPUpld1`/`SFTPUpld2` também documentam, no
código (`pkg/vm/sftp_native.go`), um trade-off de segurança deliberado: a
verificação de host key usa `ssh.InsecureIgnoreHostKey()` por padrão (a TDN não
oferece parâmetro de known_hosts/fingerprint nestas 5 funções), com um escape
hatch opcional via `ADVPP_SFTP_KNOWN_HOSTS` para quem quiser verificação estrita.

Se uma task futura encontrar uma função cujo comportamento central depende
de mutar `@var`, documente o gap da mesma forma (comentário explicando
o que funciona e o que não funciona) em vez de tentar implementar às
cegas — e adicione uma linha aqui.
