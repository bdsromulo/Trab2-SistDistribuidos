# Adaptador de Assinatura RSA — Plano de Implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ligar as primitivas RSA de `internal/signature` às interfaces `security.Signer`/`security.Verifier`, e trocar os Fakes do Principal pela assinatura real.

**Architecture:** Duas implementações novas dentro de `internal/security` (`RSASigner`, `RSAVerifier`) que canonicalizam o envelope sem o campo `Signature`, delegam a criptografia para `internal/signature` e traduzem os resultados para os erros já declarados em `security.go`. Um carregador lê as chaves do disco no layout que o `PersistKeys` produz. Nenhum arquivo do Kauan é alterado.

**Tech Stack:** Go 1.22 (toolchain 1.27), stdlib `crypto/rsa`, `crypto/x509`, `encoding/pem`, `encoding/base64`, `encoding/json`. Sem dependências novas.

## Global Constraints

- Módulo: `github.com/bdsromulo/Trab2-SistDistribuidos`.
- **Não alterar** `internal/signature/`, `internal/utils/`, `cmd/gerar-chaves/`. São da parte do Kauan.
- **Não remover** `FakeSigner`/`FakeVerifier`: os testes de `internal/messaging` e `cmd/principal` dependem deles.
- Comentários e mensagens de erro em português, como o resto do repositório.
- Mensagens de commit sem creditação de IA (o hook `.githooks/commit-msg` garante).
- Rodar a suíte inteira com `go test ./...` antes de cada commit. Baseline atual: tudo verde.
- Formato das chaves: PEM PKCS#1 — `RSA PRIVATE KEY` e `RSA PUBLIC KEY`. É o que o `PersistKeys` grava.

## Achados da investigação que o spec não previu

1. `internal/signature` **não tem leitor de chave privada**. Tem `PersistKeys` (grava) e `ReadPubKeyFromFile` (lê só a pública). A Task 3 implementa a leitura da privada com `encoding/pem` + `x509.ParsePKCS1PrivateKey`, sem tocar no pacote dele.
2. `log.Panicf` **imprime no stderr antes** de entrar em pânico. O `recover` devolve o erro corretamente, mas a linha do Kauan aparece no terminal mesmo assim. É ruído cosmético aceito, não um bug a corrigir aqui.
3. O round-trip do marshal canônico foi verificado empiricamente antes deste plano: espaços normalizados, ordem das chaves preservada, acentos e horário estáveis.

---

### Task 1: Conteúdo canônico e assinatura

**Files:**
- Create: `internal/security/signature.go`
- Create: `internal/security/signature_test.go`

**Interfaces:**
- Consumes: `events.Envelope`; `signature.GenerateKeys`, `signature.SignPayload`, `signature.VerifySignature`.
- Produces: `canonicalizar(events.Envelope) (string, error)`; `NovoSigner(*rsa.PrivateKey) RSASigner`; `RSASigner.Sign(*events.Envelope) error`. A Task 2 usa `canonicalizar` e a Task 4 usa `NovoSigner`.

- [ ] **Step 1: Escrever o teste que falha**

Criar `internal/security/signature_test.go`:

