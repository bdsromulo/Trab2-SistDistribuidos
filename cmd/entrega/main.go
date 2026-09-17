// Microsserviço Entrega: emite a nota fiscal e envia o pedido.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"time"

	e "github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
	s "github.com/bdsromulo/Trab2-SistDistribuidos/internal/signature"
	u "github.com/bdsromulo/Trab2-SistDistribuidos/internal/utils"
	uuid "github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {

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
		"fila.entrega",
		true,
		false,
		false,
		false,
		nil,
	)
	u.FailOnError(err, "Erro ao declarar a fila")

	err = ch.QueueBind(
		"fila.entrega",
		e.PagamentoAprovado,
		"eCommerce",
		false,
		nil,
	)
	u.FailOnError(err, "Erro ao bindar a fila")

	pk := s.LoadPrivateKey("cmd/entrega/keys/private_key.pem")
	if pk == nil {
		log.Fatal("Chave privada não encontrada em cmd/entrega/keys/private_key.pem. " +
			"Gere as chaves uma única vez com: go run ./cmd/gerar-chaves")
	}

	pubKeyPagamento := s.LoadPublicKey("cmd/entrega/pagamento-pub/public_key.pem")

	events, err := ch.Consume(
		"fila.entrega",
		"MS_Entrega",
		false,
		false,
		false,
		false,
		nil,
	)
	u.FailOnError(err, "Erro ao registrar o consumidor")

	var forever chan struct{}
	go func() {
		for d := range events {
			var env e.Envelope
			if err := json.Unmarshal(d.Body, &env); err != nil {
				log.Printf("Mensagem com JSON inválido. Evento descartado.")
				d.Ack(false)
				continue
			}
			if env.EventType != e.PagamentoAprovado || env.Producer != e.Pagamento {
				log.Printf("Tipo de evento (%s) não suportado no MS Entrega. Evento descartado.", env.EventType)
				d.Ack(false)
				continue
			}
			if !s.VerifySignature(pubKeyPagamento, string(env.Data), []byte(env.Signature)) {
				log.Printf("Assinatura inválida do evento %s. Evento descartado.", env.EventType)
				d.Ack(false)
				continue
			}

			var data e.PagamentoAprovadoDados
			if err := env.Decodificar(&data); err != nil {
				log.Printf("Dados inválidos no evento %s. Evento descartado.", env.EventType)
				d.Ack(false)
				continue
			}

			// Simula a emissão da nota fiscal e a preparação da entrega (1 a 3 s).
			log.Printf("Emitindo nota fiscal e preparando a entrega do pedido %s...", data.PedidoID)
			time.Sleep(time.Duration(1000+rand.IntN(2001)) * time.Millisecond)

			enviado := e.PedidoEnviadoDados{
				PedidoID:         data.PedidoID,
				NumeroNotaFiscal: fmt.Sprintf("NF-%06d", rand.IntN(1000000)),
				CodigoRastreio:   fmt.Sprintf("BR%09dBR", rand.IntN(1000000000)),
			}
			enviado_json, err := json.Marshal(enviado)
			u.FailOnError(err, "Erro ao codificar os dados do pedido enviado")

			body := e.Envelope{
				EventID:    uuid.NewString(),
				EventType:  e.PedidoEnviado,
				Producer:   e.ProdutorDoEvento[e.PedidoEnviado],
				OccurredAt: time.Now().UTC().Truncate(time.Millisecond),
				Data:       enviado_json,
				Signature:  base64.StdEncoding.EncodeToString(s.SignPayload(pk, string(enviado_json))),
			}
			body_json, err := json.Marshal(body)
			u.FailOnError(err, "Erro ao codificar o envelope do pedido enviado")

			err = ch.Publish(
				"eCommerce",
				e.PedidoEnviado,
				false,
				false,
				amqp.Publishing{
					ContentType:  "application/json",
					DeliveryMode: amqp.Persistent,
					MessageId:    body.EventID,
					Timestamp:    body.OccurredAt,
					Body:         body_json,
				})
			u.FailOnError(err, "Erro ao publicar o evento de pedido enviado")

			log.Printf("Pedido %s enviado: NF %s, rastreio %s", enviado.PedidoID, enviado.NumeroNotaFiscal, enviado.CodigoRastreio)
			d.Ack(false)
		}
	}()

	log.Printf(" [*] Aguardando pagamentos aprovados...")
	<-forever
}
