package vm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// sftpDialTimeout é o timeout de rede + handshake SSH aplicado a toda conexão SFTP
// aberta por esta categoria de natives.
const sftpDialTimeout = 30 * time.Second

// registerInterfaceSFTPNatives registra SFTPDirLs, SFTPDwld1, SFTPDwld2, SFTPUpld1
// e SFTPUpld2 (categoria TDN "Interface-SFTP"). O transporte real é
// golang.org/x/crypto/ssh (cliente SSH) + github.com/pkg/sftp (protocolo SFTP sobre
// o canal SSH) — nenhum protocolo é reimplementado à mão.
//
// TRADE-OFF DE SEGURANÇA (documentado, deliberado): nenhuma das cinco funções recebe,
// na especificação TDN, um parâmetro de fingerprint/known_hosts para validar a chave
// do host remoto. Para preservar o comportamento de "conectar em qualquer host
// informado pelos parâmetros" que a TDN descreve, a verificação de host key usa
// ssh.InsecureIgnoreHostKey() — ou seja, um ataque MITM ativo entre o AppServer e o
// host SFTP não seria detectado por esta implementação. Isto replica a mesma postura
// pragmática de httpclient_native.go (onde a validação de certificado É feita, pois a
// API HTTP tem como validar; aqui a API SFTP do TDN simplesmente não oferece meio de
// especificar known_hosts). Se uma revisão futura da TDN destas funções vier a
// documentar um parâmetro de host-key/fingerprint, ele deve substituir isto.
//
// ESCAPE HATCH: um chamador que controla seu próprio deployment e quer verificação
// estrita de host key pode definir a variável de ambiente ADVPP_SFTP_KNOWN_HOSTS
// apontando para um arquivo known_hosts (mesmo formato do OpenSSH). Quando definida,
// a conexão passa a validar a chave do host via golang.org/x/crypto/ssh/knownhosts em
// vez de ignorá-la — ver sftpHostKeyCallback. O default (variável não definida)
// permanece inalterado.
//
// LIMITAÇÃO CONHECIDA (ver docs/tdn-known-limitations.md): o parâmetro final de todas
// as cinco funções é "por referência" (@sError/@cError) e deveria devolver ao chamador
// uma mensagem de erro detalhada. O VM do AdvPP não tem mecanismo para nativas
// mutarem uma variável do chamador passada com @ (mesma limitação documentada para
// IPCWaitEx). O valor de retorno principal (array de SFTPDirLs; código numérico de
// status das demais) continua correto e é a forma suportada de checar sucesso/falha.
func (v *VM) registerInterfaceSFTPNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// SFTPDirLs(sServer, sRemotePath, sUser, sPassword, [@sError]) -> aResult
	// Lista arquivos/pastas de sRemotePath em um servidor SFTP autenticado por
	// usuário/senha. Retorna um array de nomes, ou um número (código de status,
	// ver sftpStatusFromErr) em caso de falha.
	natives["SFTPDIRLS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return sftpDirLs(args), nil
	}

	// SFTPDwld1(sFileName, sRemotePath, sServer, sUser, sPassword, [@sError]) -> nStatus
	// Baixa sRemotePath (servidor SFTP, autenticação usuário/senha) para sFileName
	// (caminho local, no Totvs Application Server). Retorna 0 em sucesso, ou um dos
	// códigos numéricos de erro documentados na TDN.
	natives["SFTPDWLD1"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return sftpDwld1(args), nil
	}

	// SFTPDwld2(sFileName, sRemotePath, sServer, sUser, [@cError]) -> nStatus
	// Baixa sRemotePath para sFileName usando autenticação por chave privada PEM.
	// A TDN carrega a chave via [SFTP] PrivateKey/PublicKey/Certpassword do INI do
	// AppServer; como o AdvPP não modela um appserver.ini, a chave é lida das
	// variáveis de ambiente ADVPP_SFTP_PRIVATE_KEY (caminho da chave privada) e,
	// opcionalmente, ADVPP_SFTP_CERT_PASSWORD (senha da chave, equivalente a
	// Certpassword). Isto é uma adaptação documentada do mecanismo de configuração,
	// não uma limitação funcional: a autenticação por chave funciona de ponta a ponta.
	natives["SFTPDWLD2"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return sftpDwld2(args), nil
	}

	// SFTPUpld1(sFileName, sRemotePath, sServer, sUser, sPassword, [@cError]) -> nStatus
	// Envia sFileName (local, no Totvs Application Server) para sRemotePath em um
	// servidor SFTP, autenticação usuário/senha.
	natives["SFTPUPLD1"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return sftpUpld1(args), nil
	}

	// SFTPUpld2(sFileName, sRemotePath, sServer, sUser, [@sError]) -> nStatus
	// Envia sFileName para sRemotePath usando autenticação por chave privada PEM
	// (mesma fonte de configuração via ambiente descrita em SFTPDwld2).
	natives["SFTPUPLD2"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return sftpUpld2(args), nil
	}
}

