---
type: skill
name: Test Generation
description: Gerar casos de teste abrangentes para o whatsmeow
skillSlug: test-generation
phases: [E, V]
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Habilidade: Geração de Testes

## Framework e Convenções
- Use o pacote padrão `testing`.
- Use `github.com/stretchr/testify/assert` para asserções (se já estiver no `go.mod`).
- Nomenclatura: `Test[Funcao]_[Condicao]`.

## Organização de Arquivos de Teste
- Coloque testes unitários no mesmo pacote: `package x_test` para testes caixa-preta, `package x` para caixa-branca.
- Testes de integração envolvendo `store` devem estar em `store_test.go`.

## Estratégias de Mocking
- Use interfaces para tudo que envolva I/O (`Store`, `Logger`, `NetConn`).
- Mocke a conexão WebSocket usando `net.Pipe` (conexão em memória) para simular frames do servidor sem rede real.

## Requisitos de Cobertura
- **Caminhos Críticos**: `binary/` (encoding/decoding) deve ter 90%+ de cobertura.
- **Criptografia**: `handshake.go` deve ter cobertura para caminhos de sucesso e falha.
- **Store**: Todos os métodos da interface devem ser exercitados por `store_test.go`.
