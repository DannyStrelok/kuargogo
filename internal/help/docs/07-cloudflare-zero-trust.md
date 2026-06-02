# ☁️ Cloudflare Zero Trust & Tunnels

Este documento guía la configuración y el despliegue de **Cloudflare Zero Trust** en tu clúster. Utilizamos túneles de Cloudflare (`cloudflared`) para exponer servicios de forma segura sin abrir puertos en tu router y **Cert-Manager** para la gestión automática de certificados SSL.

---

## 1. Conceptos Clave
- **Sin Puertos Abiertos**: El túnel establece una conexión de salida desde tu clúster hacia Cloudflare. No necesitas DMZ ni Port Forwarding.
- **Zero Trust Access**: Puedes añadir capas de autenticación (Google, GitHub, email) antes de que alguien llegue siquiera a tocar tu servidor.
- **SSL Automático Multi-dominio**: Cloudflare gestiona el cifrado en el borde y `kuargogo` automatiza la creación de certificados **Wildcard** para cada dominio gestionado.

---

## 2. Preparación de Cloudflare (Paso 0)

Para que la automatización funcione, necesitas dos cosas preparadas en el panel de Cloudflare:

### A. Delegar el control (Nameservers)
1. Añade tu dominio (ej: `tu-dominio.com`) en Cloudflare.
2. Cambia los Nameservers en tu registrador por los que te asigne Cloudflare (ej: `ada.ns.cloudflare.com`).
3. Espera a que el dominio aparezca como **Active** en el panel.

### B. Crear API Token (CRÍTICO)
Necesitas un token con permisos para que `kuargogo` pueda crear el túnel y gestionar el DNS:
1. Ve a **My Profile** -> **API Tokens** -> **Create Token**.
2. Usa el template **"Edit zone DNS"** o crea uno personalizado con:
   - **Account** -> **Cloudflare Tunnel** -> **Edit**
   - **Zone** -> **DNS** -> **Edit**
   - **Zone** -> **Zone** -> **Read**
3. Guarda el token generado.

---

## 3. Automatización Total con kuargogo (Recomendado)

`kuargogo` elimina la necesidad de configurar manualmente túneles o registros DNS en el panel web.

### Paso 1: Configurar el Token en la TUI
Navega a `⚙️ Settings` -> `☁️ Cloudflare Configuration` y rellena:
- **Email**: Tu email de Cloudflare.
- **API Token**: El token que creaste en el paso anterior.
- **Primary Domain**: Tu dominio principal.

### Paso 2: Aprovisionamiento Automático
Navega a `Audit & Maintenance` -> `Operations` -> **🌐 Cloudflare Automated Provisioning**.
Este proceso hará todo por ti:
1. Crea el túnel `kgg-<contexto>-tunnel` (ej: `kgg-default-tunnel`) en tu cuenta de Cloudflare.
2. Descarga y guarda el **Tunnel Token** automáticamente en tu configuración.
3. Despliega `cloudflared` y **Cert-Manager** en el clúster.

> [!TIP]
> Una vez completado, el archivo `kuargogo.yaml` se sincronizará automáticamente con el `infra-manager` (si está habilitado), manteniendo a tu bot de Telegram actualizado.

---

## 4. Exponer Servicios (Multi-dominio)

Para que un servicio (ej: Immich) sea accesible desde `https://fotos.tu-dominio.com`:

1. Ve a `Audit & Maintenance` -> `Operations` -> **🚀 Expose Service (Cloudflare)**.
2. Rellena los datos:
   - **Service Name**: Nombre del servicio en K8s (ej: `immich`).
   - **Subdomain**: El subdominio deseado (ej: `fotos`).
   - **Domain**: **kuargogo detectará automáticamente** todos los dominios activos en tu cuenta. Selecciona el que quieras usar.
   
3. **¿Qué sucede por detrás?**
   - El sistema crea la regla de Ingress en el túnel.
   - Sincroniza el registro CNAME en el DNS de Cloudflare.
   - **Automáticamente** asegura un certificado **Wildcard** (`*.tu-dominio.com`) para que HTTPS funcione al instante.

### 4.1. Identificando la URL interna (Target)

Para que el túnel pueda enviar el tráfico al lugar correcto, necesitas la **URL interna** del servicio. En Kubernetes, esto se conoce como FQDN (Fully Qualified Domain Name).

**El patrón estándar es:**
`http://<nombre-servicio>.<namespace>.svc.cluster.local:<puerto>`

**Cómo obtener los datos:**
1. Ejecuta: `kubectl get svc -A` para ver todos los servicios.
2. Localiza tu servicio y anota su **NAME**, **NAMESPACE** y **PORT**.

> [!IMPORTANT]
> **¿Por qué usar el nombre completo?**
> El pod del túnel (`cloudflared`) suele ejecutarse en un namespace separado (ej: `cloudflare-tunnel`). Si usas solo `http://auth-service`, el túnel lo buscará en su propio namespace y fallará. Usar el FQDN garantiza que la resolución DNS funcione desde cualquier parte del clúster.

---

## 5. Resolución Alternativa: .homelab vs Público

- **Acceso Público**: Usa el dominio de Cloudflare. El tráfico viaja seguro por el túnel.
- **Acceso Local**: Usa el sufijo `.homelab` (ej: `immich.homelab`). El tráfico es directo a la VIP del clúster (requiere configuración de DNS local o archivo `hosts`).

---

## 6. Seguridad Adicional (Cloudflare Access)

Como tus servicios ahora son públicos, es vital protegerlos con Cloudflare Access:

1. En el panel de Cloudflare Zero Trust, ve a **Access** -> **Applications**.
2. Haz clic en **Add an Application** -> **Self-hosted**.
3. Configura el dominio (ej: `fotos.tu-dominio.com`).
4. Crea una **Policy** que permita el acceso solo a tu email.
5. Ahora, Cloudflare pedirá autenticación antes de dejar pasar a nadie a tu servidor.
