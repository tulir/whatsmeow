---
type: skill
name: Commit Message
description: Gerar mensagens de commit seguindo conventional commits para o whatsmeow
skillSlug: commit-message
phases: [E, C]
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Habilidade: Mensagem de Commit

## Convenções de Formato
- **Conventional Commits**: `tipo(escopo): descrição curta`
- **Tipos Permitidos**:
  - `feat`: Nova funcionalidade (ex: suporte a mensagens de enquete).
  - `fix`: Correção de bug.
  - `chore`: Atualização de deps, ferramentas ou CI.
  - `docs`: Mudanças apenas em documentação.
  - `refactor`: Mudança de código que não corrige bug nem adiciona feature.

## Exemplos do Repositório
- `feat(client): adicionar suporte para reação a mensagens`
- `fix(store): corrigir deadlock no sqlite ao salvar sessão`
- `docs(readme): atualizar exemplo de conexão`

## Nomenclatura de Branches
- `feature/nome-da-feature`
- `bugfix/issue-id-descricao`
- `hotfix/correcao-critica`

## Versionamento Semântico
- Commits com `feat` podem disparar `MINOR` version bump (se não houver breaking change).
- Commits com `BREAKING CHANGE:` no rodapé disparam `MAJOR` version.
- Commits `fix` disparam `PATCH`.
