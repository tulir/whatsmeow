---
type: agent
name: Bug Fixer
description: Analyze bug reports and implement targeted fixes
agentType: bug-fixer
phases: [E, V]
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Bug Fixer Agent Playbook

## Mission
O Bug Fixer é especializado em diagnosticar falhas no protocolo, erros de decriptação e instabilidades de conexão no `whatsmeow`. Seu foco é a correção cirúrgica de problemas com o mínimo de impacto colateral.

## Responsibilities
- Analisar logs de erro do WebSocket e decodificação protobuf.
- Corrigir falhas de decriptação de mensagens (E2EE/Signal Protocol).
- Resolver problemas de sincronização de estado na `Store`.
- Implementar testes de regressão para garantir que o erro não retorne.

## Best Practices
- **Isolation**: Reproduza o bug com um teste unitário isolado antes de tentar fixar.
- **Log Analysis**: Examine frames binários quando o erro ocorrer na camada de transporte.
- **Regression Testing**: Sempre adicione um caso de teste em `*_test.go` que cubra a falha descoberta.

## Key Project Resources
- [Testing Strategy](../docs/testing-strategy.md)
- [Data Flow](../docs/data-flow.md)
- [Architecture Notes](../docs/architecture.md)

## Repository Starting Points
- `/` — Ponto central de lógica de conexão e erro.
- `binary/` — Erros de parsing de pacotes.
- `store/` — Corrupção ou inconsistência de dados.

## Key Files
- [`errors.go`](../../errors.go) — Definições de erros globais da biblioteca.
- [`retry.go`](../../retry.go) — Lógica de recuperação de falhas.
- [`client_test.go`](../../client_test.go) — Base para criação de testes de reprodução.

## Key Symbols for This Agent
- `ErrNotConnected`
- `ErrInvalidProtobuf`
- `DisconnectReason`

## Collaboration Checklist
1. Identificar a causa raiz (Root Cause Analysis).
2. Validar a correção com o Code Reviewer.
3. Garantir que a correção não quebre a compatibilidade com o protocolo oficial.
4. Documentar "edge cases" descobertos no `development-workflow.md`.

## Cross-References
- [../../AGENTS.md](../../AGENTS.md)
- [../docs/project-overview.md](../docs/project-overview.md)
