---
slug: project-structure
category: architecture
generatedAt: 2026-01-29
---

# Como a base de código está organizada?

O `whatsmeow` segue uma estrutura plana na raiz para a API pública, com pacotes de suporte em subdiretórios.

## Estrutura de Diretórios

### Núcleo (`/`)
- **`client.go`**: O "coração" da biblioteca. Contém a struct `Client` e métodos principais (`Connect`, `SendMessage`).
- **`handshake.go`**: Lógica de conexão inicial (Noise Protocol).
- **`message.go`**: Helpers para construção de mensagens.

### Protocolo e Rede
- **`binary/`**: (Crítico) Contém o codificador/decodificador do formato binário proprietário do WhatsApp. Tudo que entra/sai do socket passa por aqui.
- **`proto/`**: Arquivos gerados pelo Protobuf. Contém as definições de todas as mensagens (texto, imagem, enquete).
- **`socket/`**: Abstração de baixo nível da conexão WebSocket.

### Persistência e Criptografia
- **`store/`**: Define a interface `Store` para salvar sessões.
    - `store/sqlstore`: Implementação genérica SQL.
    - `store/sqlite3`: Driver específico SQLite.
- **`util/`**: Funções auxiliares de criptografia (AES, HMAC) e chaves.

### Funcionalidades Específicas
- **`appstate/`**: Sincronização de estado (histórico, contatos, mute).
- **`types/`**: Tipos globais como `JID` (identificador de usuário) e eventos.
