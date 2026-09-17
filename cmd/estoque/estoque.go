package main

import (
	"sort"
	"log"
	"os"
	"encoding/json"

	u "github.com/bdsromulo/Trab2-SistDistribuidos/internal/utils"
)

type ProdutoEstoque struct {
	ID            string `json:"id"`
	Availability  int `json:"availability"`
}

type Estoque struct {
	Produtos map[string]*ProdutoEstoque
}

func loadEstoque() Estoque {
	content, err := os.ReadFile("../../data/estoque.json")
	u.FailOnError(err, "Erro ao carregar o estoque")

	var l []ProdutoEstoque
	err = json.Unmarshal(content, &l)
	u.FailOnError(err, "Erro ao decodificar o estoque")

	e := Estoque{Produtos: make(map[string]*ProdutoEstoque, len(l))}
	for i, p := range l {
		e.Produtos[p.ID] = &l[i]
	}

	return e
}

func imprimirEstoque(est Estoque) {
	ids := make([]string, 0, len(est.Produtos))
	for id := range est.Produtos {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	log.Println("=== Estoque Atual ===")
	for _, id := range ids {
		log.Printf("  Produto %s: %d unidades", id, est.Produtos[id].Availability)
	}
}

/*func main() {
	produtos, _ := c.Carregar(c.CaminhoPadrao)
	for _, prod := range produtos {
		log.Printf("%s", prod.ID)
	}
	e := loadEstoque()
	for _, item := range e.Produtos {
		log.Printf("%s: %d", item.ID, e.Produtos[item.ID].Availability)
	}
}*/