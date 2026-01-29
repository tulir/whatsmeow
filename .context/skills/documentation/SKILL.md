---
type: skill
name: Documentation
description: Gerar e atualizar documentação técnica para o whatsmeow
skillSlug: documentation
phases: [P, C]
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Habilidade: Documentação

## Padrões de Documentação
- **Idioma**: Português do Brasil (Docs internas/.context) e Inglês (GoDoc/Comentários de Código). Verifique sempre o `USER_RULES`.
- **Formato**: Github-flavored Markdown.

## Convenções GoDoc
- Todo tipo e função exportada DEVE ter um comentário começando com o nome do tipo (em inglês, padrão Go).
  ```go
  // Connect establishes the websocket connection...
  func (c *Client) Connect() error
  ```

## Estrutura do README
- O `README.md` raiz deve focar no uso: "Como conectar", "Como consultar".
- O `.context/docs/README.md` é o "Manual do Desenvolvedor Interno".

## Documentação da API
- Use blocos de código com comentários para explicar campos complexos em `structs`.
- Documente quaisquer cenários de `panic` nos cabeçalhos das funções.
