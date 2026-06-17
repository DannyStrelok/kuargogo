# ☸️ Guía 11: Gestión del Clúster con K9s

> **Tiempo estimado**: 20-30 minutos de lectura y práctica

**K9s** es una interfaz de terminal (TUI) extremadamente potente que simplifica drásticamente la gestión y depuración de clústeres Kubernetes en tiempo real. En lugar de escribir interminables comandos `kubectl`, K9s te permite interactuar con los recursos de tu homelab utilizando atajos de teclado rápidos, inspeccionar logs en tiempo real, abrir consolas y auditar el rendimiento del sistema como un SRE.

Esta guía te proporcionará los conocimientos tácticos indispensables para moverte por tu clúster de K3s con soltura y velocidad.

---

## 🚀 1. Iniciar K9s en Kuargogo

En el entorno de `kuargogo`, puedes lanzar K9s de dos formas:

1. **Desde el CLI principal**:
   ```bash
   .\kgg.exe console
   ```
2. **Desde el Menú TUI**:
   Navega a **Menú Principal > 🚀 K9s Dashboard**.

Kuargogo buscará tu configuración de `kubeconfig` (especificada en `kuargogo.yaml` como `kubeconfig_path`) y establecerá la sesión de terminal interactiva de manera automática.

---

## 🧭 2. Navegación y Vistas de Recursos

El corazón de K9s es el **modo de comandos**. Para navegar a cualquier tipo de recurso en Kubernetes, pulsa **`:`** (dos puntos) y escribe el nombre o la abreviatura del recurso:

### Recursos del Día a Día
* **`:pods`** (o **`:po`**) ➡️ Lista de Pods (vista de contenedores en ejecución).
* **`:services`** (o **`:svc`**) ➡️ Servicios de red interna (balanceadores, IPs internas).
* **`:ingress`** (o **`:ing`**) ➡️Ver los Ingresses (rutas externas).
* **`:deployments`** (o **`:deploy`**) ➡️ Controladores de despliegue de las aplicaciones.
* **`:statefulsets`** (o **`:sts`**) ➡️ Recursos de estado (como bases de datos y colas de mensajería).
* **`:namespaces`** (o **`:ns`**) ➡️ Lista de "proyectos" o espacios lógicos de aislamiento.
* **`:pvc`** ➡️ Reclamaciones de volumen persistente (discos virtuales de Longhorn).
* **`:nodes`** (o **`:no`**) ➡️ Estado de salud física de tus nodos (`lenovo-1`, `lenovo-2`, `hp-800-g4`).
* **`:crd `** ➡️ Ver recursos personalizados (como los de CloudNativePG o Longhorn).

---

## 🎹 3. Flujo de Trabajo Teclado-Only (Atajos Esenciales)

K9s está diseñado para usarse **sin ratón**. Memorizar estos atajos aumentará tu velocidad operativa de forma exponencial:

### Navegación y Jerarquía
* **`Flechas Arriba/Abajo`** ➡️ Moverse por la lista de recursos.
* **`Enter`** ➡️ Profundizar en el recurso. 
  * *Ejemplo:* Si pulsas `Enter` sobre un Pod, verás los contenedores que contiene. Si pulsas `Enter` sobre un contenedor, verás sus puertos y variables de entorno.
* **`Escape`** o **`Backspace`** ➡️ Subir un nivel en la jerarquía (volver atrás).
* **`ctrl+a`** ➡️ Mostrar recursos de **todos los namespaces** a la vez.

### Operaciones en Caliente
* **`l`** (Logs) ➡️ Abre los **logs en tiempo real** del pod seleccionado.
* **`s`** (Shell) ➡️ Abre una **consola interactiva (TTY)** dentro del contenedor. Equivalente a un SSH remoto automático.
* **`y`** (YAML) ➡️ Abre la especificación completa del recurso en YAML estructurado.
* **`e`** (Edit) ➡️ Abre el editor de texto local para modificar el manifiesto en caliente. Al guardar y salir, los cambios se aplican inmediatamente.
* **`ctrl+d`** o **`d`** (Delete) ➡️ Elimina o destruye el recurso de forma inmediata (pidiendo confirmación previa).

### Búsqueda y Filtrado
* **`/`** ➡️ Abre un buscador de coincidencia de texto en la vista actual. Ideal para aislar pods rápidamente escribiendo parte de su nombre (ej: `/redis`).

---

## 🔧 4. Depuración de Problemas y Diagnósticos

Cuando un servicio falla en tu homelab, usa estos patrones para resolver incidencias:

### 1. Pods con Errores (`CrashLoopBackOff`, `Failed`)
1. Selecciona el pod fallido y pulsa **`l`** para ver los logs de error.
2. Si el pod acaba de reiniciarse y los logs actuales están vacíos, pulsa **`p`** dentro de la vista de logs. Esto mostrará el **"Previous Tail"**, es decir, la salida del contenedor justo antes de crashearse.
3. Si los logs pasan muy rápido, pulsa **`f`** (Freeze/Congelar) para pausar la consola y leer con calma.

### 2. Verificar Eventos de Kubernetes
Muchos fallos de arranque no aparecen en los logs de la app (por ejemplo, si falla la programación de volumen o falta memoria).
* Selecciona el pod y escribe **`ctrl+e`** (o describe el recurso) para ver la lista de eventos generada por el planificador de Kubernetes.

### 3. Redirección de Puertos Local (`Port-Forward`)
Si quieres conectarte a una base de datos o servicio web interno sin exponerlo externamente a internet:
1. Selecciona el servicio o pod.
2. Pulsa **`shift+f`** para configurar un port-forward.
3. Define el puerto local en tu PC de trabajo.
4. Para ver y apagar tus redirecciones activas, escribe **`:pf`** en la consola de comandos.

---

## 🦾 5. Funciones SRE y Auditoría

K9s incluye utilidades avanzadas para gestionar y optimizar el rendimiento global del clúster:

### Radiografía de Recursos (`:xray`)
¿Quieres ver cómo están conectadas las piezas de tu sistema?
* Escribe **`:xray pods`** o **`:xray services`** en tu namespace (ej: `clandestino-dev`).
* Te mostrará un mapa gráfico en árbol en tiempo real que asocia pods, PVCs, ConfigMaps y Servicios asociados, facilitando el entendimiento de la arquitectura.

### Análisis de Salud General con Popeye (`:popeye`)
Popeye es un auditor de Kubernetes integrado que evalúa la configuración de tus recursos:
* Escribe **`:popeye`**. 
* Analizará todo el clúster y te dará notas (de A a F) detectando recursos infrautilizados, falta de límites de memoria (que podrían congelar nodos), secrets obsoletos o configuraciones de red inseguras.

### Panel de Estado Simplificado (`:pulse`)
* Escribe **`:pulse`** para obtener un cuadro de mandos con el conteo de recursos en ejecución, errores activos y consumo consolidado de CPU y memoria.

---

## 💾 6. Tips Específicos para CloudNativePG (CNPG)

Como administradores de bases de datos PostgreSQL en el homelab:

* **Inspeccionar Clústeres CNPG**: Escribe **`:clusters.postgresql.cnpg.io`** para ver el estado de reconciliación de tus bases de datos (`clandestino-db`, etc.), cuál es el nodo primario (`rw`) y el estado de la sincronización de réplicas.
* **Archivado de Logs WAL**: Si sospechas que las copias de seguridad de Barman fallan, ve a los pods de base de datos, entra en logs (`l`), selecciona el contenedor `postgres` y busca líneas relacionadas con `archiver` o `barman-cloud-wal-archive` para diagnosticar fallos en las credenciales S3/R2.
