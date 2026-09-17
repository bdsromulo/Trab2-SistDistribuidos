// Consumidor C1: recebe promoções das categorias A e B.
package main

import (
	"encoding/json"
	"log"

	e "github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
	s "github.com/bdsromulo/Trab2-SistDistribuidos/internal/signature"
	u "github.com/bdsromulo/Trab2-SistDistribuidos/internal/utils"
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
		"Promocoes",
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	u.FailOnError(err, "Erro ao declarar a exchange")

	_, err = ch.QueueDeclare(
		"fila.C1",
		true,
		false,
		false,
		false,
		nil,
	)
	u.FailOnError(err, "Erro ao declarar a fila")

	// Bindings exatos: só as categorias A e B chegam nesta fila.
	for _, rk := range []string{e.PromocaoCategoriaA, e.PromocaoCategoriaB} {
		err = ch.QueueBind(
			"fila.C1",
			rk,
			"Promocoes",
			false,
			nil,
		)
		u.FailOnError(err, "Erro ao bindar a fila "+rk)
	}

	// O cmd/gerar-chaves distribui as públicas só para os cinco microsserviços;
	// C1 lê a pública que o Promoções guarda na própria pasta de chaves.
	pubKeyPromocoes := s.LoadPublicKey("cmd/promocoes/keys/public_key.pem")

	events, err := ch.Consume(
		"fila.C1",
		"C1",
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
			if env.Producer != e.Promocoes ||
				!s.VerifySignature(pubKeyPromocoes, string(env.Data), []byte(env.Signature)) {
				log.Printf("Assinatura inválida do evento %s. Evento descartado.", env.EventType)
				d.Ack(false)
				continue
			}

			var promo e.PromocaoDados
			if err := env.Decodificar(&promo); err != nil {
				log.Printf("Dados inválidos no evento %s. Evento descartado.", env.EventType)
				d.Ack(false)
				continue
			}
			log.Printf("[C1] Promoção categoria %s: %s de R$ %.2f por R$ %.2f (-%d%%)",
				promo.Categoria, promo.Nome, promo.PrecoOriginal, promo.PrecoPromocional, promo.Desconto)
			d.Ack(false)
		}
	}()

	log.Printf(" [*] C1 aguardando promoções das categorias A e B...")
	<-forever
}
