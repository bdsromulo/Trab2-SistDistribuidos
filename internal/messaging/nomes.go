// Package messaging cuida da comunicação com o RabbitMQ: conexão,
// declaração da topologia, publicação e consumo de eventos.
package messaging

import (
	"strings"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
)

// Exchanges. "Promocoes" fica sem acento.
const (
	ExchangeECommerce = "eCommerce" // direct
	ExchangePromocoes = "Promocoes" // topic
)

// Filas: uma por consumidor (Figura 2 do enunciado).
const (
	FilaPrincipal = "fila.principal"
	FilaEstoque   = "fila.estoque"
	FilaPagamento = "fila.pagamento"
	FilaEntrega   = "fila.entrega"
	FilaC1        = "fila.C1"
	FilaC2        = "fila.C2"
)

// ExchangeDoEvento diz em qual exchange cada tipo de evento é publicado:
// promoções vão para Promocoes; todo o resto, para eCommerce.
func ExchangeDoEvento(tipo string) string {
	if strings.HasPrefix(tipo, events.PrefixoPromocao) {
		return ExchangePromocoes
	}
	return ExchangeECommerce
}
