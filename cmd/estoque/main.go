// Microsserviço Estoque: reserva e devolução de itens.
package main

import (
	"encoding/json"
	"log"
	"time"

	e "github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
	u "github.com/bdsromulo/Trab2-SistDistribuidos/internal/utils"
	amqp "github.com/rabbitmq/amqp091-go"
	uuid "github.com/google/uuid"
)

func main() {
	est := loadEstoque()

	conn, err := amqp.Dial("amqp://ecommerce:ecommerce@localhost:5672/")
	u.FailOnError(err, "Erro ao conectar ao RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	u.FailOnError(err, "Erro ao abrir canal")
	defer ch.Close()

	ch.ExchangeDeclare(
		"eCommerce",
		"direct",
		true,  // sobrevive ao reinício do broker
		false, // não é deletada quando sem consumidores
		false, // aceita pubs externas
		false, // aguarda confirmação do server
		nil,
	)
	u.FailOnError(err, "Erro ao declarar a exchange")

	_, err = ch.QueueDeclare(
		"fila.estoque",
		true,  // sobrevive ao reinício do broker
		false, // não é deletada quando sem consumidores
		false, // não é exclusiva
		false, // aguarda confirmação do server
		nil,
	)
	u.FailOnError(err, "Erro ao declarar a fila")

	for _, s := range []string{e.PedidoCriado, e.PedidoExcluido} {
		err = ch.QueueBind(
			"fila.estoque",
			s, // binding key
			"eCommerce",
			false,
			nil,
		)
		e := "Erro ao bindar a fila " + s
		u.FailOnError(err, e)
	}

	events, err := ch.Consume(
		"fila.estoque",
		"MS_Estoque",
		false,
		false,
		false,
		false,
		nil,
	)
	u.FailOnError(err, "Erro ao registrar o consumidor")

	var forever chan struct{}
	var env e.Envelope

	// reservas guarda os itens reservados por pedido para devolução ao excluir
	reservas := make(map[string][]e.Item)

	go func() {
		estoque_ok := true
		for d := range events {
			err = json.Unmarshal(d.Body, &env)
			if env.EventType == e.PedidoCriado {
				var data e.PedidoCriadoDados
				err = env.Decodificar(&data)
				u.FailOnError(err, "Erro ao decodificar os dados do pedido no envelope")
				var itensReservados []e.Item
				for _, i := range data.Itens {
					if est.Produtos[i.ProdutoID].Availability >= i.Quantidade {
						est.Produtos[i.ProdutoID].Availability -= i.Quantidade
						itensReservados = append(itensReservados, i)
						log.Printf("Reservando %d unidades do produto %s para o pedido %s", i.Quantidade, i.ProdutoID, data.PedidoID)
						// publicar evento de estoque OK
					} else {
						log.Printf("Estoque insuficiente para o produto %s do pedido %s", i.ProdutoID, data.PedidoID)
						estoque_ok = false
						// publicar evento de estoque indisponível
						break
					}
				}
				if estoque_ok {
					p_ok := e.PedidoEstoqueOKDados{
						PedidoID:   data.PedidoID,
						ValorTotal: data.ValorTotal,
					}
					p_json, err := json.Marshal(p_ok)
					u.FailOnError(err, "Erro ao serializar os dados do pedido estoque OK")

					body := e.Envelope{
						EventID:    uuid.NewString(),
						EventType:  e.PedidoEstoqueOK,
						Producer:   e.ProdutorDoEvento[e.PedidoEstoqueOK ],
						OccurredAt: time.Now().UTC().Truncate(time.Millisecond),
						Data:       p_json,
						Signature: "",
					}

					body_json, err := json.Marshal(body)
					u.FailOnError(err, "Erro ao serializar o envelope do pedido estoque OK")

					err = ch.Publish(
						"eCommerce",
						e.PedidoEstoqueOK,
						false,
						false,
						amqp.Publishing{
							ContentType:  "application/json",
							DeliveryMode: amqp.Persistent,
							MessageId:    body.EventID,
							Timestamp:    body.OccurredAt,
							Body:         []byte(body_json),
						})
					u.FailOnError(err, "Erro ao publicar o evento de pedido estoque OK")
				} else {
					p_ind := e.EstoqueIndisponivelDados {
						PedidoID: data.PedidoID,
					}
					p_json, err := json.Marshal(p_ind)
					u.FailOnError(err, "Erro ao serializar os dados do pedido estoque indisponível")

					body := e.Envelope{
						EventID:    uuid.NewString(),
						EventType:  e.EstoqueIndisponivel,
						Producer:   e.ProdutorDoEvento[e.EstoqueIndisponivel],
						OccurredAt: time.Now().UTC().Truncate(time.Millisecond),
						Data:       p_json,
						Signature: "",
					}
					body_json, err := json.Marshal(body)
					u.FailOnError(err, "Erro ao serializar o envelope do pedido estoque indisponível")

					err = ch.Publish(
						"eCommerce",
						e.EstoqueIndisponivel,
						false,
						false,
						amqp.Publishing{
							ContentType:  "application/json",
							DeliveryMode: amqp.Persistent,
							MessageId:    body.EventID,
							Timestamp:    body.OccurredAt,
							Body:         []byte(body_json),
						})
					u.FailOnError(err, "Erro ao publicar o evento de pedido estoque indisponível")
				}
				if len(itensReservados) > 0 {
					reservas[data.PedidoID] = itensReservados
				}
			} else if env.EventType == e.PedidoExcluido {
				var data e.PedidoExcluidoDados
				err = env.Decodificar(&data)
				u.FailOnError(err, "Erro ao decodificar os dados do pedido excluído no envelope")
				for _, i := range reservas[data.PedidoID] {
					est.Produtos[i.ProdutoID].Availability += i.Quantidade
					log.Printf("Devolvendo %d unidades do produto %s (pedido %s cancelado)", i.Quantidade, i.ProdutoID, data.PedidoID)
				}
				delete(reservas, data.PedidoID)
			}
			imprimirEstoque(est)
			//log.Printf("Recebido evento: %s", d.Body)
			d.Ack(false)
		}
	}()

	log.Printf("Aguardando eventos de pedidos...")

	<-forever
}
