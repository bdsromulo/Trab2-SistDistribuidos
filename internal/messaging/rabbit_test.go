package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/security"
)

func TestTodoEventoDoECommerceTemFila(t *testing.T) {
	ligados := map[string]bool{}
	for _, b := range BindingsDaFila {
		if b.Exchange == ExchangeECommerce {
			for _, chave := range b.Chaves {
				ligados[chave] = true
			}
		}
	}
	for tipo := range events.ProdutorDoEvento {
		if ExchangeDoEvento(tipo) == ExchangeECommerce && !ligados[tipo] {
			t.Errorf("nenhuma fila recebe %q", tipo)
		}
	}
}

func TestConfigURL(t *testing.T) {
	cfg := Config{Host: "localhost", Porta: "5672", Usuario: "ecommerce", Senha: "ecommerce"}
	if got, esperado := cfg.URL(), "amqp://ecommerce:ecommerce@localhost:5672/"; got != esperado {
		t.Errorf("URL() = %q, esperava %q", got, esperado)
	}
}

func TestMontarEnvelope(t *testing.T) {
	dados := events.PedidoEstoqueOKDados{PedidoID: "7", ValorTotal: 10.5}
	env, err := MontarEnvelope(events.Estoque, events.PedidoEstoqueOK, dados, security.FakeSigner{})
	if err != nil {
		t.Fatal(err)
	}
	if env.EventID == "" || env.Producer != events.Estoque || env.EventType != events.PedidoEstoqueOK {
		t.Errorf("envelope com cabeçalho errado: %+v", env)
	}
	if env.OccurredAt.Location() != time.UTC {
		t.Errorf("OccurredAt não está em UTC: %v", env.OccurredAt)
	}
	if env.Signature != security.AssinaturaFalsa {
		t.Errorf("envelope não foi assinado: %q", env.Signature)
	}
	var lido events.PedidoEstoqueOKDados
	if err := env.Decodificar(&lido); err != nil || lido != dados {
		t.Errorf("dados = %+v (erro %v), esperava %+v", lido, err, dados)
	}
}

func corpoValido(t *testing.T) []byte {
	t.Helper()
	env, err := MontarEnvelope(events.Estoque, events.PedidoEstoqueOK, events.PedidoEstoqueOKDados{PedidoID: "7"}, security.FakeSigner{})
	if err != nil {
		t.Fatal(err)
	}
	corpo, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return corpo
}

func TestProcessar(t *testing.T) {
	ctx := context.Background()
	falha := errors.New("falhou")

	casos := []struct {
		nome          string
		corpo         []byte
		verifier      security.Verifier
		erroHandler   error
		esperaErro    bool
		esperaHandler bool
	}{
		{"válida", corpoValido(t), security.FakeVerifier{}, nil, false, true},
		{"JSON quebrado", []byte("{"), security.FakeVerifier{}, nil, true, false},
		{"assinatura inválida", corpoValido(t), security.FakeVerifier{Err: security.ErrAssinaturaInvalida}, nil, true, false},
		{"erro no handler", corpoValido(t), security.FakeVerifier{}, falha, true, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			chamou := false
			handler := func(context.Context, events.Envelope) error {
				chamou = true
				return c.erroHandler
			}
			err := Processar(ctx, c.corpo, c.verifier, handler)
			if (err != nil) != c.esperaErro {
				t.Errorf("erro = %v, esperava erro: %v", err, c.esperaErro)
			}
			if chamou != c.esperaHandler {
				t.Errorf("handler chamado = %v, esperava %v", chamou, c.esperaHandler)
			}
		})
	}
}
