# Sprints — Git Rodolfo MVP

> Basado en PRD 0.4 (`git -rodolfo-PRD.md`). 6 sprints para el MVP completo.

## Por qué este orden

El criterio de secuencia es **qué es imposible de construir sin qué**:

- Nada funciona sin persistencia de cuentas → Sprint 1.
- No se puede autenticar sin que existan llaves y cuentas → Sprint 2 antes que Sprint 3.
- No se puede forzar identidad en un repo sin `IdentityMechanism` → Sprint 4 depende de que Sprint 2-3 ya sepan verificar una cuenta.
- No se pueden traducir errores de `clone`/`current` sin que `clone`/`current` existan → Sprint 5 depende de Sprint 4.
- No tiene sentido testear ni distribuir algo que no está completo → Sprint 6 al final.

---

## Resumen

| Sprint | Foco                                                 | Depende de |
| ------ | ---------------------------------------------------- | ---------- |
| 1      | Base: proyecto, persistencia, interfaces, flags      | —          |
| 2      | Cuentas y llaves SSH                                 | Sprint 1   |
| 3      | Autenticación, GitHub CLI, edición/eliminación       | Sprint 2   |
| 4      | Identidad por repositorio: `clone`, `use`, `current` | Sprint 1–3 |
| 5      | Traducción de errores y `doctor`                     | Sprint 4   |
| 6      | Pruebas, endurecimiento, distribución                | Sprint 1–5 |

---

## Sprint 1 — Base y persistencia

**Objetivo:** que exista un binario instalable que sepa guardar y listar cuentas, aunque todavía no haga nada con SSH ni GitHub. Todo lo de aquí en adelante se apoya en esto.

**Tareas:**

- Estructura del proyecto en Go, módulo, layout de paquetes (§19.2: CLI / Application / Domain / Infrastructure).
- Dispatcher de subcomandos y arranque de `git-rodolfo` y `git rodolfo` (RF-23, RF-24) — CU-12 se puede cerrar este sprint es puramente mecánico.
- Modelo `Account` y `GlobalConfig` (§18).
- `ConfigRepository`: lectura/escritura de `~/.config/git-rodolfo/config.json`, respetando `XDG_CONFIG_HOME`.
- Escritura atómica (archivo temporal + rename) y lock de archivo (RNF-05, RNF-06).
- Interfaces `GitClient`, `SSHClient`, `SSHAgentAdapter`, `ProviderClient`, `AccountRepository` (§19.3) **definidas ya**, aunque solo se implementen parcialmente. Esto es lo que hace testeable todo lo siguiente sin Git/SSH real (RNF-09) — si se pospone, se paga después con retrabajo.
- Flags globales: `--non-interactive`, `--yes`, `--verbose` (RF-12).
- `git rodolfo accounts` (RF-02) y `git rodolfo account show <account>` (RF-03) — solo lectura, sobre datos que en este sprint se insertan a mano o vía fixture, porque `account add` no existe todavía.
- `help`, `--help`, `--version`.

**Definición de hecho:**

- Se puede instalar el binario localmente y correr `git rodolfo accounts` y `git-rodolfo accounts` con el mismo resultado.
- `config.json` sobrevive a una escritura interrumpida (kill -9 a mitad de escritura) sin corromperse.
- Dos procesos escribiendo a la vez no producen una carrera (RNF-06 probado con un test que lo fuerce).
- Existen dobles de prueba (fakes) para las cinco interfaces de infraestructura.

---

## Sprint 2 — Cuentas y llaves SSH

**Objetivo:** `account add` de punta a punta, incluida la generación y validación de llaves. Al final de este sprint el usuario puede registrar una cuenta, aunque la herramienta todavía no verifique contra GitHub real (eso es Sprint 3).

**Tareas:**

- `account add` interactivo: nombre, usuario del proveedor, nombre/correo de commit (§13.2, RF-01).
- Generación de llave Ed25519 vía `ssh-keygen`, sin sobrescribir sin confirmación (RF-06).
- Asociar llave existente por ruta: validar que exista, sea legible, sea una llave privada válida (RF-07, primera mitad).
- Huella pública (`publicKeyFingerprint`) y detección de llave duplicada entre cuentas (RF-07, segunda mitad) — este es el caso CU-02, y depende de tener la huella calculada, así que va en el mismo sprint que la generación de llaves, no antes.
- Integración con `ssh-agent`: detectar passphrase, cargar con `ssh-add`, opción de Keychain en macOS (RF-11). Rodolfo nunca lee la passphrase.
- `account add --non-interactive` con todos los flags (§13.2).
- Rechazo de proveedores no soportados en el registro (RF-25, mitad de alta).

**Definición de hecho:**

- Se pueden registrar dos cuentas con llaves distintas.
- Intentar asociar la misma llave a una segunda cuenta se bloquea con el mensaje de §21 ("La llave ya pertenece a otra cuenta"), no con un error genérico.
- Una llave con passphrase queda cargada en el agente sin que la herramienta la haya visto ni almacenado.
- El flujo no interactivo produce el mismo estado final que el interactivo (mismo test, dos caminos).