```go
package security

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/signature"
)

// envelopeDeTeste monta um envelope válido do Principal, sem assinatura.
func envelopeDeTeste() events.Envelope {
	return events.Envelope{
		EventID:    "evt-1",
		EventType:  events.PedidoCriado,
		Producer:   events.Principal,
		OccurredAt: time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC),
		Data:       json.RawMessage(`{"pedido_id":"p-1"}`),
	}
}

func TestSignPreencheAssinaturaEmBase64(t *testing.T) {
	privada := signature.GenerateKeys()
	env := envelopeDeTeste()

	if err := NovoSigner(privada).Sign(&env); err != nil {
		t.Fatalf("Sign devolveu erro: %v", err)
	}
	if env.Signature == "" {
		t.Fatal("Sign nao preencheu o campo Signature")
	}
	bruta, err := base64.StdEncoding.DecodeString(env.Signature)
	if err != nil {
		t.Fatalf("Signature nao e base64 valido: %v", err)
	}
	conteudo, err := canonicalizar(env)
	if err != nil {
		t.Fatalf("canonicalizar: %v", err)
	}
	if !signature.VerifySignature(&privada.PublicKey, conteudo, bruta) {
		t.Fatal("a assinatura gravada nao confere com o conteudo canonico")
	}
}

// O campo Signature nao pode entrar no conteudo assinado: se entrasse,
// assinar mudaria o proprio conteudo e a verificacao nunca fecharia.
func TestCanonicalizarIgnoraOCampoSignature(t *testing.T) {
	env := envelopeDeTeste()
	semAssinatura, err := canonicalizar(env)
	if err != nil {
		t.Fatal(err)
	}
	env.Signature = "qualquer-coisa"
	comAssinatura, err := canonicalizar(env)
	if err != nil {
		t.Fatal(err)
	}
	if semAssinatura != comAssinatura {
		t.Fatalf("canonicalizar considerou o Signature\nsem: %s\ncom: %s", semAssinatura, comAssinatura)
	}
}

// O que o produtor assina tem de ser byte a byte o que o consumidor confere,
// mesmo depois de a mensagem passar por JSON no RabbitMQ.
func TestCanonicalizarEstavelAposRoundTripJSON(t *testing.T) {
	env := envelopeDeTeste()
	env.Data = json.RawMessage(`{  "b" : 2,
	   "a":  1, "texto": "promoção São Paulo" }`)
	antes, err := canonicalizar(env)
	if err != nil {
		t.Fatal(err)
	}

	corpo, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	var recebido events.Envelope
	if err := json.Unmarshal(corpo, &recebido); err != nil {
		t.Fatal(err)
	}
	depois, err := canonicalizar(recebido)
	if err != nil {
		t.Fatal(err)
	}
	if antes != depois {
		t.Fatalf("round-trip instavel\nantes : %s\ndepois: %s", antes, depois)
	}
}

func TestSignerSemChaveDevolveErro(t *testing.T) {
	env := envelopeDeTeste()
	if err := NovoSigner(nil).Sign(&env); err == nil {
		t.Fatal("esperava erro ao assinar sem chave privada, veio nil")
	}
}
```

- [ ] **Step 2: Rodar o teste e confirmar que falha**

Run: `go test ./internal/security -run 'TestSign|TestCanonicalizar' -v`
Expected: FAIL na compilação, com `undefined: NovoSigner` e `undefined: canonicalizar`.

- [ ] **Step 3: Implementar o mínimo**

Criar `internal/security/signature.go`:

```go
package security

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/signature"
)

// conteudoAssinado é o envelope sem o campo Signature: é exatamente isso que
// a chave privada assina e que a pública confere.
//
// Os nomes e as tags repetem os de events.Envelope de propósito. A ordem dos
// campos de uma struct no json.Marshal do Go é determinística, então produtor
// e consumidor chegam aos mesmos bytes.
type conteudoAssinado struct {
	EventID    string          `json:"event_id"`
	EventType  string          `json:"event_type"`
	Producer   string          `json:"producer"`
	OccurredAt time.Time       `json:"occurred_at"`
	Data       json.RawMessage `json:"data"`
}

// canonicalizar devolve os bytes assinados de um envelope, como string porque
// é essa a assinatura de signature.SignPayload.
func canonicalizar(env events.Envelope) (string, error) {
	bytes, err := json.Marshal(conteudoAssinado{
		EventID:    env.EventID,
		EventType:  env.EventType,
		Producer:   env.Producer,
		OccurredAt: env.OccurredAt,
		Data:       env.Data,
	})
	if err != nil {
		return "", fmt.Errorf("serializar conteúdo assinado: %w", err)
	}
	return string(bytes), nil
}

// RSASigner assina envelopes com a chave privada do próprio processo.
type RSASigner struct {
	privada *rsa.PrivateKey
}

// NovoSigner cria o assinante do processo a partir da chave privada dele.
func NovoSigner(privada *rsa.PrivateKey) RSASigner {
	return RSASigner{privada: privada}
}

// Sign canonicaliza o envelope, assina e grava a assinatura em base64.
//
// O recover existe porque signature.SignPayload sinaliza erro com log.Panicf:
// sem ele, um problema na chave derrubaria o processo inteiro.
func (s RSASigner) Sign(env *events.Envelope) (err error) {
	if s.privada == nil {
		return errors.New("signer sem chave privada")
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("assinar %s: %v", env.EventType, r)
		}
	}()

	conteudo, err := canonicalizar(*env)
	if err != nil {
		return err
	}
	env.Signature = base64.StdEncoding.EncodeToString(signature.SignPayload(s.privada, conteudo))
	return nil
}

var _ Signer = RSASigner{}
```

- [ ] **Step 4: Rodar os testes e confirmar que passam**

