---
type: skill
name: Refactoring
description: Refatoração segura de código passo-a-passo para o whatsmeow
skillSlug: refactoring
phases: [E]
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Habilidade: Refatoração

## Padrões Comuns de Refatoração
- **Extract Method**: `client.go` é muito extenso. Lógicas dentro dos cases de `handleEvent` devem ser extraídas para métodos privados como `handleMessageEvent`, `handleReceiptEvent`.
- **Interface Segregation**: Se a `Store` ficar muito grande, defina interfaces menores (`SessionStore`, `DeviceStore`).

## Detecção de Code Smells
- **Funções Longas**: > 50 linhas é um sinal de alerta (exceto para grandes blocos `switch/case` de mensagens do protocolo).
- **Números Mágicos**: Constantes do protocolo (tags, versões) devem ser definidas em `binary/constants.go`.

## Procedimentos de Refatoração Segura
1. Rode `go test ./...` -> TUDO DEVE PASSAR.
2. Aplique a refatoração.
3. Rode `go test ./...` usando `-race`.
4. Rode `golangci-lint run`.

## Requisitos de Teste
- Nenhuma mudança na API pública é permitida sem bump de versão maior.
- Refatorar pacotes internos (`internals/`) requer verificação de todos os consumidores em `client.go`.
