package vm

import (
	"os"
	"os/user"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerControledeacessoNatives registra funções de controle de acesso:
// ADUserValid, ComputerName, GetAuthArgs, LogUserName.
func (v *VM) registerControledeacessoNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// ADUserValid(cDomainName, cUserName, cPassword) -> lValid
	// Valida credenciais de usuário no Active Directory.
	// Retorna .T. se autenticação bem-sucedida, .F. caso contrário.
	// NOTA: Implementação atual retorna .F. pois AdvPP (Go) não possui integração
	// real com AD/LDAP. Ver docs/tdn-known-limitations.md para detalhes.
	natives["ADUSERVALID"] = func(args []advplrt.Value) (advplrt.Value, error) {
		domainOrSID := getArg(args, 0)
		userName := getArg(args, 1)
		password := getArg(args, 2)

		// Converte argumentos para string
		domainStr := ""
		userStr := ""
		passStr := ""

		if domainVal, ok := domainOrSID.(*advplrt.StringValue); ok {
			domainStr = domainVal.Val
		}
		if userVal, ok := userName.(*advplrt.StringValue); ok {
			userStr = userVal.Val
		}
		if passVal, ok := password.(*advplrt.StringValue); ok {
			passStr = passVal.Val
		}

		// Validação básica: argumentos não devem ser todos vazios
		if domainStr == "" && userStr == "" && passStr == "" {
			return advplrt.NewBool(false), nil
		}

		// AdvPP não possui integração com AD/LDAP: retorna .F. conservativamente.
		// Ver docs/tdn-known-limitations.md para detalhes.
		return advplrt.NewBool(false), nil
	}

	// ComputerName() -> cRet
	// Retorna o nome da máquina (hostname) onde o cliente está sendo executado.
	natives["COMPUTERNAME"] = func(args []advplrt.Value) (advplrt.Value, error) {
		hostname, err := os.Hostname()
		if err != nil {
			// Se não conseguir obter o hostname, retorna string vazia
			return advplrt.NewString(""), nil
		}
		return advplrt.NewString(hostname), nil
	}

	// GetAuthArgs() -> oAuthMap
	// Recupera parâmetros utilizados para autenticação.
	// Retorna um objeto (THashMap) com chaves como "ECPF", "SAML".
	// NOTA: AdvPP retorna THashMap vazio pois não possui autenticação real (e-CPF, SAML).
	// Ver docs/tdn-known-limitations.md
	natives["GETAUTHARGS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		// Cria objeto do tipo THASMAP (simulando THashMap do Protheus)
		authMap := advplrt.NewObject("THASMAP", nil)
		// Em contexto de AdvPP, nenhuma credencial real disponível para popular o mapa
		return authMap, nil
	}

	// LogUserName() -> cRet
	// Obtém o nome do usuário logado no sistema operacional.
	// Retorna uma string com o login do usuário.
	natives["LOGUSERNAME"] = func(args []advplrt.Value) (advplrt.Value, error) {
		currentUser, err := user.Current()
		if err != nil {
			// Se não conseguir, tenta variável de ambiente
			username := os.Getenv("USER")
			if username == "" {
				username = os.Getenv("USERNAME") // Windows
			}
			return advplrt.NewString(username), nil
		}
		return advplrt.NewString(currentUser.Username), nil
	}
}
