// Adulterador: publica eventos inválidos para demonstrar que os serviços
// descartam tudo o que não passa na verificação de assinatura.
//
// Para cada evento alvo são publicadas três mensagens:
//  1. sem assinatura;
//  2. assinada com uma chave que não é a do produtor (mensagem forjada);
//  3. assinada pelo produtor verdadeiro e com o data alterado depois
//     (mensagem adulterada no caminho).
//
// pedido.criado vai para o Estoque; pagamento.aprovado, para o Principal e a
// Entrega. Os três devem registrar "Assinatura inválida ... Evento descartado".
package main

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"log"
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

	// Chave nova, que nenhum serviço conhece: simula um atacante.
	chaveFalsa := s.GenerateKey()

	item := e.Item{ProdutoID: "P01", Nome: "Fone de ouvido Bluetooth", Quantidade: 1, PrecoUnitario: 199.90}
	itemAlterado := item
	itemAlterado.Quantidade = 50

	alvos := []struct {
		tipo       string
		original   any
		adulterado any
	}{
		{
			e.PedidoCriado,
			e.PedidoCriadoDados{PedidoID: "adulterado", Itens: []e.Item{item}, ValorTotal: 199.90},
			e.PedidoCriadoDados{PedidoID: "adulterado", Itens: []e.Item{itemAlterado}, ValorTotal: 199.90},
		},
		{
			e.PagamentoAprovado,
			e.PagamentoAprovadoDados{PedidoID: "adulterado", ValorTotal: 199.90},
			e.PagamentoAprovadoDados{PedidoID: "adulterado", ValorTotal: 0.01},
		},
	}

	for _, alvo := range alvos {
		produtor := e.ProdutorDoEvento[alvo.tipo]
		caminho := "cmd/" + produtor + "/keys/private_key.pem"
		chaveProdutor := s.LoadPrivateKey(caminho)
		if chaveProdutor == nil {
			log.Fatalf("Chave privada não encontrada em %s. "+
				"Gere as chaves uma única vez com: go run ./cmd/gerar-chaves", caminho)
		}

		original, err := json.Marshal(alvo.original)
		u.FailOnError(err, "Erro ao codificar os dados originais")
		adulterado, err := json.Marshal(alvo.adulterado)
		u.FailOnError(err, "Erro ao codificar os dados adulterados")

		publicar(ch, alvo.tipo, produtor, original, "",
			"sem assinatura")
		publicar(ch, alvo.tipo, produtor, original, assinar(chaveFalsa, original),
			"assinada com chave que não é do "+produtor)
		publicar(ch, alvo.tipo, produtor, adulterado, assinar(chaveProdutor, original),
			"assinada pelo "+produtor+" e com o data alterado depois")
	}

	log.Printf("Pronto: confira nos terminais dos serviços que as 6 mensagens foram descartadas.")
}

func assinar(chave *rsa.PrivateKey, data []byte) string {
	return base64.StdEncoding.EncodeToString(s.SignPayload(chave, string(data)))
}

// publicar manda o envelope como se viesse do produtor verdadeiro.
func publicar(ch *amqp.Channel, tipo, produtor string, data []byte, assinatura, descricao string) {
	body := e.Envelope{
		EventID:    uuid.NewString(),
		EventType:  tipo,
		Producer:   produtor,
		OccurredAt: time.Now().UTC().Truncate(time.Millisecond),
		Data:       data,
		Signature:  assinatura,
	}
	body_json, err := json.Marshal(body)
	u.FailOnError(err, "Erro ao codificar o envelope")

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
			Body:         body_json,
		})
	u.FailOnError(err, "Erro ao publicar a mensagem adulterada")

	log.Printf("Publicado %s: %s", tipo, descricao)
}
