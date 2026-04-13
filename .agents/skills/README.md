# Skills (Playbooks) — libatbus-go

Actionable guides for common workflows in this module.

Each skill is a directory containing a `SKILL.md` file with YAML frontmatter, following the [Agent Skills](https://agentskills.io/) specification.

| Skill             | Directory          | Description                                                                               |
| ----------------- | ------------------ | ----------------------------------------------------------------------------------------- |
| Build             | `build/`           | Build, code generation, dependencies                                                      |
| Testing           | `testing/`         | Run and write tests, cross-language vectors, concurrency testing                           |
| Protocol & Crypto | `protocol-crypto/` | ECDH handshake, encryption/compression negotiation, message framing, access token auth     |
| Architecture      | `architecture/`    | Module structure, concurrency model, C++ parity patterns                                   |

## Key Components

- **Node** (`impl/atbus_node.go`) — Central bus node: init, routing, event loop, crypto config, GC
- **Endpoint** (`impl/atbus_endpoint.go`) — Remote node with ctrl + data connections, statistics
- **Connection** (`impl/atbus_connection.go`) — Single connection state machine (kConnecting → kHandshaking → kConnected)
- **ConnectionContext** (`impl/atbus_connection_context.go`) — ECDH handshake, cipher/compression, message pack/unpack
- **MessageHandler** (`message_handle/atbus_message_handler.go`) — Dispatch table for register, ping/pong, forward, auth
- **Topology** (`impl/atbus_topology.go`) — Peer registry, upstream/downstream relation types
- **Channels** — Transport: TCP (`ipv4://`/`ipv6://`), Unix (`unix://`), pipe (`pipe://`)
- **Protocol** (`protocol/libatbus_protocol.proto`) — Protobuf v3 wire format
