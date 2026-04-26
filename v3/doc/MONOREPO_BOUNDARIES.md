# Nexus v3 — Fronteras Del Monorepo

## Decision

Nexus v3 sigue en monorepo durante esta etapa. La separacion fisica a repos distintos queda para despues de estabilizar contratos, tests y ownership.

El monorepo contiene productos separados:

- **Nexus**: governance/control plane. La carpeta tecnica temporal sigue siendo `review/`.
- **Companion**: empleado digital generalista. Consume Nexus por API.
- **Console**: interfaz operativa para Nexus y Companion.
- **Connectors**: capacidades operativas internas de Companion.

## Regla Central

Nexus decide. Companion trabaja. Connectors conectan.

Nexus no debe contener connectors ni logica de ejecucion externa. Companion no debe decidir por si mismo si una accion sensible puede ejecutarse. Todo side effect debe pasar por Nexus y quedar reportado como resultado/evidencia.

## core Y modules

`core` y `modules` son librerias del ecosistema Pablo, no carpetas compartidas de conveniencia.

Solo se mueve codigo ahi cuando es:

- agnostico de negocio y producto;
- independiente de Nexus, Companion, Pymes y Ponti;
- usable por cualquier proyecto;
- testeable y versionable como libreria;
- estable como contrato publico.

El contrato de connectors queda dentro de Companion hasta que haya reutilizacion real por un segundo consumidor.

## Camino A Separacion Fisica

Antes de separar repos deben existir:

- API publica estable de Nexus;
- API publica estable de Companion;
- cliente/SDK claro para Companion -> Nexus;
- contrato interno estable de connectors;
- smoke/e2e que validen ambos productos;
- documentacion de setup por producto;
- releases independientes posibles.

Cuando eso este listo, el corte natural es:

```text
nexus/
  governance API

companion/
  employee agent
  connectors/

console/
  operations UI
```
