# Nexus v2 Pre-Production Checklist

Relacionado:

- [README.md](README.md)
- [MVP.md](MVP.md)
- [TECHNICAL_REFERENCE.md](TECHNICAL_REFERENCE.md)
- [PROD_CHECKLIST.md](PROD_CHECKLIST.md)

Estado actual: pendiente.

Objetivo:

- endurecer el MVP sin abrir dominio nuevo
- cerrar brechas operativas, de seguridad y de observabilidad
- llegar a una instancia candidata para pasar al gate final de produccion

Importante:

- este checklist se ejecuta despues del MVP
- cuando este checklist quede completo, el siguiente paso obligatorio es [PROD_CHECKLIST.md](PROD_CHECKLIST.md)
- no se deberia salir a produccion solo con MVP cerrado

## Alcance

Este tramo no agrega producto nuevo.

Se limita a:

- seguridad
- observabilidad
- operacion
- deploy
- base de datos
- pruebas de sistema
- documentacion operativa

Queda fuera:

- `ai-runtime`
- tenants / orgs
- RBAC completo
- playbooks nuevos de dominio
- nuevas features de negocio

## 1. Seguridad de aplicacion

- [ ] mover secrets reales fuera de `.env` locales y de `docker compose`
- [x] definir fuente de verdad para API keys de pre-prod
- [x] documentar rotacion de API keys (ver [OPS.md](OPS.md))
- [x] separar claramente:
  - admin keys
  - service keys
- [x] confirmar que `/healthz` y `/readyz` son los unicos endpoints sin auth
- [x] revisar que errores de auth no filtren detalles internos
- [x] definir politica minima de CORS si algun endpoint va a exponerse via browser
- [x] revisar headers de seguridad y proxy forwarding si hay ingress
- [ ] confirmar TLS en el entorno de pre-produccion

## 2. Observabilidad

- [x] logs estructurados JSON en los tres servicios
- [x] request ID consistente en todo request entrante y saliente
- [x] correlacion entre:
  - action
  - incident
  - alert
  - audit
- [x] metricas de servicio tipo RED:
  - request rate
  - error rate
  - latency
- [x] metricas de negocio minimas:
  - actions created
  - actions blocked
  - actions approved
  - actions executed
  - incidents created
  - alerts created
- [x] dashboard minimo para health del sistema
- [x] alertas minimas de infraestructura:
  - servicio caido
  - latencia alta
  - error rate alto
  - DB no disponible

## 2b. Resiliencia de runtime

- [ ] idempotencia en `POST /v1/actions`:
  - header `Idempotency-Key`
  - tabla de dedup con TTL 24h
  - si key ya existe y esta dentro del TTL, retornar resultado anterior sin crear duplicado
  - approve/reject/lease/execute usan idempotencia semantica via state machine (no key generica)
- [ ] graceful degradation en data-plane cuando control-plane no responde:
  - cache local de resources: soft TTL 30s, hard TTL 15m
  - cache local de policies: soft TTL 30s, hard TTL 5m
  - si control-plane no responde y cache esta fresca: usar cache, marcar decision como `degraded_context` en audit
  - si cache miss o hard TTL excedido: fail closed (deny)
  - cada entry de cache incluye version, fetched_at, expires_at
  - loguear toda degradacion

## 3. PostgreSQL y datos

- [x] definir sizing inicial de pools por servicio
- [x] validar timeouts de conexion y query
- [ ] validar estrategia `up-only` de migrations en deploy
- [ ] correr migrations en un entorno limpio
- [x] probar backup manual
- [x] probar restore manual
- [x] probar restart con estado persistido en pre-prod
- [ ] revisar indices de tablas principales con datos de prueba reales
- [x] definir retencion inicial de audit:
  - audit records: 90 dias en hot storage (PostgreSQL)
  - registros mas viejos: purge con job periodico o manual
  - en produccion: evaluar export a cold storage (S3) antes del purge
  - idempotency keys: 24h TTL con purge periodico

## 4. Pruebas

- [x] mantener verde:
  - `go test ./...`
  - `go test -race ./...`
  - `golangci-lint run ./...`
  - `make milestone`
- [x] agregar una corrida estable de stack completo en CI (ver `.github/workflows/v2-ci.yml`)
- [ ] repetir e2e autenticado contra entorno desplegado
- [x] agregar smoke de restart con persistencia real
- [x] probar degradacion controlada:
  - control-plane caido: data-plane usa cache y crea acciones (verificado en compose)
  - control-workers caido: data-plane sigue decidiendo (best-effort ya validado)
  - idempotencia: misma key retorna mismo action ID sin duplicar (verificado en compose)
  - script disponible: `scripts/smoke/run-degradation-flow.sh`
  - pendiente para infra real: DB no disponible, cache expirado + control-plane caido
- [x] confirmar comportamiento esperado de `audit` best effort bajo fallo

## 5. Deploy y operacion

- [x] confirmar graceful shutdown en los tres servicios
- [x] confirmar readiness separada de health
- [x] definir estrategia de rollout: rolling update (ver [OPS.md](OPS.md))
- [x] definir rollback minimo (ver [OPS.md](OPS.md))
- [x] documentar configuracion requerida por servicio (ver [OPS.md](OPS.md))
- [x] congelar imagenes/tagging para pre-prod (ver [OPS.md](OPS.md) — Image tagging convention)
- [ ] validar que compose local tenga equivalente real en el entorno objetivo

## 6. Runbooks minimos

- [x] runbook para (ver [OPS.md](OPS.md)):
  - servicio no levanta
  - DB no conecta
  - migrations fallan
  - auth falla
  - actions quedan bloqueadas inesperadamente
- [x] runbook de restore de DB (ver [OPS.md](OPS.md))
- [x] runbook de rotacion de API keys (ver [OPS.md](OPS.md))
- [x] runbook de rollback (ver [OPS.md](OPS.md))

## 7. Documentacion

- [x] mantener alineados:
  - [DEFINITION.md](DEFINITION.md) — actualizado con idempotencia, degradation, links a ROADMAP y OPS
  - [MVP.md](MVP.md) — sin cambios (MVP no cambio)
  - [TECHNICAL_REFERENCE.md](TECHNICAL_REFERENCE.md) — actualizado con idempotencia, degradation, links
  - [ENDPOINT_FLOWS.md](ENDPOINT_FLOWS.md) — actualizado con idempotency check y cache en POST /v1/actions
- [x] documentar variables de entorno reales de pre-prod (ver [OPS.md](OPS.md))
- [x] documentar topologia de despliegue (ver [OPS.md](OPS.md))
- [x] documentar quien consume cada API key (ver [OPS.md](OPS.md))

## Exit Criteria

Este checklist se considera cerrado cuando:

- todas las casillas de este documento estan completas
- existe entorno de pre-produccion funcional y estable
- el sistema ya fue probado con persistencia real, auth real y rollback basico
- hay runbooks minimos y visibilidad operativa suficiente

Cuando eso pase:

- no se sale a produccion todavia
- se ejecuta el gate final en [PROD_CHECKLIST.md](PROD_CHECKLIST.md)

## Notas de implementacion ya resueltas

- la fuente de verdad prevista para secrets y API keys de staging/prod es AWS Secrets Manager via `v2/infra`
- `v2/infra` ya genera por defecto:
  - admin API key
  - data-plane service API key
  - control-workers service API key
  - Prometheus API key
- los servicios ECS consumen secrets agregados por servicio
- los operadores pueden recuperar las API keys individuales desde sus secretos dedicados
