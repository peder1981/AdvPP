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
	// real com AD/LDAP. Apenas valida argumentos e documenta a limitação.
	natives["ADUSERSVALID"] = func(args []advplrt.Value) (advplrt.Value, error) {
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

		// NOTA: AdvPP (compilador Go) não possui integração com AD/LDAP.
		// Autenticação real AD/LDAP requer bibliotecas nativas (advapi32, netapi32)
		// ou bibliotecas Go especializadas (github.com/go-ldap/ldap).
		// Aqui retornamos .F. (falso) para indicar que a autenticação não foi realizada,
		// mantendo compatibilidade com código que espera um booleano como retorno.
		// Ver docs/tdn-known-limitations.md
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
	// Em contexto de teste sem autenticação real, retorna mapa vazio.
	natives["GETAUTHARGS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		// Cria objeto do tipo THASMAP (simulando THashMap do Protheus)
		authMap := advplrt.NewObject("THASMAP", nil)

		// Em contexto de AdvPP (compilador Go), não há mecanismo de autenticação
		// real (e-CPF, SAML, etc.). O mapa é retornado vazio.
		// Em produção no Protheus, este seria preenchido com credenciais reais.
		// Ver docs/tdn-known-limitations.md

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
