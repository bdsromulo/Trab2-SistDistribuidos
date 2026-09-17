package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/messaging"
)

// Servico junta os pedidos e a publicação de eventos. É usado pelo menu
// (ações do usuário) e pelo consumo da fila.principal (eventos recebidos).
type Servico struct {
	pedidos *Pedidos
	pub     messaging.Publisher
	avisar  func(msg string) // mostra no terminal as mudanças vindas dos eventos
}

func NovoServico(pedidos *Pedidos, pub messaging.Publisher, avisar func(string)) *Servico {
	return &Servico{pedidos: pedidos, pub: pub, avisar: avisar}
}

// CriarPedido registra o pedido e publica pedido.criado.
func (s *Servico) CriarPedido(ctx context.Context, itens []events.Item) (Pedido, error) {
	pedido := s.pedidos.Criar(itens)
	err := s.pub.Publish(ctx, events.PedidoCriado, events.PedidoCriadoDados{
		PedidoID:   pedido.ID,
		Itens:      pedido.Itens,
		ValorTotal: pedido.ValorTotal,
	})
	if err != nil {
		s.pedidos.Remover(pedido.ID) // ninguém soube do pedido, então ele não existe
		return Pedido{}, err
	}
	return pedido, nil
}

// ExcluirPeloUsuario exclui o pedido a pedido do usuário e publica pedido.excluido.
func (s *Servico) ExcluirPeloUsuario(ctx context.Context, id string) error {
	return s.excluir(ctx, id, events.MotivoUsuario, "")
}

// excluir marca o pedido e, se foi esta chamada que o excluiu, publica
// pedido.excluido para o Estoque devolver a reserva.
func (s *Servico) excluir(ctx context.Context, id, motivo, detalhe string) error {
	if _, err := s.pedidos.Excluir(id, motivo, detalhe); err != nil {
		return err
	}
	return s.pub.Publish(ctx, events.PedidoExcluido, events.PedidoExcluidoDados{PedidoID: id, Motivo: motivo})
}

// TratarEvento é o handler da fila.principal: atualiza o status do pedido.
func (s *Servico) TratarEvento(ctx context.Context, env events.Envelope) error {
	var (
		pedidoID string
		err      error
	)
	switch env.EventType {
	case events.PedidoEstoqueOK:
		var d events.PedidoEstoqueOKDados
		if err := env.Decodificar(&d); err != nil {
			return err
		}
		pedidoID = d.PedidoID
		_, err = s.pedidos.Avancar(d.PedidoID, StatusAguardandoPagto, "estoque reservado")

	case events.EstoqueIndisponivel:
		var d events.EstoqueIndisponivelDados
		if err := env.Decodificar(&d); err != nil {
			return err
		}
		pedidoID = d.PedidoID
		err = s.excluir(ctx, d.PedidoID, events.MotivoFaltaEstoque, "")

	case events.PagamentoAprovado:
		var d events.PagamentoAprovadoDados
		if err := env.Decodificar(&d); err != nil {
			return err
		}
		pedidoID = d.PedidoID
		_, err = s.pedidos.Avancar(d.PedidoID, StatusPago, "")

	case events.PagamentoRecusado:
		var d events.PagamentoRecusadoDados
		if err := env.Decodificar(&d); err != nil {
			return err
		}
		pedidoID = d.PedidoID
		err = s.excluir(ctx, d.PedidoID, events.MotivoPagamentoRecusado, d.Motivo)

	case events.PedidoEnviado:
		var d events.PedidoEnviadoDados
		if err := env.Decodificar(&d); err != nil {
			return err
		}
		pedidoID = d.PedidoID
		_, err = s.pedidos.Avancar(d.PedidoID, StatusEnviado,
			fmt.Sprintf("NF %s, rastreio %s", d.NumeroNotaFiscal, d.CodigoRastreio))

	default:
		return fmt.Errorf("evento inesperado na fila.principal: %s", env.EventType)
	}

	// Pedido desconhecido, já excluído ou evento repetido: nada a fazer,
	// mas a mensagem foi entendida, então não é erro (recebe ack).
	if errors.Is(err, ErrPedidoNaoEncontrado) || errors.Is(err, ErrPedidoExcluido) ||
		errors.Is(err, ErrStatusAntigo) || errors.Is(err, ErrPedidoEnviado) {
		s.avisar(fmt.Sprintf("%s do pedido %s ignorado: %v", env.EventType, pedidoID, err))
		return nil
	}
	if err != nil {
		return err
	}

	pedido, _ := s.pedidos.Buscar(pedidoID)
	s.avisar(fmt.Sprintf("pedido %s: %s", pedidoID, descreverStatus(pedido)))
	return nil
}

// descreverStatus junta status, motivo e detalhe numa linha.
func descreverStatus(p Pedido) string {
	texto := string(p.Status)
	if p.Motivo != "" {
		texto += " (" + p.Motivo + ")"
	}
	if p.Detalhe != "" {
		texto += " - " + p.Detalhe
	}
	return texto
}
