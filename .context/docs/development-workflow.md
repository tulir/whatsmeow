---
type: doc
name: development-workflow
description: Day-to-day engineering processes, branching, and contribution guidelines
category: workflow
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Development Workflow

Este documento descreve o fluxo de trabalho diário para desenvolvedores que contribuem para o `whatsmeow`, garantindo consistência e qualidade no código.

## Branching & Releases
- **Branch Principal**: `master` deve sempre estar em estado estável.
- **Contribuições**: Devem ser feitas através de Pull Requests vindos de branches temáticas (ex: `feat/add-poll-support`, `fix/decryption-error`).
- **Versilnamento**: Seguimos o Semantic Versioning (SemVer) através de tags Git (ex: `v0.1.0`).

## Local Development
- **Instalar Dependências**:
  ```bash
  go mod tidy
  ```
- **Rodar Testes**:
  ```bash
  go test ./...
  ```
- **Gerar Código (Protobuf)**:
  ```bash
  go generate ./...
  ```

## Code Review Expectations
- **Checklist de Revisão**:
  - O código segue as convenções idiomáticas de Go (`Effective Go`).
  - Novos recursos vêm acompanhados de testes unitários ou de integração.
  - Se houve mudança no protocolo, os arquivos `.proto` foram atualizados e o código gerado foi incluído no commit.
  - Playbooks de agentes IA devem ser consultados em `AGENTS.md` para colaboração automatizada.

## Onboarding Tasks
Para novos desenvolvedores, recomendamos:
1. Ler o [Project Overview](./project-overview.md).
2. Configurar o ambiente conforme o [Tooling Guide](./tooling.md).
3. Tentar rodar o exemplo básico de conexão para entender o fluxo de pareamento via QR Code.

## Cross-References
- [Testing Strategy](./testing-strategy.md)
- [Tooling Guide](./tooling.md)
