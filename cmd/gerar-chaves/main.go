package main

import (
	"log"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/signature"
)

func main() {
	servicos := []string{"entrega", "estoque", "pagamento", "principal", "promocoes"}
	for _, servico := range servicos {
		privada := signature.GenerateKey()
		signature.PersistKeys(privada, servico)
	}
	log.Printf("Chaves geradas para %d serviços e públicas distribuídas.", len(servicos))
}
