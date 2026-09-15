// Microsserviço Principal: menu no terminal, pedidos e status.
//
// Duas goroutines trabalham juntas: a principal lê o menu no terminal e
// outra consome a fila.principal, atualizando o status dos pedidos.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/catalogo"
	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/messaging"
	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/security"
)

func main() {
	produtos, err := catalogo.Carregar(catalogo.CaminhoPadrao)
	if err != nil {
		log.Fatal(err)
	}

	conn, chConsumo, err := messaging.Conectar(messaging.CarregarConfig())
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	// Canal separado para publicar: o de consumo fica com a outra goroutine.
	chPublicacao, err := conn.Channel()
	if err != nil {
		log.Fatal(err)
	}

	if err := messaging.DeclararExchanges(chConsumo); err != nil {
		log.Fatal(err)
	}
	if err := messaging.DeclararFila(chConsumo, messaging.FilaPrincipal); err != nil {
		log.Fatal(err)
	}

	// Enquanto o security real não existe, assinatura e verificação são falsas.
	pub := messaging.NovoPublisher(chPublicacao, events.Principal, security.FakeSigner{})
	pedidos := NovosPedidos()
	avisar := func(msg string) { fmt.Printf("\n[evento] %s\n", msg) }
	servico := NovoServico(pedidos, pub, avisar)

	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()

	go func() {
		err := messaging.Consumir(ctx, chConsumo, messaging.FilaPrincipal, security.FakeVerifier{}, servico.TratarEvento)
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Fatalf("consumo da %s parou: %v", messaging.FilaPrincipal, err)
		}
	}()

	NovoMenu(servico, pedidos, produtos, os.Stdin, os.Stdout).Executar(ctx)
	fmt.Println("Até mais!")
}
