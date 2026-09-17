// Microsserviço Promoções: publica promoções aleatórias.
package main

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"math"
	"math/rand/v2"
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
		"Promocoes",
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	u.FailOnError(err, "Erro ao declarar a exchange")

	pk := s.LoadPrivateKey("cmd/promocoes/keys/private_key.pem")
	if pk == nil {
		log.Fatal("Chave privada não encontrada em cmd/promocoes/keys/private_key.pem. " +
			"Gere as chaves uma única vez com: go run ./cmd/gerar-chaves")
	}

	log.Printf(" [*] Publicando promoções a cada 20 a 30 s...")
	for {
		p := produtos[rand.IntN(len(produtos))]
		desconto := 5 + rand.IntN(46) // de 5% a 50%

		promo := e.PromocaoDados{
			ProdutoID:        p.ID,
			Nome:             p.Nome,
			Categoria:        p.Categoria,
			PrecoOriginal:    p.Preco,
			Desconto:         desconto,
			PrecoPromocional: math.Round(p.Preco*float64(100-desconto)) / 100,
		}
		promo_json, err := json.Marshal(promo)
		u.FailOnError(err, "Erro ao codificar os dados da promoção")

		// A routing key carrega a categoria: promocao.categoria.A, B ou C.
		tipo := e.TipoPromocao(p.Categoria)
		body := e.Envelope{
			EventID:    uuid.NewString(),
			EventType:  tipo,
			Producer:   e.ProdutorDoEvento[tipo],
			OccurredAt: time.Now().UTC().Truncate(time.Millisecond),
			Data:       promo_json,
			Signature:  base64.StdEncoding.EncodeToString(s.SignPayload(pk, string(promo_json))),
		}
		body_json, err := json.Marshal(body)
		u.FailOnError(err, "Erro ao codificar o envelope da promoção")

		err = ch.Publish(
			"Promocoes",
			tipo,
			false,
			false,
			amqp.Publishing{
				ContentType:  "application/json",
				DeliveryMode: amqp.Persistent,
				MessageId:    body.EventID,
				Timestamp:    body.OccurredAt,
				Body:         body_json,
			})
		u.FailOnError(err, "Erro ao publicar a promoção")

		log.Printf("Publicado %s: %s de R$ %.2f por R$ %.2f (-%d%%)",
			tipo, promo.Nome, promo.PrecoOriginal, promo.PrecoPromocional, promo.Desconto)

		time.Sleep(time.Duration(20+rand.IntN(11)) * time.Second)
	}
}
