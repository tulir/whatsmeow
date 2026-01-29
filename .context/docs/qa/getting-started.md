---
slug: getting-started
category: getting-started
generatedAt: 2026-01-29
---

# Como eu configuro e rodo este projeto?

O `whatsmeow` é uma biblioteca em Go, então o fluxo é um pouco diferente de projetos Node.js ou Python.

## Pré-requisitos
- **Go 1.21+**: A biblioteca requer suporte a features modernas do Go.
- **GCC**: Necessário para compilar o SQLite (cgo) usado na `Store` padrão.

## Instalação

```bash
# Clone o repositório
git clone https://github.com/tulir/whatsmeow.git
cd whatsmeow

# Baixe as dependências
go mod tidy
```

## Como Rodar um Exemplo
A biblioteca não é um executável por si só, mas você pode rodar exemplos ou testes para verificar.

```bash
# Rodar todos os testes
go test ./...

# Se houver uma pasta de exemplos (comumente em mdtest ou exemplos externos)
go run .
```

> **Nota**: Para uso em produção, você importará esta biblioteca em seu projeto com `import "go.mau.fi/whatsmeow"`.