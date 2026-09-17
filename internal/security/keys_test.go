package security

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/signature"
)

// verifierCom grava as públicas dadas no layout <pasta>/<produtor>-pub e
// devolve um verificador apontando para essa pasta.
func verifierCom(t *testing.T, publicas map[string]*rsa.PublicKey) RSAVerifier {
	t.Helper()
	pasta := t.TempDir()
	for produtor, publica := range publicas {
		dir := filepath.Join(pasta, produtor+"-pub")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		gravarPEM(t, filepath.Join(dir, "public_key.pem"), "RSA PUBLIC KEY", x509.MarshalPKCS1PublicKey(publica))
	}
	return NovoVerifier(pasta)
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

// entrarEm troca o diretório corrente durante o teste, como se o serviço
// tivesse sido iniciado de dentro dessa pasta.
func entrarEm(t *testing.T, dir string) {
	t.Helper()
	anterior, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(anterior) })
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

// De ponta a ponta com o PersistKeys do Kauan: o Principal gera e distribui
// a chave rodando de dentro de cmd/principal, e o verificador do Estoque, de
// dentro de cmd/estoque, aceita o que ele assinou.
func TestVerifierAceitaChaveDistribuidaPeloPersistKeys(t *testing.T) {
	raiz := t.TempDir()
	for _, ms := range []string{events.Principal, events.Estoque} {
		if err := os.MkdirAll(filepath.Join(raiz, "cmd", ms), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	entrarEm(t, filepath.Join(raiz, "cmd", events.Principal))
	privada := signature.GenerateKeys()
	signature.PersistKeys(privada, events.Principal)

	env := assinado(t, privada, envelopeDeTeste())
	v := NovoVerifier(filepath.Join(raiz, "cmd", events.Estoque))
	if err := v.Verify(env); err != nil {
		t.Fatalf("o Estoque recusou evento assinado pelo Principal: %v", err)
	}
}

// O produtor gera um par novo a cada partida; o consumidor que já estava de
// pé precisa aceitar a chave nova sem reiniciar.
func TestVerifierUsaChaveNovaDepoisQueProdutorReinicia(t *testing.T) {
	antiga := signature.GenerateKeys()
	v := verifierCom(t, map[string]*rsa.PublicKey{events.Principal: &antiga.PublicKey})

	nova := signature.GenerateKeys()
	gravarPEM(t, CaminhoPublica(v.pasta, events.Principal), "RSA PUBLIC KEY", x509.MarshalPKCS1PublicKey(&nova.PublicKey))

	if err := v.Verify(assinado(t, nova, envelopeDeTeste())); err != nil {
		t.Fatalf("recusou evento assinado com a chave nova: %v", err)
	}
	if err := v.Verify(assinado(t, antiga, envelopeDeTeste())); !errors.Is(err, ErrAssinaturaInvalida) {
		t.Fatalf("esperava recusar a chave antiga com ErrAssinaturaInvalida, veio: %v", err)
	}
}

// ReadPubKeyFromFile entra em pânico com PEM inválido: o verificador precisa
// conter isso e só recusar a mensagem.
func TestVerifierRecusaPEMCorrompidoSemEntrarEmPanico(t *testing.T) {
	privada := signature.GenerateKeys()
	v := verifierCom(t, map[string]*rsa.PublicKey{events.Principal: &privada.PublicKey})
	if err := os.WriteFile(CaminhoPublica(v.pasta, events.Principal), []byte("isso nao e um PEM"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := v.Verify(assinado(t, privada, envelopeDeTeste())); !errors.Is(err, ErrProdutorDesconhecido) {
		t.Fatalf("esperava ErrProdutorDesconhecido, veio: %v", err)
	}
}

func TestVerifierRecusaProducerQueApontaParaForaDaPasta(t *testing.T) {
	privada := signature.GenerateKeys()
	env := envelopeDeTeste()
	env.Producer = "../principal"
	env = assinado(t, privada, env)
	v := verifierCom(t, map[string]*rsa.PublicKey{events.Principal: &privada.PublicKey})

	if err := v.Verify(env); !errors.Is(err, ErrProdutorDesconhecido) {
		t.Fatalf("esperava ErrProdutorDesconhecido, veio: %v", err)
	}
}
