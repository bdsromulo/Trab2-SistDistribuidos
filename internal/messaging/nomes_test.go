package messaging

import (
	"context"
	"slices"
	"testing"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
)

func TestExchangeDoEvento(t *testing.T) {
	for tipo, produtor := range events.ProdutorDoEvento {
		esperado := ExchangeECommerce
		if produtor == events.Promocoes {
			esperado = ExchangePromocoes
		}
		if got := ExchangeDoEvento(tipo); got != esperado {
			t.Errorf("ExchangeDoEvento(%q) = %q, esperava %q", tipo, got, esperado)
		}
	}
}

func TestFakePublisherRegistraNaOrdem(t *testing.T) {
	var p FakePublisher
	ctx := context.Background()
	if err := p.Publish(ctx, events.PedidoCriado, events.PedidoCriadoDados{PedidoID: "1"}); err != nil {
		t.Fatal(err)
	}
	if err := p.Publish(ctx, events.PedidoExcluido, events.PedidoExcluidoDados{PedidoID: "1", Motivo: events.MotivoUsuario}); err != nil {
		t.Fatal(err)
	}

	esperado := []string{events.PedidoCriado, events.PedidoExcluido}
	if got := p.Tipos(); !slices.Equal(got, esperado) {
		t.Errorf("Tipos() = %v, esperava %v", got, esperado)
	}
	if d, ok := p.Publicados()[1].Data.(events.PedidoExcluidoDados); !ok || d.Motivo != events.MotivoUsuario {
		t.Errorf("dados do segundo evento errados: %+v", p.Publicados()[1].Data)
	}
}
