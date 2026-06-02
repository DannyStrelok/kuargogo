# ⛵ Gestión de Proyectos con GitOps y Secretos

Este documento detalla el flujo de trabajo para desplegar aplicaciones de forma declarativa en el clúster homelab de **The Rack** utilizando ArgoCD y manteniendo la seguridad de los datos sensibles.

## 1. Filosofía GitOps en kuargogo

En `kuargogo`, la infraestructura y las aplicaciones se gestionan como código (IaC). Esto significa que la "verdad" de lo que debe estar corriendo en tu clúster reside en tus repositorios de Git.
- **ArgoCD**: Es el motor encargado de vigilar tus repositorios y aplicar los cambios automáticamente en Kubernetes.
- **Declarativo**: Si borras una app de tu `kuargogo.yaml` y sincronizas, esta desaparecerá del clúster físicamente.

## 2. Acceso a Repositorios Privados

Para proyectos reales (proyectos de clientes, código propietario), no usaremos repositorios públicos. ArgoCD necesita credenciales para clonar estos repositorios.

### 🔑 Configuración de Credenciales
Debes generar un **Personal Access Token (PAT)** en tu proveedor (GitHub, GitLab, etc.) con permisos de lectura (`read:repo`).

**Vía CLI**:
```bash
kgg gitops repo add https://github.com/usuario/mi-proyecto-privado.git ghp_tu_token_aqui
```

**Vía TUI**:
Navega a `⛵ GitOps Management` -> `🔑 Private Repository Credentials` -> `➕ Add New Credential`.

> [!NOTE]
> Una vez añadida la credencial, `kgg ops argocd` creará automáticamente un Secret de tipo `repository` en el namespace de ArgoCD para que la autenticación sea transparente.

---

## 3. La Regla de Oro: ¡No subas Secretos a Git!

GitOps exige que todo esté en Git, pero **nunca** debes subir un objeto `Secret` de Kubernetes con contraseñas en texto plano (Base64 no es cifrado).

Para solucionar esto, `kuargogo` instala automáticamente el controlador de **Sealed Secrets (Bitnami)**.

### 🔐 Flujo de Trabajo con Sealed Secrets

Este flujo te permite cifrar tus contraseñas localmente para que solo tu clúster de **The Rack** pueda descifrarlas.

#### Paso 1: Instalar `kubeseal` localmente
Descarga el binario `kubeseal` en tu máquina de desarrollo (Admin PC):
[Release de Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets/releases)

#### Paso 2: Obtener el Certificado Público del Clúster
Necesitas la clave pública de tu clúster para cifrar. Ejecuta:
```bash
kubeseal --fetch-cert \
  --controller-name=sealed-secrets \
  --controller-namespace=kube-system > pub-cert.pem
```

#### Paso 3: Crear y Sellar el Secreto
1. Crea un secreto normal localmente (NO lo subas a Git):
   ```bash
   kubectl create secret generic my-db-pass \
     --from-literal=password=MiPasswordMuySeguro \
     --dry-run=client -o yaml > secret-plain.yaml
   ```

2. Cífralo usando el certificado del clúster:
   ```bash
   kubeseal --format=yaml --cert=pub-cert.pem < secret-plain.yaml > secret-sealed.yaml
   ```

#### Paso 4: Commit y Push
Ahora puedes borrar `secret-plain.yaml` de tu disco de forma segura. El archivo `secret-sealed.yaml` contiene solo datos cifrados y es **seguro subirlo a Git** (incluso en repositorios públicos).

ArgoCD detectará el `SealedSecret`, lo desplegará, y el controlador en el clúster generará el `Secret` original automáticamente en memoria.

---

## 4. Despliegue de un Proyecto Real

1. **Configura el Proyecto**: Usa `kgg gitops project add` para crear un grupo lógico.
2. **Configura la App**: Usa `kgg gitops app add` apuntando al repo Git y la ruta de los manifiestos.
3. **Cifra tus Secretos**: Sigue el flujo de `kubeseal` anterior para cualquier contraseña.
4. **Sincroniza**: Navega al TUI -> `Infrastructure` -> `Deploy ArgoCD GitOps` o ejecuta:
   ```bash
   kgg ops argocd
   ```
5. **Acceso y Contraseña**: 
   - La URL local configurada es **`http://argocd.homelab`**. 
   - Asegúrate de mapear la VIP en tu archivo `hosts` (`192.168.1.200 argocd.homelab`).
   - El sistema extraerá automáticamente la contraseña inicial de administración y la mostrará en el TUI al finalizar el despliegue (`🔑 Initial Admin Password: ...`).

---

## 5. Acceso Remoto Seguro (Cloudflare)

Si tienes configurado el módulo de Cloudflare, puedes exponer el panel de ArgoCD a través de un túnel Zero Trust:
1. Despliega el controlador: `kgg ops cloudflare`.
2. En el panel de Cloudflare, crea un "Public Hostname" apuntando `argocd.tu-dominio.com` al servicio `argocd-server.argocd.svc:443` (usando HTTPS y saltando la verificación TLS en el origen).

---

## 6. 🚢 Kargo: Motor de Promociones y Ciclo de Vida

