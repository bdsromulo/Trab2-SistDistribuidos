package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/catalogo"
	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
)

// Menu é a interface de terminal do Principal.
type Menu struct {
	servico  *Servico
	pedidos  *Pedidos
	produtos []catalogo.Produto
	entrada  *bufio.Scanner
	saida    io.Writer
}

func NovoMenu(servico *Servico, pedidos *Pedidos, produtos []catalogo.Produto, entrada io.Reader, saida io.Writer) *Menu {
	return &Menu{servico: servico, pedidos: pedidos, produtos: produtos, entrada: bufio.NewScanner(entrada), saida: saida}
}

// Executar mostra o menu até o usuário escolher sair (ou a entrada acabar).
func (m *Menu) Executar() {
	for {
		fmt.Fprintln(m.saida, "\n===== E-commerce =====")
		fmt.Fprintln(m.saida, "1) Ver produtos")
		fmt.Fprintln(m.saida, "2) Fazer pedido")
		fmt.Fprintln(m.saida, "3) Consultar pedidos")
		fmt.Fprintln(m.saida, "4) Excluir pedido")
		fmt.Fprintln(m.saida, "0) Sair")

		opcao, ok := m.perguntar("Opção: ")
		if !ok {
			return
		}
		switch opcao {
		case "1":
			m.mostrarProdutos()
		case "2":
			m.fazerPedido()
		case "3":
			m.mostrarPedidos()
		case "4":
			m.excluirPedido()
		case "0":
			return
		default:
			fmt.Fprintln(m.saida, "Opção inválida.")
		}
	}
}

// perguntar mostra o texto e lê uma linha. ok é false quando a entrada acabou.
func (m *Menu) perguntar(texto string) (resposta string, ok bool) {
	fmt.Fprint(m.saida, texto)
	if !m.entrada.Scan() {
		return "", false
	}
	return strings.TrimSpace(m.entrada.Text()), true
}

func (m *Menu) mostrarProdutos() {
	fmt.Fprintln(m.saida, "\nCódigo  Cat.  Preço       Produto")
	for _, p := range m.produtos {
		fmt.Fprintf(m.saida, "%-6s  %-4s  R$ %8.2f  %s\n", p.ID, p.Categoria, p.Preco, p.Nome)
	}
}

func (m *Menu) buscarProduto(codigo string) (catalogo.Produto, bool) {
	for _, p := range m.produtos {
		if strings.EqualFold(p.ID, codigo) {
			return p, true
		}
	}
	return catalogo.Produto{}, false
}

func (m *Menu) fazerPedido() {
	m.mostrarProdutos()
	var itens []events.Item
	for {
		codigo, ok := m.perguntar("\nCódigo do produto (Enter para terminar): ")
		if !ok || codigo == "" {
			break
		}
		produto, achou := m.buscarProduto(codigo)
		if !achou {
			fmt.Fprintln(m.saida, "Produto não encontrado.")
			continue
		}
		texto, ok := m.perguntar("Quantidade: ")
		if !ok {
			break
		}
		qtd, err := strconv.Atoi(texto)
		if err != nil || qtd <= 0 {
			fmt.Fprintln(m.saida, "Quantidade inválida.")
			continue
		}
		itens = adicionarItem(itens, produto, qtd)
		fmt.Fprintf(m.saida, "Adicionado: %d x %s\n", qtd, produto.Nome)
	}

	if len(itens) == 0 {
		fmt.Fprintln(m.saida, "Pedido vazio, nada foi feito.")
		return
	}
	pedido, err := m.servico.CriarPedido(itens)
	if err != nil {
		fmt.Fprintf(m.saida, "Erro ao criar o pedido: %v\n", err)
		return
	}
	fmt.Fprintf(m.saida, "Pedido %s criado, total R$ %.2f. Status: %s\n", pedido.ID, pedido.ValorTotal, pedido.Status)
}

// adicionarItem soma a quantidade se o produto já estiver no pedido.
func adicionarItem(itens []events.Item, produto catalogo.Produto, qtd int) []events.Item {
	for i := range itens {
		if itens[i].ProdutoID == produto.ID {
			itens[i].Quantidade += qtd
			return itens
		}
	}
	return append(itens, events.Item{
		ProdutoID:     produto.ID,
		Nome:          produto.Nome,
		Quantidade:    qtd,
		PrecoUnitario: produto.Preco,
	})
}

func (m *Menu) mostrarPedidos() {
	lista := m.pedidos.Listar()
	if len(lista) == 0 {
		fmt.Fprintln(m.saida, "Nenhum pedido.")
		return
	}
	for _, p := range lista {
		fmt.Fprintf(m.saida, "\nPedido %s  (%s)  total R$ %.2f\n", p.ID, p.CriadoEm.Format("15:04:05"), p.ValorTotal)
		fmt.Fprintf(m.saida, "  Status: %s\n", descreverStatus(p))
		for _, it := range p.Itens {
			fmt.Fprintf(m.saida, "  - %d x %s (R$ %.2f)\n", it.Quantidade, it.Nome, it.PrecoUnitario)
		}
	}
}

func (m *Menu) excluirPedido() {
	id, ok := m.perguntar("Código do pedido: ")
	if !ok || id == "" {
		return
	}
	if err := m.servico.ExcluirPeloUsuario(id); err != nil {
		fmt.Fprintf(m.saida, "Não foi possível excluir: %v\n", err)
		return
	}
	fmt.Fprintf(m.saida, "Pedido %s excluído.\n", id)
}
