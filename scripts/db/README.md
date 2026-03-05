# DB Utilities

Utilidades de infraestructura para PostgreSQL.

## Scripts

| Script | Descripción |
|--------|-------------|
| `wait-for-db.sh` | Espera hasta 60s a que PostgreSQL responda `SELECT 1` |

## Uso

```bash
# Con DB URL por defecto (localhost:5432/nexus)
bash scripts/db/wait-for-db.sh

# Con DB URL custom
bash scripts/db/wait-for-db.sh "postgres://user:pass@host:5432/mydb"

# Ayuda
./scripts/db/wait-for-db.sh --help
```

Usado internamente por `seed_demo.sh` y otros scripts que necesitan garantizar que la DB esté lista antes de operar.