Run: `go test ./internal/security -run 'TestSign|TestCanonicalizar' -v`
Expected: PASS nos quatro testes.

Run: `go test ./...`
Expected: tudo `ok` ou `no test files`. Nenhum FAIL.

- [ ] **Step 5: Commit**

```bash
git add internal/security/signature.go internal/security/signature_test.go && git commit -m "feat: assina envelopes com RSA usando as primitivas do internal/signature"
```

---

### Task 2: Verificação e tradução dos erros

**Files:**
- Modify: `internal/security/signature.go` (acrescentar ao final)
- Modify: `internal/security/signature_test.go` (acrescentar ao final)

**Interfaces:**
- Consumes: `canonicalizar` e `NovoSigner` da Task 1; `events.ProdutorDoEvento`; os erros `ErrSemAssinatura`, `ErrProdutorDesconhecido`, `ErrEventoNaoPertenceAoProdutor`, `ErrAssinaturaInvalida` de `security.go`.
- Produces: `NovoVerifier(map[string]*rsa.PublicKey) RSAVerifier`; `RSAVerifier.Verify(events.Envelope) error`. A Task 4 usa `NovoVerifier`.

- [ ] **Step 1: Escrever os testes que falham**

Acrescentar `"crypto/rsa"` e `"errors"` ao bloco de imports de `internal/security/signature_test.go`, e acrescentar ao final do arquivo:

```go
// assinado devolve um envelope pronto, assinado com a chave dada.
func assinado(t *testing.T, privada *rsa.PrivateKey, env events.Envelope) events.Envelope {
	t.Helper()
	if err := NovoSigner(privada).Sign(&env); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	return env
}

func TestVerifyAceitaEnvelopeIntacto(t *testing.T) {
	privada := signature.GenerateKeys()
	env := assinado(t, privada, envelopeDeTeste())
	v := NovoVerifier(map[string]*rsa.PublicKey{events.Principal: &privada.PublicKey})

	if err := v.Verify(env); err != nil {
		t.Fatalf("esperava aceitar, recusou com: %v", err)
	}
}

func TestVerifyRecusaDataAdulterado(t *testing.T) {
	privada := signature.GenerateKeys()
	env := assinado(t, privada, envelopeDeTeste())
	// exatamente o que o cmd/adulterador vai fazer: mexer no data depois de assinar
	env.Data = json.RawMessage(`{"pedido_id":"p-999"}`)
	v := NovoVerifier(map[string]*rsa.PublicKey{events.Principal: &privada.PublicKey})

	if err := v.Verify(env); !errors.Is(err, ErrAssinaturaInvalida) {
		t.Fatalf("esperava ErrAssinaturaInvalida, veio: %v", err)
	}
}

func TestVerifyRecusaAssinaturaDeOutroProdutor(t *testing.T) {
	doPrincipal := signature.GenerateKeys()
	outra := signature.GenerateKeys()
	// diz ser do Principal, mas foi assinado com outra chave
	env := assinado(t, outra, envelopeDeTeste())
	v := NovoVerifier(map[string]*rsa.PublicKey{events.Principal: &doPrincipal.PublicKey})

	if err := v.Verify(env); !errors.Is(err, ErrAssinaturaInvalida) {
		t.Fatalf("esperava ErrAssinaturaInvalida, veio: %v", err)
	}
}

func TestVerifyRecusaEnvelopeSemAssinatura(t *testing.T) {
	privada := signature.GenerateKeys()
	v := NovoVerifier(map[string]*rsa.PublicKey{events.Principal: &privada.PublicKey})

	if err := v.Verify(envelopeDeTeste()); !errors.Is(err, ErrSemAssinatura) {
		t.Fatalf("esperava ErrSemAssinatura, veio: %v", err)
	}
}

func TestVerifyRecusaProdutorDesconhecido(t *testing.T) {
	privada := signature.GenerateKeys()
	env := assinado(t, privada, envelopeDeTeste())
	v := NovoVerifier(map[string]*rsa.PublicKey{}) // nenhuma chave carregada

	if err := v.Verify(env); !errors.Is(err, ErrProdutorDesconhecido) {
		t.Fatalf("esperava ErrProdutorDesconhecido, veio: %v", err)
	}
}

// Sem esta checagem, o Pagamento poderia publicar um pedido.enviado assinado
// com a propria chave e o Principal aceitaria como se fosse da Entrega.
func TestVerifyRecusaEventoQueNaoEDoProdutor(t *testing.T) {
	doPagamento := signature.GenerateKeys()
	env := envelopeDeTeste()
	env.Producer = events.Pagamento
	env.EventType = events.PedidoEnviado // o dono desse evento e a Entrega
	env = assinado(t, doPagamento, env)
	v := NovoVerifier(map[string]*rsa.PublicKey{events.Pagamento: &doPagamento.PublicKey})

	if err := v.Verify(env); !errors.Is(err, ErrEventoNaoPertenceAoProdutor) {
		t.Fatalf("esperava ErrEventoNaoPertenceAoProdutor, veio: %v", err)
	}
}

func TestVerifyRecusaBase64InvalidoSemEntrarEmPanico(t *testing.T) {
	privada := signature.GenerateKeys()
	env := assinado(t, privada, envelopeDeTeste())
	env.Signature = "isso!nao(e)base64"
	v := NovoVerifier(map[string]*rsa.PublicKey{events.Principal: &privada.PublicKey})

	if err := v.Verify(env); !errors.Is(err, ErrAssinaturaInvalida) {
		t.Fatalf("esperava ErrAssinaturaInvalida, veio: %v", err)
	}
}
```

