// Gera os pares de chaves RSA de cada produtor.
package main

import (
	"log"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/signature"
)

func main() {
	FILE_NAME := "public_key.pem"
	private_key := signature.GenerateKeys()
	signature.PersistKeys(private_key)
	//log.Printf("Chave privada: %v e chave pública: %v", private_key, public_key)
	payload := "PAYLOAD DE TESTE"
	//hash := signature.BuildPayloadHash([]byte(payload))
	//log.Printf("Hash do payload: %x", hash)
	s := signature.SignPayload(private_key, payload)
	log.Printf("Assinatura: %x", s)
	//s = []byte("1")
	public_key := signature.ReadPubKeyFromFile(FILE_NAME)
	b := signature.VerifySignature(public_key, payload, s)
	if b == false {
		log.Panicf("\nAssinatura inválida!")
	}
	log.Printf("Assinatura válida!")
	signature.PrintfPrivateKey(private_key)
	signature.PrintfPublicKey(public_key)
}
