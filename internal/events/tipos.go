package events

// Produtores de eventos. O nome vai no campo Producer do envelope e é usado
// para escolher a chave pública na verificação da assinatura.
const (
	Principal = "principal"
	Estoque   = "estoque"
	Pagamento = "pagamento"
	Entrega   = "entrega"
	Promocoes = "promocoes"
)

// Tipos de evento. O EventType do envelope é sempre igual à routing key.
const (
	PedidoCriado        = "pedido.criado"
	PedidoExcluido      = "pedido.excluido"
	PedidoEstoqueOK     = "pedido.estoque_ok"
	EstoqueIndisponivel = "estoque.indisponivel"
	PagamentoAprovado   = "pagamento.aprovado"
	PagamentoRecusado   = "pagamento.recusado"
	PedidoEnviado       = "pedido.enviado"

	PrefixoPromocao    = "promocao.categoria."
	PromocaoCategoriaA = PrefixoPromocao + "A"
	PromocaoCategoriaB = PrefixoPromocao + "B"
	PromocaoCategoriaC = PrefixoPromocao + "C"
)

// TipoPromocao devolve a routing key da promoção de uma categoria (A, B ou C).
func TipoPromocao(categoria string) string {
	return PrefixoPromocao + categoria
}

// ProdutorDoEvento diz qual serviço pode publicar cada tipo de evento
// (Figura 1 do enunciado). A verificação rejeita um evento cujo Producer
// não seja o dono do tipo.
var ProdutorDoEvento = map[string]string{
	PedidoCriado:        Principal,
	PedidoExcluido:      Principal,
	PedidoEstoqueOK:     Estoque,
	EstoqueIndisponivel: Estoque,
	PagamentoAprovado:   Pagamento,
	PagamentoRecusado:   Pagamento,
	PedidoEnviado:       Entrega,
	PromocaoCategoriaA:  Promocoes,
	PromocaoCategoriaB:  Promocoes,
	PromocaoCategoriaC:  Promocoes,
}
