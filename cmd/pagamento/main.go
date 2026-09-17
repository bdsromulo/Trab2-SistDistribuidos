// Microsserviço Pagamento: aprova ou recusa os pagamentos.
package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"math/big"
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

	ch.ExchangeDeclare(
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
		"fila.pagamento",
		true,
		false,
		false,
		false,
		nil,
	)
	u.FailOnError(err, "Erro ao declarar a fila")

	err = ch.QueueBind(
		"fila.pagamento",
		e.PedidoEstoqueOK,
		"eCommerce",
		false,
		nil,
	)
	u.FailOnError(err, "Erro ao bindar a fila")

	pk := s.LoadPrivateKey("cmd/pagamento/keys/private_key.pem")
	if pk == nil {
		log.Fatal("Chave privada não encontrada em cmd/pagamento/keys/private_key.pem. " +
			"Gere as chaves uma única vez com: go run ./cmd/gerar-chaves")
	}

	pub_key_ms_e := s.LoadPublicKey("cmd/pagamento/estoque-pub/public_key.pem")

	events, err := ch.Consume(
		"fila.pagamento",
		"MS_Pagamento",
		false,
		false,
		false,
		false,
		nil,
	)
	u.FailOnError(err, "Erro ao registrar o consumidor")

	var forever chan struct{}
	var env e.Envelope
	go func() {
		for d := range events {
			err = json.Unmarshal(d.Body, &env)
			if !s.VerifySignature(pub_key_ms_e, string(env.Data), []byte(env.Signature)) {
				log.Printf("Assinatura inválida do evento %s. Evento descartado.", env.EventType)
				d.Ack(false)
				continue
			}
			if env.EventType == e.PedidoEstoqueOK {
				random_number, err := rand.Int(rand.Reader, big.NewInt(100))
				if random_number.Cmp(big.NewInt(80)) <= 0 {
					var data e.PedidoEstoqueOKDados
					err = env.Decodificar(&data)
					u.FailOnError(err, "Erro ao decodificar os dados do envelope")

					pag_aprv := e.PagamentoAprovadoDados{
						PedidoID:   data.PedidoID,
						ValorTotal: data.ValorTotal,
					}
					pag_json, err := json.Marshal(pag_aprv)
					u.FailOnError(err, "Erro ao codificar os dados do pagamento aprovado")
					body := e.Envelope{
						EventID:    uuid.NewString(),
						EventType:  e.PagamentoAprovado,
						Producer:   e.ProdutorDoEvento[e.PagamentoAprovado],
						OccurredAt: time.Now().UTC().Truncate(time.Millisecond),
						Data:       pag_json,
						Signature:  base64.StdEncoding.EncodeToString(s.SignPayload(pk, string(pag_json))),
					}

					body_json, err := json.Marshal(body)
					u.FailOnError(err, "Erro ao codificar o envelope do pagamento aprovado")

					err = ch.Publish(
						"eCommerce",
						e.PagamentoAprovado,
						false,
						false,
						amqp.Publishing{
							ContentType:  "application/json",
							DeliveryMode: amqp.Persistent,
							MessageId:    body.EventID,
							Timestamp:    body.OccurredAt,
							Body:         []byte(body_json),
						})
					u.FailOnError(err, "Erro ao publicar o evento de pagamento aprovado")
				} else {
					var data e.PedidoEstoqueOKDados
					err = env.Decodificar(&data)
					pag_rec := e.PagamentoRecusadoDados{
						PedidoID: data.PedidoID,
						Motivo:   data.PedidoID + " - Pagamento recusado pelo MS Pagamento",
					}
					pag_json, err := json.Marshal(pag_rec)
					u.FailOnError(err, "Erro ao codificar os dados do pagamento recusado")

					body := e.Envelope{
						EventID:    uuid.NewString(),
						EventType:  e.PagamentoRecusado,
						Producer:   e.ProdutorDoEvento[e.PagamentoRecusado],
						OccurredAt: time.Now().UTC().Truncate(time.Millisecond),
						Data:       pag_json,
						Signature:  base64.StdEncoding.EncodeToString(s.SignPayload(pk, string(pag_json))),
					}
					body_json, err := json.Marshal(body)
					u.FailOnError(err, "Erro ao codificar o envelope do pagamento recusado")

					err = ch.Publish(
						"eCommerce",
						e.PagamentoRecusado,
						false,
						false,
						amqp.Publishing{
							ContentType:  "application/json",
							DeliveryMode: amqp.Persistent,
							MessageId:    body.EventID,
							Timestamp:    body.OccurredAt,
							Body:         []byte(body_json),
						})
					u.FailOnError(err, "Erro ao publicar o evento de pagamento recusado")
				}
			} else {
				log.Printf("Tipo de evento (%s) não suportado no MS Pagamento. Evento descartado.", env.EventType)
				d.Ack(false)
				continue
			}
			d.Ack(false)
		}
	}()

	log.Printf(" [*] Aguardando eventos de estoque...")
	<-forever
}
