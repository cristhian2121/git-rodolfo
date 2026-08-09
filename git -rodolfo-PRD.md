# PRD — Git Rodolfo MVP

> **Versión del documento:** 0.4 — documento único.
> Cambios respecto a 0.1, 0.2 y 0.3 en el [Anexo A](#anexo-a--historial-de-cambios).
> Esta versión es autosuficiente: la decisión de arquitectura que antes vivía en `ADR-0001-mecanismo-de-identidad-ssh.md` está incorporada en §12.2. Ese archivo queda retirado.

## 1. Información general

**Nombre del producto:** Git Rodolfo
**Nombre del binario:** `git-rodolfo`
**Versión:** MVP 0.4
**Tipo de producto:** Herramienta de línea de comandos
**Plataformas iniciales:** macOS y Linux
**Proveedor soportado:** GitHub (único proveedor del MVP)
**Usuarios objetivo:** Desarrolladores que utilizan varias cuentas de Git en un mismo computador

---

## 2. Descripción para público general

### ¿Qué problema soluciona?

Una persona puede tener varias cuentas de GitHub en el mismo computador: una personal, otra del trabajo, otra de la universidad o una diferente para cada cliente.

El problema es que Git puede terminar usando la cuenta incorrecta. Esto puede provocar errores como:

- Crear un commit con el correo personal en un proyecto laboral.
- Intentar descargar un repositorio privado con una cuenta sin permisos.
- Hacer `push` utilizando la llave SSH equivocada.
- Tener que modificar manualmente archivos y configuraciones técnicas.
- No saber qué cuenta está utilizando un proyecto.

### ¿Cómo lo soluciona?

Git Rodolfo reúne las cuentas del usuario en una sola herramienta.

El usuario podrá registrar sus cuentas, seleccionarlas desde un menú y decirle a Git Rodolfo qué cuenta debe utilizar para cada repositorio.

La herramienta se encargará de configurar automáticamente Git y SSH, evitando que el usuario tenga que editar comandos o archivos manualmente.

Por ejemplo:

```bash
git rodolfo clone git@github.com:company/project.git
```

Git Rodolfo mostrará las cuentas registradas, el usuario seleccionará una y el repositorio se clonará usando la identidad correcta.

Cuando algo falle, la herramienta traducirá los mensajes ambiguos de Git y GitHub en una explicación que indique la causa probable y el comando que la corrige.

---

## 3. Resumen ejecutivo

Git Rodolfo será una herramienta CLI para administrar múltiples cuentas e identidades Git desde un mismo computador.

Permitirá:

- Registrar cuentas existentes de GitHub.
- Crear la configuración local necesaria para utilizar cada cuenta.
- Generar o asociar llaves SSH.
- Editar y eliminar cuentas registradas, con la opción de borrar también su llave.
- Seleccionar una cuenta mediante un menú interactivo.
- Clonar repositorios con la cuenta correcta.
- Asociar una cuenta a un repositorio existente.
- Verificar qué identidad utiliza cada proyecto.
- Diagnosticar errores de Git y SSH.

Git Rodolfo no reemplazará Git. Funcionará como una capa de orquestación sobre Git, SSH y, opcionalmente, GitHub CLI.

El producto se distribuirá mediante un ejecutable llamado:

```text
git-rodolfo
```

Gracias al mecanismo de comandos externos de Git, podrá ejecutarse de ambas maneras:

```bash
git-rodolfo accounts
git rodolfo accounts
```

---

## 4. Contexto del problema

Git maneja diferentes conceptos que para muchos usuarios parecen ser una sola cuenta, pero técnicamente son configuraciones separadas.

### 4.1 Identidad de commits

Git utiliza:

```bash
git config user.name
git config user.email
```

Estos valores determinan el nombre y correo que aparecerán en los commits.

### 4.2 Autenticación

SSH determina qué llave utiliza el computador para autenticarse contra GitHub.

Punto crítico y poco conocido: cuando el usuario tiene varias llaves cargadas en `ssh-agent`, SSH las ofrece **en orden** y GitHub autentica con la primera que reconoce. Por eso un usuario puede acabar autenticado como su cuenta personal en un repositorio laboral aunque toda su configuración de Git sea correcta.

Lo importante de este fallo es que **no produce un error**: la operación simplemente se ejecuta como otra persona. No hay nada que reportar al usuario porque, desde el punto de vista de Git, no ha pasado nada anormal. La única forma robusta de evitarlo es restringir explícitamente la llave por repositorio (`IdentitiesOnly`).

### 4.3 Acceso al repositorio

El remoto del repositorio determina qué host se utiliza para ejecutar:

- `clone`
- `fetch`
- `pull`
- `push`

### 4.4 Sesión de GitHub CLI

GitHub CLI mantiene sus propias sesiones autenticadas y puede tener una cuenta activa distinta.

Esta separación obliga al desarrollador a administrar manualmente:

- Nombres y correos.
- Llaves privadas y públicas.
- El archivo `~/.ssh/config`.
- URLs remotas.
- Sesiones de GitHub CLI.
- Configuraciones locales de cada proyecto.

Git Rodolfo centralizará estas configuraciones en perfiles administrables.

### 4.5 Restricciones y comportamientos del proveedor

Hechos de GitHub que condicionan el diseño:

- **Una llave pública SSH solo puede estar registrada en una cuenta.** Al intentar registrarla en una segunda, GitHub responde `Key is already in use`. Esto invalida el escenario "reutilizar mi llave personal para la cuenta laboral" y obliga a una llave por cuenta.
- **`ssh -T git@github.com` siempre termina con código de salida 1**, incluso cuando la autenticación es correcta. El éxito debe determinarse leyendo la salida (`Hi <usuario>! You've successfully authenticated`), no el código de salida.
- **GitHub devuelve el mismo error para "el repositorio no existe" y "no tienes permiso".** En ambos casos responde `ERROR: Repository not found`. Lo hace deliberadamente, para que no sea posible enumerar repositorios privados. Consecuencia directa para el producto: **este error concreto no puede pasarse al usuario tal cual**, porque en el escenario multicuenta la causa real casi siempre es la segunda y el mensaje afirma la primera. Ver RF-21.
- **En `push`, GitHub sí nombra al usuario** (`Permission to org/repo.git denied to <usuario>`). Ese mensaje es útil tal como viene y no requiere traducción.
- **`gh ssh-key add` registra la llave en la cuenta activa de GitHub CLI**, que puede no ser la cuenta que se está configurando.
- Algunas organizaciones con SSO exigen autorizar la llave por separado, además de registrarla.

### 4.6 Alternativas existentes y por qué no bastan

| Alternativa | Qué resuelve | Qué deja fuera |
|---|---|---|
| `includeIf "gitdir:~/work/**"` nativo de Git | Identidad de commits automática por carpeta. Es la solución estándar y funciona bien. | No toca SSH: no resuelve autenticación, ni llaves, ni remotos. Exige disciplina de carpetas y edición manual de `~/.gitconfig`. |
| `~/.ssh/config` con alias por cuenta, a mano | Autenticación correcta por repositorio. | Configuración manual, propensa a errores de precedencia. No configura identidad de commits. No diagnostica. Barrera alta para la persona 9.4. |
| `gh auth switch` | Cambiar la cuenta activa de GitHub CLI. | Es estado global: no resuelve identidad por repositorio ni SSH. Solo aplica a operaciones hechas con `gh`. |
| Scripts personales / alias de shell | Todo, para quien ya sabe. | No es un producto: no hay diagnóstico, ni validación, ni onboarding. |

**Posicionamiento.** Git Rodolfo no inventa un mecanismo nuevo: usa los mismos que un experto configuraría a mano. Su valor está en tres cosas que ninguna alternativa cubre junta:

1. **Cobertura completa** de las tres capas a la vez (identidad de commits + autenticación SSH + remoto), que hoy se configuran en tres lugares distintos.
2. **Onboarding guiado y verificado**: al terminar, la herramienta confirma que la autenticación realmente funciona.
3. **Traducción de errores**: convertir los mensajes ambiguos del §4.5 en una causa y una acción.

---

## 5. Objetivo del producto

Permitir que un desarrollador administre y utilice varias cuentas Git desde una sola herramienta, reduciendo errores de identidad y autenticación.

El usuario deberá poder ejecutar comandos como:

```bash
git rodolfo accounts
git rodolfo account add
git rodolfo account edit
git rodolfo account remove
git rodolfo clone <repository-url>
git rodolfo use
git rodolfo current
git rodolfo doctor
```

La herramienta deberá traducir la cuenta seleccionada a las configuraciones necesarias de Git y SSH.

---

## 6. Objetivos del MVP

El MVP debe permitir:

1. Registrar una cuenta existente de GitHub.
2. Crear un perfil local asociado a esa cuenta.
3. Generar una llave SSH nueva.
4. Asociar una llave SSH existente, validando que no esté ya en uso por otra cuenta registrada.
5. Gestionar la carga de la llave en `ssh-agent` cuando tenga passphrase.
6. Registrar la llave pública en GitHub mediante GitHub CLI, verificando previamente que `gh` esté autenticado con la cuenta correcta.
7. Listar todas las cuentas registradas.
8. Consultar los detalles de una cuenta.
9. Editar una cuenta registrada.
10. Eliminar una cuenta de Git Rodolfo, con la opción de borrar también sus archivos de llave previa advertencia.
11. Seleccionar una cuenta mediante un menú interactivo.
12. Clonar un repositorio utilizando una cuenta seleccionada.
13. Configurar localmente el nombre y correo del repositorio.
14. Asociar una cuenta a un repositorio existente, **exista o no un remoto `origin`**.
15. Consultar qué cuenta utiliza el repositorio actual.
16. Traducir los errores de autenticación ambiguos en causa y acción.
17. Diagnosticar inconsistencias de Git, SSH y GitHub.
18. Ejecutarse de forma no interactiva mediante flags, sin requerir un terminal.
19. Ejecutar la herramienta mediante `git-rodolfo` o `git rodolfo`.

---

## 7. Fuera del alcance del MVP

No forman parte de la primera versión:

- Creación o eliminación de cuentas reales en GitHub.
- **Cuenta activa global.** Ver §11.2.
- **Hooks de Git** (`pre-commit`, `pre-push`) que bloqueen operaciones con identidad incorrecta.
- **`doctor --fix`.** El diagnóstico reporta y sugiere el comando; no aplica correcciones.
- **Escaneo automático de `~/.ssh`.** El usuario indica la ruta de su llave.
- **Salida `--json`.**
- Cambio automático de identidad al entrar a una carpeta.
- Interceptar directamente `git clone`, `git pull`, `git push` o `git fetch`.
- Reemplazar el ejecutable de Git.
- Modificar automáticamente todos los repositorios del computador.
- Interfaz gráfica o TUI completa.
- Sincronización de perfiles entre computadores.
- Administración de contraseñas.
- Almacenamiento de tokens en texto plano.
- Firma de commits con GPG o SSH.
- Soporte para Windows.
- **Cualquier proveedor distinto de GitHub.**
- Telemetría automática.
- Plugins externos.
- Gestión de repositorios remotos diferentes de `origin`.
- Autocompletado de shell.

La herramienta administrará cuentas locales vinculadas a GitHub, pero no creará ni eliminará cuentas dentro del proveedor.

---

## 8. Alcance del proveedor

### GitHub

Único proveedor soportado en el MVP. Incluye:

- Configuración de identidad de commits por repositorio.
- Configuración de la llave SSH por repositorio.
- Verificación de autenticación.
- Integración opcional con GitHub CLI.
- Registro de la llave pública, cuando `gh` esté autenticado con la cuenta correcta.
- Transformación de URLs HTTPS y SSH.

### Otros proveedores

Fuera del MVP. El modelo de datos conserva `provider` y `hostname` para no bloquear la extensión futura, pero el MVP solo acepta `provider: "github"` y rechaza con un mensaje claro las URLs de otros hosts.

**Justificación del recorte:** soportar Bitbucket "mediante SSH genérico" y otros proveedores "de forma experimental" añade superficie de error y casos de prueba sin usuarios que lo validen. Es más valioso que GitHub funcione perfectamente.

---

## 9. Personas

### 9.1 Desarrollador con cuenta personal y laboral

Trabaja en proyectos personales y empresariales desde el mismo computador. Necesita evitar que los commits laborales utilicen su correo personal.

### 9.2 Consultor con varios clientes

Tiene cuentas o credenciales diferentes para cada cliente. Necesita cambiar rápidamente de identidad y verificar cuál utiliza cada proyecto.

### 9.3 Estudiante y desarrollador

Utiliza una cuenta académica y otra personal. Necesita clonar repositorios privados de ambas cuentas sin cambiar manualmente las configuraciones.

### 9.4 Desarrollador nuevo en Git y SSH

Conoce los comandos básicos de Git, pero no domina la configuración de llaves SSH. Necesita una experiencia guiada y errores que expliquen qué pasó.

---

## 10. Propuesta de valor

Sin Git Rodolfo, un usuario puede necesitar ejecutar varios pasos:

```bash
git config user.name "Cristhian Delgado"
git config user.email "cristhian@company.com"
ssh-add ~/.ssh/id_ed25519_company
git clone git@github.com:company/private-repository.git
```

…y aun así puede autenticarse con la llave equivocada si el agente tiene varias cargadas, en cuyo caso GitHub responderá que el repositorio no existe.

Con Git Rodolfo podrá ejecutar:

```bash
git rodolfo clone git@github.com:company/private-repository.git
```

Después seleccionará una cuenta:

```text
Select an account:

❯ Personal
  Company
  University
```

Git Rodolfo realizará las configuraciones necesarias, verificará que la autenticación funcione y mostrará el resultado.

---

## 11. Principios del MVP

1. Git Rodolfo complementa Git, no lo reemplaza.
2. **No hay identidad implícita.** La cuenta se selecciona explícitamente o se deriva del repositorio en el que se está trabajando. Nunca de un estado global oculto.
3. No se modificarán configuraciones globales. Toda configuración de identidad es local al repositorio.
4. Cada repositorio tendrá su propia identidad, completa y verificable.
5. Las llaves privadas nunca se almacenarán dentro del archivo de configuración.
6. Toda modificación sensible deberá ser reversible mediante un comando de la propia herramienta.
7. La herramienta deberá mostrar claramente qué cuenta, correo y llave se están utilizando.
8. **Los errores de Git y SSH se muestran tal cual, salvo cuando son ambiguos o engañosos.** En esos casos —y solo en esos— se traducen a causa y acción, conservando siempre el mensaje original.
9. La herramienta prefiere configuración local por repositorio sobre modificar archivos compartidos del sistema.
10. **Las operaciones destructivas se ofrecen, no se imponen ni se prohíben.** Se explica qué se va a borrar, qué puede romperse y se pide confirmación explícita.
11. La integración con Git deberá ser simple y no invasiva.
12. Todo comando interactivo debe tener un equivalente no interactivo.

### 11.2 Sobre la eliminación de la "cuenta activa" global

La versión 0.1 contemplaba un `activeAccountId` global. Se elimina del MVP porque:

- Contradice los principios 2 y 4.
- Reintroduce el problema que el producto resuelve: un estado global invisible que el usuario cree conocer y no coincide con la realidad.
- Genera ambigüedad sin respuesta clara: ¿qué debe mostrar `current` en un repositorio sin cuenta asignada pero con una cuenta activa global?

En su lugar, el selector recuerda **la última cuenta utilizada** solo para preseleccionarla visualmente. Es una ayuda de UI, no configuración.

---

## 12. Integración con Git

### 12.1 Ejecución como subcomando

Git permite extender sus comandos mediante ejecutables cuyo nombre comience con `git-`. El ejecutable del producto será `git-rodolfo`.

Cuando el usuario ejecute `git rodolfo accounts`, Git buscará en el `PATH` un programa llamado `git-rodolfo` y le enviará los argumentos. Ambos comandos serán equivalentes:

```bash
git-rodolfo accounts
git rodolfo accounts
```

Git Rodolfo **no** interceptará `git clone`, `git push` ni `git pull`. El usuario deberá invocar explícitamente `git rodolfo <comando>`.

### 12.2 Mecanismo de identidad SSH — decisión

Esta sección documenta como decisión de arquitectura la elección del mecanismo que fuerza la llave SSH correcta por repositorio: qué opciones se evaluaron, por qué se descartó una y qué implica la elegida. Se conserva aquí, dentro del PRD, en vez de en un documento de decisión aparte.

#### El problema que resuelve

Cuando un usuario tiene varias llaves cargadas en `ssh-agent`, OpenSSH las ofrece al servidor **en orden** y GitHub autentica con la primera que reconoce. Consecuencia: un repositorio puede tener `user.email` perfectamente configurado para la cuenta laboral y aun así autenticar como la cuenta personal en cada `push`. Esto produce el síntoma más confuso del dominio —"mi configuración está bien pero GitHub dice que no tengo permisos"— y explica por qué la solución no puede limitarse a configurar Git (ver también §4.2).

La única forma robusta de resolverlo es **restringir explícitamente qué llave se ofrece**, mediante `IdentitiesOnly=yes` junto con la llave concreta. La decisión que sigue no es *si* hacerlo, sino *dónde* expresarlo.

#### Opciones evaluadas

**Opción A — Alias de host en `~/.ssh/config` + reescritura del remoto**

```sshconfig
# BEGIN GIT-RODOLFO: lean-tech
Host github-leantech
  HostName github.com
  User git
  IdentityFile ~/.ssh/id_ed25519_leantech
  IdentitiesOnly yes
# END GIT-RODOLFO: lean-tech
```

```bash
git remote set-url origin git@github-leantech:lean-tech/project.git
```

**Opción B — `core.sshCommand` local al repositorio**

```bash
git config --local core.sshCommand "ssh -i '~/.ssh/id_ed25519_leantech' -o IdentitiesOnly=yes"
```

El remoto permanece canónico: `git@github.com:lean-tech/project.git`.

| Criterio | A — Alias SSH | B — `core.sshCommand` |
|---|---|---|
| Resuelve la selección de llave | Sí | Sí |
| Modifica `~/.ssh/config` | **Sí** — archivo compartido, fuera del control del producto | **No** |
| Requiere backup y escritura defensiva de un archivo del sistema | Sí | No |
| Vulnerable a precedencia de bloques previos del usuario | **Sí** | No |
| El remoto queda canónico | **No** — host ficticio | Sí |
| Compatible con `gh`, IDE y CI que parsean el remoto | **No** | Sí |
| Reversible | Editar `~/.ssh/config` + restaurar el remoto | `git config --unset` |
| Alcance del efecto | Global al host, para cualquier herramienta | Solo operaciones Git de ese repositorio |
| Funciona fuera de Git (`ssh`, `scp`, `rsync`) | **Sí** | No |
| Visible con `git remote -v` | Sí — autodocumentado | No — hay que mirar `.git/config` |
| Versión mínima de Git | Cualquiera | 2.10 (el MVP ya exige 2.30) |
| Configuración por repositorio o por cuenta | Por cuenta (un alias sirve a N repos) | Por repositorio (se escribe N veces) |

**Los dos puntos que deciden:**

1. **`~/.ssh/config` es un archivo que no nos pertenece.** Si lo corrompemos, rompemos conexiones SSH del usuario a servidores, despliegues y hosts que nada tienen que ver con Git. Además tiene un modo de fallo sutil que ninguna salvaguarda cubre por completo: en `ssh_config` las directivas `IdentityFile` se **acumulan** entre todos los bloques que coinciden con el host, así que un `Host *` previo del usuario sigue aportando llaves aunque nuestro bloque declare `IdentitiesOnly yes`. El bloque puede parecer correcto al leer el archivo y no serlo al resolverse; verificarlo exige `ssh -G <alias>` (configuración efectiva), no leer el archivo.
2. **Un remoto con host ficticio rompe el ecosistema.** `git@github-leantech:lean-tech/project.git` no es una URL de GitHub para nadie que no sea SSH. `gh repo view`, `gh pr create`, la detección de repositorio de los IDE y algunos runners de CI locales dejan de funcionar o funcionan a medias — un costo difuso que el usuario no atribuirá a Git Rodolfo.

**Lo que se pierde al elegir B, honestamente:**

- No aplica fuera de Git (`ssh -T`, `scp`). Impacto bajo: el producto es sobre Git. La validación de autenticación debe pasar la llave explícitamente de todos modos.
- Se escribe por repositorio, no por cuenta. Sin impacto real: el usuario ejecuta `clone` o `use` igual en ambos casos.
- Menos autodocumentado: `git remote -v` ya no revela la identidad. Se compensa con `git rodolfo current`.
- `core.sshCommand` es una cadena que Git pasa al shell; hay que citar la ruta de la llave para tolerar espacios. Detalle de implementación.

#### Decisión

**El mecanismo por defecto es `core.sshCommand` local al repositorio.** El alias en `~/.ssh/config` queda fuera del MVP, previsto como mejora posterior opt-in (§26).

Configuración aplicada por `clone` y `use`:

```bash
git config --local user.name       "Cristhian Delgado"
git config --local user.email      "cristhian@leantech.com"
git config --local rodolfo.account "lean-tech"
git config --local core.sshCommand "ssh -i '/Users/roshi/.ssh/id_ed25519_leantech' -o IdentitiesOnly=yes"
```

El remoto no se toca.

Para `clone`, donde el repositorio todavía no existe, la identidad se fuerza en la propia invocación y se persiste después:

```bash
git -c core.sshCommand="ssh -i '<key>' -o IdentitiesOnly=yes" clone git@github.com:lean-tech/project.git
git -C project config --local core.sshCommand "ssh -i '<key>' -o IdentitiesOnly=yes"
```

**Notas de implementación:**

- **Rutas absolutas.** Se expande `~` antes de escribir, en vez de depender de que el shell lo haga. Se cita la ruta para tolerar espacios.
- **`GIT_SSH_COMMAND` tiene precedencia sobre `core.sshCommand`.** Si el usuario la tiene exportada en su entorno, anula la configuración del repositorio sin dar señales. Es el único modo de fallo del mecanismo elegido; `doctor` lo detecta y advierte (validación 8, §13.10).
- **`ssh-agent` sigue funcionando.** `IdentitiesOnly=yes` restringe *qué llaves se ofrecen*, no *de dónde salen*: si la llave indicada con `-i` está cargada en el agente, SSH la usa desde ahí. Esto permite que las llaves con passphrase sigan siendo cómodas (RF-11).
- **La verificación de autenticación no usa `core.sshCommand`**, porque no pasa por Git. Se ejecuta directamente:
  ```bash
  ssh -T -i <key> -o IdentitiesOnly=yes git@github.com
  ```
  recordando que este comando **siempre devuelve código de salida 1**; el éxito se determina leyendo `Hi <usuario>!` en la salida (§4.5).
- **Puerto 443.** Para redes que bloquean el 22, el mismo mecanismo lo soporta sin cambiar el remoto: `ssh -i <key> -o IdentitiesOnly=yes -p 443` contra `ssh.github.com`. Fuera del MVP, pero la decisión no lo impide.

**Requisito derivado:** `core.sshCommand` exige Git ≥ 2.10; el MVP ya requiere Git ≥ 2.30 (RNF-02).

**Aislamiento en el código.** La decisión queda encapsulada tras la interfaz `IdentityMechanism` (§19.3): ningún otro componente sabe cómo se aplica la identidad. Si en el futuro se implementa el modo alias, el cambio queda contenido en una segunda implementación (`SSHAliasMechanism`) sin tocar el resto de la aplicación.

#### Consecuencias

**Positivas:**

- Desaparece del camino por defecto el mayor riesgo técnico del producto: no hay backups, bloques delimitados ni fallos por precedencia que gestionar.
- El remoto sigue siendo una URL real de GitHub: `gh`, IDE y CI siguen funcionando.
- La reversión es un `git config --unset`, lo que hace que `use --clear` (RF-20) sea trivial y confiable.
- La configuración es inspeccionable con `git config --local --list`, sin herramientas del producto.

**Negativas:**

- El mecanismo no aplica fuera de Git. Aceptado: el producto es sobre Git.
- `GIT_SSH_COMMAND` en el entorno puede anular la configuración sin avisar. Mitigado con la validación 8 de `doctor`.

**Neutras:**

- Git ≥ 2.10 pasa a ser un requisito duro, ya cubierto por RNF-02 (Git ≥ 2.30).

#### Criterios de validación de esta decisión

Se considera validada si, en pruebas con dos cuentas reales:

1. Un `push` desde un repositorio gestionado autentica con la cuenta correcta **teniendo la otra llave cargada en `ssh-agent`**. Es la prueba que falla con una configuración ingenua y la razón de ser de esta decisión.
2. `gh repo view` y `gh pr create` funcionan sin ajustes dentro de un repositorio gestionado.
3. `~/.ssh/config` permanece byte a byte idéntico tras registrar dos cuentas y clonar dos repositorios.
4. `git rodolfo use --clear` deja `.git/config` sin rastro de la herramienta.
5. Una llave con passphrase cargada en el agente no vuelve a pedirla en cada operación.

Si (1) falla, el mecanismo no cumple su función y hay que reabrir esta decisión.

---

## 13. Experiencia principal

### 13.1 Listar cuentas

Comando:

```bash
git rodolfo accounts
```

Resultado esperado:

```text
Git accounts:

  Personal      cristhian@gmail.com        ✓ verified
  Lean Tech     cristhian@leantech.com     ✓ verified
  University    cd@university.edu          ✗ authentication failed

3 accounts. Run "git rodolfo account show <account>" for details.
```

`accounts` es un comando **de solo lectura**. No abre un menú de acciones ni cambia estado. Las operaciones se realizan con sus subcomandos explícitos.

El selector interactivo aparece únicamente cuando un comando necesita elegir una cuenta y no se indicó cuál (`clone`, `use`). En ese caso deberá permitir navegar con las flechas, seleccionar con Enter, buscar escribiendo y cancelar con `Ctrl+C`.

---

### 13.2 Agregar una cuenta

Comando:

```bash
git rodolfo account add
```

La herramienta no creará una cuenta real en GitHub. Registrará localmente una cuenta que el usuario ya haya creado.

Flujo esperado:

```text
Account name: Lean Tech

Provider username: cristhiandelgado-work
Commit name: Cristhian Delgado
Commit email: cristhian@leantech.com

SSH key:
❯ Generate a new key
  Use an existing key
  Configure later
```

**Si el usuario genera una llave:**

```text
SSH key filename: id_ed25519_leantech
Add a passphrase? [y/N]
```

Ejecuta conceptualmente:

```bash
ssh-keygen -t ed25519 -C "cristhian@leantech.com" -f ~/.ssh/id_ed25519_leantech
```

No sobrescribe un archivo existente sin confirmación explícita.

**Si el usuario indica una llave existente:**

```text
Path to the private key: ~/.ssh/id_ed25519_leantech
```

Validaciones:

- El archivo existe, es legible y es una llave privada válida.
- Tiene su llave pública asociada o puede derivarse.
- **No está ya asociada a otra cuenta registrada.** GitHub no admite la misma llave pública en dos cuentas (§4.5), así que la herramienta lo impide y lo explica:

  ```text
  The key ~/.ssh/id_ed25519 is already used by the account "Personal".

  GitHub does not allow the same public key on two accounts.
  Generate a new key for "Lean Tech".
  ```

**Manejo de passphrase.** Si la llave tiene passphrase, la herramienta ofrece cargarla en `ssh-agent`:

```text
This key is protected with a passphrase.

❯ Add it to ssh-agent now (recommended)
  Add it to ssh-agent and remember it in the macOS Keychain
  Skip — I'll enter the passphrase each time
```

Delega en `ssh-add` (con `--apple-use-keychain` en macOS cuando aplique). Git Rodolfo **nunca lee, solicita ni almacena la passphrase**: `ssh-add` la pide directamente al usuario.

**Registro de la llave pública:**

```text
How do you want to register the public key?

❯ Add using GitHub CLI
  Copy the public key to the clipboard
  Configure later
```

Verificación previa obligatoria al usar GitHub CLI: `gh ssh-key add` registra la llave en la cuenta activa de `gh`, que puede no ser la que se está configurando.

```text
GitHub CLI is authenticated as "cristhiandelgado" (Personal),
but you are configuring "cristhiandelgado-work".

❯ Switch GitHub CLI account (gh auth switch)
  Log in with the correct account (gh auth login)
  Copy the public key instead
  Cancel
```

**Prueba final de autenticación:**

```text
Testing authentication...

✓ SSH key accepted
✓ GitHub account detected: cristhiandelgado-work
✓ Account saved
```

Se ejecuta con la llave forzada y el resultado se determina **leyendo la salida**, no el código de salida (§4.5):

```bash
ssh -T -i ~/.ssh/id_ed25519_leantech -o IdentitiesOnly=yes git@github.com
```

Si el usuario detectado no coincide con el registrado, la herramienta lo advierte: es la señal inequívoca de que la llave pertenece a otra cuenta.

**Modo no interactivo:**

```bash
git rodolfo account add \
  --name "Lean Tech" \
  --username cristhiandelgado-work \
  --commit-name "Cristhian Delgado" \
  --commit-email cristhian@leantech.com \
  --key ~/.ssh/id_ed25519_leantech \
  --non-interactive
```

---

### 13.3 Editar una cuenta

Comando:

```bash
git rodolfo account edit
git rodolfo account edit lean-tech
```

El usuario podrá modificar nombre visible, nombre de commit, correo de commit, usuario del proveedor y llave SSH.

Antes de guardar se mostrará un resumen con el antes y el después:

```text
Update account "Lean Tech"?

Commit email:
old-email@leantech.com
→ cristhian@leantech.com

SSH key:
~/.ssh/id_ed25519_old
→ ~/.ssh/id_ed25519_leantech

Confirm? [y/N]
```

Los repositorios existentes no se modificarán automáticamente:

```text
This account is used by repositories configured with Git Rodolfo.

Run "git rodolfo use lean-tech" inside each affected repository
to apply the updated configuration.
```

Si el cambio deja una llave anterior sin usar por ninguna cuenta, la herramienta lo menciona y ofrece borrarla con las mismas advertencias de §13.5. No la borra sin preguntar.

---

### 13.4 Consultar una cuenta

Comando:

```bash
git rodolfo account show lean-tech
```

Resultado esperado:

```text
Account: Lean Tech
Provider: GitHub
Username: cristhiandelgado-work
Commit name: Cristhian Delgado
Commit email: cristhian@leantech.com
SSH key: ~/.ssh/id_ed25519_leantech
Authentication: verified 2 hours ago
```

Reglas de presentación:

- La herramienta nunca deberá mostrar el contenido de la llave privada.
- El estado de autenticación **siempre se muestra junto a su antigüedad** (`lastVerifiedAt`). Un estado sin fecha es un estado que miente: `verified` hace tres meses no significa nada. Si supera los 7 días, se muestra como `not verified recently`.

---

### 13.5 Eliminar una cuenta

Comando:

```bash
git rodolfo account remove
git rodolfo account remove lean-tech
```

Son dos decisiones separadas: borrar el perfil y borrar la llave. La segunda solo se pregunta si la primera se confirma.

**Paso 1 — eliminar el perfil:**

```text
Remove account "Lean Tech" from Git Rodolfo?

This removes the local profile only.
Your GitHub account and your repositories are not affected.

Confirm? [y/N] y

✓ Account removed.
```

**Paso 2 — ofrecer borrar la llave:**

```text
Also delete this account's SSH key files?

  ~/.ssh/id_ed25519_leantech
  ~/.ssh/id_ed25519_leantech.pub

⚠ This cannot be undone.
  Other applications may be using this key — servers, deploys,
  other tools. Any of them would stop working.

  The key is still registered on GitHub. Deleting these files does
  not revoke it there; remove it from GitHub SSH settings as well.

Delete the key files? [y/N] N
```

Reglas:

- La respuesta por defecto es **no**. Nunca se borra la llave sin una confirmación afirmativa explícita.
- Si la llave está asociada a otra cuenta registrada, **no se ofrece borrarla** y se indica el motivo.
- El borrado del perfil y el de la llave son independientes: si el borrado de archivos falla (permisos, archivo en uso), el perfil ya eliminado permanece eliminado y el error se reporta sin dejar estado a medias.
- En modo no interactivo hay que pedirlo explícitamente: `--delete-key`. Sin ese flag, la llave se conserva.

Eliminar una cuenta **no** significa eliminar la cuenta del proveedor, revocar la llave en GitHub ni modificar repositorios existentes. Los repositorios ya configurados seguirán funcionando, porque su `core.sshCommand` apunta al archivo de llave directamente — de ahí la advertencia del paso 2.

---

### 13.6 Clonar un repositorio

Comando:

```bash
git rodolfo clone git@github.com:lean-tech/project.git
```

Flujo:

1. Validar que la URL sea de GitHub y esté soportada.
2. Utilizar la cuenta indicada con `--account` o mostrar el selector.
3. Verificar que la llave SSH exista y sea legible.
4. Ejecutar el clone forzando la identidad:
   ```bash
   git -c core.sshCommand="ssh -i '<key>' -o IdentitiesOnly=yes" clone <url> <dest>
   ```
5. Persistir `core.sshCommand` en la configuración local del repositorio clonado.
6. Configurar `user.name` y `user.email` locales.
7. Registrar `rodolfo.account` en `.git/config`.
8. Mostrar el resumen.

Resultado:

```text
Repository cloned successfully.

Repository: project
Account: Lean Tech
Commit name: Cristhian Delgado
Commit email: cristhian@leantech.com
SSH key: ~/.ssh/id_ed25519_leantech
Remote: git@github.com:lean-tech/project.git
```

Si el clone falla, se aplica RF-21 (§13.9).

También deberá permitirse indicar la cuenta y usar URLs HTTPS:

```bash
git rodolfo clone git@github.com:lean-tech/project.git --account lean-tech
git rodolfo clone https://github.com/lean-tech/project.git
```

**Conversión de HTTPS a SSH.** No es automática y silenciosa: la herramienta pregunta.

```text
This is an HTTPS URL. Git Rodolfo authenticates with SSH.

❯ Clone using SSH (git@github.com:lean-tech/project.git)
  Keep HTTPS — Git Rodolfo will only configure name and email
  Cancel
```

Si el usuario mantiene HTTPS, la autenticación queda a cargo de su credential helper y la herramienta lo advierte en el resumen. Esto importa porque hay redes corporativas que bloquean el puerto 22.

---

### 13.7 Asociar una cuenta a un repositorio existente

Dentro del repositorio:

```bash
git rodolfo use
git rodolfo use lean-tech
```

La herramienta actualizará:

```bash
git config --local user.name "<commit-name>"
git config --local user.email "<commit-email>"
git config --local rodolfo.account "<account-id>"
git config --local core.sshCommand "ssh -i '<key>' -o IdentitiesOnly=yes"
```

**Repositorios sin remoto.** `use` debe funcionar en un repositorio creado con `git init` que todavía no tiene `origin`:

```text
Repository identity updated.

Account: Lean Tech
Commit email: cristhian@leantech.com
Remote: none — identity will apply when you add one.
```

**Repositorios con remoto.** El remoto **no se modifica**: la URL canónica ya funciona porque la llave se fuerza vía `core.sshCommand`. La herramienta solo valida que el host sea `github.com` y advierte si no lo es.

Resultado:

```text
Repository identity updated.

Account: Lean Tech
Commit email: cristhian@leantech.com
SSH key: ~/.ssh/id_ed25519_leantech
Remote: git@github.com:lean-tech/project.git (unchanged)
```

**Revertir.** El principio 6 exige reversibilidad explícita:

```bash
git rodolfo use --clear
```

Elimina `user.name`, `user.email`, `core.sshCommand` y `rodolfo.account` locales. Devuelve el repositorio a su comportamiento por defecto de Git.

---

### 13.8 Consultar la cuenta del repositorio

Comando:

```bash
git rodolfo current
```

Resultado esperado:

```text
Repository: project
Account: Lean Tech
Commit name: Cristhian Delgado
Commit email: cristhian@leantech.com
SSH key: ~/.ssh/id_ed25519_leantech
Remote: git@github.com:lean-tech/project.git

Status: correctly configured
```

Si el repositorio no tiene cuenta asignada, `current` lo dice sin ambigüedad —no hay cuenta global de la que echar mano—:

```text
Repository: project

This repository is not managed by Git Rodolfo.
It will use your global Git identity: cristhian@gmail.com

Run:
git rodolfo use
```

Si existe una inconsistencia:

```text
Warning: The Git email does not match the assigned account.

Expected:
cristhian@leantech.com

Current:
cristhian@gmail.com

Run:
git rodolfo use lean-tech
```

---

### 13.9 Traducción de errores de autenticación

Regla general (principio 8): **los errores de Git y SSH se muestran tal cual**. Un repositorio que no existe, una red caída o un disco lleno ya se explican solos, y reescribirlos sería añadir ruido.

La excepción son los mensajes **ambiguos o engañosos** documentados en §4.5. En esos casos la herramienta añade contexto y conserva el mensaje original.

**Caso principal — `Repository not found`.** GitHub responde lo mismo cuando el repositorio no existe y cuando existe pero la cuenta autenticada no tiene acceso. En el escenario multicuenta la causa suele ser la segunda, y el mensaje afirma la primera.

Cuando `clone`, `fetch` o `pull` fallan con ese error, Git Rodolfo comprueba con qué cuenta se autenticó realmente y muestra:

```text
GitHub says the repository was not found.

This means one of two things:
  - The repository does not exist, or
  - The account you are using does not have access to it.

You authenticated as: cristhiandelgado (Personal)
The repository belongs to: lean-tech

You have an account for that organization: "Lean Tech"

Try:
git rodolfo clone git@github.com:lean-tech/project.git --account lean-tech

Original error:
ERROR: Repository not found.
fatal: Could not read from remote repository.
```

**Casos que no se traducen:**

- `Permission to org/repo.git denied to <usuario>` en `push`: ya nombra al usuario, es claro tal cual.
- `Permission denied (publickey)`: es inequívoco —la llave no está registrada en GitHub— y solo se le añade el comando sugerido.
- Cualquier otro error de Git: pasa sin modificar.

---

### 13.10 Diagnóstico

Comando:

```bash
git rodolfo doctor
```

`doctor` **reporta y sugiere**; no corrige. Cada hallazgo incluye el comando exacto que lo resuelve.

Validaciones del MVP:

| # | Validación |
|---|---|
| 1 | Git y SSH están instalados y cumplen la versión mínima. |
| 2 | Todas las llaves configuradas existen y son legibles. |
| 3 | Los permisos de las llaves privadas son seguros (`600`). |
| 4 | Cada cuenta autentica correctamente contra GitHub y **el usuario devuelto coincide** con `providerUsername`. |
| 5 | Ninguna llave está compartida entre dos cuentas registradas. |
| 6 | El repositorio actual tiene cuenta asignada, y `user.name`, `user.email` y `core.sshCommand` coinciden con ella. |
| 7 | El remoto `origin` apunta a un host soportado. |
| 8 | La variable de entorno `GIT_SSH_COMMAND` no está anulando la configuración del repositorio. |

La validación 8 cubre el único modo de fallo del mecanismo elegido: `GIT_SSH_COMMAND` tiene precedencia sobre `core.sshCommand`, así que si el usuario la tiene exportada, la herramienta queda sin efecto sin dar señales.

Ejemplo:

```text
Git Rodolfo Doctor

✓ Git 2.43 installed
✓ SSH installed
✓ Personal key found (~/.ssh/id_ed25519)
✓ Lean Tech key found (~/.ssh/id_ed25519_leantech)
✓ Personal authenticates as cristhiandelgado
✗ Lean Tech authentication failed
✓ No shared keys between accounts
✓ Current repository matches account "Lean Tech"

1 issue found.

Lean Tech — GitHub rejected the SSH key.
Possible causes:
  - The public key has not been added to GitHub.
  - The organization requires SSO authorization for this key.
Run: git rodolfo account show lean-tech
```

---

## 14. Comandos del MVP

### Administración de cuentas

```bash
git rodolfo accounts
git rodolfo account show <account>
git rodolfo account add
git rodolfo account edit [account]
git rodolfo account remove [account] [--delete-key]
```

### Administración de repositorios

```bash
git rodolfo clone <repository-url> [--account <account>]
git rodolfo use [account] [--clear]
git rodolfo current
```

### Diagnóstico

```bash
git rodolfo doctor
```

### Flags globales

```bash
--non-interactive    # falla en lugar de preguntar; obligatorio en CI y pruebas
--yes                # asume confirmación afirmativa en operaciones no destructivas
--verbose            # muestra los comandos ejecutados
```

`--yes` **no** cubre el borrado de llaves: eso requiere `--delete-key` explícito.

### Ayuda y versión

```bash
git rodolfo help
git rodolfo --help
git rodolfo --version
```

Todos los comandos también deberán funcionar mediante `git-rodolfo <command>`.

---

## 15. Requerimientos funcionales

### RF-01: Registrar cuentas existentes

El sistema debe permitir registrar una cuenta previamente creada en GitHub, asociando cuenta externa, identidad de commits y llave SSH.

### RF-02: Listar cuentas

El sistema debe listar todas las cuentas registradas con su correo y estado de autenticación.

### RF-03: Consultar una cuenta

El sistema debe permitir consultar los detalles y estado de una cuenta sin exponer información sensible. El estado de autenticación debe presentarse siempre junto a su antigüedad.

### RF-04: Editar cuentas

El sistema debe permitir modificar los datos de una cuenta sin eliminarla y crearla nuevamente.

### RF-05: Eliminar cuentas

El sistema debe permitir eliminar el perfil local de una cuenta, previa confirmación, sin afectar la cuenta externa ni los repositorios existentes.

### RF-06: Generar llaves SSH

El sistema debe permitir generar llaves Ed25519. No debe sobrescribir archivos existentes sin confirmación.

### RF-07: Utilizar llaves existentes

El sistema debe permitir asociar una llave existente indicando su ruta, verificando que:

- El archivo exista y sea legible.
- Sea una llave privada válida.
- Tenga una llave pública asociada o pueda derivarse.
- **No esté ya asociada a otra cuenta registrada**, porque GitHub no admite la misma llave pública en dos cuentas (§4.5).

### RF-08: Eliminar la llave junto con la cuenta

Tras eliminar un perfil, el sistema debe **ofrecer** borrar los archivos de llave privada y pública de esa cuenta. Requisitos:

- Confirmación explícita, con `no` como respuesta por defecto.
- Advertencia de que la acción es irreversible y de que otras aplicaciones podrían depender de esa llave.
- Aviso de que la llave sigue registrada en GitHub y debe retirarse allí por separado.
- No se ofrece si la llave está asociada a otra cuenta registrada.
- En modo no interactivo requiere el flag `--delete-key`; `--yes` no basta.

### RF-09: Configurar la identidad SSH del repositorio

El sistema debe forzar la llave por repositorio mediante:

```bash
git config --local core.sshCommand "ssh -i '<key>' -o IdentitiesOnly=yes"
```

### RF-10: No modificar archivos compartidos del sistema

En el modo por defecto, el sistema **no debe escribir en `~/.ssh/config`** ni modificar la URL del remoto.

### RF-11: Gestionar passphrases mediante ssh-agent

Cuando una llave tenga passphrase, el sistema debe ofrecer cargarla en `ssh-agent` (con integración opcional con el Keychain en macOS). El sistema nunca debe leer, solicitar ni almacenar la passphrase.

### RF-12: Ejecución no interactiva

Todo comando debe poder ejecutarse sin terminal interactivo mediante flags. Con `--non-interactive`, la ausencia de un dato obligatorio debe producir un error claro, nunca un valor por defecto silencioso.

### RF-13: Validar autenticación

Después de agregar o editar una cuenta, el sistema debe validar la conexión SSH forzando la llave de la cuenta, determinar el resultado **leyendo la salida** (no el código de salida) y comparar el usuario devuelto con `providerUsername`.

### RF-14: Registrar llaves en GitHub

Cuando GitHub CLI esté disponible, la herramienta podrá registrar la llave pública mediante `gh`, **previa verificación de que la cuenta activa de `gh` coincide con la que se está configurando**. Si no coincide, debe ofrecer cambiarla o dar instrucciones manuales.

### RF-15: Clonar repositorios

El sistema debe poder clonar un repositorio utilizando la cuenta seleccionada, forzando la identidad desde la primera conexión.

### RF-16: Tratar URLs HTTPS

El sistema debe reconocer URLs HTTPS y ofrecer explícitamente convertirlas a SSH o conservarlas. No debe convertirlas de forma silenciosa.

### RF-17: Configurar Git localmente

Al asignar una cuenta, deberá ejecutar `git config --local user.name` y `user.email`. No deberá modificar estos valores globalmente.

### RF-18: Guardar la cuenta del repositorio

El sistema deberá almacenar la cuenta asignada en la configuración local de Git:

```bash
git config --local rodolfo.account lean-tech
```

### RF-19: Operar sin remoto

`use` y `current` deben funcionar en repositorios sin remoto `origin`, configurando y reportando la identidad sin error.

### RF-20: Revertir la configuración de un repositorio

`git rodolfo use --clear` debe eliminar toda la configuración escrita por la herramienta en el repositorio.

### RF-21: Traducir errores ambiguos de autenticación

Los errores de Git y SSH se muestran sin modificar, salvo los ambiguos documentados en §4.5.

Cuando una operación falle con `Repository not found`, el sistema debe:

- Determinar con qué usuario de GitHub se autenticó realmente.
- Explicar que la causa puede ser inexistencia **o** falta de acceso.
- Indicar el propietario del repositorio según la URL.
- Sugerir una cuenta registrada que corresponda a ese propietario, si existe.
- Incluir el mensaje original de Git al final.

### RF-22: Diagnosticar inconsistencias

El sistema deberá detectar diferencias entre cuenta registrada, configuración de Git, llave SSH, remoto y usuario autenticado, incluyendo la presencia de `GIT_SSH_COMMAND` en el entorno. Cada hallazgo debe incluir el comando que lo corrige. `doctor` no aplica correcciones.

### RF-23: Ejecutarse como subcomando Git

Todos los comandos deberán funcionar mediante `git rodolfo <command>`.

### RF-24: Ejecutarse directamente

Todos los comandos deberán funcionar también mediante `git-rodolfo <command>`.

### RF-25: Rechazar proveedores no soportados

El sistema debe rechazar con un mensaje claro cualquier URL o cuenta que no corresponda a GitHub, sin intentar una configuración parcial.

---

## 16. Requerimientos no funcionales

### RNF-01: Seguridad

- No almacenar contenido de llaves privadas, contraseñas, tokens ni passphrases.
- No imprimir secretos.
- **No eliminar llaves privadas sin confirmación explícita del usuario** (RF-08).
- Utilizar permisos seguros.
- Pedir confirmación antes de operaciones destructivas.

### RNF-02: Compatibilidad

- macOS y Linux.
- Git 2.30 o superior.
- OpenSSH.
- GitHub mediante SSH.

### RNF-03: Capacidad de respuesta

Los comandos que solo leen configuración local deben ser perceptiblemente instantáneos. Los que requieren red (`doctor`, validación de autenticación) deben mostrar progreso y aplicar un timeout configurable de 10 segundos por conexión.

### RNF-04: Idempotencia

Ejecutar varias veces una operación no deberá duplicar cuentas ni entradas de configuración.

### RNF-05: Recuperación

Toda operación sobre `config.json` se hará mediante escritura atómica (archivo temporal + `rename`), de modo que una interrupción no deje el archivo corrupto.

### RNF-06: Concurrencia

El acceso de escritura a `config.json` debe protegerse con un lock de archivo, para que dos terminales simultáneos no corrompan la configuración.

### RNF-07: Usabilidad

- Lenguaje claro y errores accionables.
- Navegación con teclado.
- No exigir conocimientos avanzados de SSH.
- Mostrar siempre un resumen antes de cambios importantes.

### RNF-08: Privacidad

El MVP no enviará telemetría ni información del usuario a servicios externos, excepto cuando el usuario solicite una operación explícita contra GitHub.

### RNF-09: Testabilidad

- La lógica de negocio deberá poder probarse sin ejecutar comandos reales de Git o SSH, mediante dobles de prueba de `GitClient`, `SSHClient` y `ProviderClient`.
- Las pruebas de integración usarán repositorios Git locales reales y un contenedor con un servidor SSH, sin depender de GitHub.
- Existirá una suite end-to-end manual, documentada, con dos cuentas de prueba reales, que cubre los criterios de aceptación que exigen un proveedor real (§22).

---

## 17. Persistencia

Ruta en macOS y Linux (respetando `XDG_CONFIG_HOME` cuando esté definido):

```text
~/.config/git-rodolfo/config.json
```

Ejemplo:

```json
{
  "version": 1,
  "settings": {
    "lastUsedAccountId": "lean-tech"
  },
  "accounts": [
    {
      "id": "lean-tech",
      "displayName": "Lean Tech",
      "gitName": "Cristhian Delgado",
      "gitEmail": "cristhian@leantech.com",
      "provider": "github",
      "providerUsername": "cristhiandelgado-work",
      "hostname": "github.com",
      "sshUser": "git",
      "privateKeyPath": "~/.ssh/id_ed25519_leantech",
      "publicKeyPath": "~/.ssh/id_ed25519_leantech.pub",
      "publicKeyFingerprint": "SHA256:0Xk…",
      "authenticationStatus": "verified",
      "lastVerifiedAt": "2026-08-01T20:00:00Z",
      "createdAt": "2026-08-01T20:00:00Z",
      "updatedAt": "2026-08-01T20:00:00Z"
    }
  ]
}
```

Notas:

- `lastUsedAccountId` es únicamente una ayuda de UI para preseleccionar en el menú (§11.2). No es una cuenta activa y nunca determina la configuración de un repositorio.
- `publicKeyFingerprint` permite detectar llaves compartidas entre cuentas (RF-07, RF-08) sin volver a leer los archivos.
- `authenticationStatus` nunca se muestra sin `lastVerifiedAt`.

No se almacenarán contraseñas, tokens de acceso, contenido de llaves privadas, passphrases ni sesiones de GitHub.

---

## 18. Modelo de datos

### Account

```text
id: string
displayName: string
gitName: string
gitEmail: string
provider: string                  // "github" en el MVP
providerUsername: string
hostname: string
sshUser: string
privateKeyPath: string
publicKeyPath: string
publicKeyFingerprint: string
authenticationStatus: enum        // unverified | verified | failed
lastVerifiedAt: datetime | null
createdAt: datetime
updatedAt: datetime
```

### Settings

```text
lastUsedAccountId: string | null
```

### GlobalConfig

```text
version: integer
settings: Settings
accounts: Account[]
```

### RepositoryConfig

Almacenado en `.git/config`:

```text
rodolfo.account: string
user.name: string
user.email: string
core.sshCommand: string
```

---

## 19. Arquitectura propuesta

### 19.1 Lenguaje recomendado

Se recomienda implementar Git Rodolfo en Go: produce un único binario, no requiere runtime, tiene buen soporte multiplataforma y buen rendimiento de inicio, y facilita la distribución mediante Homebrew y releases.

### 19.2 Componentes

```text
CLI Layer
 ├── accounts
 ├── account show / add / edit / remove
 ├── clone
 ├── use
 ├── current
 └── doctor

Application Layer
 ├── AccountService
 ├── IdentityMechanism            // aplica y revierte la identidad del repositorio
 ├── RepositoryIdentityService
 ├── CloneService
 ├── AuthenticationService
 ├── ErrorTranslator              // RF-21
 └── DiagnosticService

Domain Layer
 ├── Account
 ├── Provider
 ├── SSHIdentity
 ├── RepositoryIdentity
 └── ValidationResult

Infrastructure Layer
 ├── GitAdapter
 ├── SSHAdapter
 ├── SSHAgentAdapter
 ├── GitHubCLIAdapter
 ├── FileSystemAdapter
 ├── ProcessRunner
 └── ConfigRepository
```

`IdentityMechanism` aísla la decisión de §12.2: el resto de la aplicación no sabe cómo se aplica la identidad. Esto deja la puerta abierta al modo alias sin tocar nada más.

### 19.3 Interfaces conceptuales

```go
type GitClient interface {
	IsRepository(path string) bool
	CloneWithSSHCommand(remoteURL, destination, sshCommand string) error
	SetLocalConfig(key, value string) error
	GetLocalConfig(key string) (string, error)
	UnsetLocalConfig(key string) error
	GetRemoteURL(remote string) (string, error)
}

type SSHClient interface {
	GenerateKey(path, comment string, withPassphrase bool) error
	ValidateKey(path string) error
	PublicKeyFingerprint(path string) (string, error)
	KeyHasPassphrase(path string) (bool, error)
	TestConnectionWithKey(host, keyPath string) (AuthResult, error)
}

type SSHAgentAdapter interface {
	IsRunning() bool
	IsKeyLoaded(keyPath string) (bool, error)
	AddKey(keyPath string, useKeychain bool) error
}

type IdentityMechanism interface {
	Apply(repoPath string, account Account) error
	Clear(repoPath string) error
	Verify(repoPath string, account Account) (ValidationResult, error)
}

type AccountRepository interface {
	List() ([]Account, error)
	FindByID(id string) (*Account, error)
	FindByFingerprint(fp string) (*Account, error)
	Save(account Account) error
	Update(account Account) error
	Delete(id string) error
}

type ProviderClient interface {
	VerifyAuthentication(account Account) (AuthResult, error)
	ActiveCLIUsername() (string, error)
	RegisterPublicKey(account Account, publicKey string) error
}

type ErrorTranslator interface {
	Translate(gitOutput string, ctx OperationContext) (string, bool)
}
```

`AuthResult` incluye el usuario detectado, para poder compararlo con `providerUsername` (RF-13) y para alimentar la traducción de errores (RF-21).

---

## 20. Casos de uso

### CU-01: Agregar una cuenta laboral

**Dado** que el usuario ya tiene una cuenta creada en GitHub,
**cuando** ejecuta `git rodolfo account add`,
**entonces** puede registrarla, generar o indicar una llave SSH y verificar que la autenticación funcione.

### CU-02: Intentar reutilizar la llave personal

**Dado** que el usuario indica una llave ya asociada a otra cuenta,
**cuando** intenta guardarla,
**entonces** la herramienta lo impide y explica que GitHub no admite la misma llave en dos cuentas.

### CU-03: Editar el correo de una cuenta

**Dado** que una cuenta ya está registrada,
**cuando** ejecuta `git rodolfo account edit`,
**entonces** puede cambiar el correo y la herramienta le indica qué repositorios debe actualizar.

### CU-04: Eliminar un perfil conservando la llave

**Dado** que una cuenta ya no se utiliza,
**cuando** ejecuta `git rodolfo account remove` y responde que no al borrado de llave,
**entonces** se elimina el perfil local y los archivos de llave permanecen intactos.

### CU-05: Eliminar un perfil y su llave

**Dado** que la llave solo servía para esa cuenta,
**cuando** el usuario confirma el borrado tras leer la advertencia,
**entonces** se eliminan los archivos y se le recuerda retirar la llave de GitHub.

### CU-06: Clonar con una cuenta laboral

**Dado** que existen varias cuentas,
**cuando** ejecuta `git rodolfo clone <url>` y selecciona la laboral,
**entonces** el repositorio se clona con la llave laboral, incluso si el agente tiene otras llaves cargadas.

### CU-07: Clonar con la cuenta equivocada

**Dado** que el usuario clona un repositorio privado con una cuenta sin acceso,
**cuando** GitHub responde `Repository not found`,
**entonces** la herramienta explica que puede ser falta de acceso, indica con qué usuario se autenticó y sugiere la cuenta correcta.

### CU-08: Configurar un repositorio existente

**Dado** que un repositorio utiliza una identidad incorrecta,
**cuando** ejecuta `git rodolfo use lean-tech`,
**entonces** se actualizan nombre, correo y llave, sin modificar el remoto.

### CU-09: Configurar un repositorio recién creado

**Dado** un repositorio creado con `git init` y sin remoto,
**cuando** ejecuta `git rodolfo use lean-tech`,
**entonces** se configura la identidad y se informa que aún no hay remoto.

### CU-10: Detectar una inconsistencia

**Dado** que el correo del repositorio no coincide con su cuenta,
**cuando** ejecuta `git rodolfo current`,
**entonces** se muestra una advertencia y una acción correctiva.

### CU-11: Revertir la gestión de un repositorio

**Dado** un repositorio configurado por la herramienta,
**cuando** ejecuta `git rodolfo use --clear`,
**entonces** se elimina toda la configuración aplicada.

### CU-12: Ejecutar como extensión de Git

**Dado** que `git-rodolfo` está instalado en el `PATH`,
**cuando** el usuario ejecuta `git rodolfo accounts`,
**entonces** Git Rodolfo deberá ejecutarse correctamente.

---

## 21. Manejo de errores

Los errores de Git y SSH se muestran tal cual. Solo se traducen los ambiguos (§4.5, RF-21), y siempre conservando el mensaje original.

Ejemplo incorrecto:

```text
Process exited with code 128.
```

Ejemplo correcto:

```text
GitHub rejected the SSH authentication for "Lean Tech".

Possible causes:
- The public key has not been added to GitHub.
- The organization requires SSO authorization for this key.

Run:
git rodolfo doctor
```

### El repositorio no se encontró (error ambiguo — se traduce)

```text
GitHub says the repository was not found.

This means one of two things:
  - The repository does not exist, or
  - The account you are using does not have access to it.

You authenticated as: cristhiandelgado (Personal)
The repository belongs to: lean-tech

You have an account for that organization: "Lean Tech"

Try:
git rodolfo clone git@github.com:lean-tech/project.git --account lean-tech

Original error:
ERROR: Repository not found.
fatal: Could not read from remote repository.
```

### La llave no existe

```text
The SSH key configured for "Lean Tech" was not found.

Expected path:
~/.ssh/id_ed25519_leantech

Run:
git rodolfo account edit lean-tech
```

### La llave ya pertenece a otra cuenta

```text
This key is already used by the account "Personal".

GitHub does not allow the same public key on two accounts.
Generate a new key for "Lean Tech".
```

### La llave autentica como otro usuario

```text
The key authenticated successfully, but as "cristhiandelgado".

You registered this account as "cristhiandelgado-work".
This key belongs to a different GitHub account.
```

### El directorio no es un repositorio

```text
The current directory is not a Git repository.

Run this command inside a repository or use:
git rodolfo clone <repository-url>
```

### La cuenta no existe

```text
Account "lean-tech" was not found.

Run:
git rodolfo accounts
```

### El proveedor no está soportado

```text
Git Rodolfo only supports GitHub in this version.

Remote:
git@gitlab.com:team/project.git

No changes were made.
```

### GitHub CLI está autenticado con otra cuenta

```text
GitHub CLI is authenticated as "cristhiandelgado",
but this account is "cristhiandelgado-work".

Adding the key now would register it on the wrong account.

Run:
gh auth switch
```

---

## 22. Criterios de aceptación

El MVP será considerado funcional cuando:

1. El usuario pueda instalar un único binario.
2. `git-rodolfo` funcione directamente y `git rodolfo` funcione como extensión de Git.
3. El usuario pueda registrar al menos dos cuentas.
4. La herramienta impida asociar la misma llave a dos cuentas.
5. La herramienta pueda generar llaves Ed25519 y usar llaves existentes indicadas por ruta.
6. Una llave con passphrase pueda cargarse en `ssh-agent` desde la herramienta.
7. La validación de autenticación reporte correctamente el éxito pese a que `ssh -T` devuelva código 1, y detecte cuando la llave pertenece a otra cuenta.
8. El usuario pueda editar una cuenta.
9. El usuario pueda eliminar una cuenta conservando su llave.
10. El usuario pueda eliminar una cuenta y sus archivos de llave tras una confirmación explícita, habiendo visto la advertencia.
11. El borrado de llave no ocurra nunca con solo `--yes` ni por defecto.
12. El usuario pueda clonar un repositorio privado con la cuenta seleccionada, **teniendo otras llaves cargadas en `ssh-agent`**.
13. Al clonar con la cuenta equivocada, la herramienta explique que puede ser un problema de acceso y sugiera la cuenta correcta, mostrando el error original.
14. El repositorio clonado utilice el nombre y correo correctos y conserve el remoto canónico.
15. `git rodolfo use` funcione en un repositorio con remoto y en uno creado con `git init` sin remoto.
16. `git rodolfo use --clear` deje el repositorio sin rastro de configuración de la herramienta.
17. `git rodolfo current` detecte inconsistencias y distinga un repositorio no gestionado.
18. `git rodolfo doctor` detecte llaves faltantes, permisos inseguros, llaves compartidas, fallos de autenticación y `GIT_SSH_COMMAND` en el entorno.
19. La herramienta no almacene secretos y no escriba en `~/.ssh/config`.
20. Todos los comandos funcionen con `--non-interactive`.
21. El flujo funcione en macOS y en al menos una distribución Linux.
22. Las funciones principales tengan pruebas automatizadas, con las de integración corriendo sin acceso a GitHub.
23. Los errores incluyan una acción recomendada.

---

## 23. Métricas de validación

El MVP no enviará telemetría. Las métricas se recopilarán mediante pruebas manuales y entrevistas.

### Métrica primaria

**Errores de identidad por semana**, medida antes y después de adoptar la herramienta:

- Commits con el correo equivocado (medibles con `git log --format='%ae'` sobre los repos del usuario).
- Operaciones rechazadas por autenticación incorrecta.

Se toma una línea base de dos semanas antes de instalar la herramienta. Sin línea base la métrica no significa nada.

### Métricas secundarias

- Tiempo para registrar la primera cuenta.
- Tiempo para configurar un repositorio existente.
- Porcentaje de usuarios que completan el onboarding sin documentación externa.
- Número de problemas detectados por `doctor`.
- **Tiempo hasta resolver un fallo de acceso**, comparado con el mensaje crudo de GitHub. Es la medida directa del valor de RF-21.

### Objetivos iniciales

- Registrar una cuenta en menos de tres minutos.
- Configurar un repositorio existente en menos de treinta segundos.
- Que al menos ocho de cada diez usuarios completen el flujo sin documentación externa.
- Reducir los errores de identidad respecto a la línea base.

---

## 24. Riesgos

| Riesgo | Mitigación |
|---|---|
| **Confusión entre identidad y autenticación.** El correo de Git no determina qué llave se usa. | Mostrar por separado nombre, correo, cuenta del proveedor, llave y remoto en `current` y `account show`. |
| **Llave equivocada elegida por `ssh-agent`.** Con varias llaves cargadas, GitHub autentica con la primera reconocida, sin error. | `IdentitiesOnly=yes` en todas las operaciones. Es la razón de fondo de la decisión de §12.2. |
| **`Repository not found` interpretado como "no existe"** cuando en realidad es falta de acceso. | Traducción del error con el usuario autenticado y la cuenta sugerida (RF-21). |
| **`GIT_SSH_COMMAND` en el entorno anula `core.sshCommand`.** Único modo de fallo del mecanismo elegido. | Validación 8 de `doctor`. |
| **Borrado de una llave todavía en uso.** | Confirmación explícita con `no` por defecto, advertencia sobre otras aplicaciones, no se ofrece si otra cuenta la usa, y flag dedicado en modo no interactivo (RF-08). |
| **La misma llave en dos cuentas.** GitHub la rechaza. | Validar por huella antes de guardar (RF-07) y en `doctor`. |
| **`gh` autenticado con otra cuenta** al registrar la llave pública. | Verificar `ActiveCLIUsername()` antes de ejecutar `gh ssh-key add`. |
| **Organizaciones con SSO.** La llave requiere autorización adicional. | Detectar el fallo específico y mostrar instrucciones. |
| **Red que bloquea el puerto 22.** | Detectarlo y sugerir SSH sobre el puerto 443 (`ssh.github.com`). |
| **Edición de cuentas usadas por repositorios.** | Advertir e indicar los repositorios afectados; no modificarlos automáticamente. |
| **Varios remotos.** | Solo se considera `origin`. Con `core.sshCommand` la identidad aplica a todos los remotos del repositorio, lo que reduce el impacto. |
| **Interceptar Git.** | No se hace: se exige el uso explícito de `git rodolfo`. |
| **Escritura concurrente en la configuración.** | Lock de archivo y escritura atómica (RNF-05, RNF-06). |

---

## 25. Roadmap del MVP

### Fase 1 — Configuración base

- Estructura del proyecto.
- Modelo de cuentas y persistencia con escritura atómica y lock.
- `accounts`, `account show`.
- Flags globales.

### Fase 2 — Administración de cuentas

- `account add`: generación de llave e indicación de llave existente.
- Validación por huella (llave duplicada).
- Integración con `ssh-agent`.
- Validación de autenticación con detección de usuario.
- `account edit`.
- `account remove`, incluido el borrado opcional de llave.

### Fase 3 — Integración con repositorios

- `IdentityMechanism` (`core.sshCommand`).
- `clone`, incluido el tratamiento de URLs HTTPS.
- `use`, incluido el caso sin remoto, y `use --clear`.
- `current`.

### Fase 4 — Errores y diagnóstico

- `ErrorTranslator` y el caso `Repository not found`.
- `doctor` con las ocho validaciones.
- Catálogo de errores accionables.

### Fase 5 — Distribución

- Builds para macOS y Linux.
- Releases en GitHub y fórmula de Homebrew.
- Script de instalación.
- Documentación y guía de solución de problemas.

---

## 26. Mejoras posteriores

Ordenadas por valor estimado:

1. **Hook `pre-commit` opt-in** que bloquee commits con identidad incorrecta. Recortado del MVP porque su cobertura real es estrecha: solo protege repositorios que la herramienta ya configuró correctamente, y en esos la identidad ya está bien salvo que algo se desalinee después. Reconsiderar si las pruebas muestran que la desalineación posterior es frecuente.
2. **`doctor --fix`** para correcciones locales seguras: permisos de llaves e identidad del repositorio.
3. **Modo alias** (`--mechanism=ssh-alias`) para quienes necesiten el alias fuera de Git, con backup, bloques delimitados y verificación con `ssh -G`.
4. **Escaneo de `~/.ssh`** para sugerir llaves existentes al registrar una cuenta.
5. **Salida `--json`** para scripting.
6. Autocompletado para Bash, Zsh y Fish.
7. Cambio automático según la carpeta, generando `includeIf`.
8. Importación de configuraciones SSH y bloques existentes.
9. Soporte completo para GitLab y Bitbucket.
10. Soporte para Windows.
11. Hook `pre-push`.
12. Firma de commits.
13. Integración con 1Password y otros agentes SSH.
14. Exportación, importación y sincronización de perfiles.
15. TUI completa.
16. Soporte para varios remotos.

---

## 27. Decisiones finales del MVP

1. El nombre provisional del producto será Git Rodolfo.
2. El ejecutable será `git-rodolfo` y podrá ejecutarse como `git rodolfo`.
3. GitHub será el **único** proveedor soportado.
4. La autenticación será mediante SSH.
5. El MVP administrará cuentas locales, no creará cuentas reales.
6. Se incluirán creación, consulta, edición y eliminación de perfiles.
7. **Al eliminar una cuenta se ofrecerá borrar su llave**, con advertencia y confirmación explícita. No se borra por defecto ni se prohíbe.
8. No se interceptarán comandos estándar de Git.
9. **No habrá hooks en el MVP.** La herramienta configura y diagnostica; no bloquea operaciones de Git.
10. **No existirá una cuenta activa global.** La identidad vive en el repositorio.
11. **El mecanismo de identidad será `core.sshCommand` local al repositorio** (ver §12.2).
12. **No se modificará `~/.ssh/config` ni la URL del remoto.**
13. No se modificarán globalmente `user.name` ni `user.email`.
14. **Los errores de Git se muestran tal cual, salvo los ambiguos**, que se traducen conservando el original.
15. `doctor` reporta y sugiere; no corrige.
16. Todo comando tendrá un modo no interactivo.
17. Los repositorios existentes no se actualizarán automáticamente al editar una cuenta.
18. El lenguaje será Go.
19. Las plataformas iniciales serán macOS y Linux.

---

## 28. Definición de éxito

Git Rodolfo habrá validado su propuesta cuando un desarrollador con dos o más cuentas pueda:

1. Instalar la herramienta.
2. Registrar sus cuentas existentes.
3. Generar o asociar llaves SSH y confirmar que funcionan.
4. Editar perfiles, y eliminarlos decidiendo él mismo qué pasa con la llave.
5. Clonar un repositorio privado con la cuenta correcta, aunque tenga varias llaves en el agente.
6. Crear commits con la identidad correcta.
7. Ejecutar `pull` y `push` con la llave correcta.
8. Cambiar o retirar la cuenta asociada a un proyecto.
9. Entender qué pasó cuando algo falla, sin tener que interpretar un mensaje que dice lo contrario de lo que ocurre.
10. Diagnosticar errores sin editar manualmente archivos de configuración.

El flujo principal deberá completarse sin que el usuario necesite entender en detalle cómo funciona `~/.ssh/config` —y sin que ese archivo se modifique en absoluto.

---

## Anexo A — Historial de cambios

### 0.3 → 0.4 (consolidación en un solo documento)

El PRD y `ADR-0001-mecanismo-de-identidad-ssh.md` eran dos archivos porque la decisión de arquitectura se redactó como ADR independiente durante la revisión de 0.1 a 0.2. En esta versión se fusiona el contenido completo del ADR dentro de §12.2 (contexto del problema, opciones evaluadas con su tabla comparativa, decisión, notas de implementación, consecuencias y criterios de validación), y las referencias cruzadas que apuntaban al archivo externo (§12.2, §24, Anexo A) ahora apuntan a §12.2 del propio PRD. El archivo `ADR-0001-mecanismo-de-identidad-ssh.md` queda retirado; no se pierde contenido, solo se deja de tener dos documentos que mantener sincronizados.

### 0.2 → 0.3 (recorte de alcance)

**Añadido:**

| Cambio | Motivo |
|---|---|
| **Borrado opcional de la llave al eliminar una cuenta** (RF-08, §13.5). | La versión 0.2 lo prohibía. Era paternalista: si la llave solo servía para esa cuenta, dejar basura en `~/.ssh` no protege de nada. Se ofrece con advertencia y confirmación explícita, `no` por defecto y flag dedicado en modo no interactivo. |
| **Traducción de `Repository not found`** (RF-21, §13.9). | GitHub devuelve el mismo error para "no existe" y "no tienes acceso". En el escenario multicuenta la causa suele ser la segunda y el mensaje afirma la primera. Es el único punto donde pasar el error crudo desinforma. |
| **Principio 8: los errores se muestran tal cual salvo los ambiguos.** | Formaliza que la traducción es la excepción, no la norma. |
| **Validación de `GIT_SSH_COMMAND` en `doctor`.** | Único modo de fallo del mecanismo elegido en §12.2. |

**Recortado del MVP** (movido a §26, con orden de prioridad):

- Hook `pre-commit`. Su cobertura real resultó más estrecha de lo estimado: solo protege repositorios que la herramienta ya configuró bien.
- `doctor --fix`. Reportar el comando exacto es suficiente en la primera versión.
- Escaneo de `~/.ssh`. El usuario indica la ruta de su llave.
- Salida `--json`. Se conserva `--non-interactive`, necesario para pruebas.
- Modo alias, ya diferido en 0.2, ahora ordenado dentro de las mejoras posteriores.

### 0.1 → 0.2 (revisión de fondo)

| Cambio | Motivo |
|---|---|
| **Mecanismo por defecto: `core.sshCommand` en lugar de alias SSH + reescritura del remoto.** | Elimina el mayor riesgo técnico y evita romper `gh`, IDE y CI. Ver §12.2. |
| **Eliminada la cuenta activa global (`activeAccountId`).** | Contradecía los principios 2 y 4 y reintroducía estado global invisible. |
| **Alcance reducido a GitHub.** | Bitbucket "genérico" y "otros proveedores experimentales" añadían superficie sin usuarios que la validaran. |
| **`accounts` pasa a ser solo lectura**; se elimina `account list`. | Cumplía tres funciones distintas y duplicaba los subcomandos. |
| Añadidos: validación de llave duplicada, lectura de la salida de `ssh -T`, verificación de la cuenta activa de `gh`, `ssh-agent`, `use` sin remoto, `use --clear`, restricciones del proveedor (§4.5), alternativas existentes (§4.6), lock y escritura atómica, estrategia de pruebas. | Vacíos detectados en la revisión de la 0.1. |
