package main

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/catalogo"
	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/messaging"
)

var itensTeste = []events.Item{
	{ProdutoID: "P01", Nome: "Fone", Quantidade: 2, PrecoUnitario: 199.90},
	{ProdutoID: "P06", Nome: "Caderno", Quantidade: 1, PrecoUnitario: 24.90},
}

func novoTeste(t *testing.T) (*Servico, *Pedidos, *messaging.FakePublisher, Pedido) {
	t.Helper()
	pedidos := NovosPedidos()
	pub := &messaging.FakePublisher{}
	s := NovoServico(pedidos, pub, func(string) {})
	pedido, err := s.CriarPedido(context.Background(), itensTeste)
	if err != nil {
		t.Fatal(err)
	}
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
	if err := s.TratarEvento(context.Background(), env); err != nil {
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
	publicados := pub.Publicados()
	if len(publicados) != 1 || publicados[0].EventType != events.PedidoCriado {
		t.Fatalf("publicados = %v", pub.Tipos())
	}
	d := publicados[0].Data.(events.PedidoCriadoDados)
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
	if !slices.Equal(pub.Tipos(), []string{events.PedidoCriado}) {
		t.Errorf("fluxo feliz não deveria publicar nada além de pedido.criado: %v", pub.Tipos())
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

	tratar(t, s, evento(t, events.EstoqueIndisponivel, events.EstoqueIndisponivelDados{
		PedidoID:       pedido.ID,
		ItensFaltantes: []events.ItemFaltante{{ProdutoID: "P01", Solicitado: 2, Disponivel: 0}},
	}))

	p, _ := pedidos.Buscar(pedido.ID)
	if p.Status != StatusExcluido || p.Motivo != events.MotivoFaltaEstoque || !strings.Contains(p.Detalhe, "P01") {
		t.Errorf("pedido depois da falta de estoque: %+v", p)
	}
	ultimo := pub.Publicados()[len(pub.Publicados())-1]
	if ultimo.EventType != events.PedidoExcluido ||
		ultimo.Data.(events.PedidoExcluidoDados).Motivo != events.MotivoFaltaEstoque {
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
	if !slices.Equal(pub.Tipos(), []string{events.PedidoCriado, events.PedidoExcluido}) {
		t.Errorf("publicados = %v", pub.Tipos())
	}
}

func TestExclusaoPeloUsuario(t *testing.T) {
	s, _, pub, pedido := novoTeste(t)
	ctx := context.Background()

	if err := s.ExcluirPeloUsuario(ctx, pedido.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.ExcluirPeloUsuario(ctx, pedido.ID); !errors.Is(err, ErrPedidoExcluido) {
		t.Errorf("segunda exclusão: %v", err)
	}
	// Evento atrasado de um pedido já excluído é ignorado sem publicar nada.
	tratar(t, s, evento(t, events.PagamentoRecusado, events.PagamentoRecusadoDados{PedidoID: pedido.ID}))

	if !slices.Equal(pub.Tipos(), []string{events.PedidoCriado, events.PedidoExcluido}) {
		t.Errorf("publicados = %v", pub.Tipos())
	}
	if d := pub.Publicados()[1].Data.(events.PedidoExcluidoDados); d.Motivo != events.MotivoUsuario {
		t.Errorf("motivo = %q", d.Motivo)
	}
}

func TestPedidoEnviadoNaoPodeSerExcluido(t *testing.T) {
	s, _, _, pedido := novoTeste(t)
	tratar(t, s, evento(t, events.PedidoEnviado, events.PedidoEnviadoDados{PedidoID: pedido.ID}))

	if err := s.ExcluirPeloUsuario(context.Background(), pedido.ID); !errors.Is(err, ErrPedidoEnviado) {
		t.Errorf("esperava ErrPedidoEnviado, veio %v", err)
	}
}

func TestEventoDePedidoDesconhecidoEIgnorado(t *testing.T) {
	s, _, _, _ := novoTeste(t)
	tratar(t, s, evento(t, events.PedidoEstoqueOK, events.PedidoEstoqueOKDados{PedidoID: "nao-existe"}))
}

func TestMenuFazPedido(t *testing.T) {
	pedidos := NovosPedidos()
	pub := &messaging.FakePublisher{}
	s := NovoServico(pedidos, pub, func(string) {})
	produtos := []catalogo.Produto{{ID: "P01", Nome: "Fone", Categoria: "A", Preco: 10}}
	entrada := strings.NewReader("2\np01\n2\nP01\n1\n\n0\n") // P01 duas vezes: soma no mesmo item
	var saida strings.Builder

	NovoMenu(s, pedidos, produtos, entrada, &saida).Executar(context.Background())

	lista := pedidos.Listar()
	if len(lista) != 1 || len(lista[0].Itens) != 1 || lista[0].Itens[0].Quantidade != 3 || lista[0].ValorTotal != 30 {
		t.Fatalf("pedidos = %+v\nsaída:\n%s", lista, saida.String())
	}
}
