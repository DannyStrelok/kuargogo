# 🤖 Automatización CI/CD con GitHub Actions

Este documento explica cómo configurar tus repositorios privados para que los servicios se actualicen automáticamente en tu clúster de **The Rack** cada vez que subes código nuevo.

---

## 1. El Ciclo de Vida del Despliegue

Utilizamos una estrategia de **GitOps Activo**:
1.  **CI (GitHub Actions)**: Compila y sube la imagen de Docker.
2.  **Writeback**: La Action escribe la nueva versión de la imagen en el archivo YAML de tu repositorio.
3.  **Sync (ArgoCD)**: El clúster detecta el cambio en el repositorio y aplica la nueva versión.

---

## 2. Preparación del Repositorio

Necesitas crear un secreto en GitHub para que el Action pueda actualizar tus archivos:
1. Ve a tu repositorio en GitHub -> **Settings** -> **Secrets and variables** -> **Actions**.
2. Crea un **New repository secret**:
   - **Name**: `GITOPS_TOKEN`
   - **Value**: Tu GitHub PAT (Personal Access Token) con permisos de `repo`.

---

## 3. Plantilla de GitHub Action

Crea un archivo en tu repo en `.github/workflows/deploy.yml` con el siguiente contenido. Esta plantilla está optimizada para el flujo de trabajo de `kuargogo`:

```yaml
name: CI/CD Pipeline

on:
  push:
    branches: [ main ]
    tags: [ 'v*' ]

jobs:
  build-and-deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Code
        uses: actions/checkout@v4
        with:
          token: ${{ secrets.GITOPS_TOKEN }}

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to DockerHub (O tu registro privado)
        uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}

      - name: Build and Push Image
        uses: docker/build-push-action@v5
        with:
          push: true
          # Usamos el SHA del commit como tag para asegurar inmutabilidad
          tags: tu-usuario/tu-app:${{ github.sha }}

      - name: Update Kubernetes Manifests
        run: |
          # Instalamos yq para editar el YAML de forma segura
          sudo wget https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64 -O /usr/bin/yq && sudo chmod +x /usr/bin/yq
          
          # Actualizamos la versión de la imagen en el archivo de despliegue
          # Cambia 'k8s/deployment.yaml' por la ruta real de tu archivo
          yq -i '.spec.template.spec.containers[0].image = "tu-usuario/tu-app:" + "${{ github.sha }}"' k8s/deployment.yaml
          
      - name: Commit and Push Changes
        run: |
          git config --global user.name "github-actions[bot]"
          git config --global user.email "github-actions[bot]@users.noreply.github.com"
          git add k8s/deployment.yaml
          git commit -m "chore: update image to ${{ github.sha }} [skip ci]"
          git push
```

---

## 4. Por qué el "Writeback" es fundamental

ArgoCD monitoriza tu repositorio de Git. Si solo compilas la imagen pero no cambias el tag en el YAML, ArgoCD seguirá viendo `mi-app:v1` y no hará nada, incluso si has sobrescrito la imagen en el registro.

Al hacer que GitHub Action **escriba** el nuevo tag (`mi-app:sha-12345`) en el YAML:
1.  **Auditoría**: Sabes exactamente qué versión está corriendo viendo el historial de Git.
2.  **Rollback**: Si algo falla, solo tienes que hacer un `git revert` y el clúster volverá a la versión anterior automáticamente.
3.  **Inmutabilidad**: Evitas los problemas de caché de imágenes asociados al tag `:latest`.

---

## 5. Integración con kuargogo

Asegúrate de que la aplicación esté añadida a tu configuración local para que el clúster la gestione:

```bash
kgg gitops app add --name clandestino-v1 --repo https://github.com/tu-usuario/repo.git --path k8s
```

> [!TIP]
> Si prefieres no usar el SHA y usar tags de versión (como `v1.2.3`), puedes cambiar `${{ github.sha }}` por `${{ github.ref_name }}` en la plantilla anterior.

---

## 6. Enterprise GitOps: Promociones Avanzadas con Kargo

Cuando gestionas entornos profesionales (como `dev`, `staging` y `prod`), la actualización automática directa vía writeback de CI/CD puede ser arriesgada para entornos productivos. Aquí es donde entra **Kargo**, el motor de promociones GitOps empresarial de última generación integrado en **The Rack** y completamente gestionado por `kuargogo`.

