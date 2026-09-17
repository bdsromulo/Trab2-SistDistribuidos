// Microsserviço Principal: menu no terminal, pedidos e status.
//
// Duas goroutines trabalham juntas: a principal lê o menu no terminal e
// outra consome a fila.principal, atualizando o status dos pedidos.
package main

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/catalogo"
	e "github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
	s "github.com/bdsromulo/Trab2-SistDistribuidos/internal/signature"
	u "github.com/bdsromulo/Trab2-SistDistribuidos/internal/utils"
	uuid "github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	produtos, err := catalogo.Carregar(catalogo.CaminhoPadrao)
	u.FailOnError(err, "Erro ao carregar o catálogo")

	conn, err := amqp.Dial("amqp://ecommerce:ecommerce@localhost:5672/")
	u.FailOnError(err, "Erro ao conectar ao RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	u.FailOnError(err, "Erro ao abrir canal")
	defer ch.Close()

	err = ch.ExchangeDeclare(
		"eCommerce",
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	u.FailOnError(err, "Erro ao declarar a exchange")

	_, err = ch.QueueDeclare(
		"fila.principal",
		true,
		false,
		false,
		false,
		nil,
	)
	u.FailOnError(err, "Erro ao declarar a fila")

	for _, rk := range []string{e.PedidoEstoqueOK, e.EstoqueIndisponivel, e.PagamentoAprovado, e.PagamentoRecusado, e.PedidoEnviado} {
		err = ch.QueueBind(
			"fila.principal",
			rk,
			"eCommerce",
			false,
			nil,
		)
		u.FailOnError(err, "Erro ao bindar a fila "+rk)
	}

	pk := s.LoadPrivateKey("cmd/principal/keys/private_key.pem")
	if pk == nil {
		log.Fatal("Chave privada não encontrada em cmd/principal/keys/private_key.pem. " +
			"Gere as chaves uma única vez com: go run ./cmd/gerar-chaves")
	}

	// O Principal recebe eventos de três produtores: cada evento é conferido
	// com a chave pública de quem é o dono daquele tipo de evento.
	chavesPublicas := map[string]*rsa.PublicKey{
		e.Estoque:   s.LoadPublicKey("cmd/principal/estoque-pub/public_key.pem"),
		e.Pagamento: s.LoadPublicKey("cmd/principal/pagamento-pub/public_key.pem"),
		e.Entrega:   s.LoadPublicKey("cmd/principal/entrega-pub/public_key.pem"),
	}

	events, err := ch.Consume(
		"fila.principal",
		"MS_Principal",
		false,
		false,
		false,
		false,
		nil,
	)
	u.FailOnError(err, "Erro ao registrar o consumidor")

	// O menu e o consumo publicam no mesmo canal, cada um na sua goroutine.
	var mu sync.Mutex
	publicar := func(tipo string, dados any) error {
		mu.Lock()
		defer mu.Unlock()
		return publicarEvento(ch, pk, tipo, dados)
	}

	pedidos := NovosPedidos()
	avisar := func(msg string) { fmt.Printf("\n[evento] %s\n", msg) }
	servico := NovoServico(pedidos, publicar, avisar)

	go func() {
		for d := range events {
			var env e.Envelope
			if err := json.Unmarshal(d.Body, &env); err != nil {
				avisar("Mensagem com JSON inválido. Evento descartado.")
				d.Ack(false)
				continue
			}

			produtor := e.ProdutorDoEvento[env.EventType]
			chave := chavesPublicas[produtor]
			if chave == nil || env.Producer != produtor ||
				!s.VerifySignature(chave, string(env.Data), []byte(env.Signature)) {
				avisar(fmt.Sprintf("Assinatura inválida do evento %s. Evento descartado.", env.EventType))
				d.Ack(false)
				continue
			}

			if err := servico.TratarEvento(env); err != nil {
				avisar(fmt.Sprintf("Erro ao tratar %s: %v. Evento descartado.", env.EventType, err))
			}
			d.Ack(false)
		}
	}()

	NovoMenu(servico, pedidos, produtos, os.Stdin, os.Stdout).Executar()
	fmt.Println("Até mais!")
}

// publicarEvento monta o envelope, assina o data com a chave do Principal e
// publica na exchange eCommerce, do mesmo jeito que o Estoque e o Pagamento.
func publicarEvento(ch *amqp.Channel, pk *rsa.PrivateKey, tipo string, dados any) error {
	dadosJSON, err := json.Marshal(dados)
	if err != nil {
		return fmt.Errorf("serializar dados de %s: %w", tipo, err)
	}

	body := e.Envelope{
		EventID:    uuid.NewString(),
		EventType:  tipo,
		Producer:   e.ProdutorDoEvento[tipo],
		OccurredAt: time.Now().UTC().Truncate(time.Millisecond),
		Data:       dadosJSON,
		Signature:  base64.StdEncoding.EncodeToString(s.SignPayload(pk, string(dadosJSON))),
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("serializar envelope de %s: %w", tipo, err)
	}

	err = ch.Publish(
		"eCommerce",
		tipo,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    body.EventID,
			Timestamp:    body.OccurredAt,
			Body:         bodyJSON,
		})
	if err != nil {
		return fmt.Errorf("publicar %s: %w", tipo, err)
	}
	return nil
}
