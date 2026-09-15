package signature

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"
)

func GenerateKeys() *rsa.PrivateKey {
	private_key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Panicf("Erro ao gerar chave privada: %s", err)
	}
	return private_key
}

func BuildPayloadHash(content []byte) [32]byte {
	h := sha256.Sum256(content)
	return h
}

func SignPayload(private_key *rsa.PrivateKey, content string) []byte {
	h := BuildPayloadHash([]byte(content))
	signature, err := rsa.SignPKCS1v15(rand.Reader, private_key, crypto.SHA256, h[:])
	if err != nil {
		log.Panicf("Erro ao assinar o payload: %s", err)
	}
	return signature
	//log.Printf("Assinatura do payload: %x", signature)
}

func VerifySignature(public_key *rsa.PublicKey, content string, signature []byte) bool {
	h := BuildPayloadHash([]byte(content))
	err := rsa.VerifyPKCS1v15(public_key, crypto.SHA256, h[:], signature)
	if err != nil {
		return false
	}
	return true
}

func PersistKeys(private_key *rsa.PrivateKey, servicoNome string) {
	pub_key := &private_key.PublicKey

	os.RemoveAll("key")
	os.MkdirAll("key", 0755)

	priv_file, err := os.Create("key/private_key.pem")
	if err != nil {
		log.Panicf("Erro ao criar arquivo de chave privada: %s", err)
	}
	defer priv_file.Close()

	pem.Encode(priv_file, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(private_key),
	})

	pub_file, err := os.Create("public_key.pem")
	if err != nil {
		log.Panicf("Erro ao criar arquivo de chave pública: %s", err)
	}
	defer pub_file.Close()

	pem.Encode(pub_file, &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(pub_key),
	})

	microservicos := []string{"entrega", "estoque", "pagamento", "principal", "promocoes"}
	for _, ms := range microservicos {
		dirPath := fmt.Sprintf("../../cmd/%s/%s-pub", ms, servicoNome)
		os.RemoveAll(dirPath)
		err := os.MkdirAll(dirPath, 0755)
		if err != nil {
			log.Panicf("Erro ao criar diretório para chave pública do serviço %s: %s", ms, err)
		}

		pub_dest := fmt.Sprintf("%s/public_key.pem", dirPath)
		pub_dest_file, err := os.Create(pub_dest)
		if err != nil {
			log.Panicf("Erro ao criar arquivo de chave pública no serviço %s: %s", ms, err)
		}
		defer pub_dest_file.Close()

		pem.Encode(pub_dest_file, &pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: x509.MarshalPKCS1PublicKey(pub_key),
		})
		log.Printf("Chave pública distribuída para %s", ms)
	}
}

func ReadPubKeyFromFile(path string) *rsa.PublicKey {
	data, _ := os.ReadFile(path)
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "RSA PUBLIC KEY" {
		log.Panicf("Erro ao decodificar chave pública do arquivo: %s", path)
	}
	pub, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		log.Panicf("Erro ao parsear a chave pública do arquivo: %s", err)
	}
	return pub
}

func PrintfPrivateKey(private_key *rsa.PrivateKey) {
	bytes := x509.MarshalPKCS1PrivateKey(private_key)
	pem := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: bytes,
	})
	log.Printf("%s", string(pem))
}

func PrintfPublicKey(public_key *rsa.PublicKey) {
	bytes := x509.MarshalPKCS1PublicKey(public_key)
	pem := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: bytes,
	})
	log.Printf("%s", string(pem))
}
