# Demo

Demostración guiada de las features de Nexus gateway.

## Scripts

| Script | Descripción |
|--------|-------------|
| `demo.sh` | Walkthrough de 5 minutos: health, egress, DLP, idempotency, audit, SSRF |

## Uso

```bash
export NEXUS_API_KEY="nexus-core-local-key"
bash scripts/demo/demo.sh
# o
./scripts/demo/demo.sh --help
```

## Prerequisitos

Stack corriendo, migraciones aplicadas, seed ejecutado:

```bash
make up && make migrate-up && make seed
```

## Pasos del demo

1. Health check
2. Setup egress (echo + transfer → mock-tools)
3. DLP deny: credit card enviado a tool external
4. WRITE con idempotency + timeout budget
5. Replay (misma key, sin re-ejecución upstream)
6. Audit export JSONL con hash-chain
7. SSRF/egress protection (bloqueo de 169.254.169.254)
