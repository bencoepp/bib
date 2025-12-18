# P2P Networking

This section covers the peer-to-peer networking layer and wire protocols.

## In This Section

| Document | Description |
|----------|-------------|
| [P2P Networking](p2p-networking.md) | Peer discovery, data transfer, and pub/sub messaging |
| [Protocols Reference](protocols.md) | Wire protocols and message formats |

## Overview

Bib uses [libp2p](https://libp2p.io/) for decentralized peer-to-peer communication:

```
┌─────────────────────────────────────────────────────────────┐
│                      P2P Stack                               │
├─────────────────────────────────────────────────────────────┤
│  Application Protocols: Discovery, Data, Jobs, Sync         │
├─────────────────────────────────────────────────────────────┤
│  PubSub: GossipSub for real-time updates                    │
├─────────────────────────────────────────────────────────────┤
│  DHT: Kademlia for peer/content discovery                   │
├─────────────────────────────────────────────────────────────┤
│  Transport: TCP/QUIC | Security: Noise | Mux: Yamux         │
└─────────────────────────────────────────────────────────────┘
```

## Protocol Summary

| Protocol | ID | Purpose |
|----------|-----|---------|
| Discovery | `/bib/discovery/1.0.0` | Catalog exchange and peer info |
| Data | `/bib/data/1.0.0` | Dataset and chunk transfers |
| Jobs | `/bib/jobs/1.0.0` | Distributed job coordination |
| Sync | `/bib/sync/1.0.0` | State synchronization |

## Key Features

- **Decentralized** — No central server required
- **Encrypted** — All connections use Noise protocol
- **Resilient** — Automatic peer discovery and reconnection
- **Efficient** — Chunked transfers with resumable downloads

---

[← Back to Documentation](../README.md)

