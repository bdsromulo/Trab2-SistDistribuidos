package messaging

import (
	"net"
	"net/url"
	"os"

	"github.com/joho/godotenv"
)

// Config guarda os dados de conexão com o RabbitMQ.
type Config struct {
	Host    string
	Porta   string
	Usuario string
	Senha   string
}

// CarregarConfig lê a conexão das variáveis de ambiente. Se existir um
// arquivo .env na pasta atual, ele é carregado antes; variáveis já definidas
// no terminal têm prioridade. O que faltar usa os valores do .env.example.
func CarregarConfig() Config {
	_ = godotenv.Load() // sem .env, seguem as variáveis do terminal e os padrões

	return Config{
		Host:    valorOuPadrao("RABBITMQ_HOST", "localhost"),
		Porta:   valorOuPadrao("RABBITMQ_PORT", "5672"),
		Usuario: valorOuPadrao("RABBITMQ_USER", "ecommerce"),
		Senha:   valorOuPadrao("RABBITMQ_PASSWORD", "ecommerce"),
	}
}

// URL monta o endereço AMQP, por exemplo amqp://ecommerce:ecommerce@localhost:5672/.
func (c Config) URL() string {
	u := url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(c.Usuario, c.Senha),
		Host:   net.JoinHostPort(c.Host, c.Porta),
		Path:   "/",
	}
	return u.String()
}

func valorOuPadrao(nome, padrao string) string {
	if v := os.Getenv(nome); v != "" {
		return v
	}
	return padrao
}
