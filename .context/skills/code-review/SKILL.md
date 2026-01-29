---
type: skill
name: Code Review
description: Revisão de qualidade de código e melhores práticas para o whatsmeow
skillSlug: code-review
phases: [R, V]
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Habilidade: Revisão de Código

## Diretrizes de Revisão
- **Go Idiomático**: O código deve seguir o `Effective Go`.
    - Evite nomes repetitivos (ex: `client.ClientConfig` -> `client.Config`).
    - Strings de erro não devem ser capitalizadas (ex: `errors.New("falha na conexão")`).
- **Concorrência**: Verifique o uso correto de `sync.Mutex` e `sync.RWMutex`.
    - Travas (locks) nunca devem ser seguradas durante chamadas de rede bloqueantes.
- **Performance**: Fique atento a alocações desnecessárias em caminhos críticos como `binary.Unmarshal`.

## Considerações de Segurança
- **Vazamento de PII**: Verifique se números de telefone ou conteúdo de mensagens não são logados no nível `INFO`.
- **Correção Criptográfica**: Qualquer alteração em `util/crypto.go` ou `handshake.go` exige uma segunda revisão de especialista (Security Auditor).

## Padrões Comuns para Verificar
- **Interface Store**: Alterações em `store/` devem suportar todos os drivers (Memória, SQLite, Postgres).
- **Geração de Protobuf**: Se arquivos `.proto` forem alterados, os arquivos `*.pb.go` devem ser regenerados e commitados.

## Estilo e Convenção
- Execute `gofumpt` em todos os arquivos modificados.
- Garanta que todas as funções exportadas tenham comentários GoDoc em inglês (padrão da comunidade Go), mas a documentação interna/PRs em PT-BR.