**Nota:** todavía no hay verificación de autenticación real ni registro en GitHub — el "✓ SSH key accepted / ✓ GitHub account detected" del flujo de §13.2 se implementa en el Sprint 3. Este sprint deja la cuenta guardada con `authenticationStatus: unverified`.

---

## Sprint 3 — Autenticación, GitHub CLI, edición y eliminación

**Objetivo:** cerrar el ciclo de vida completo de una cuenta: que `add` termine verificando de verdad, y que `edit`/`remove` existan.

**Tareas:**

- Prueba de autenticación SSH forzando la llave, leyendo la salida en vez del código de salida (RF-13, §4.5) — esto es lo que cierra el flujo de `account add` empezado en el Sprint 2.
- Comparación del usuario devuelto por SSH contra `providerUsername`; mensaje específico cuando no coincide (§21, "La llave autentica como otro usuario").
- `ActiveCLIUsername()` sobre `gh`, verificación previa a `gh ssh-key add`, flujo de "GitHub CLI está autenticado con otra cuenta" (RF-14, §13.2).
- `account edit` con resumen antes/después y advertencia de repositorios afectados (RF-04, §13.3).
- `account remove`: dos confirmaciones separadas — perfil primero, llave después con advertencia — y `--delete-key` en modo no interactivo (RF-05, RF-08, §13.5). Depende de tener la huella (Sprint 2) para no ofrecer borrar una llave usada por otra cuenta.
- `account show` ahora muestra `authenticationStatus` real junto a `lastVerifiedAt`, degradando a "not verified recently" pasados 7 días.

**Definición de hecho:**

- `account add` termina en "✓ SSH key accepted / ✓ GitHub account detected" contra una cuenta de prueba real.
- CU-04 y CU-05 (eliminar conservando o borrando la llave) se ejecutan ambos y dejan el estado esperado.
- `account edit` avisa qué repositorios locales quedan desactualizados (aunque `use`/`current` para aplicarlo llega en el Sprint 4 — aquí solo se detecta y advierte a partir de `rodolfo.account` si ya existiera de pruebas manuales).

---

## Sprint 4 — Identidad por repositorio: `clone`, `use`, `current`

**Objetivo:** la mitad del producto que toca repositorios reales. Es el sprint más grande porque implementa la decisión de arquitectura completa de §12.2.

**Tareas:**

- `IdentityMechanism` y su única implementación del MVP, `SSHCommandMechanism` (Apply/Clear/Verify sobre `core.sshCommand`) — RF-09, RF-10, §12.2.
- `git rodolfo clone`: validación de URL, selector de cuenta, clone forzando `-c core.sshCommand=...`, persistencia local, `user.name`/`user.email`/`rodolfo.account` (RF-15, RF-17, RF-18, §13.6).
- Tratamiento de URLs HTTPS: preguntar SSH vs. mantener HTTPS, nunca convertir en silencio (RF-16).
- `git rodolfo use`, incluido el caso sin `origin` (RF-19, §13.7).
- `git rodolfo use --clear` (RF-20).
- `git rodolfo current`, incluida la distinción entre "no gestionado por Rodolfo" y "gestionado con inconsistencia" (§13.8).

**Definición de hecho:**

- Clonar un repositorio privado real con la cuenta correcta funciona **teniendo otra llave cargada en `ssh-agent`** — esta es la prueba de validación 1 de §12.2, y es la que de verdad certifica que el mecanismo elegido cumple su función. Si falla aquí, no se avanza al Sprint 5.
- `gh repo view` y `gh pr create` funcionan sin ajustes dentro de un repo clonado por Rodolfo (valida que el remoto quedó canónico).
- `use --clear` deja `.git/config` sin ninguna entrada de Rodolfo, verificable con `git config --local --list`.
- `use` configura identidad en un repo recién creado con `git init`, sin remoto, sin error.

---

## Sprint 5 — Traducción de errores y diagnóstico

**Objetivo:** la capa que convierte los mensajes ambiguos de GitHub en algo accionable, y el comando que audita todo el sistema de una vez.

**Tareas:**

- `ErrorTranslator` y el caso `Repository not found` (RF-21, §13.9, §21) — necesita que `clone`/`current` ya existan (Sprint 4) para interceptar sus fallos.
- Confirmar qué casos **no** se traducen: `Permission denied (publickey)` solo recibe el comando sugerido; el resto de errores de Git pasa intacto (principio 8).
- `git rodolfo doctor` con las ocho validaciones de §13.10, incluida la detección de `GIT_SSH_COMMAND` en el entorno (validación 8, el único modo de fallo del mecanismo de §12.2).
- Catálogo completo de mensajes de error de §21 revisado contra la implementación real (que el texto coincida con lo que el código efectivamente produce).

**Definición de hecho:**

