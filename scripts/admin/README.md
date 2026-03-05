# Admin

Setup y validación de la consola de administración.

## Scripts

| Script | Descripción |
|--------|-------------|
| `quickstart_admin.sh` | Stack completa + seed + egress + validación RBAC, REST, MCP, A2A, admin bootstrap |

## Uso

```bash
bash scripts/admin/quickstart_admin.sh
# o
./scripts/admin/quickstart_admin.sh --help
```

## Qué valida

1. Stack levantada y migrada
2. Seed con org demo y API keys
3. Egress configurado para echo y transfer
4. RBAC: 403 sin scopes
5. Admin bootstrap endpoint
6. REST `/v1/run` (echo)
7. MCP `tools/list`
8. A2A call

Al final imprime API key, URL de admin console y URL de métricas.