// sftpAddr normaliza o endereço do servidor, acrescentando a porta SSH padrão (22)
// quando o chamador não especificou uma explicitamente.
func sftpAddr(server string) string {
	if strings.Contains(server, ":") {
		return server
	}
	return server + ":22"
}

// sftpDial abre a conexão SSH e o cliente SFTP sobre ela. O chamador é responsável
// por fechar ambos (nesta ordem: client, depois conn) quando não forem mais usados.
func sftpDial(server string, config *ssh.ClientConfig) (*ssh.Client, *sftp.Client, error) {
	conn, err := ssh.Dial("tcp", sftpAddr(server), config)
	if err != nil {
		return nil, nil, err
	}
	client, err := sftp.NewClient(conn)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	return conn, client, nil
}

// sftpHostKeyCallback decide como validar a chave do host SFTP remoto.
//
// Por padrão (nenhuma configuração), replica o comportamento pragmático descrito em
// registerInterfaceSFTPNatives: ssh.InsecureIgnoreHostKey(), já que nenhuma das 5
// funções TDN oferece parâmetro de known_hosts/fingerprint. Isso permanece o default
// para preservar paridade com a especificação.
//
// ESCAPE HATCH: quando a variável de ambiente ADVPP_SFTP_KNOWN_HOSTS aponta para um
// arquivo known_hosts (mesmo formato usado por ssh/sftp de linha de comando), a
// verificação estrita via golang.org/x/crypto/ssh/knownhosts é usada em seu lugar —
// qualquer chamador que controle seu próprio deployment pode optar por segurança
// real sem alterar o comportamento default de ninguém mais.
func sftpHostKeyCallback() (ssh.HostKeyCallback, error) {
	knownHostsPath := strings.Trim(os.Getenv("ADVPP_SFTP_KNOWN_HOSTS"), " ")
	if knownHostsPath == "" {
		return ssh.InsecureIgnoreHostKey(), nil //nolint:gosec // ver comentário de segurança em registerInterfaceSFTPNatives
	}
	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("falha ao carregar ADVPP_SFTP_KNOWN_HOSTS %q: %w", knownHostsPath, err)
	}
	return callback, nil
}

// sftpConnectPassword conecta usando autenticação usuário/senha (SFTPDirLs, SFTPDwld1,
// SFTPUpld1). Ver comentário de segurança sobre InsecureIgnoreHostKey em
// registerInterfaceSFTPNatives e o escape hatch em sftpHostKeyCallback.
func sftpConnectPassword(server, user, password string) (*ssh.Client, *sftp.Client, error) {
	hostKeyCallback, err := sftpHostKeyCallback()
	if err != nil {
		return nil, nil, err
	}
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: hostKeyCallback,
		Timeout:         sftpDialTimeout,
	}
	return sftpDial(server, config)
}

