# Nexus v2 Production Readiness Checklist

Relacionado:

- [README.md](README.md)
- [MVP.md](MVP.md)
- [PRE_PROD.md](PRE_PROD.md)
- [TECHNICAL_REFERENCE.md](TECHNICAL_REFERENCE.md)

Estado actual: bloqueado hasta cerrar [PRE_PROD.md](PRE_PROD.md).

Objetivo:

- decidir si `v2` puede salir a produccion
- no agregar features
- validar que lo endurecido en pre-prod ya funciona bajo criterios de produccion

## Precondicion obligatoria

- [ ] [PRE_PROD.md](PRE_PROD.md) completo y firmado internamente

Sin eso, este checklist no deberia correrse.

## 1. Seguridad y acceso

- [ ] secrets fuera de archivos locales y fuera del repo
- [ ] API keys rotables y con owner claro
- [ ] separacion entre credenciales:
  - admin
  - servicios
  - operadores humanos si aplica
- [ ] TLS activo en todos los accesos externos
- [ ] auditoria de cambios admin funcionando

## 2. Operacion

- [ ] on-call o responsable operativo definido
- [ ] contacto/escalacion definido
- [ ] alertas activas y enrutadas
- [ ] dashboards visibles para el equipo
- [ ] runbooks accesibles

## 3. Datos

- [ ] backup automatizado configurado
- [ ] restore probado recientemente
- [ ] estrategia de migraciones validada
- [ ] capacidad inicial de DB aceptable para la carga esperada

## 4. Release

- [ ] version/tag de release definido
- [ ] rollback probado
- [ ] estrategia de despliegue definida
- [ ] smoke post-deploy definido
- [ ] criterio de abortar rollout definido

## 5. Validacion final

- [ ] smoke en entorno productivo o equivalente inmediato
- [ ] create -> decision -> incident/alert -> audit validado
- [ ] health/readiness validados
- [ ] logs y metricas visibles
- [ ] errores criticos de arranque y de DB visibles para operacion

## Decision

Solo cuando todo lo anterior este cerrado:

- [ ] aprobado para salir a produccion

Si algun punto falla:

- [ ] no se libera
- [ ] se vuelve a [PRE_PROD.md](PRE_PROD.md) o al backlog tecnico correspondiente
