# 🚀 Guía de Inicio: Proyecto Nuevo desde Cero

Esta guía describe el flujo de trabajo estándar paso a paso para desplegar un nuevo proyecto (compuesto por uno o múltiples microservicios) en tu infraestructura utilizando `kuargogo`, ArgoCD y Kargo.

---

## Fase 1: Preparación de Repositorios (Git)

La arquitectura GitOps requiere una separación clara entre el código de la aplicación y la infraestructura.

### 1. Repositorio(s) de Aplicación (App Repos)
Aquí vivirá el código fuente de tus microservicios (ej. `auth-service`, `chat-service`).
- **Requisito**: Deben contener un `Dockerfile`.
- **CI/CD**: Configura un pipeline (como GitHub Actions) que compile la imagen Docker y la suba a tu registro (ej. `ghcr.io/tu-usuario/auth-service`) cada vez que haya un merge a la rama principal, o cuando crees un Tag.

### 2. Repositorio de Operaciones (Ops Repo)
Aquí vivirán los manifiestos YAML de Kubernetes que dictan cómo se despliega la app.
- **Estructura Recomendada**: Crea un repositorio (ej. `ProjectName-ops`) con carpetas separadas para cada entorno:
  ```text
  /environments
    /dev
      kustomization.yaml
      deployment.yaml
    /test
      kustomization.yaml
    /prod
      kustomization.yaml
  ```
- **Nota**: Al menos debes tener los archivos `kustomization.yaml` base en cada carpeta. Kargo se encargará de inyectar las versiones de las imágenes aquí.

---

## Fase 2: Configuración en kuargogo

Abre la terminal de `kuargogo` (vía TUI o comandos) para configurar el plano de control.

### 1. Configurar Credenciales Git/OCI (Punto Crítico)
Si tu Ops Repo o tus Imágenes Docker son privados, necesitas darle acceso al clúster. Este es uno de los puntos que genera más fricción si no se hace correctamente:

- **TUI**: `⛵ GitOps Management` -> `🔑 Private Repository Credentials`.
- **Token (PAT)**: Necesitas un Personal Access Token de GitHub con permisos `read:packages` (para OCI) y `repo` (para Git).
- **El Registro OCI (Registry)**: Si pones `ghcr.io` aquí, `kuargogo` hará **magia**: creará automáticamente un `ImagePullSecret` en todos los namespaces para que Kubernetes sepa cómo descargar tus imágenes privadas. A su vez, ArgoCD y Kargo usarán este mismo token para leer tus repositorios de código.

### 2. Crear el Proyecto Lógico
Agrupa tus recursos bajo un mismo paraguas:
- **TUI**: `⛵ GitOps Management` -> `📁 Manage GitOps Projects` -> `➕ Add Project`.
- Marca la opción **Managed Environment (ApplicationSet)**. Esto le dirá a ArgoCD que cree automáticamente una "App" por cada entorno (dev, test, prod) leyendo de tu Ops Repo.

### 3. Configurar la Pipeline de Kargo
Aquí definimos cómo viajarán los microservicios por tus entornos.
- **TUI**: `App & AI Ecosystem` -> `Kargo Promotions` -> `⚙️ Configure Kargo`.
- Rellena los datos:
  - **Main Image**: `ghcr.io/tu-usuario/auth-service`
  - **Additional Images**: `ghcr.io/tu-usuario/chat-service` (si tienes múltiples microservicios).
  - **Git Ops Repo URL**: `https://github.com/tu-usuario/ProjectName-ops.git`.
  - **Stages**: Escribe la tubería y las rutas a las carpetas del Ops Repo:
    `dev:environments/dev,test:environments/test,prod:environments/prod`

---

## Fase 3: Gestión de Secretos (El Flujo SealedSecrets)

El mayor cambio de paradigma en GitOps es: **Nunca subas contraseñas (Secretos de Kubernetes en texto plano o Base64) a Git.**

El flujo que debes adoptar es el de **Sealed Secrets (Criptografía Asimétrica)**. Funciona igual que enviar un mensaje encriptado:

1. **La Llave Pública**: Descargas el certificado público de tu clúster (solo sirve para encriptar, no para desencriptar).
   ```bash
   kubeseal --fetch-cert --controller-name=sealed-secrets --controller-namespace=kube-system > pub-cert.pem
   ```