// sftpConnectKey conecta usando autenticação por chave privada PEM (SFTPDwld2,
// SFTPUpld2). Ver comentário sobre ADVPP_SFTP_PRIVATE_KEY/ADVPP_SFTP_CERT_PASSWORD
// em registerInterfaceSFTPNatives.
func sftpConnectKey(server, user string) (*ssh.Client, *sftp.Client, error) {
	keyPath := strings.Trim(os.Getenv("ADVPP_SFTP_PRIVATE_KEY"), " ")
	if keyPath == "" {
		return nil, nil, fmt.Errorf("ADVPP_SFTP_PRIVATE_KEY não configurada (equivalente à chave PrivateKey da seção [SFTP] do appserver.ini no Protheus real)")
	}
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("falha ao ler chave privada %q: %w", keyPath, err)
	}

	var signer ssh.Signer
	if pass := os.Getenv("ADVPP_SFTP_CERT_PASSWORD"); pass != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(pass))
	} else {
		signer, err = ssh.ParsePrivateKey(keyData)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("falha ao decodificar chave privada: %w", err)
	}

	hostKeyCallback, err := sftpHostKeyCallback()
	if err != nil {
		return nil, nil, err
	}
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hostKeyCallback,
		Timeout:         sftpDialTimeout,
	}
	return sftpDial(server, config)
}

// sftpStatusFromErr faz um mapeamento best-effort de erros de conexão/autenticação/
// transferência da biblioteca golang.org/x/crypto/ssh + github.com/pkg/sftp (transporte
// real usado por este VM) para os códigos numéricos documentados na TDN — que na
// verdade descrevem a taxonomia de erros de libcurl/libssh2 usada pelo AppServer C++
// real. Os dois runtimes não expõem os mesmos códigos internos; esta função escolhe o
// código TDN mais próximo pelo texto/tipo do erro e cai no código genérico 87
// ("Erro SSH geral") quando não há correspondência clara. Não é (e não pretende ser)
// uma réplica byte-a-byte do runtime C++ original.
func sftpStatusFromErr(err error) float64 {
	if err == nil {
		return 0
	}
	if os.IsNotExist(err) {
		return 94
	}
	if os.IsPermission(err) {
		return 101
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "unable to authenticate"), strings.Contains(msg, "authentication failed"):
		return 84
	case strings.Contains(msg, "connection refused"):
		return 67
	case strings.Contains(msg, "no such file"), strings.Contains(msg, "not exist"), strings.Contains(msg, "no such directory"):
		return 94
	case strings.Contains(msg, "permission denied"):
		return 101
	case strings.Contains(msg, "no route to host"), strings.Contains(msg, "no such host"),
		strings.Contains(msg, "network is unreachable"), strings.Contains(msg, "i/o timeout"):
		return 87
	default:
		return 87
	}
}

// sftpDirLs implementa SFTPDirLs.
func sftpDirLs(args []advplrt.Value) advplrt.Value {
	server := strings.Trim(getArgString(args, 0, ""), " ")
	remotePath := strings.Trim(getArgString(args, 1, ""), " ")
	user := getArgString(args, 2, "")
	password := getArgString(args, 3, "")

	if server == "" || remotePath == "" {
		return advplrt.NewNumber(-1111)
	}

	conn, client, err := sftpConnectPassword(server, user, password)
	if err != nil {
		return advplrt.NewNumber(sftpStatusFromErr(err))
	}
	defer conn.Close()
	defer client.Close()

	entries, err := client.ReadDir(remotePath)
	if err != nil {
		return advplrt.NewNumber(sftpStatusFromErr(err))
	}

	names := make([]advplrt.Value, 0, len(entries))
	for _, e := range entries {
		names = append(names, advplrt.NewString(e.Name()))
	}
	return advplrt.NewArray(names)
}

// sftpDwld1 implementa SFTPDwld1 (download, autenticação usuário/senha).
func sftpDwld1(args []advplrt.Value) advplrt.Value {
	localPath := strings.Trim(getArgString(args, 0, ""), " ")
	remotePath := strings.Trim(getArgString(args, 1, ""), " ")
	server := strings.Trim(getArgString(args, 2, ""), " ")
	user := getArgString(args, 3, "")
	password := getArgString(args, 4, "")

	if localPath == "" || remotePath == "" || server == "" {
		return advplrt.NewNumber(-1111)
	}

	conn, client, err := sftpConnectPassword(server, user, password)
	if err != nil {
		return advplrt.NewNumber(sftpStatusFromErr(err))
	}
	defer conn.Close()
	defer client.Close()

	return advplrt.NewNumber(sftpDownload(client, remotePath, localPath))
}