- [ ] **Step 2: Rodar os testes e confirmar que falham**

Run: `go test ./internal/security -run TestVerify -v`
Expected: FAIL na compilação, com `undefined: NovoVerifier`.

- [ ] **Step 3: Implementar o mínimo**

Acrescentar ao final de `internal/security/signature.go`:

```go
// RSAVerifier confere assinaturas com as chaves públicas dos produtores,
// carregadas uma única vez na partida do processo.
type RSAVerifier struct {
	publicas map[string]*rsa.PublicKey
}

// NovoVerifier cria o verificador a partir do mapa produtor -> chave pública.
func NovoVerifier(publicas map[string]*rsa.PublicKey) RSAVerifier {
	return RSAVerifier{publicas: publicas}
}

// Verify devolve nil somente se a mensagem for autêntica e íntegra.
//
// A ordem das checagens vai do mais barato para o mais caro, e cada recusa
// usa o erro já declarado em security.go para o consumidor poder distinguir
// o motivo no log.
func (v RSAVerifier) Verify(env events.Envelope) error {
	if env.Signature == "" {
		return ErrSemAssinatura
	}

	publica, conhecido := v.publicas[env.Producer]
	if !conhecido {
		return fmt.Errorf("%w: %q", ErrProdutorDesconhecido, env.Producer)
	}

	// Impede um produtor legítimo de assinar um evento que não é dele.
	dono, existe := events.ProdutorDoEvento[env.EventType]
	if !existe || dono != env.Producer {
		return fmt.Errorf("%w: %q não publica %q", ErrEventoNaoPertenceAoProdutor, env.Producer, env.EventType)
	}

	bruta, err := base64.StdEncoding.DecodeString(env.Signature)
	if err != nil {
		return fmt.Errorf("%w: base64 malformado", ErrAssinaturaInvalida)
	}

	conteudo, err := canonicalizar(env)
	if err != nil {
		return err
	}
	if !signature.VerifySignature(publica, conteudo, bruta) {
		return ErrAssinaturaInvalida
	}
	return nil
}

var _ Verifier = RSAVerifier{}
```

- [ ] **Step 4: Rodar os testes e confirmar que passam**

Run: `go test ./internal/security -v`
Expected: PASS em todos, incluindo os da Task 1 e os dos Fakes que já existiam.

Run: `go test ./...`
Expected: nenhum FAIL.

- [ ] **Step 5: Commit**

```bash
git add internal/security/signature.go internal/security/signature_test.go && git commit -m "feat: verifica assinatura e recusa evento adulterado ou de produtor errado"
```

---

### Task 3: Carregamento das chaves do disco

**Files:**
- Create: `internal/security/keys.go`
- Create: `internal/security/keys_test.go`

**Interfaces:**
- Consumes: `events.ProdutorDoEvento`; `signature.ReadPubKeyFromFile`; `envelopeDeTeste`, `NovoSigner` e `NovoVerifier` das Tasks 1–2 (usados no teste de ponta a ponta).
- Produces: `CarregarChaves(raiz, processo string) (*rsa.PrivateKey, map[string]*rsa.PublicKey, error)`; `Produtores() []string`; `CaminhoPrivada(raiz, processo string) string`; `CaminhoPublica(raiz, processo, produtor string) string`. A Task 4 usa `CarregarChaves`.

