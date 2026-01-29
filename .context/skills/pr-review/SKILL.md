---
type: skill
name: Pr Review
description: Revisar pull requests contra padrões e melhores práticas do whatsmeow
skillSlug: pr-review
phases: [R, V]
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Habilidade: Revisão de PR

## Checklist de Revisão
- [ ] **Contexto**: O PR tem uma descrição clara do "Porquê"?
- [ ] **Testes**: Novos testes foram adicionados em `_test.go`? Os testes existentes passam?
- [ ] **Lint**: O código passou pelo `golangci-lint` sem erros?
- [ ] **Gofumpt**: O código está formatado com `gofumpt` (mais estrito que gofmt)?

## Padrões de Qualidade
- **Complexidade Ciclomática**: Funções não devem ter muitos aninhamentos (`if/else/for`).
- **Tratamento de Erros**: Erros nunca devem ser ignorados (`_ = func()`). Sempre verifique ou logue.
- **Comentários**: Código público deve ter GoDoc. Comentários explicativos devem ser usados em blocos de lógica complexa (crypto/binary).

## Requisitos de Teste Pré-Merge
- CI GitHub Actions deve estar verde.
- Se tocar em `store/`, testar manualmente com SQLite e Postgres (se possível).

## Expectativas de Documentação
- Se o PR adiciona um novo recurso ao `Client`, o `README.md` ou `docs/` deve ser atualizado.
- Mudanças na `Store` exigem nota sobre migração de dados.
