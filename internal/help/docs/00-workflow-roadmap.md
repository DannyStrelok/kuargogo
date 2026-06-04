# 🗺️ Hoja de Ruta: Flujo de Trabajo

> **Propósito**: Esta guía define la secuencia estratégica para construir, asegurar y escalar tu homelab. Seguir este orden garantiza que cada componente tenga sus dependencias listas antes de ejecutarse.

---

## 🏗️ Fases del Ciclo de Vida

El éxito de una infraestructura automatizada reside en el orden. No intentes construir el tejado (Apps) sin tener los cimientos (Storage).

### Fase 1: Preparación del Terreno (Readiness)
Antes de tocar el clúster, cada nodo debe ser una "isla" estable.
1.  **Instalación manual**: Sigue la [Guía 1](01-hardware-preparation.md) para instalar Debian y configurar BIOS.
2.  **Acceso SSH**: Genera tus llaves y distribúyelas (`kgg ssh-keygen` -> `kgg ssh-copy`).
3.  **Preparación de Discos**: Si tus equipos tienen discos secundarios para datos (8TB, SSDs extras), **este es el momento de formatearlos**.
    *   **Acción**: `kgg storage mount` (TUI: Mount Storage).
    *   **Por qué**: Longhorn necesita encontrar discos limpios y montados para empezar a replicar datos.

### Fase 2: Bootstrap del Clúster (The Cluster)
En lugar de instalar pieza por pieza, usamos la orquestación total.
1.  **Site Deploy**: Usa el comando "Full Site Deploy" (Ansible `site.yml`).
    *   **Acción**: TUI: Cluster Lifecycle -> Full Site Deploy.
    *   **Resultado**: Provisioning -> K3s HA -> Registry Cache -> Longhorn -> Postgres.
2.  **Verificación**:
    *   `kgg node status`: Revisa que los 3 servers estén `Ready`.
    *   `kgg doctor`: Confirma que el hardware no tiene temperaturas altas ni discos llenos.

### Fase 3: Ecosistema de Servicios (The Apps)
Ahora que el clúster es estable, instalamos las herramientas de gestión en este orden:

| Orden | Acción | Comando | Motivo |
|:---|:---|:---|:---|
| **1** | **Ingress & SSL** | `kgg ops cloudflare` | Configura el acceso seguro y tus certificados SSL. |
| **2** | **GitOps Core** | `kgg ops argocd` | La base para desplegar apps de forma declarativa. |
| **3** | **Observabilidad** | `kgg ops observability` | Logs, métricas y alertas para saber qué pasa en tu rack. |
| **4** | **Disaster Recovery** | `kgg ops backup` | Configura Velero (S3) para que nada de lo anterior se pierda. |

---

## 🛠️ Matriz de Comandos Estratégicos

| Qué quieres hacer | Comando | Cuándo |
|:---|:---|:---|
| **Añadir un nuevo nodo** | `kgg bootstrap` | Siempre que metas hardware nuevo al rack. |
| **Actualizar todo el rack** | `kgg ops update` | Semanalmente, para parches de seguridad de Debian. |
| **Desplegar una App** | `kgg app deploy` | Solo después de haber completado la Fase 3. |
| **Mantenimiento físico** | `kgg cluster drain` | Antes de abrir un equipo o cambiarle un disco. |

---

## 💡 Consejos de Arquitecto

*   **No "ensucies" la RPi**: Mantén la Raspberry Pi solo para `infra-manager`. No instales Docker ni herramientas manuales en ella; deja que `kuargogo` la gestione.
*   **Etiquetas de GPU**: Si un nodo tiene una NVIDIA, ponle la etiqueta `gpu=nvidia` en tu `kuargogo.yaml` **antes** de hacer el `site deploy`. El instalador detectará la etiqueta y configurará los drivers automáticamente.
*   **Backups**: No consideres tu homelab "terminado" hasta que `kgg ops backup` reporte éxito. En un homelab, el hardware falla; los backups no.

---

**Siguiente paso recomendado** → [Guía 1: Preparación del Hardware](01-hardware-preparation.md)