**Nota:** `internal/signature` não tem leitor de chave privada, só `ReadPubKeyFromFile`. A privada é lida aqui com `encoding/pem` + `x509.ParsePKCS1PrivateKey`, que é o formato exato que o `PersistKeys` grava.

- [ ] **Step 1: Escrever os testes que falham**

Criar `internal/security/keys_test.go`:

```go
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
```

- [ ] **Step 2: Rodar os testes e confirmar que falham**

Run: `go test ./internal/security -run 'TestProdutores|TestCarregarChaves|TestChavesCarregadas' -v`
Expected: FAIL na compilação, com `undefined: Produtores` e `undefined: CarregarChaves`.

- [ ] **Step 3: Implementar o mínimo**

Criar `internal/security/keys.go`:

```go
package security

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/signature"
)

// Produtores devolve, em ordem, os produtores distintos que assinam eventos.
func Produtores() []string {
	vistos := map[string]bool{}
	lista := make([]string, 0, 5)
	for _, produtor := range events.ProdutorDoEvento {
		if !vistos[produtor] {
			vistos[produtor] = true
			lista = append(lista, produtor)
		}
	}
	sort.Strings(lista)
	return lista
}

// CaminhoPrivada e CaminhoPublica descrevem o layout que o cmd/gerar-chaves
// produz. Os serviços rodam com `go run ./cmd/<processo>`, então o diretório
// corrente é a raiz do repositório e a raiz passada aqui é ".".
func CaminhoPrivada(raiz, processo string) string {
	return filepath.Join(raiz, "cmd", processo, "key", "private_key.pem")
}

func CaminhoPublica(raiz, processo, produtor string) string {
	return filepath.Join(raiz, "cmd", processo, produtor+"-pub", "public_key.pem")
}

// CarregarChaves lê a chave privada do processo e as públicas de todos os
// produtores. Chave faltando é erro de partida, não erro por mensagem: é
// melhor o serviço não subir do que descartar eventos silenciosamente depois.
func CarregarChaves(raiz, processo string) (*rsa.PrivateKey, map[string]*rsa.PublicKey, error) {
	privada, err := lerPrivada(CaminhoPrivada(raiz, processo))
	if err != nil {
		return nil, nil, err
	}

	publicas := make(map[string]*rsa.PublicKey, 5)
	for _, produtor := range Produtores() {
		publica, err := lerPublica(CaminhoPublica(raiz, processo, produtor))
		if err != nil {
			return nil, nil, fmt.Errorf("chave pública de %q: %w", produtor, err)
		}
		publicas[produtor] = publica
	}
	return privada, publicas, nil
}

// lerPrivada existe porque internal/signature só sabe gravar a chave privada
// (PersistKeys); não há um leitor lá. O formato é o mesmo que ele grava:
// PEM PKCS#1.
func lerPrivada(caminho string) (*rsa.PrivateKey, error) {
	bytes, err := os.ReadFile(caminho)
	if err != nil {
		return nil, fmt.Errorf("ler %s: %w", caminho, err)
	}
	bloco, _ := pem.Decode(bytes)
	if bloco == nil || bloco.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("%s não é um PEM 'RSA PRIVATE KEY'", caminho)
	}
	privada, err := x509.ParsePKCS1PrivateKey(bloco.Bytes)
	if err != nil {
		return nil, fmt.Errorf("interpretar %s: %w", caminho, err)
	}
	return privada, nil
}

// lerPublica delega para o leitor do Kauan, contendo o log.Panicf dele: sem o
// recover, um .pem ausente derrubaria o serviço com stack trace em vez de uma
// mensagem que diz qual arquivo faltou.
func lerPublica(caminho string) (publica *rsa.PublicKey, err error) {
	defer func() {
		if r := recover(); r != nil {
			publica, err = nil, fmt.Errorf("ler %s: %v", caminho, r)
		}
	}()
	if _, err := os.Stat(caminho); err != nil {
		return nil, fmt.Errorf("ler %s: %w", caminho, err)
	}
	return signature.ReadPubKeyFromFile(caminho), nil
}
```

- [ ] **Step 4: Rodar os testes e confirmar que passam**

Run: `go test ./internal/security -v`
Expected: PASS em todos.

Run: `go test ./...`
Expected: nenhum FAIL.

- [ ] **Step 5: Commit**

```bash
git add internal/security/keys.go internal/security/keys_test.go && git commit -m "feat: carrega as chaves RSA do layout gerado pelo cmd/gerar-chaves"
```

---

