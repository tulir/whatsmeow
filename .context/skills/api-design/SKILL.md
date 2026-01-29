---
type: skill
name: Api Design
description: Design de APIs da biblioteca Go seguindo melhores práticas para o whatsmeow
skillSlug: api-design
phases: [P, R]
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Habilidade: Design de API

## Padrões de Design
- **Biblioteca vs Serviço**: Como o `whatsmeow` é uma biblioteca, a API é um conjunto de structs/interfaces Go, não endpoints REST.
- **Orientado a Eventos**: A principal maneira de receber dados é via `AddEventHandler`. Garanta que eventos sejam estritamente tipados (`*events.Message`, NÃO `interface{}`).

## Convenções de Nomenclatura
- Métodos no `Client` devem ser verbos: `Connect`, `Disconnect`, `SendMessage`.
- Structs de configuração devem ser nomeadas `Config` ou `Options`.

## Formato de Requisição/Resposta
- Use `context.Context` como o primeiro argumento para todos os métodos bloqueantes (`SendMessage`, `Upload`).
- Retorne o padrão `(result, error)`.

## Política de Versionamento
- Respeite o SemVer.
- Mudanças que quebram compatibilidade (Breaking changes) na struct `Client` exigem `v2`.
- Prefira adicionar novos métodos em vez de alterar assinaturas dos existentes.