2. **Encriptación Local**: Creas tu secreto localmente en tu PC (ej. la contraseña de base de datos para `auth-service`) y lo "sellas":
   ```bash
   kubeseal --format=yaml --cert=pub-cert.pem < mi-secreto-plano.yaml > SealedSecret.yaml
   ```
3. **Commit a Git**: Subes el archivo `SealedSecret.yaml` a la carpeta `environments/dev` de tu Ops Repo. Es totalmente seguro, aunque el repo fuera público.
4. **La Magia (Desencriptación)**: Cuando ArgoCD sincroniza y aplica el `SealedSecret` en tu clúster, el controlador interno usa su **Llave Privada** para desencriptarlo y crear un `Secret` normal de Kubernetes en memoria, listo para que lo use tu microservicio.

> [!WARNING]
> Si pierdes el clúster y tienes que reconstruirlo desde cero, la Llave Privada cambiará, por lo que tus `SealedSecrets` antiguos en Git ya no servirán. Tendrás que volver a generar los archivos `.yaml` sellados con el nuevo certificado público.

---

## Fase 4: Despliegue y Sincronización

¡Es hora de darle vida a la configuración!

### 1. Sincronizar el Estado GitOps
Esto le enviará toda tu configuración a ArgoCD y a Kargo.
- **TUI**: Selecciona `🚀 Deploy ArgoCD GitOps` y luego `⚙️ Sync Kargo State`.
- **Alternativa CLI**: `kgg ops argocd` y `kgg kargo sync`.

### 2. Lanzar la Primera Imagen
- Ejecuta tu GitHub Action o compila tu imagen de aplicación y súbela a GHCR.
- Kargo (el Warehouse) estará vigilando GHCR. En cuanto vea la nueva imagen, creará automáticamente un **Freight** (Paquete de Carga).

### 3. Promocionar a Dev
- Abre la interfaz web de Kargo (`https://kargo.homelab`).
- Verás tu Freight recién detectado. Haz clic en **Promote** hacia la etapa `dev`.
- **¿Qué pasa por debajo?** Kargo hará un commit en la carpeta `/environments/dev` de tu Ops Repo cambiando la versión de la imagen. ArgoCD detectará el cambio y desplegará la app en el namespace de `dev` en tu clúster.

### 4. Flujo Continuo
A partir de este momento, tu flujo diario será:
1. Programar código -> `git push`.
2. El CI/CD compila la imagen de tu microservicio (ej. `auth-service`) y la sube al registry.
3. Entras a la web de Kargo, ves el nuevo paquete y lo apruebas promocionándolo por tus entornos (`dev` -> `test` -> `prod`). Todo el código de infraestructura se actualiza solo.

---

## ⚠️ Puntos de Fricción y Errores Comunes

Para ahorrarte dolores de cabeza, revisa esta lista cuando algo no funcione a la primera:

1. **"ImagePullBackOff" en los Pods**: 
   - **Causa**: Kubernetes no tiene permisos para descargar tu imagen privada desde GHCR/DockerHub.
   - **Solución**: Asegúrate de que en la TUI configuraste tu credencial con el campo `Registry` relleno (ej. `ghcr.io`). Luego haz un `Deploy ArgoCD GitOps` para que `kuargogo` inyecte el `ImagePullSecret` en el namespace de tu app.
2. **Kargo no detecta nuevas imágenes (No genera Freights)**:
   - **Causa**: O el token PAT de Kargo expiró, o el `Semver Constraint` configurado excluye tu nueva versión (ej. taggeaste `v2.0` pero el semver exigía `^1.0.0`).
3. **ArgoCD marca la aplicación como "OutOfSync" o falla el despliegue**:
   - **Causa**: Pusiste errores de sintaxis YAML en tu carpeta del entorno (ej. en `environments/dev/kustomization.yaml`). Revisa los logs en la UI de ArgoCD.
4. **Microservicio crashea por "Secret Not Found"**:
   - **Causa**: Olvidaste subir el `SealedSecret.yaml`, o le pusiste un nombre diferente al que espera tu `Deployment`. ArgoCD no te avisa si a tu app le falta un secreto hasta que el Pod falla al arrancar.

¡Enhorabuena! Tienes un sistema con calidad empresarial bajo tu control.
