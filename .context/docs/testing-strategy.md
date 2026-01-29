---
type: doc
name: testing-strategy
description: Test frameworks, patterns, coverage requirements, and quality gates
category: testing
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Testing Strategy

A qualidade e a confiabilidade da biblioteca `whatsmeow` são asseguradas através de uma estratégia de testes baseada nas ferramentas padrão do ecossistema Go, com foco em estabilidade do protocolo e integridade dos dados.

## Test Types
- **Unit Tests**: Testes de funções puras, codificação/decodificação (`binary/`) e lógica de negócio. Arquivos nomeados como `*_test.go`.
- **Integration Tests**: Validação da interação entre o `Client` e a `Store`. Exemplos em [`store/store_test.go`](../store/store_test.go).
- **Network/E2E Tests**: Simulações de conexão WebSocket e handshakes. Devido à natureza do WhatsApp, muitos destes testes utilizam mocks de rede para garantir determinismo.

## Running Tests
- **Rodar todos os testes**:
  ```bash
  go test ./...
  ```
- **Rodar com cobertura**:
  ```bash
  go test -coverprofile=coverage.out ./...
  go tool cover -html=coverage.out
  ```
- **Rodar um teste específico**:
  ```bash
  go test -v -run TestNomeDoTeste ./pacote
  ```

## Quality Gates
- **Formatação**: O código deve passar por `go fmt` ou `gofumpt`.
- **Linting**: Sem avisos críticos do `golangci-lint`.
- **Race Detection**: Testes devem passar com o detector de concorrência habilitado em mudanças críticas:
  ```bash
  go test -race ./...
  ```

## Troubleshooting
- **CGO_ENABLED**: Se estiver testando a store SQLite nativa (`go-sqlite3`), certifique-se de que o CGO está habilitado e o compilador C está presente.
- **Timeouts**: Testes de rede podem falhar em ambientes com latência alta; use `-timeout 30s` se necessário.

## Cross-References
- [Development Workflow](./development-workflow.md)