Kargo empaqueta tus artefactos (imágenes de contenedor, commits de Git, configuraciones, etc.) en paquetes inmutables llamados **Freight** (Carga) y permite promoverlos de forma controlada, secuencial y segura a través de tus diferentes entornos.

---

### 🏛️ Conceptos Clave de Kargo

Para dominar el flujo GitOps corporativo, es fundamental comprender cómo Kargo modela el pipeline de entrega:

1. **Project (Proyecto)**: Un entorno lógico o límite de seguridad en Kargo (mapeado a un namespace de Kubernetes en el clúster) que agrupa múltiples pipelines, secretos de Git/Registro y configuraciones de despliegue.
2. **Warehouse (Almacén)**: El recolector de artefactos. Monitoriza tus orígenes (ej. GitHub Container Registry o repositorios Git) y, al detectar novedades que cumplen tus reglas, las empaqueta en un **Freight**.
3. **Freight (Carga)**: Un artefacto inmutable y versionado. Representa una combinación específica de imágenes de contenedor y commits de Git. Se desplaza a lo largo del pipeline.
4. **Stage (Etapa)**: Un entorno objetivo (como `dev`, `staging`, `prod`) asociado a un path o rama del repositorio de GitOps.
5. **Promotion (Promoción)**: El proceso de aplicar de forma activa un **Freight** específico en una **Stage**.

---

### 🔒 Integración Completa: Repositorios Privados y la Relación entre ArgoCD, Kargo y kuargogo

Cuando utilizas un repositorio privado de GitOps como **`https://github.com/DannyStrelok/clandestino_ops.git`**, la seguridad y la sincronización de credenciales se vuelven primordiales. `kuargogo` simplifica esta gestión declarando las credenciales de forma segura y distribuyéndolas automáticamente a los controladores del clúster.

#### 🔄 Diagrama de Flujo y Relación de Componentes

El siguiente diagrama detalla cómo interactúan tu código fuente, las imágenes de contenedor, el repositorio privado de configuración `clandestino_ops` y los controladores en tu clúster de **The Rack**:

```mermaid
graph TD
    %% Estilos Modernos
    classDef repository fill:#1f2937,stroke:#4b5563,stroke-width:2px,color:#f3f4f6;
    classDef engine fill:#1e3a8a,stroke:#3b82f6,stroke-width:2px,color:#eff6ff;
    classDef action fill:#065f46,stroke:#10b981,stroke-width:2px,color:#ecfdf5;
    classDef client fill:#7c2d12,stroke:#f97316,stroke-width:2px,color:#fff7ed;

    %% Nodos
    subgraph "1. Ciclo de Código y CI/CD"
        Code[Repo de Aplicación: auth-service] -->|Git Push| GHA[GitHub Actions]
        GHA -->|1. Build & Push Image: 0.0.1| GHCR[(GHCR: auth-service)]
    end

    subgraph "2. Orquestación Local (Tu PC)"
        KGG[kuargogo.yaml + Credentials] -->|2. kgg gitops sync| K8sSec[Secrets en Kubernetes]
    end

    subgraph "3. Repositorio GitOps Privado"
        OpsRepo[(Private Repo: clandestino_ops)]
    end

    subgraph "4. Clúster (The Rack)"
        Kargo[Controlador Kargo]
        ArgoCD[Controlador ArgoCD]
        AppPods[Pods en Ejecución: auth-service]

        K8sSec -->|Acceso Seguro Git| Kargo
        K8sSec -->|Acceso Seguro Git| ArgoCD

        Kargo -->|3. Escucha Tags SemVer| GHCR
        Kargo -->|4. Escribe Image Tag dev/staging/prod| OpsRepo
        ArgoCD -->|5. Lee Manifestos Sincronizados| OpsRepo
        ArgoCD -->|6. Despliega/Actualiza Pods| AppPods
    end

    %% Aplicar Estilos
    class Code,OpsRepo,GHCR repository;
    class Kargo,ArgoCD engine;
    class GHA action;
    class KGG client;
```

#### 🛠️ ¿Cómo se relaciona cada componente?

