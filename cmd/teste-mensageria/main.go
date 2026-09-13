// Teste manual da mensageria: publica um pedido.estoque_ok na exchange
// eCommerce e consome da fila.pagamento, como o Pagamento fará.
// Rodar com o RabbitMQ no ar: go run ./cmd/teste-mensageria
package main

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/messaging"
	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/security"
)

func main() {
	conn, ch, err := messaging.Conectar(messaging.CarregarConfig())
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	if err := messaging.DeclararExchanges(ch); err != nil {
		log.Fatal(err)
	}
	if err := messaging.DeclararFila(ch, messaging.FilaPagamento); err != nil {
		log.Fatal(err)
	}

	ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelar()

	// Enquanto o security real não existe, assinatura e verificação são falsas.
	pub := messaging.NovoPublisher(ch, events.Estoque, security.FakeSigner{})
	dados := events.PedidoEstoqueOKDados{PedidoID: "teste-1", ValorTotal: 99.9}
	if err := pub.Publish(ctx, events.PedidoEstoqueOK, dados); err != nil {
		log.Fatal(err)
	}
	log.Printf("publicado %s em %s", events.PedidoEstoqueOK, messaging.ExchangeECommerce)

	err = messaging.Consumir(ctx, ch, messaging.FilaPagamento, security.FakeVerifier{},
		func(_ context.Context, env events.Envelope) error {
			var d events.PedidoEstoqueOKDados
			if err := env.Decodificar(&d); err != nil {
				return err
			}
			log.Printf("recebido %s de %s: pedido %s, R$ %.2f (id %s)",
				env.EventType, env.Producer, d.PedidoID, d.ValorTotal, env.EventID)
			cancelar() // um evento basta para o teste
			return nil
		})
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