### Task 4: Ligar o Principal à assinatura real

**Files:**
- Modify: `cmd/principal/main.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: `CarregarChaves`, `NovoSigner`, `NovoVerifier` das Tasks 1–3.
- Produces: nada. É o fim da cadeia.

- [ ] **Step 1: Confirmar que os testes do Principal passam antes de mexer**

Run: `go test ./cmd/principal -v`
Expected: PASS. Estes testes usam os Fakes e **não** podem quebrar — o `main.go` não é coberto por eles, então servem de rede de segurança para garantir que a troca não vaze para o resto do pacote.

- [ ] **Step 2: Carregar as chaves na partida**

Em `cmd/principal/main.go`, acrescentar logo depois do bloco do `catalogo.Carregar` e **antes** do `messaging.Conectar` (para falhar antes de abrir conexão com o broker):

```go
	// As chaves são carregadas antes de tudo: sem assinatura real o serviço
	// não sobe, em vez de rodar silenciosamente sem garantia de autenticidade.
	privada, publicas, err := security.CarregarChaves(".", events.Principal)
	if err != nil {
		log.Fatalf("carregar chaves (rode o cmd/gerar-chaves): %v", err)
	}
```

- [ ] **Step 3: Trocar os Fakes pelas implementações reais**

Substituir estas duas linhas:

```go
	// Enquanto o security real não existe, assinatura e verificação são falsas.
	pub := messaging.NovoPublisher(chPublicacao, events.Principal, security.FakeSigner{})
```

por:

```go
	pub := messaging.NovoPublisher(chPublicacao, events.Principal, security.NovoSigner(privada))
```

E, dentro da goroutine de consumo, substituir:

```go
		err := messaging.Consumir(ctx, chConsumo, messaging.FilaPrincipal, security.FakeVerifier{}, servico.TratarEvento)
```

por:

```go
		err := messaging.Consumir(ctx, chConsumo, messaging.FilaPrincipal, security.NovoVerifier(publicas), servico.TratarEvento)
```

- [ ] **Step 4: Confirmar que compila e que a suíte continua verde**

Run: `go build ./... && go vet ./cmd/principal && go test ./...`
Expected: build sem saída, vet sem saída, nenhum FAIL.

- [ ] **Step 5: Confirmar que o Principal falha com mensagem clara sem as chaves**

Run: `go run ./cmd/principal`
Expected: encerra imediatamente com `carregar chaves (rode o cmd/gerar-chaves): ler cmd\principal\key\private_key.pem: ...`, **sem stack trace** e sem tentar conectar no RabbitMQ.

Este passo é a verificação de que o `recover` da Task 3 está no lugar. Se aparecer um stack trace, a contenção falhou.

- [ ] **Step 6: Atualizar o README**

Acrescentar à seção de execução do `README.md`, antes da instrução de subir os serviços, uma subseção "Chaves" explicando: que é preciso rodar `cd cmd/gerar-chaves && go run . && cd ../..` antes de subir os serviços; que cada processo lê a privada em `cmd/<processo>/key/private_key.pem` e as públicas em `cmd/<processo>/<produtor>-pub/public_key.pem`; que as privadas não são versionadas; e que um serviço sem chaves não sobe, porque assinatura desligada em silêncio esconderia justamente o que o trabalho precisa demonstrar.

- [ ] **Step 7: Commit**

```bash
git add cmd/principal/main.go README.md && git commit -m "feat: Principal passa a assinar e verificar eventos de verdade"
```

---

## Verificação final

- [ ] `go build ./...` sem erro.
- [ ] `go vet ./...` sem apontamentos.
- [ ] `go test ./...` inteiramente verde.
- [ ] `grep -rn "FakeSigner\|FakeVerifier" cmd/` não retorna nada (os Fakes continuam existindo em `internal/security`, mas nenhum serviço os usa).
- [ ] `git log --format=%B ce5a970..HEAD` sem nenhuma linha de coautoria de IA.

## Fora de escopo, registrado

Estas ficam para depois e **não** devem ser feitas neste plano:

- As duas pendências do Kauan (seção 9 do spec): o `PersistKeys` sobrescreve sempre a mesma chave privada, e `c1`/`c2` não recebem pasta de chave pública. Decisão explícita de deixar como estão.
- Os serviços em stub: `estoque`, `pagamento`, `entrega`, `promocoes`, `c1`, `c2`, `adulterador`. Cada um vai ligar as próprias chaves com as mesmas três chamadas da Task 4.
