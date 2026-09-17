package main

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/catalogo"
	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
)

var itensTeste = []events.Item{
	{ProdutoID: "P01", Nome: "Fone", Quantidade: 2, PrecoUnitario: 199.90},
	{ProdutoID: "P06", Nome: "Caderno", Quantidade: 1, PrecoUnitario: 24.90},
}

// publicado é um evento que o serviço mandou publicar durante o teste.
type publicado struct {
	tipo  string
	dados any
}

// gravador substitui o RabbitMQ nos testes: só anota o que foi publicado.
type gravador struct {
	publicados []publicado
}

func (g *gravador) publicar(tipo string, dados any) error {
	g.publicados = append(g.publicados, publicado{tipo, dados})
	return nil
}

func (g *gravador) tipos() []string {
	tipos := make([]string, len(g.publicados))
	for i, p := range g.publicados {
		tipos[i] = p.tipo
	}
	return tipos
}

func novoTeste(t *testing.T) (*Servico, *Pedidos, *gravador, Pedido) {
	t.Helper()
	pedidos := NovosPedidos()
	pub := &gravador{}
	s := NovoServico(pedidos, pub.publicar, func(string) {})
	s.janela = time.Hour // o teste encerra a janela na mão, sem esperar o timer
	pedido, err := s.CriarPedido(itensTeste)
	if err != nil {
		t.Fatal(err)
	}
	s.confirmar(pedido.ID)
	pedido, _ = pedidos.Buscar(pedido.ID)
	return s, pedidos, pub, pedido
}

// evento monta um envelope como chegaria da fila.
func evento(t *testing.T, tipo string, dados any) events.Envelope {
	t.Helper()
	bruto, err := json.Marshal(dados)
	if err != nil {
		t.Fatal(err)
	}
	return events.Envelope{EventType: tipo, Producer: events.ProdutorDoEvento[tipo], Data: bruto}
}

func tratar(t *testing.T, s *Servico, env events.Envelope) {
	t.Helper()
	if err := s.TratarEvento(env); err != nil {
		t.Fatalf("TratarEvento(%s): %v", env.EventType, err)
	}
}

func status(t *testing.T, pedidos *Pedidos, id string) Status {
	t.Helper()
	p, ok := pedidos.Buscar(id)
	if !ok {
		t.Fatalf("pedido %s sumiu", id)
	}
	return p.Status
}

func TestCriarPedidoPublicaPedidoCriado(t *testing.T) {
	_, _, pub, pedido := novoTeste(t)

	if pedido.ValorTotal != 424.70 || pedido.Status != StatusAguardandoEstoque {
		t.Errorf("pedido criado errado: %+v", pedido)
	}
	if len(pub.publicados) != 1 || pub.publicados[0].tipo != events.PedidoCriado {
		t.Fatalf("publicados = %v", pub.tipos())
	}
	d := pub.publicados[0].dados.(events.PedidoCriadoDados)
	if d.PedidoID != pedido.ID || d.ValorTotal != 424.70 || len(d.Itens) != 2 {
		t.Errorf("dados de pedido.criado errados: %+v", d)
	}
}

func TestFluxoFeliz(t *testing.T) {
	s, pedidos, pub, pedido := novoTeste(t)
	id := pedido.ID

	tratar(t, s, evento(t, events.PedidoEstoqueOK, events.PedidoEstoqueOKDados{PedidoID: id}))
	if got := status(t, pedidos, id); got != StatusAguardandoPagto {
		t.Errorf("depois do estoque_ok: %s", got)
	}
	tratar(t, s, evento(t, events.PagamentoAprovado, events.PagamentoAprovadoDados{PedidoID: id}))
	tratar(t, s, evento(t, events.PedidoEnviado, events.PedidoEnviadoDados{PedidoID: id, NumeroNotaFiscal: "1"}))
	if got := status(t, pedidos, id); got != StatusEnviado {
		t.Errorf("no fim: %s", got)
	}
	if !slices.Equal(pub.tipos(), []string{events.PedidoCriado}) {
		t.Errorf("fluxo feliz não deveria publicar nada além de pedido.criado: %v", pub.tipos())
	}
}

func TestStatusNaoVolta(t *testing.T) {
	s, pedidos, _, pedido := novoTeste(t)
	id := pedido.ID

	tratar(t, s, evento(t, events.PagamentoAprovado, events.PagamentoAprovadoDados{PedidoID: id}))
	tratar(t, s, evento(t, events.PedidoEstoqueOK, events.PedidoEstoqueOKDados{PedidoID: id})) // atrasado
	if got := status(t, pedidos, id); got != StatusPago {
		t.Errorf("status voltou para %s", got)
	}
}

func TestEstoqueIndisponivelExcluiPedido(t *testing.T) {
	s, pedidos, pub, pedido := novoTeste(t)

	tratar(t, s, evento(t, events.EstoqueIndisponivel, events.EstoqueIndisponivelDados{PedidoID: pedido.ID}))

	p, _ := pedidos.Buscar(pedido.ID)
	if p.Status != StatusExcluido || p.Motivo != events.MotivoFaltaEstoque {
		t.Errorf("pedido depois da falta de estoque: %+v", p)
	}
	ultimo := pub.publicados[len(pub.publicados)-1]
	if ultimo.tipo != events.PedidoExcluido ||
		ultimo.dados.(events.PedidoExcluidoDados).Motivo != events.MotivoFaltaEstoque {
		t.Errorf("esperava pedido.excluido por falta de estoque, publicou %+v", ultimo)
	}
}

