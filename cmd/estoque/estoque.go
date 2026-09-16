package main

import (
	//"log"
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
	content, err := os.ReadFile("data/estoque.json")
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