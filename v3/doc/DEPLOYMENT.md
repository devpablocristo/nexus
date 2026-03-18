# Nexus v3 — Deployment

## Desarrollo local (Docker Compose)

### Requisitos

- Docker + Docker Compose
- Go 1.26+ (solo para desarrollo sin Docker)
- Node.js 20+ (solo para desarrollo del console)

### Levantar todo

```bash
cd v3/
make up          # copia .env.example → .env si no existe, levanta containers
```

Servicios:
- Console: http://localhost:13001
- API: http://localhost:18084
- PostgreSQL: localhost:15434

### Comandos útiles

```bash
make down        # bajar containers
make build       # rebuild images
make logs        # ver logs en tiempo real
make test        # go test ./... en review/
make qa          # build + vet + test -race
make smoke       # smoke tests contra API corriendo
make e2e         # end-to-end lifecycle test
make acceptance  # smoke + e2e
make dev         # correr review/ localmente (requiere postgres en :15434)
```

### Variables de entorno

Archivo `.env.example` como referencia canónica:

```bash
COMPOSE_PROJECT_NAME=nexus-v3

# Puertos
NEXUS_REVIEW_PORT=18084
NEXUS_CONSOLE_PORT=13001
NEXUS_POSTGRES_PORT=15434

# API keys
NEXUS_REVIEW_ADMIN_API_KEY=nexus-review-admin-dev-key
NEXUS_REVIEW_PROMETHEUS_API_KEY=nexus-review-prometheus-dev-key

# AI
ANTHROPIC_API_KEY=          # opcional, sin key usa fallback

# Config
NEXUS_REVIEW_APPROVAL_TTL=3600   # segundos
```

### Variables internas (dentro del container)

| Variable | Descripción | Ejemplo |
|----------|-------------|---------|
| `PORT` | Puerto HTTP | 8080 |
| `DATABASE_URL` | Connection string PostgreSQL | postgres://...@postgres:5432/nexus_review |
| `NEXUS_API_KEYS` | Pares key=value separados por coma | admin=key1,prometheus=key2 |
| `ANTHROPIC_API_KEY` | API key de Claude (opcional) | sk-ant-... |
| `APPROVAL_DEFAULT_TTL` | TTL de approvals en segundos | 3600 |

## Agregar un nuevo servicio

1. Crear `v3/{servicio}/` con la estructura Go estándar
2. En PostgreSQL, crear database: `CREATE DATABASE nexus_{servicio}`
3. Agregar servicio al `docker-compose.yml`:
   ```yaml
   {servicio}:
     build:
       context: .
       dockerfile: {servicio}/Dockerfile
     environment:
       DATABASE_URL: "postgres://postgres:postgres@postgres:5432/nexus_{servicio}?sslmode=disable"
     depends_on:
       postgres:
         condition: service_healthy
   ```
4. Agregar sección en `console/`
5. Agregar scripts en `scripts/`

## Producción (futuro)

Opciones evaluadas para MVP:
- **fly.io** — deploy simple, postgres managed, SSL incluido
- **Railway** — similar a fly.io, buena DX
- **AWS ECS + RDS** — más control, más complejidad

El monolito modular actual deploya como un solo container + postgres. Sin orquestación compleja.
