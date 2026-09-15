package main

import (
	"errors"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
)

// Status de um pedido. Fora o Excluido, eles só andam para a frente.
type Status string

const (
	StatusAguardandoEstoque Status = "aguardando estoque"
	StatusAguardandoPagto   Status = "aguardando pagamento" // estoque reservado
	StatusPago              Status = "pagamento aprovado"
	StatusEnviado           Status = "enviado"
	StatusExcluido          Status = "excluído"
)

// etapa dá a posição de cada status no fluxo, para impedir que ele volte.
var etapa = map[Status]int{
	StatusAguardandoEstoque: 1,
	StatusAguardandoPagto:   2,
	StatusPago:              3,
	StatusEnviado:           4,
}

// Pedido guardado pelo Principal.
type Pedido struct {
	ID         string
	Itens      []events.Item
	ValorTotal float64
	Status     Status
	Motivo     string // motivo da exclusão (events.Motivo...)
	Detalhe    string // informação extra do último evento (nota fiscal, itens em falta...)
	CriadoEm   time.Time
}

// Erros das operações sobre pedidos.
var (
	ErrPedidoNaoEncontrado = errors.New("pedido não encontrado")
	ErrPedidoExcluido      = errors.New("pedido já foi excluído")
	ErrPedidoEnviado       = errors.New("pedido já foi enviado e não pode ser excluído")
	ErrStatusAntigo        = errors.New("status igual ou anterior ao atual")
)

// Pedidos guarda os pedidos em memória. O menu e o consumo de eventos rodam
// em goroutines diferentes, por isso todo acesso passa pelo mutex.
type Pedidos struct {
	mu    sync.Mutex
	porID map[string]*Pedido
	ids   []string // ordem de criação, para listar
}

func NovosPedidos() *Pedidos {
	return &Pedidos{porID: map[string]*Pedido{}}
}

// Criar registra um pedido novo, aguardando o estoque.
func (p *Pedidos) Criar(itens []events.Item) Pedido {
	total := 0.0
	for _, it := range itens {
		total += float64(it.Quantidade) * it.PrecoUnitario
	}
	pedido := &Pedido{
		ID:         uuid.NewString()[:8], // curto, para digitar no menu
		Itens:      append([]events.Item(nil), itens...),
		ValorTotal: math.Round(total*100) / 100,
		Status:     StatusAguardandoEstoque,
		CriadoEm:   time.Now(),
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.porID[pedido.ID] = pedido
	p.ids = append(p.ids, pedido.ID)
	return *pedido
}

// Remover apaga um pedido (usado quando a publicação de pedido.criado falha).
func (p *Pedidos) Remover(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.porID, id)
	for i, atual := range p.ids {
		if atual == id {
			p.ids = append(p.ids[:i], p.ids[i+1:]...)
			break
		}
	}
}

// Buscar devolve uma cópia do pedido.
func (p *Pedidos) Buscar(id string) (Pedido, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	pedido, ok := p.porID[id]
	if !ok {
		return Pedido{}, false
	}
	return *pedido, true
}

// Listar devolve cópias de todos os pedidos, na ordem de criação.
func (p *Pedidos) Listar() []Pedido {
	p.mu.Lock()
	defer p.mu.Unlock()
	lista := make([]Pedido, 0, len(p.ids))
	for _, id := range p.ids {
		lista = append(lista, *p.porID[id])
	}
	return lista
}

// Avancar muda o status do pedido, desde que ele ande para a frente.
// Pedido excluído não muda mais: eventos atrasados sobre ele são ignorados.
func (p *Pedidos) Avancar(id string, novo Status, detalhe string) (Pedido, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	pedido, ok := p.porID[id]
	if !ok {
		return Pedido{}, ErrPedidoNaoEncontrado
	}
	if pedido.Status == StatusExcluido {
		return *pedido, ErrPedidoExcluido
	}
	if etapa[novo] <= etapa[pedido.Status] {
		return *pedido, ErrStatusAntigo
	}
	pedido.Status = novo
	pedido.Detalhe = detalhe
	return *pedido, nil
}

// Excluir marca o pedido como excluído. Como só a primeira chamada tem
// sucesso, pedido.excluido é publicado no máximo uma vez por pedido.
func (p *Pedidos) Excluir(id, motivo, detalhe string) (Pedido, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	pedido, ok := p.porID[id]
	if !ok {
		return Pedido{}, ErrPedidoNaoEncontrado
	}
	switch pedido.Status {
	case StatusExcluido:
		return *pedido, ErrPedidoExcluido
	case StatusEnviado:
		return *pedido, ErrPedidoEnviado
	}
	pedido.Status = StatusExcluido
	pedido.Motivo = motivo
	pedido.Detalhe = detalhe
	return *pedido, nil
}
