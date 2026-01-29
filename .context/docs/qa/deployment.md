---
slug: deployment
category: operations
generatedAt: 2026-01-29
---

# Como eu faço deploy deste projeto?

Como o `whatsmeow` é uma biblioteca, seu "deploy" geralmente significa publicá-la como um módulo Go ou fazer deploy da aplicação que a consome.

## Publicando a Biblioteca
1. Garanta que `go.mod` está atualizado.
2. Crie uma tag Git seguindo SemVer (ex: `v0.1.0`).
3. O proxy oficial do Go detectará a nova tag automaticamente.

## Fazendo Deploy de Apps com whatsmeow
Se você está fazendo deploy de um bot:

### Docker
É a forma recomendada devido à dependência do SQLite (CGO).
```dockerfile
FROM golang:1.21-alpine
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY . .
RUN go build -o bot
CMD ["./bot"]
```

### Persistência em Produção
- **Postgres**: Recomendado para produção. Configure o driver `store/sqlstore` com a URL do Postgres.
- **Volumes**: Se usar SQLite, monte o arquivo `.db` em um volume persistente para não perder a sessão (e ter que escanear o QR Code novamente).

### Logs
Recomendamos plugar um `waLog.Logger` customizado para enviar logs para sistemas como Datadog ou CloudWatch, filtrando dados sensíveis.