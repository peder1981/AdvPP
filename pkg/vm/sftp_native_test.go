package vm

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// --- Infraestrutura de teste: servidor SFTP embutido em processo ---
//
// Não há servidor SFTP real disponível neste ambiente de testes unitários. Como
// golang.org/x/crypto/ssh e github.com/pkg/sftp suportam nativamente o papel de
// servidor (não só de cliente), os testes abaixo sobem um servidor SSH+SFTP real
// (em goroutine, em uma porta local efêmera, servindo um diretório temporário) e
// conectam os natives contra ele de ponta a ponta — cobrindo o caminho de rede real
// (handshake SSH, autenticação por senha e por chave, protocolo SFTP), não apenas
// validação de argumentos.
//
// O que NÃO é (e não pretende ser) coberto aqui: comportamento contra um servidor
// SFTP de produção real (OpenSSH, servidores gerenciados, etc.) — políticas de
// host key reais, latência de rede, servidores com extensões SFTP não-padrão, ou
// interoperabilidade com todo o universo de implementações SFTP existentes. Isso
// fica fora do escopo desta suíte de testes unitários e requer teste de integração
// contra um servidor real/containerizado.

func generateTestHostKey(t *testing.T) ssh.Signer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("falha ao gerar chave de host de teste: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("falha ao criar signer: %v", err)
	}
	return signer
}

// testSFTPServer descreve um servidor SFTP de teste rodando em processo.
type testSFTPServer struct {
	Addr    string
	Dir     string
	HostKey ssh.PublicKey // chave pública de host, para testes de known_hosts
}

// startTestSFTPServerPassword sobe um servidor SFTP de teste que aceita apenas
// autenticação por usuário/senha (user/password esperados).
func startTestSFTPServerPassword(t *testing.T, user, password string) testSFTPServer {
	t.Helper()
	dir := t.TempDir()
	hostKey := generateTestHostKey(t)

	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == user && string(pass) == password {
				return nil, nil
			}
			return nil, errors.New("permission denied")
		},
	}
	config.AddHostKey(hostKey)

	srv := startTestSFTPListener(t, config, dir)
	srv.HostKey = hostKey.PublicKey()
	return srv
}

// startTestSFTPServerKey sobe um servidor SFTP de teste que aceita apenas
// autenticação por chave pública. Grava a chave privada de cliente em um arquivo
// PEM temporário e retorna seu caminho junto com o endereço do servidor.
func startTestSFTPServerKey(t *testing.T, user string) (testSFTPServer, string) {
	t.Helper()
	dir := t.TempDir()
	hostKey := generateTestHostKey(t)

	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("falha ao gerar chave de cliente de teste: %v", err)
	}
	clientSigner, err := ssh.NewSignerFromKey(clientKey)
	if err != nil {
		t.Fatalf("falha ao criar signer de cliente: %v", err)
	}
	clientPub := clientSigner.PublicKey()

	pemPath := filepath.Join(t.TempDir(), "id_rsa")
	pemBytes := marshalRSAPrivateKeyPEM(t, clientKey)
	if err := os.WriteFile(pemPath, pemBytes, 0o600); err != nil {
		t.Fatalf("falha ao gravar chave privada de teste: %v", err)
	}

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(c ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
			if c.User() == user && string(pubKey.Marshal()) == string(clientPub.Marshal()) {
				return nil, nil
			}
			return nil, errors.New("permission denied")
		},
	}
	config.AddHostKey(hostKey)

	return startTestSFTPListener(t, config, dir), pemPath
}

func startTestSFTPListener(t *testing.T, config *ssh.ServerConfig, dir string) testSFTPServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("falha ao abrir listener de teste: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			netConn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleTestSFTPConn(netConn, config)
		}
	}()

	return testSFTPServer{Addr: listener.Addr().String(), Dir: dir}
}

func handleTestSFTPConn(netConn net.Conn, config *ssh.ServerConfig) {
	sshConn, chans, reqs, err := ssh.NewServerConn(netConn, config)
	if err != nil {
		netConn.Close()
		return
	}
	defer sshConn.Close()
	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unsupported channel type")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}
		go func(in <-chan *ssh.Request) {
			for req := range in {
				ok := req.Type == "subsystem"
				if ok {
					var payload struct{ Name string }
					ssh.Unmarshal(req.Payload, &payload)
					if payload.Name != "sftp" {
						ok = false
					}
				}
				req.Reply(ok, nil)
				if ok {
					server, err := sftp.NewServer(channel)
					if err == nil {
						server.Serve()
					}
					channel.Close()
				}
			}
		}(requests)
	}
}

func marshalRSAPrivateKeyPEM(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	return pem.EncodeToMemory(block)
}

// --- Testes ---