Mientras que ArgoCD sincroniza el estado de Git con el clúster, **Kargo** es el motor que gestiona cómo una versión específica de tu aplicación (un "Freight") viaja a través de diferentes entornos (Stages) de forma segura.

### 🏗️ Conceptos Fundamentales

1. **Warehouse**: La fuente de la verdad de tus artefactos. Puede vigilar un repositorio Git (buscando tags/commits) o un registro de contenedores (OCI) buscando nuevas versiones de imagen.
2. **Freight**: Un "paquete" virtual que contiene una combinación específica de versiones de artefactos (ej: App v1.2.0 + Config commit a1b2c3d).
3. **Stage**: Un entorno lógico (ej: `uat`, `prod`). Cada stage tiene políticas que definen qué Freight puede aceptar.

### 🧩 Entendiendo los Campos de Configuración

Para que el flujo funcione, es vital entender cómo se relacionan estos campos con tu infraestructura:

*   **Namespace**: Es donde residen los recursos de control de Kargo. No tiene por qué ser el mismo que el de tus aplicaciones.
*   **Project Name**: Agrupa tus Warehouses y Stages. Debe estar vinculado a un `AppProject` de ArgoCD para que Kargo tenga permisos de "escribir" cambios en ese proyecto.
*   **Warehouse Name**: El nombre del "vigilante" encargado de detectar nuevas versiones.
*   **Git Repository (Ops Repo)**: Es tu repositorio de **infraestructura** (donde están tus YAMLs/Kustomize). Kargo realizará commits automáticos en este repo para actualizar las versiones.
*   **Manifests Path**: La subcarpeta dentro del repo de infraestructura donde Kargo debe aplicar los cambios (ej: `manifests/dev`).

### ⚙️ Configuración en kuargogo

Para empezar a usar Kargo, debes inicializar su configuración:

**Vía CLI**:
```bash
kgg kargo init
# O configura campos específicos
kgg kargo set --namespace kargo --project homelab-ops --repo ghcr.io/tu-usuario/tu-repo
```

**Vía TUI**:
Navega a `App & AI Ecosystem` -> `Kargo Promotions` -> `⚙️ Configure Kargo`.

### 🚀 Despliegue y Acceso

1. **Instalación**: Una vez configurado, selecciona `🚀 Deploy Kargo Engine`. 
2. **Credenciales**: Al igual que ArgoCD, `kuargogo` generará una contraseña segura de administración y te la mostrará al finalizar (también quedará guardada en tu configuración de forma segura).
3. **Panel Web**:
   - URL: **`https://kargo.homelab`** (mapea esta URL a la VIP de tu clúster).
   - Usuario: `admin`.

### 🔄 Ciclo de Promoción: ¿Cómo se habla con ArgoCD?

El flujo de "Promoción" funciona así:
1. El **Warehouse** detecta un cambio (nuevo commit o imagen) y crea un **Freight**.
2. Tú **Promocionas** ese Freight a un Stage (ej: `uat`).
3. Kargo localiza el **Manifests Path** en tu **Ops Repo** y actualiza la versión (ej: edita el `kustomization.yaml` automáticamente).
4. **ArgoCD** detecta ese commit en el repo de Ops y despliega la nueva versión en el clúster.

---

## 7. Estrategia Multi-Entorno (Best Practices)

Para proyectos profesionales o complejos, se recomienda seguir una estructura de **un repositorio de Ops con carpetas por entorno**.

### 📂 Estructura Recomendada del Repo de Ops
```text
/mi-repo-ops
  /environments
    /dev   <-- Manifiestos de Desarrollo
    /test  <-- Manifiestos de Testing/UAT
    /prod  <-- Manifiestos de Producción
```

### 🛠️ Configuración Multi-Entorno en kuargogo
Para que Kargo sepa qué carpeta tocar en cada promoción, usa el formato `nombre:ruta` en la configuración de **Stages**:

**Ejemplo de configuración en TUI**:
*   **Stages**: `dev:environments/dev,test:environments/test,prod:environments/prod`

**Ejemplo vía CLI**:
```bash
kgg kargo set --path environments/default # Path base
# Los stages se configuran preferiblemente vía TUI o editando kuargogo.yaml para mayor precisión
```

### 🏁 Resumen del Flujo de Trabajo Unificado

| Paso | Herramienta | Acción | Resultado |
| :--- | :--- | :--- | :--- |
| **1** | **Git (App)** | `git push` código | Nueva imagen generada por tu CI/CD. |
| **2** | **Kargo** | Detecta imagen | Se crea un **Freight** (v1.2.0). |
| **3** | **kuargogo / UI** | `kgg kargo promote dev` | Kargo hace commit en `/environments/dev`. |
| **4** | **ArgoCD** | Auto-Sync | El clúster de **Dev** se actualiza. |
| **5** | **kuargogo / UI** | `kgg kargo promote test` | Kargo hace commit en `/environments/test`. |
| **6** | **ArgoCD** | Auto-Sync | El clúster de **Test** se actualiza. |

> [!IMPORTANT]
> Nunca edites los archivos dentro de `/environments/*` manualmente para cambiar versiones. Deja que **Kargo** sea el único "escritor" en esas carpetas para mantener la trazabilidad total de quién promocionó qué y cuándo.
