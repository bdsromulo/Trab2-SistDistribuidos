package security

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	v := verifierCom(t, map[string]*rsa.PublicKey{events.Principal: &privada.PublicKey})

	if err := v.Verify(env); err != nil {
		t.Fatalf("esperava aceitar, recusou com: %v", err)
	}
}

func TestVerifyRecusaDataAdulterado(t *testing.T) {
	privada := signature.GenerateKeys()
	env := assinado(t, privada, envelopeDeTeste())
	// exatamente o que o cmd/adulterador vai fazer: mexer no data depois de assinar
	env.Data = json.RawMessage(`{"pedido_id":"p-999"}`)
	v := verifierCom(t, map[string]*rsa.PublicKey{events.Principal: &privada.PublicKey})

	if err := v.Verify(env); !errors.Is(err, ErrAssinaturaInvalida) {
		t.Fatalf("esperava ErrAssinaturaInvalida, veio: %v", err)
	}
}

func TestVerifyRecusaAssinaturaDeOutroProdutor(t *testing.T) {
	doPrincipal := signature.GenerateKeys()
	outra := signature.GenerateKeys()
	// diz ser do Principal, mas foi assinado com outra chave
	env := assinado(t, outra, envelopeDeTeste())
	v := verifierCom(t, map[string]*rsa.PublicKey{events.Principal: &doPrincipal.PublicKey})

	if err := v.Verify(env); !errors.Is(err, ErrAssinaturaInvalida) {
		t.Fatalf("esperava ErrAssinaturaInvalida, veio: %v", err)
	}
}

func TestVerifyRecusaEnvelopeSemAssinatura(t *testing.T) {
	privada := signature.GenerateKeys()
	v := verifierCom(t, map[string]*rsa.PublicKey{events.Principal: &privada.PublicKey})

	if err := v.Verify(envelopeDeTeste()); !errors.Is(err, ErrSemAssinatura) {
		t.Fatalf("esperava ErrSemAssinatura, veio: %v", err)
	}
}

func TestVerifyRecusaProdutorDesconhecido(t *testing.T) {
	privada := signature.GenerateKeys()
	env := assinado(t, privada, envelopeDeTeste())
	v := verifierCom(t, map[string]*rsa.PublicKey{}) // o Principal ainda não distribuiu a chave

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
	v := verifierCom(t, map[string]*rsa.PublicKey{events.Pagamento: &doPagamento.PublicKey})

	if err := v.Verify(env); !errors.Is(err, ErrEventoNaoPertenceAoProdutor) {
		t.Fatalf("esperava ErrEventoNaoPertenceAoProdutor, veio: %v", err)
	}
}

func TestVerifyRecusaBase64InvalidoSemEntrarEmPanico(t *testing.T) {
	privada := signature.GenerateKeys()
	env := assinado(t, privada, envelopeDeTeste())
	env.Signature = "isso!nao(e)base64"
	v := verifierCom(t, map[string]*rsa.PublicKey{events.Principal: &privada.PublicKey})

	if err := v.Verify(env); !errors.Is(err, ErrAssinaturaInvalida) {
		t.Fatalf("esperava ErrAssinaturaInvalida, veio: %v", err)
	}
}