1. **El Repositorio de Código (`auth-service`)**: Donde se desarrolla la lógica de negocio. Al hacer push, el CI/CD (GitHub Actions) construye la imagen y la publica en un registro OCI (ej. `ghcr.io`). **No contiene archivos de despliegue de Kubernetes.**
2. **El Repositorio de GitOps Privado (`clandestino_ops`)**: Es el cerebro de la infraestructura. Contiene los manifiestos de Kubernetes estructurados por entornos (`dev`, `staging`, `prod`). **Es privado por seguridad para no exponer tu configuración interna.**
3. **El Gestor de Credenciales (`kuargogo`)**: Centraliza los secretos. Al configurar tus credenciales de GitHub en `kuargogo.yaml`, al ejecutar `kgg gitops sync` o `kgg kargo sync`, la CLI genera de forma automática:
   - Secretos tipo `helm.sh/release.v1` o de configuración Git en el namespace `argocd` (para que ArgoCD pueda leer tu repo privado).
   - Secretos de repositorio en el namespace `kargo` (para que Kargo pueda clonar, modificar archivos de manifiestos y realizar *git push* automáticos con la nueva versión promovida).
4. **El Lector de Manifiestos (ArgoCD)**: Monitoriza en modo **solo lectura** el repositorio privado `clandestino_ops` y se asegura de que el estado real del clúster de Kubernetes coincida exactamente con lo declarado en Git.
5. **El Promotor de Versiones (Kargo)**: Actúa como un **escritor automatizado**. Al solicitar una promoción (ej. `kgg kargo promote dev`), Kargo clona `clandestino_ops` usando la credencial autorizada, cambia la imagen en la carpeta correspondiente (`apps/auth-service/environments/dev/kustomization.yaml`), crea un commit de auditoría y lo sube de vuelta a GitHub. Al instante, ArgoCD detecta la modificación y actualiza los Pods en el clúster.

#### 🔑 Configuración Práctica de Credenciales en `kuargogo.yaml`

Para habilitar este flujo con `clandestino_ops`, declara tu repositorio de GitOps en el bloque de credenciales de la siguiente manera:

```yaml
gitops:
  # 1. Configuración de Acceso Seguro a Repositorio Privado
  credentials:
    - url: https://github.com/DannyStrelok/clandestino_ops.git
      username: DannyStrelok
      password: "ghp_tuPersonalAccessTokenDeGitHubAqui" # Al guardar el archivo, kuargogo lo encriptará automáticamente
      
  # 2. Configuración de Pipelines de Kargo apuntando al repositorio privado
  pipelines:
    - name: auth-pipeline
      namespace: kargo
      project: homelab
      warehouse:
        name: auth-warehouse
        repo: ghcr.io/gridsovereign/auth-service
        path: https://github.com/DannyStrelok/clandestino_ops.git  # Tu repo privado
        image_selection_strategy: SemVer
        semver: "^0.0.1"
      stages:
        - name: dev
          path: apps/auth-service/environments/dev
        - name: staging
          path: apps/auth-service/environments/staging
          requires:
            - dev
        - name: prod
          path: apps/auth-service/environments/prod
          requires:
            - staging
```

> [!IMPORTANT]
> Cuando realices `kgg gitops sync` o `kgg kargo sync`, la CLI encriptará el token utilizando la **Master Key** integrada de `kuargogo` (evitando almacenar contraseñas en texto claro en tu disco), y provisionará las credenciales tanto en ArgoCD como en Kargo automáticamente.

---

### ⚙️ El Motor de Kargo: `kargo_engine` y su Propósito en el Clúster

Además de los pipelines y las credenciales de repositorios, `kuargogo` te permite declarar y aprovisionar la configuración base del propio servidor de Kargo en el clúster a través del bloque `kargo_engine` dentro de `gitops`.

#### ¿Para qué sirve `kargo_engine`?

Cuando instalas o actualizas el clúster con Ansible (ej. ejecutando los playbooks de **The Rack** a través de `kgg`), el controlador de Kargo y su interfaz gráfica (Kargo Dashboard) requieren credenciales y claves de seguridad globales. El bloque `kargo_engine` encapsula esta información sensible para la infraestructura:

