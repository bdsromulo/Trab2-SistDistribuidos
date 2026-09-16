package security

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/signature"
)

// montarLayout cria, sob raiz, o layout que o PersistKeys do Kauan produz:
//
//	cmd/<processo>/key/private_key.pem
//	cmd/<processo>/<produtor>-pub/public_key.pem
func montarLayout(t *testing.T, raiz, processo string) *rsa.PrivateKey {
	t.Helper()
	privada := signature.GenerateKeys()

	dirPriv := filepath.Join(raiz, "cmd", processo, "key")
	if err := os.MkdirAll(dirPriv, 0o755); err != nil {
		t.Fatal(err)
	}
	gravarPEM(t, filepath.Join(dirPriv, "private_key.pem"),
		"RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(privada))

	for _, produtor := range Produtores() {
		dirPub := filepath.Join(raiz, "cmd", processo, produtor+"-pub")
		if err := os.MkdirAll(dirPub, 0o755); err != nil {
			t.Fatal(err)
		}
		gravarPEM(t, filepath.Join(dirPub, "public_key.pem"),
			"RSA PUBLIC KEY", x509.MarshalPKCS1PublicKey(&privada.PublicKey))
	}
	return privada
}

func gravarPEM(t *testing.T, caminho, tipo string, bytes []byte) {
	t.Helper()
	f, err := os.Create(caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: tipo, Bytes: bytes}); err != nil {
		t.Fatal(err)
	}
}

func TestProdutoresTrazOsCincoSemRepetir(t *testing.T) {
	produtores := Produtores()
	if len(produtores) != 5 {
		t.Fatalf("esperava 5 produtores, veio %d: %v", len(produtores), produtores)
	}
	vistos := map[string]bool{}
	for _, p := range produtores {
		if vistos[p] {
			t.Fatalf("produtor repetido: %s", p)
		}
		vistos[p] = true
	}
	for _, esperado := range []string{events.Principal, events.Estoque, events.Pagamento, events.Entrega, events.Promocoes} {
		if !vistos[esperado] {
			t.Fatalf("faltou o produtor %s em %v", esperado, produtores)
		}
	}
}

func TestCarregarChavesLeOLayoutDoPersistKeys(t *testing.T) {
	raiz := t.TempDir()
	privada := montarLayout(t, raiz, events.Principal)

	lida, publicas, err := CarregarChaves(raiz, events.Principal)
	if err != nil {
		t.Fatalf("CarregarChaves: %v", err)
	}
	if !lida.Equal(privada) {
		t.Fatal("a chave privada lida nao e a que foi gravada")
	}
	if len(publicas) != 5 {
		t.Fatalf("esperava 5 chaves publicas, veio %d", len(publicas))
	}
	for _, produtor := range Produtores() {
		if publicas[produtor] == nil {
			t.Fatalf("faltou a chave publica de %s", produtor)
		}
	}
}

// A chave lida do disco tem de funcionar de ponta a ponta, nao so existir.
func TestChavesCarregadasAssinamEVerificam(t *testing.T) {
	raiz := t.TempDir()
	montarLayout(t, raiz, events.Principal)

	privada, publicas, err := CarregarChaves(raiz, events.Principal)
	if err != nil {
		t.Fatal(err)
	}
	env := envelopeDeTeste()
	if err := NovoSigner(privada).Sign(&env); err != nil {
		t.Fatal(err)
	}
	if err := NovoVerifier(publicas).Verify(env); err != nil {
		t.Fatalf("assinou com a privada do disco mas a publica do disco recusou: %v", err)
	}
}

func TestCarregarChavesFalhaSemPrivadaNomeandoOArquivo(t *testing.T) {
	raiz := t.TempDir()
	montarLayout(t, raiz, events.Principal)
	if err := os.Remove(filepath.Join(raiz, "cmd", events.Principal, "key", "private_key.pem")); err != nil {
		t.Fatal(err)
	}

	_, _, err := CarregarChaves(raiz, events.Principal)
	if err == nil {
		t.Fatal("esperava erro com a privada ausente, veio nil")
	}
	if !strings.Contains(err.Error(), "private_key.pem") {
		t.Fatalf("o erro nao nomeia o arquivo que faltou: %v", err)
	}
}

// ReadPubKeyFromFile entra em panico com arquivo ausente: o carregador
// precisa conter isso e devolver erro, senao o servico cai com stack trace.
func TestCarregarChavesFalhaSemPublicaSemEntrarEmPanico(t *testing.T) {
	raiz := t.TempDir()
	montarLayout(t, raiz, events.Principal)
	if err := os.Remove(filepath.Join(raiz, "cmd", events.Principal, events.Estoque+"-pub", "public_key.pem")); err != nil {
		t.Fatal(err)
	}

	_, _, err := CarregarChaves(raiz, events.Principal)
	if err == nil {
		t.Fatal("esperava erro com a publica ausente, veio nil")
	}
	if !strings.Contains(err.Error(), events.Estoque) {
		t.Fatalf("o erro nao diz de qual produtor era a chave: %v", err)
	}
}

func TestCarregarChavesFalhaComPEMCorrompido(t *testing.T) {
	raiz := t.TempDir()
	montarLayout(t, raiz, events.Principal)
	alvo := filepath.Join(raiz, "cmd", events.Principal, "key", "private_key.pem")
	if err := os.WriteFile(alvo, []byte("isso nao e um PEM"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := CarregarChaves(raiz, events.Principal); err == nil {
		t.Fatal("esperava erro com PEM corrompido, veio nil")
	}
}