func TestPagamentoRecusadoPublicaExclusaoUmaVez(t *testing.T) {
	s, pedidos, pub, pedido := novoTeste(t)
	recusado := evento(t, events.PagamentoRecusado, events.PagamentoRecusadoDados{PedidoID: pedido.ID, Motivo: "cartão recusado"})

	tratar(t, s, recusado)
	tratar(t, s, recusado) // mensagem repetida

	if got := status(t, pedidos, pedido.ID); got != StatusExcluido {
		t.Errorf("status: %s", got)
	}
	if !slices.Equal(pub.tipos(), []string{events.PedidoCriado, events.PedidoExcluido}) {
		t.Errorf("publicados = %v", pub.tipos())
	}
}

func TestExclusaoPeloUsuario(t *testing.T) {
	s, _, pub, pedido := novoTeste(t)

	if err := s.ExcluirPeloUsuario(pedido.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.ExcluirPeloUsuario(pedido.ID); !errors.Is(err, ErrPedidoExcluido) {
		t.Errorf("segunda exclusão: %v", err)
	}
	// Evento atrasado de um pedido já excluído é ignorado sem publicar nada.
	tratar(t, s, evento(t, events.PagamentoRecusado, events.PagamentoRecusadoDados{PedidoID: pedido.ID}))

	if !slices.Equal(pub.tipos(), []string{events.PedidoCriado, events.PedidoExcluido}) {
		t.Errorf("publicados = %v", pub.tipos())
	}
	if d := pub.publicados[1].dados.(events.PedidoExcluidoDados); d.Motivo != events.MotivoUsuario {
		t.Errorf("motivo = %q", d.Motivo)
	}
}

func TestCancelamentoNaJanelaNaoPublicaNada(t *testing.T) {
	pedidos := NovosPedidos()
	pub := &gravador{}
	s := NovoServico(pedidos, pub.publicar, func(string) {})
	s.janela = time.Hour
	pedido, _ := s.CriarPedido(itensTeste)
	if pedido.Status != StatusAguardandoConfirmacao || len(pub.publicados) != 0 {
		t.Fatalf("durante a janela: status %s, publicados %v", pedido.Status, pub.tipos())
	}

	if err := s.ExcluirPeloUsuario(pedido.ID); err != nil {
		t.Fatal(err)
	}
	s.confirmar(pedido.ID) // a janela acaba depois do cancelamento

	if got := status(t, pedidos, pedido.ID); got != StatusExcluido {
		t.Errorf("status: %s", got)
	}
	if len(pub.publicados) != 0 {
		t.Errorf("pedido cancelado na janela não deveria publicar nada: %v", pub.tipos())
	}
}

func TestPedidoPagoNaoPodeSerExcluido(t *testing.T) {
	s, pedidos, pub, pedido := novoTeste(t)
	tratar(t, s, evento(t, events.PedidoEstoqueOK, events.PedidoEstoqueOKDados{PedidoID: pedido.ID}))
	tratar(t, s, evento(t, events.PagamentoAprovado, events.PagamentoAprovadoDados{PedidoID: pedido.ID}))

	if err := s.ExcluirPeloUsuario(pedido.ID); !errors.Is(err, ErrPedidoPago) {
		t.Errorf("esperava ErrPedidoPago, veio %v", err)
	}
	if got := status(t, pedidos, pedido.ID); got != StatusPago {
		t.Errorf("status: %s", got)
	}
	if !slices.Equal(pub.tipos(), []string{events.PedidoCriado}) {
		t.Errorf("não deveria publicar pedido.excluido: %v", pub.tipos())
	}
}

func TestPedidoEnviadoNaoPodeSerExcluido(t *testing.T) {
	s, _, _, pedido := novoTeste(t)
	tratar(t, s, evento(t, events.PedidoEnviado, events.PedidoEnviadoDados{PedidoID: pedido.ID}))

	if err := s.ExcluirPeloUsuario(pedido.ID); !errors.Is(err, ErrPedidoEnviado) {
		t.Errorf("esperava ErrPedidoEnviado, veio %v", err)
	}
}

func TestEventoDePedidoDesconhecidoEIgnorado(t *testing.T) {
	s, _, _, _ := novoTeste(t)
	tratar(t, s, evento(t, events.PedidoEstoqueOK, events.PedidoEstoqueOKDados{PedidoID: "nao-existe"}))
}

func TestMenuFazPedido(t *testing.T) {
	pedidos := NovosPedidos()
	pub := &gravador{}
	s := NovoServico(pedidos, pub.publicar, func(string) {})
	s.janela = time.Hour
	produtos := []catalogo.Produto{{ID: "P01", Nome: "Fone", Categoria: "A", Preco: 10}}
	entrada := strings.NewReader("2\np01\n2\nP01\n1\n\n0\n") // P01 duas vezes: soma no mesmo item
	var saida strings.Builder

	NovoMenu(s, pedidos, produtos, entrada, &saida).Executar()

	lista := pedidos.Listar()
	if len(lista) != 1 || len(lista[0].Itens) != 1 || lista[0].Itens[0].Quantidade != 3 || lista[0].ValorTotal != 30 {
		t.Fatalf("pedidos = %+v\nsaída:\n%s", lista, saida.String())
	}
}