* **`admin_password` (Contraseña de Administrador)**: Define la clave para el usuario administrador por defecto (`admin`). Durante el despliegue con Ansible, `kuargogo` encripta y transmite este valor para inicializar de forma segura la API.
* **`admin_password_hash` (Hash Bcrypt de la Contraseña)**: Kargo Dashboard requiere el hash Bcrypt de la contraseña para validar los inicios de sesión web. Si especificas este hash, Ansible configurará directamente el acceso seguro.
* **`token_signing_key` (Clave de Firma de Tokens JWT)**: Kargo emite tokens JWT (JSON Web Tokens) para autenticar a los usuarios en su API y su Dashboard. Al definir una clave persistente aquí, evitas que Kargo regenere una firma aleatoria cada vez que el clúster se reinstala o actualiza con Ansible. Esto garantiza que las sesiones activas de los usuarios no se cierren inesperadamente tras un despliegue de infraestructura.

#### Ejemplo de Declaración Completa en `kuargogo.yaml`

A continuación se muestra cómo añadir y estructurar el motor de Kargo junto con las credenciales y los pipelines en tu archivo `kuargogo.yaml`:

```yaml
gitops:
  # 1. Configuración del Motor de Kargo (Nivel Infraestructura)
  kargo_engine:
    admin_password: "MiPasswordSuperSeguro123!"       # kuargogo lo encriptará automáticamente al guardarse
    admin_password_hash: "$2a$10$8C5X1K2..."          # Hash Bcrypt correspondiente
    token_signing_key: "clave-secreta-para-firmar-jwt"  # Clave persistente para tokens, kuargogo la encriptará
    
  # 2. Configuración de Acceso Seguro a Repositorio Privado
  credentials:
    - url: https://github.com/DannyStrelok/clandestino_ops.git
      username: DannyStrelok
      password: "ghp_tuPersonalAccessTokenDeGitHubAqui"
```

---

### 📂 Organización de Proyectos y Pipelines en GitOps

Para mantener un homelab o clúster empresarial ordenado, debes seguir buenas prácticas de diseño y estructuración:

#### 1. Estructura de Proyectos en Kargo
- **Por qué separar por Proyectos**: Kargo aísla los recursos por proyectos lógicos. Si tienes un equipo de frontend y otro de backend, o si tienes servicios compartidos vs aplicaciones de negocio, utiliza proyectos distintos para limitar el acceso mediante RBAC y mantener limpios los tableros.
- **Mapeo**: Por defecto, un proyecto `homelab` gestionará sus stages y pipelines en un namespace asignado en Kubernetes.

#### 2. Estructura del Repositorio de GitOps
Recomendamos una estructura monorepo basada en **Kustomize** o subcarpetas claras. Esto permite que Kargo actualice las etiquetas de imágenes de forma aislada sin afectar a otros entornos:

```text
homelab-gitops/
├── apps/
│   └── auth-service/
│       ├── base/                       # Manifiestos base comunes (Deployments, Services)
│       │   ├── deployment.yaml
│       │   ├── kustomization.yaml
│       │   └── service.yaml
│       └── environments/
│           ├── dev/                    # Kargo actualiza la imagen aquí
│           │   ├── kustomization.yaml
│           │   └── patches.yaml
│           ├── staging/                # Kargo actualiza la imagen aquí tras promover a Staging
│           │   ├── kustomization.yaml
│           │   └── patches.yaml
│           └── prod/                   # Kargo actualiza la imagen aquí tras promover a Prod
│               ├── kustomization.yaml
│               └── patches.yaml
```

En cada una de las subcarpetas de entorno (`environments/dev/kustomization.yaml`), Kargo buscará y reescribirá la etiqueta de la imagen correspondiente usando comandos declarativos.

#### 3. Organización de Pipelines
Cada microservicio independiente con su propio ciclo de vida debe poseer su propio **Pipeline** (`KargoPipeline` en `kuargogo.yaml`) con su propio `Warehouse` dedicado. Esto evita acoplamientos innecesarios (por ejemplo, desplegar `auth-service` no debería obligar a desplegar un servicio de base de datos no relacionado).

---

### 📈 Ejemplo Práctico: Configuración y Ciclo de Vida con `auth-service`

