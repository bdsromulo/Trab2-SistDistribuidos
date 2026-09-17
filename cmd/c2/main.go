// Consumidor C2: recebe promoções de todas as categorias.
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
		"fila.C2",
		true,
		false,
		false,
		false,
		nil,
	)
	u.FailOnError(err, "Erro ao declarar a fila")

	// "*" substitui exatamente uma palavra: casa com A, B, C e qualquer
	// categoria nova, sem precisar de um binding por categoria.
	err = ch.QueueBind(
		"fila.C2",
		e.PrefixoPromocao+"*",
		"Promocoes",
		false,
		nil,
	)
	u.FailOnError(err, "Erro ao bindar a fila")

	// O cmd/gerar-chaves distribui as públicas só para os cinco microsserviços;
	// C2 lê a pública que o Promoções guarda na própria pasta de chaves.
	pubKeyPromocoes := s.LoadPublicKey("cmd/promocoes/keys/public_key.pem")

	events, err := ch.Consume(
		"fila.C2",
		"C2",
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
			log.Printf("[C2] Promoção categoria %s: %s de R$ %.2f por R$ %.2f (-%d%%)",
				promo.Categoria, promo.Nome, promo.PrecoOriginal, promo.PrecoPromocional, promo.Desconto)
			d.Ack(false)
		}
	}()

	log.Printf(" [*] C2 aguardando promoções de todas as categorias...")
	<-forever
}
