# Protocolo para agentes e assistentes de IA

Estas regras se aplicam a todo o repositório.

## Autoria de commits

- Nenhuma LLM, agente ou assistente de IA deve se autoatribuir autoria ou coautoria em commits.
- Não adicionar trailers como `Co-authored-by`, `Signed-off-by` ou equivalentes em nome de Codex, ChatGPT, OpenAI, Claude, Anthropic, Antigravity ou qualquer outra ferramenta de IA.
- Não inserir mensagens promocionais, marcas d'água ou frases como "gerado por IA" em mensagens de commit.
- As mensagens de commit devem descrever somente a alteração realizada, de forma objetiva.
- O nome e o e-mail do autor do commit devem permanecer os configurados pelo responsável humano no Git.

## Princípio de trabalho

- A pessoa responsável pelo trabalho deve conseguir abrir, executar, explicar e defender todo o código.
- Alterações devem privilegiar clareza, organização simples e documentação didática.
- Não publicar (`push`) nem criar commits sem solicitação explícita do responsável humano.

## Código do Kauan: não alterar

A dupla dividiu o trabalho. A parte do Kauan é **somente leitura** para
qualquer LLM, agente ou assistente de IA usado pelo Rômulo:

| Caminho | Conteúdo |
|---|---|
| `internal/signature/` | geração e leitura de chaves, assinatura e verificação |
| `internal/utils/` | `FailOnError` |
| `cmd/estoque/` | microsserviço Estoque |
| `cmd/pagamento/` | microsserviço Pagamento |
| `cmd/gerar-chaves/` | geração e distribuição das chaves de todos os serviços |
| `data/estoque.json` | estoque inicial |
| `EstoqueIndisponivelDados` em `internal/events/dados.go` | formato do evento definido por ele |

- Não editar, renomear, apagar, reformatar nem "corrigir" nada nesses caminhos,
  nem por `sed`, script ou comando de shell.
- O resto do projeto se adapta ao código dele: mesmas funções de
  `internal/signature`, mesmo formato de assinatura (só o campo `data`, em
  base64), mesmos caminhos de chave e o mesmo estilo de chamadas ao RabbitMQ.
- Se algo nesses arquivos parecer errado ou impedir uma tarefa, parar e avisar
  o Rômulo, para ele combinar com o Kauan. A mudança, se houver, é feita por ele.
- Só o próprio Kauan pode pedir alteração nesses arquivos.

Na máquina do Rômulo, um hook local do Claude Code
(`.claude/hooks/protege-codigo-kauan.sh`, fora do Git) recusa edições nesses
caminhos.


## Garantia automática (hook)

A regra acima não depende de boa vontade da ferramenta: o hook versionado
`.githooks/commit-msg` remove qualquer trailer de coautoria de IA, assinatura
`Signed-off-by`/`Assisted-by` de ferramenta, frase "Generated with ..." ou
linha com 🤖 antes de o commit ser gravado.

Cada pessoa da dupla precisa ativá-lo uma vez, após clonar:

```
git config core.hooksPath .githooks
```

Nunca usar `--no-verify`: isso ignora o hook.