Utilizaremos el contenedor corporativo de autenticación como ejemplo principal:
* **Contenedor base:** `ghcr.io/gridsovereign/auth-service:0.0.1`
* **Repositorio de GitOps:** `https://github.com/gridsovereign/homelab-gitops.git`

#### ⚙️ Paso 1: Declaración en `kuargogo.yaml`

El archivo de configuración de tu `kuargogo` (ubicado usualmente en `~/.kuargogo/kuargogo.yaml`) debe declarar esta arquitectura de manera clara e intuitiva. A continuación se muestra un ejemplo completo que ilustra cómo estructurar pipelines bajo la sección `gitops`:

```yaml
gitops:
  projects:
    - name: homelab
      description: "Proyecto de aplicaciones Core"
  pipelines:
    - name: auth-pipeline
      namespace: kargo
      project: homelab
      warehouse:
        name: auth-warehouse
        repo: ghcr.io/gridsovereign/auth-service
        path: https://github.com/gridsovereign/homelab-gitops.git
        image_selection_strategy: SemVer
        semver: "^0.0.1"  # Regla de Semantic Versioning
      stages:
        - name: dev
          path: apps/auth-service/environments/dev
        - name: staging
          path: apps/auth-service/environments/staging
          requires:
            - dev
        - name: prod
          path: apps/auth-service/environments/prod
          requires:
            - staging
```

> [!NOTE]
> La sección `requires` define la secuencialidad lineal del pipeline. No puedes promover a `staging` si el Freight no ha sido desplegado primero con éxito en `dev`, ni a `prod` sin pasar previamente por `staging`.

---

### 🎯 Entendiendo SemVer (Semantic Versioning) en Kargo

Kargo utiliza políticas SemVer para controlar qué imágenes de contenedor calificarán para la creación de un nuevo **Freight**. Esto evita que imágenes inestables o de pruebas rompan tus entornos estables.

Tomando como punto de partida la versión de inicio **`0.0.1`**, aquí explicamos el comportamiento de Kargo según la restricción (`semver`) que definas en el pipeline:

| Restricción | Ejemplo Aceptado | Ejemplo Rechazado | Explicación Técnica |
| :--- | :--- | :--- | :--- |
| **`^0.0.1`** (Por defecto) | `0.0.2`, `0.0.12`, `0.1.0` | `1.0.0`, `0.0.1-rc1` | Permite cambios menores e incrementos de parches compatibles con la versión principal `0`. Si el CI publica la `0.1.0`, Kargo la procesará. Si publica la `1.0.0` (cambio mayor con posibles breaking changes), **no** la promoverá automáticamente. |
| **`~0.0.1`** | `0.0.2`, `0.0.57` | `0.1.0`, `1.0.0` | Solo permite actualizaciones de parches dentro de la versión menor actual (`0.0.x`). Útil para entornos de soporte a largo plazo (LTS) donde solo deseas bugfixes estrictos. |
| **`0.0.x`** | `0.0.2`, `0.0.15` | `0.1.0`, `0.0.1-alpha` | Comportamiento similar a `~0.0.1`, limitando la selección al patrón wildcard de parche. |
| **`*`** | `0.0.2`, `0.1.5`, `2.4.1` | `unstable-latest` | Permite cualquier versión de contenedor que cumpla el formato de versionado semántico estándar (X.Y.Z). |

#### 🔄 Escenario Real Paso a Paso
1. **Publicación Inicial**: Tu CI compila y sube `ghcr.io/gridsovereign/auth-service:0.0.1`.
2. **Detección**: El `Warehouse` de Kargo lee el registro de imágenes, detecta la `0.0.1` compatible con tu regla `^0.0.1` y genera el primer **Freight** (ej. `auth-pipeline-abc123d`).
3. **Nueva Versión de Parche**: Corriges un bug y subes la versión `0.0.2`. El Warehouse genera inmediatamente un nuevo Freight.
4. **Nueva Versión de Feature**: Implementas una nueva API en la versión `0.1.0`. Como `^0.0.1` permite cambios de versiones menores en versiones principales `0`, Kargo **acepta** la `0.1.0`.
5. **Nueva Versión Mayor**: Rompes la API de login y publicas `1.0.0`. Como `^0.0.1` bloquea versiones mayor a `0` en la posición principal, Kargo **ignora** esta imagen para este pipeline. Deberás actualizar de forma explícita tu `kuargogo.yaml` a `^1.0.0` cuando decidas migrar.

