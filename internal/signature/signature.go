package signature

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"os"
)

func GenerateKey() *rsa.PrivateKey {
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
}

func VerifySignature(public_key *rsa.PublicKey, content string, signature []byte) bool {
	raw_sig, err := base64.StdEncoding.DecodeString(string(signature))
	if err != nil {
		return false
	}
	h := BuildPayloadHash([]byte(content))
	err = rsa.VerifyPKCS1v15(public_key, crypto.SHA256, h[:], raw_sig)
	return err == nil
}

func PersistKeys(private_key *rsa.PrivateKey, servicoNome string) {
	pub_key := &private_key.PublicKey

	dir_path := fmt.Sprintf("cmd/%s/keys", servicoNome)
	os.MkdirAll(dir_path, 0755)

	priv_file, err := os.Create(dir_path + "/private_key.pem")
	if err != nil {
		log.Panicf("Erro ao criar arquivo de chave privada: %s", err)
	}
	pem.Encode(priv_file, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(private_key),
	})
	priv_file.Close()

	pub_file, err := os.Create(dir_path + "/public_key.pem")
	if err != nil {
		log.Panicf("Erro ao criar arquivo de chave pública: %s", err)
	}
	pem.Encode(pub_file, &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(pub_key),
	})
	pub_file.Close()

	microservicos := []string{"entrega", "estoque", "pagamento", "principal", "promocoes"}
	for _, ms := range microservicos {
		if ms != servicoNome {
			pub_dir := fmt.Sprintf("cmd/%s/%s-pub", ms, servicoNome)
			os.MkdirAll(pub_dir, 0755)

			pub_dest_file, err := os.Create(pub_dir + "/public_key.pem")
			if err != nil {
				log.Panicf("Erro ao criar arquivo de chave pública no serviço %s: %s", ms, err)
			}
			pem.Encode(pub_dest_file, &pem.Block{
				Type:  "RSA PUBLIC KEY",
				Bytes: x509.MarshalPKCS1PublicKey(pub_key),
			})
			pub_dest_file.Close()
			log.Printf("Chave pública distribuída para %s", ms)
		}
	}
}

func LoadPublicKey(path string) *rsa.PublicKey {
	pem_data, _ := os.ReadFile(path)
	pem_block, _ := pem.Decode(pem_data)
	if pem_block == nil || pem_block.Type != "RSA PUBLIC KEY" {
		log.Panicf("Erro ao decodificar chave pública do arquivo: %s", path)
	}
	public_key, err := x509.ParsePKCS1PublicKey(pem_block.Bytes)
	if err != nil {
		log.Panicf("Erro ao parsear a chave pública do arquivo: %s", err)
	}
	return public_key
}

func LoadPrivateKey(path string) *rsa.PrivateKey {
	pem_data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	pem_block, _ := pem.Decode(pem_data)
	if pem_block == nil || pem_block.Type != "RSA PRIVATE KEY" {
		return nil
	}
	private_key, err := x509.ParsePKCS1PrivateKey(pem_block.Bytes)
	if err != nil {
		return nil
	}
	return private_key
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