// sftpDwld2 implementa SFTPDwld2 (download, autenticação por chave PEM).
func sftpDwld2(args []advplrt.Value) advplrt.Value {
	localPath := strings.Trim(getArgString(args, 0, ""), " ")
	remotePath := strings.Trim(getArgString(args, 1, ""), " ")
	server := strings.Trim(getArgString(args, 2, ""), " ")
	user := getArgString(args, 3, "")

	if localPath == "" || remotePath == "" || server == "" {
		return advplrt.NewNumber(-1111)
	}

	conn, client, err := sftpConnectKey(server, user)
	if err != nil {
		return advplrt.NewNumber(sftpStatusFromErr(err))
	}
	defer conn.Close()
	defer client.Close()

	return advplrt.NewNumber(sftpDownload(client, remotePath, localPath))
}

// sftpDownload baixa remotePath (servidor SFTP) para localPath (disco local),
// compartilhado por SFTPDwld1 e SFTPDwld2.
func sftpDownload(client *sftp.Client, remotePath, localPath string) float64 {
	remoteFile, err := client.Open(remotePath)
	if err != nil {
		return sftpStatusFromErr(err)
	}
	defer remoteFile.Close()

	if dir := filepath.Dir(localPath); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}

	localFile, err := os.Create(localPath)
	if err != nil {
		return -1112
	}
	defer localFile.Close()

	if _, err := remoteFile.WriteTo(localFile); err != nil {
		return sftpStatusFromErr(err)
	}
	return 0
}

// sftpUpld1 implementa SFTPUpld1 (upload, autenticação usuário/senha).
func sftpUpld1(args []advplrt.Value) advplrt.Value {
	localPath := strings.Trim(getArgString(args, 0, ""), " ")
	remotePath := strings.Trim(getArgString(args, 1, ""), " ")
	server := strings.Trim(getArgString(args, 2, ""), " ")
	user := getArgString(args, 3, "")
	password := getArgString(args, 4, "")

	if localPath == "" || remotePath == "" || server == "" {
		return advplrt.NewNumber(-1111)
	}

	conn, client, err := sftpConnectPassword(server, user, password)
	if err != nil {
		return advplrt.NewNumber(sftpStatusFromErr(err))
	}
	defer conn.Close()
	defer client.Close()

	return advplrt.NewNumber(sftpUpload(client, localPath, remotePath))
}

// sftpUpld2 implementa SFTPUpld2 (upload, autenticação por chave PEM).
func sftpUpld2(args []advplrt.Value) advplrt.Value {
	localPath := strings.Trim(getArgString(args, 0, ""), " ")
	remotePath := strings.Trim(getArgString(args, 1, ""), " ")
	server := strings.Trim(getArgString(args, 2, ""), " ")
	user := getArgString(args, 3, "")

	if localPath == "" || remotePath == "" || server == "" {
		return advplrt.NewNumber(-1111)
	}

	conn, client, err := sftpConnectKey(server, user)
	if err != nil {
		return advplrt.NewNumber(sftpStatusFromErr(err))
	}
	defer conn.Close()
	defer client.Close()

	return advplrt.NewNumber(sftpUpload(client, localPath, remotePath))
}

// sftpUpload envia localPath (disco local) para remotePath (servidor SFTP),
// compartilhado por SFTPUpld1 e SFTPUpld2.
func sftpUpload(client *sftp.Client, localPath, remotePath string) float64 {
	localFile, err := os.Open(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return -1111
		}
		return -1112
	}
	defer localFile.Close()

	if dir := filepath.Dir(remotePath); dir != "" && dir != "." {
		_ = client.MkdirAll(dir)
	}

	remoteFile, err := client.Create(remotePath)
	if err != nil {
		return sftpStatusFromErr(err)
	}
	defer remoteFile.Close()

	if _, err := remoteFile.ReadFrom(localFile); err != nil {
		return sftpStatusFromErr(err)
	}
	return 0
}