---

### 🛠️ Configuración y Operación Paso a Paso con `kuargogo`

#### Paso 1: Configurar el Pipeline
Tienes dos alternativas para realizar la configuración del pipeline en `kuargogo`:

##### Método A: Interfaz de Usuario de Terminal (TUI)
1. Ejecuta el comando principal:
   ```bash
   kgg
   ```
2. Navega con las flechas hasta **🚢 Kargo Promotions** y pulsa `Enter`.
3. Selecciona **➕ Add New Pipeline** y rellena el formulario con los datos de tu `auth-service` y tu repositorio de GitOps. Las stages secuenciales se enlazarán automáticamente.
4. Confirma y pulsa guardar. La configuración se escribirá de forma transparente en tu `kuargogo.yaml`.

##### Método B: Comandos de CLI Rápidos
Si prefieres automatizar la configuración desde un terminal o script:
```bash
# Inicializa el motor de Kargo si es la primera vez
kgg kargo init

# Declara el pipeline de auth-service en un único comando
kgg kargo set --pipeline auth-pipeline \
  --namespace kargo \
  --project homelab \
  --repo ghcr.io/gridsovereign/auth-service \
  --semver "^0.0.1" \
  --path "https://github.com/gridsovereign/homelab-gitops.git" \
  --stages "dev:apps/auth-service/environments/dev,staging:apps/auth-service/environments/staging,prod:apps/auth-service/environments/prod"
```

#### Paso 2: Sincronizar el Clúster
Asegura que la configuración local declarada en tu `kuargogo.yaml` se aplique y cree los recursos nativos en Kubernetes (`Warehouse`, `Stage`, `Project`):
```bash
kgg kargo sync
```

#### Paso 3: Monitorear y Operar Promociones

##### 1. Listar las Cargas Disponibles (Freight)
Verás todos los paquetes inmutables creados por el Warehouse a partir de las imágenes publicadas:
```bash
kgg kargo freight --pipeline auth-pipeline
```
*Ejemplo de respuesta:*
```text
📦 Available Freight in project 'homelab' for pipeline 'auth-pipeline':
- ID: auth-pipeline-abc123d  (Image: ghcr.io/gridsovereign/auth-service:0.0.1)
- ID: auth-pipeline-xyz987w  (Image: ghcr.io/gridsovereign/auth-service:0.0.2)
```

##### 2. Promover una Carga a DEV
Para desplegar la versión `0.0.2` (`auth-pipeline-xyz987w`) al entorno inicial de desarrollo:
```bash
kgg kargo promote dev auth-pipeline-xyz987w --pipeline auth-pipeline
```

##### 3. Promover Secuencialmente a STAGING
Una vez validada en `dev`, el Freight califica para ser promovido a `staging`:
```bash
kgg kargo promote staging auth-pipeline-xyz987w --pipeline auth-pipeline
```

##### 4. Promover a PRODUCCIÓN
Cuando tu equipo apruebe las pruebas en preproducción, lanza la promoción final a producción:
```bash
kgg kargo promote prod auth-pipeline-xyz987w --pipeline auth-pipeline
```

---

### 🛡️ Mejores Prácticas de CI/CD para Kargo

1. **Evita `:latest` y tags mudables**: Kargo funciona mejor con tags inmutables de SemVer (`v0.0.1`, `0.0.2`).
2. **Usa Tags de Release en CI**: Configura tus GitHub Actions para que compilen y publiquen imágenes con el tag semántico exacto cada vez que crees una *Release* o un tag de Git (`v*`):
   ```yaml
   tags: ghcr.io/gridsovereign/auth-service:${{ github.ref_name }}
   ```
3. **No edites a mano los archivos de entornos**: Permite que sea Kargo quien gestione de forma exclusiva los archivos dentro de `apps/auth-service/environments/*` mediante sus transiciones automatizadas. Esto previene conflictos de sincronización (drift) y mantiene el pipeline 100% predecible.

