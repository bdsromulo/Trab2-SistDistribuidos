package events

// Item é um produto dentro de um pedido.
type Item struct {
	ProdutoID     string  `json:"produto_id"`
	Nome          string  `json:"nome"`
	Quantidade    int     `json:"quantidade"`
	PrecoUnitario float64 `json:"preco_unitario"`
}

// PedidoCriadoDados é publicado pelo Principal ao cadastrar um pedido.
type PedidoCriadoDados struct {
	PedidoID   string  `json:"pedido_id"`
	Itens      []Item  `json:"itens"`
	ValorTotal float64 `json:"valor_total"`
}

// Motivos de exclusão de um pedido.
const (
	MotivoUsuario           = "usuario"
	MotivoFaltaEstoque      = "falta_estoque"
	MotivoPagamentoRecusado = "pagamento_recusado"
)

// PedidoExcluidoDados é publicado pelo Principal, no máximo uma vez por pedido.
type PedidoExcluidoDados struct {
	PedidoID string `json:"pedido_id"`
	Motivo   string `json:"motivo"`
}

// PedidoEstoqueOKDados é publicado pelo Estoque quando reserva todos os itens.
type PedidoEstoqueOKDados struct {
	PedidoID   string  `json:"pedido_id"`
	ValorTotal float64 `json:"valor_total"`
}

// ItemFaltante é um item que o Estoque não conseguiu atender.
type ItemFaltante struct {
	ProdutoID  string `json:"produto_id"`
	Solicitado int    `json:"solicitado"`
	Disponivel int    `json:"disponivel"`
}

// EstoqueIndisponivelDados é publicado pelo Estoque quando falta algum item.
type EstoqueIndisponivelDados struct {
	PedidoID       string         `json:"pedido_id"`
	ItensFaltantes []ItemFaltante `json:"itens_faltantes"`
}

// PagamentoAprovadoDados é publicado pelo Pagamento.
type PagamentoAprovadoDados struct {
	PedidoID   string  `json:"pedido_id"`
	ValorTotal float64 `json:"valor_total"`
}

// PagamentoRecusadoDados é publicado pelo Pagamento.
type PagamentoRecusadoDados struct {
	PedidoID string `json:"pedido_id"`
	Motivo   string `json:"motivo"`
}

// PedidoEnviadoDados é publicado pela Entrega.
type PedidoEnviadoDados struct {
	PedidoID         string `json:"pedido_id"`
	NumeroNotaFiscal string `json:"numero_nota_fiscal"`
	CodigoRastreio   string `json:"codigo_rastreio"`
}

// PromocaoDados é publicado pelo Promoções em promocao.categoria.<A|B|C>.
type PromocaoDados struct {
	ProdutoID        string  `json:"produto_id"`
	Nome             string  `json:"nome"`
	Categoria        string  `json:"categoria"`
	PrecoOriginal    float64 `json:"preco_original"`
	Desconto         int     `json:"desconto"` // em %, de 5 a 50
	PrecoPromocional float64 `json:"preco_promocional"`
}