- Clonar con la cuenta equivocada produce el mensaje de CU-07 (posible falta de acceso + cuenta sugerida + error original de Git), no el mensaje crudo de GitHub.
- `doctor` detecta, en una corrida contra fixtures preparadas: llave faltante, permisos inseguros, llave compartida entre cuentas, fallo de autenticación y `GIT_SSH_COMMAND` exportada.
- Cada hallazgo de `doctor` imprime el comando exacto que lo resuelve (RF-22).

---

## Sprint 6 — Pruebas, endurecimiento y distribución

**Objetivo:** que el MVP sea instalable por alguien que no sea el desarrollador, y que la cobertura de pruebas sostenga los criterios de aceptación del §22.

**Tareas:**

- Completar la suite de pruebas unitarias sobre los fakes de las cinco interfaces (RNF-09), cerrando huecos que quedaron pendientes en los sprints 1-5.
- Pruebas de integración contra repositorios Git locales reales y un contenedor con servidor SSH, sin depender de GitHub.
- Documentar y ejecutar la suite end-to-end manual con dos cuentas reales, cubriendo los 23 criterios de aceptación de §22.
- Build para macOS y Linux, script de instalación, fórmula de Homebrew, release en GitHub.
- Documentación de usuario y guía de solución de problemas (para los casos que `doctor` no puede arreglar solo).
- Recorrido final de §22 punto por punto, marcando qué queda pendiente si algo no cierra.

**Definición de hecho:**

- Alguien externo al desarrollo instala el binario desde cero (Homebrew o script) y completa el flujo de §28 sin ayuda.
- Los 23 criterios de aceptación de §22 están verificados uno por uno, con evidencia (test automatizado o resultado manual documentado).
- El release queda publicado y es instalable.

---

## Trazabilidad: requerimientos → sprint

| Requerimiento                                      | Sprint                                   |
| -------------------------------------------------- | ---------------------------------------- |
| RF-01 Registrar cuentas                            | 2                                        |
| RF-02 Listar cuentas                               | 1                                        |
| RF-03 Consultar cuenta                             | 1 (básico) / 3 (estado real)             |
| RF-04 Editar cuentas                               | 3                                        |
| RF-05 Eliminar cuentas                             | 3                                        |
| RF-06 Generar llaves                               | 2                                        |
| RF-07 Usar llaves existentes / detectar duplicadas | 2                                        |
| RF-08 Borrar llave junto con la cuenta             | 3                                        |
| RF-09 `core.sshCommand` por repositorio            | 4                                        |
| RF-10 No tocar `~/.ssh/config`                     | 4                                        |
| RF-11 `ssh-agent` / passphrase                     | 2                                        |
| RF-12 Modo no interactivo                          | 1 (flags), validado en cada sprint       |
| RF-13 Validar autenticación                        | 3                                        |
| RF-14 Registrar llave en GitHub vía `gh`           | 3                                        |
| RF-15 Clonar repositorios                          | 4                                        |
| RF-16 Tratar URLs HTTPS                            | 4                                        |
| RF-17 Configurar Git localmente                    | 4                                        |
| RF-18 Guardar cuenta del repositorio               | 4                                        |
| RF-19 Operar sin remoto                            | 4                                        |
| RF-20 `use --clear`                                | 4                                        |
| RF-21 Traducir errores ambiguos                    | 5                                        |
| RF-22 `doctor`                                     | 5                                        |
| RF-23 / RF-24 Subcomando Git / binario directo     | 1                                        |
| RF-25 Rechazar proveedores no soportados           | 2 (alta) / 4 (clone/use)                 |
| RNF-05 / RNF-06 Recuperación y concurrencia        | 1                                        |
| RNF-09 Testabilidad                                | 1 (infraestructura) / 6 (suite completa) |
| §22 Criterios de aceptación (todos)                | 6 (verificación final)                   |

---

## Riesgos de calendario

Lo que más probablemente estire un sprint más allá:

- **Sprint 3.** El comportamiento de `gh` y de organizaciones con SSO no siempre es predecible en pruebas manuales; si aparece un caso no documentado en §4.5, puede tomar días adicionales de investigación.
- **Sprint 4.** Es el sprint con más superficie y con la prueba de validación más estricta (push con llave equivocada cargada en el agente). Si esa prueba falla, según §12.2 hay que reabrir la decisión de arquitectura — eso no es una tarea de un sprint, es un evento que rehace parte del Sprint 4.
- **Sprint 6.** La aprobación de un tap de Homebrew y la configuración de release en GitHub dependen de servicios externos y pueden no cerrar en los 5 días si hay fricción de configuración (firmas, notarización en macOS).

Si alguno de estos se materializa, el ajuste correcto es **extender ese sprint**, no comprimir el siguiente — el orden de dependencias de este plan asume que cada sprint entrega una base sólida para el que sigue.

---

## Backlog post-MVP (no programado)

Lo que quedó en §26 del PRD no tiene sprint asignado porque no es parte del MVP. Se menciona aquí solo para que quede claro que no se perdió, no para sugerir que se planifique ya: hook `pre-commit`, `doctor --fix`, modo alias SSH, escaneo de `~/.ssh`, salida `--json`, autocompletado, soporte GitLab/Bitbucket/Windows, y el resto de la lista de §26.
