# Bootstrap

Setup completo del entorno en un solo comando.

## Scripts

| Script | Descripción |
|--------|-------------|
| `bootstrap.sh` | Copia `.env.example` → `.env`, levanta stack, migraciones, seed |

## Uso

```bash
bash scripts/bootstrap/bootstrap.sh
# o
./scripts/bootstrap/bootstrap.sh --help
```

## Qué hace

```
.env.example → .env  →  make up  →  make migrate-up  →  make seed
```

Después de ejecutar, el stack queda listo para demo o e2e.
