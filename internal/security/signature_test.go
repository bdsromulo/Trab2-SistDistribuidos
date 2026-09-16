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