func TestSFTPDirLs(t *testing.T) {
	srv := startTestSFTPServerPassword(t, "user", "password")
	if err := os.Mkdir(filepath.Join(srv.Dir, "sub"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srv.Dir, "arquivo.txt"), []byte("conteudo"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["SFTPDIRLS"].Fn([]advplrt.Value{
		advplrt.NewString(srv.Addr),
		advplrt.NewString(srv.Dir),
		advplrt.NewString("user"),
		advplrt.NewString("password"),
	})
	if err != nil {
		t.Fatalf("SFTPDirLs retornou erro Go: %v", err)
	}

	arr, ok := got.(*advplrt.ArrayValue)
	if !ok {
		t.Fatalf("SFTPDirLs = %v (%T), esperava array", got, got)
	}
	names := map[string]bool{}
	for _, el := range arr.Elements {
		names[advplrt.ToString(el)] = true
	}
	if !names["sub"] || !names["arquivo.txt"] {
		t.Errorf("SFTPDirLs = %v, esperava conter 'sub' e 'arquivo.txt'", names)
	}
}

func TestSFTPDirLsCaminhoVazio(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["SFTPDIRLS"].Fn([]advplrt.Value{
		advplrt.NewString("algumhost"),
		advplrt.NewString(""),
		advplrt.NewString("user"),
		advplrt.NewString("password"),
	})
	if err != nil {
		t.Fatalf("erro Go inesperado: %v", err)
	}
	n, ok := got.(*advplrt.NumberValue)
	if !ok || n.Val != -1111 {
		t.Errorf("SFTPDirLs com sRemotePath vazio = %v, quer -1111", got)
	}
}

func TestSFTPDirLsAutenticacaoInvalida(t *testing.T) {
	srv := startTestSFTPServerPassword(t, "user", "password")

	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["SFTPDIRLS"].Fn([]advplrt.Value{
		advplrt.NewString(srv.Addr),
		advplrt.NewString(srv.Dir),
		advplrt.NewString("user"),
		advplrt.NewString("senha-errada"),
	})
	if err != nil {
		t.Fatalf("erro Go inesperado: %v", err)
	}
	n, ok := got.(*advplrt.NumberValue)
	if !ok || n.Val == 0 {
		t.Errorf("SFTPDirLs com senha errada = %v, quer código de erro != 0", got)
	}
}

func TestSFTPDwld1(t *testing.T) {
	srv := startTestSFTPServerPassword(t, "user", "password")
	remoteFile := filepath.Join(srv.Dir, "origem.txt")
	if err := os.WriteFile(remoteFile, []byte("dados de teste"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	localFile := filepath.Join(t.TempDir(), "destino.txt")

	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["SFTPDWLD1"].Fn([]advplrt.Value{
		advplrt.NewString(localFile),
		advplrt.NewString(remoteFile),
		advplrt.NewString(srv.Addr),
		advplrt.NewString("user"),
		advplrt.NewString("password"),
	})
	if err != nil {
		t.Fatalf("erro Go inesperado: %v", err)
	}
	n, ok := got.(*advplrt.NumberValue)
	if !ok || n.Val != 0 {
		t.Fatalf("SFTPDwld1 = %v, quer 0 (sucesso)", got)
	}

	content, err := os.ReadFile(localFile)
	if err != nil {
		t.Fatalf("arquivo local não foi criado: %v", err)
	}
	if string(content) != "dados de teste" {
		t.Errorf("conteúdo baixado = %q, quer %q", content, "dados de teste")
	}
}

func TestSFTPDwld1ArquivoRemotoInexistente(t *testing.T) {
	srv := startTestSFTPServerPassword(t, "user", "password")
	localFile := filepath.Join(t.TempDir(), "destino.txt")

	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["SFTPDWLD1"].Fn([]advplrt.Value{
		advplrt.NewString(localFile),
		advplrt.NewString(filepath.Join(srv.Dir, "nao-existe.txt")),
		advplrt.NewString(srv.Addr),
		advplrt.NewString("user"),
		advplrt.NewString("password"),
	})
	if err != nil {
		t.Fatalf("erro Go inesperado: %v", err)
	}
	n, ok := got.(*advplrt.NumberValue)
	if !ok || n.Val == 0 {
		t.Errorf("SFTPDwld1 de arquivo inexistente = %v, quer código de erro != 0", got)
	}
}

func TestSFTPDwld1ParametrosObrigatoriosFaltando(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["SFTPDWLD1"].Fn([]advplrt.Value{
		advplrt.NewString(""),
		advplrt.NewString("/home/user/x.txt"),
		advplrt.NewString("host"),
		advplrt.NewString("user"),
		advplrt.NewString("password"),
	})
	if err != nil {
		t.Fatalf("erro Go inesperado: %v", err)
	}
	n, ok := got.(*advplrt.NumberValue)
	if !ok || n.Val != -1111 {
		t.Errorf("SFTPDwld1 com sFileName vazio = %v, quer -1111", got)
	}
}

func TestSFTPUpld1(t *testing.T) {
	srv := startTestSFTPServerPassword(t, "user", "password")
	localFile := filepath.Join(t.TempDir(), "origem.txt")
	if err := os.WriteFile(localFile, []byte("upload de teste"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	remoteFile := filepath.Join(srv.Dir, "sub", "destino.txt")

	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["SFTPUPLD1"].Fn([]advplrt.Value{
		advplrt.NewString(localFile),
		advplrt.NewString(remoteFile),
		advplrt.NewString(srv.Addr),
		advplrt.NewString("user"),
		advplrt.NewString("password"),
	})
	if err != nil {
		t.Fatalf("erro Go inesperado: %v", err)
	}
	n, ok := got.(*advplrt.NumberValue)
	if !ok || n.Val != 0 {
		t.Fatalf("SFTPUpld1 = %v, quer 0 (sucesso)", got)
	}

	content, err := os.ReadFile(remoteFile)
	if err != nil {
		t.Fatalf("arquivo remoto não foi criado: %v", err)
	}
	if string(content) != "upload de teste" {
		t.Errorf("conteúdo enviado = %q, quer %q", content, "upload de teste")
	}
}

func TestSFTPUpld1ArquivoLocalInexistente(t *testing.T) {
	srv := startTestSFTPServerPassword(t, "user", "password")

	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["SFTPUPLD1"].Fn([]advplrt.Value{
		advplrt.NewString(filepath.Join(t.TempDir(), "nao-existe.txt")),
		advplrt.NewString(filepath.Join(srv.Dir, "destino.txt")),
		advplrt.NewString(srv.Addr),
		advplrt.NewString("user"),
		advplrt.NewString("password"),
	})
	if err != nil {
		t.Fatalf("erro Go inesperado: %v", err)
	}
	n, ok := got.(*advplrt.NumberValue)
	if !ok || n.Val != -1111 {
		t.Errorf("SFTPUpld1 com arquivo local inexistente = %v, quer -1111", got)
	}
}

func TestSFTPDwld2ComChavePEM(t *testing.T) {
	srv, keyPath := startTestSFTPServerKey(t, "user")
	remoteFile := filepath.Join(srv.Dir, "origem.txt")
	if err := os.WriteFile(remoteFile, []byte("dados via chave"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	localFile := filepath.Join(t.TempDir(), "destino.txt")

	t.Setenv("ADVPP_SFTP_PRIVATE_KEY", keyPath)

	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["SFTPDWLD2"].Fn([]advplrt.Value{
		advplrt.NewString(localFile),
		advplrt.NewString(remoteFile),
		advplrt.NewString(srv.Addr),
		advplrt.NewString("user"),
	})
	if err != nil {
		t.Fatalf("erro Go inesperado: %v", err)
	}
	n, ok := got.(*advplrt.NumberValue)
	if !ok || n.Val != 0 {
		t.Fatalf("SFTPDwld2 = %v, quer 0 (sucesso)", got)
	}

	content, err := os.ReadFile(localFile)
	if err != nil {
		t.Fatalf("arquivo local não foi criado: %v", err)
	}
	if string(content) != "dados via chave" {
		t.Errorf("conteúdo baixado = %q, quer %q", content, "dados via chave")
	}
}

func TestSFTPDwld2SemChaveConfigurada(t *testing.T) {
	t.Setenv("ADVPP_SFTP_PRIVATE_KEY", "")

	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["SFTPDWLD2"].Fn([]advplrt.Value{
		advplrt.NewString(filepath.Join(t.TempDir(), "destino.txt")),
		advplrt.NewString("/home/user/origem.txt"),
		advplrt.NewString("algumhost"),
		advplrt.NewString("user"),
	})
	if err != nil {
		t.Fatalf("erro Go inesperado: %v", err)
	}
	n, ok := got.(*advplrt.NumberValue)
	if !ok || n.Val == 0 {
		t.Errorf("SFTPDwld2 sem ADVPP_SFTP_PRIVATE_KEY = %v, quer código de erro != 0", got)
	}
}

func TestSFTPUpld2ComChavePEM(t *testing.T) {
	srv, keyPath := startTestSFTPServerKey(t, "user")
	localFile := filepath.Join(t.TempDir(), "origem.txt")
	if err := os.WriteFile(localFile, []byte("upload via chave"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	remoteFile := filepath.Join(srv.Dir, "destino.txt")

	t.Setenv("ADVPP_SFTP_PRIVATE_KEY", keyPath)

	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["SFTPUPLD2"].Fn([]advplrt.Value{
		advplrt.NewString(localFile),
		advplrt.NewString(remoteFile),
		advplrt.NewString(srv.Addr),
		advplrt.NewString("user"),
	})
	if err != nil {
		t.Fatalf("erro Go inesperado: %v", err)
	}
	n, ok := got.(*advplrt.NumberValue)
	if !ok || n.Val != 0 {
		t.Fatalf("SFTPUpld2 = %v, quer 0 (sucesso)", got)
	}

	content, err := os.ReadFile(remoteFile)
	if err != nil {
		t.Fatalf("arquivo remoto não foi criado: %v", err)
	}
	if string(content) != "upload via chave" {
		t.Errorf("conteúdo enviado = %q, quer %q", content, "upload via chave")
	}
}

func TestSFTPUpld2ParametrosObrigatoriosFaltando(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["SFTPUPLD2"].Fn([]advplrt.Value{
		advplrt.NewString("/local/arquivo.txt"),
		advplrt.NewString(""),
		advplrt.NewString("host"),
		advplrt.NewString("user"),
	})
	if err != nil {
		t.Fatalf("erro Go inesperado: %v", err)
	}
	n, ok := got.(*advplrt.NumberValue)
	if !ok || n.Val != -1111 {
		t.Errorf("SFTPUpld2 com sRemotePath vazio = %v, quer -1111", got)
	}
}

// TestSFTPConnectComKnownHostsOptIn cobre o escape hatch ADVPP_SFTP_KNOWN_HOSTS:
// quando configurado com a chave de host correta, a conexão deve funcionar
// normalmente (verificação estrita passa); quando configurado com uma chave
// diferente da real, a conexão deve falhar (verificação estrita rejeita o MITM
// simulado). Sem a variável definida (comportamento default, já coberto pelos
// demais testes desta suíte), a verificação é pulada.
func TestSFTPConnectComKnownHostsOptIn(t *testing.T) {
	srv := startTestSFTPServerPassword(t, "user", "password")
	if err := os.WriteFile(filepath.Join(srv.Dir, "arquivo.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	t.Run("chave correta no known_hosts: conecta normalmente", func(t *testing.T) {
		knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
		line := knownhosts.Line([]string{srv.Addr}, srv.HostKey)
		if err := os.WriteFile(knownHostsPath, []byte(line+"\n"), 0o600); err != nil {
			t.Fatalf("setup known_hosts: %v", err)
		}
		t.Setenv("ADVPP_SFTP_KNOWN_HOSTS", knownHostsPath)

		v := NewVM(&compiler.Bytecode{}, false)
		got, err := v.natives["SFTPDIRLS"].Fn([]advplrt.Value{
			advplrt.NewString(srv.Addr),
			advplrt.NewString(srv.Dir),
			advplrt.NewString("user"),
			advplrt.NewString("password"),
		})
		if err != nil {
			t.Fatalf("erro Go inesperado: %v", err)
		}
		if _, ok := got.(*advplrt.ArrayValue); !ok {
			t.Errorf("SFTPDirLs com known_hosts correto = %v (%T), esperava array (conexão deveria ter passado na verificação estrita)", got, got)
		}
	})

	t.Run("chave diferente no known_hosts: rejeita a conexão", func(t *testing.T) {
		wrongKey := generateTestHostKey(t)
		knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
		line := knownhosts.Line([]string{srv.Addr}, wrongKey.PublicKey())
		if err := os.WriteFile(knownHostsPath, []byte(line+"\n"), 0o600); err != nil {
			t.Fatalf("setup known_hosts: %v", err)
		}
		t.Setenv("ADVPP_SFTP_KNOWN_HOSTS", knownHostsPath)

		v := NewVM(&compiler.Bytecode{}, false)
		got, err := v.natives["SFTPDIRLS"].Fn([]advplrt.Value{
			advplrt.NewString(srv.Addr),
			advplrt.NewString(srv.Dir),
			advplrt.NewString("user"),
			advplrt.NewString("password"),
		})
		if err != nil {
			t.Fatalf("erro Go inesperado: %v", err)
		}
		if _, ok := got.(*advplrt.ArrayValue); ok {
			t.Errorf("SFTPDirLs com known_hosts incorreto = array, esperava código de erro (verificação estrita deveria ter rejeitado a chave divergente)")
		}
		n, ok := got.(*advplrt.NumberValue)
		if !ok || n.Val == 0 {
			t.Errorf("SFTPDirLs com known_hosts incorreto = %v, quer código de erro != 0", got)
		}
	})
}

func TestSftpStatusFromErr(t *testing.T) {
	if got := sftpStatusFromErr(nil); got != 0 {
		t.Errorf("sftpStatusFromErr(nil) = %v, quer 0", got)
	}
}
