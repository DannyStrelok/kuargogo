# ☸️ Guía 3: Clúster Kubernetes y Servicios

> [!TIP]
> **Hoja de Ruta**: Esta guía corresponde a la **Fase 2 (Bootstrap)** y **Fase 3 (Ecosistema)** de la [Hoja de Ruta](00-workflow-roadmap.md).

> **Tiempo estimado**: 20-30 minutos  
> **Prerequisito**: [Guía 2: Provisioning](02-provisioning.md) completada

Esta guía cubre la creación del clúster K3s en Alta Disponibilidad, almacenamiento distribuido, y la puesta en marcha de servicios vía **Full Site Deploy** (Recomendado) o comandos individuales (Avanzado).

---

## 🚀 1. Despliegue Total del Sitio (Recomendado)

En lugar de instalar pieza por pieza, `kuargogo` permite lanzar la orquestación completa de tu rack en un solo paso. Esto garantiza que el orden de los componentes (K3s, Registry Cache, Longhorn, NFS) sea perfecto.

```bash
./kgg.exe site
```

**Qué sucede internamente:**
1.  **Provisioning**: Asegura el endurecimiento del SO en todos los nodos.
2.  **Foundation**: Monta los manifiestos del Registry Cache y NFS.
3.  **K3s Init**: Levanta el primer nodo master con Kube-VIP.
4.  **K3s Join**: Une al resto de servers y agents al clúster.
5.  **Longhorn**: Despliega el almacenamiento replicado una vez el clúster es estable.

> [!IMPORTANT]
> **Check de Almacenamiento**: Si tus nodos tienen discos secundarios dedicados a Longhorn, recuerda haber ejecutado `kgg pwr mount` (Fase 1) **antes** de lanzar este comando.

---

## ☁️ 2. Inicialización Manual (Alternativa)

Si prefieres mayor control o necesitas depurar un paso específico, puedes usar los comandos individuales:

### 2.1. Requisitos para Alta Disponibilidad

K3s en modo HA utiliza **etcd embebido**, que requiere un número impar de nodos para mantener el quórum:

| Nodos master | Tolerancia a fallos |
|:---|:---|
| 3 | Puede perder 1 nodo |
| 5 | Puede perder 2 nodos |

> [!IMPORTANT]
> `kgg cluster init --ha` validará que tienes al menos 3 nodos con rol `master` en tu `kuargogo.yaml`. Si no los tienes, editalo antes de continuar.

### 1.2. Inicializar el primer nodo (Seed)

```bash
./kgg.exe cluster init --ha
```

Esto instala K3s Server en el **primer nodo master** de tu configuración (en el ejemplo, `hp-prodesk`) con etcd embebido habilitado.

Al finalizar, verás:
```
✅ Master initialized successfully!
🔑 Cluster Token: K10xxxxxxxxxxxxx...
```

> [!NOTE]
> El token se usa internamente para unir los demás nodos. `kuargogo` lo obtiene automáticamente del primer master en los pasos siguientes.

### 1.3. Unir los nodos restantes

```bash
./kgg.exe cluster join --node 192.168.1.102
./kgg.exe cluster join --node 192.168.1.103
```

El CLI detecta automáticamente el rol de cada nodo en la configuración:
- Los nodos con rol `master` se unen como **server** (control plane)
- Los nodos con rol `worker` se unen como **agent** (solo carga de trabajo)

Si el nodo tiene la etiqueta `gpu: nvidia`, recibirás un recordatorio:
```
🎮 GPU node detected. Ensure 'kgg setup-gpu' has been run on this node.
```

---

## ✅ 2. Verificación del Clúster

### 2.1. Estado general

```bash
./kgg.exe node status
```

Muestra la salud general del sistema: estado de K3s y estado del hardware.

### 2.2. Diagnóstico completo

```bash
./kgg.exe doctor
```

Recopila métricas de **todos los nodos** en paralelo usando Ansible: CPU, memoria, temperatura, estado de discos. Si todos los nodos reportan correctamente, tu clúster está operativo.

### 2.3. Consola Kubernetes (opcional)

Si tienes [K9s](https://k9scli.io/) instalado, puedes abrir la consola visual directamente:

```bash
./kgg.exe console
```

Esto lanza K9s con el kubeconfig auto-configurado apuntando a tu clúster.

---

## 💾 3. Almacenamiento Distribuido (Longhorn)

[Longhorn](https://longhorn.io/) proporciona almacenamiento replicado para tus aplicaciones Kubernetes.

### 3.1. Desplegar Longhorn

```bash
./kgg.exe storage init
```

Esto instala Longhorn en el clúster y configura las políticas de almacenamiento (Overprovisioning 100%).

### 3.2. Verificar estado

```bash
./kgg.exe storage status
```

---

## 🛠️ 4. Infrastructure Manager (Control Plane)

Si tienes el Infrastructure Manager (Raspberry Pi), puedes configurar el gestor de clúster:

### 4.1. Provisionar la Raspberry Pi

```bash
./kgg.exe infra init
```

Esto configura automáticamente en la RPi:
- Mosquitto (broker MQTT)
- Servicio de Shutdown
- Agente Python de monitorización

### 4.2. Verificación del Gestor
El servicio `kgg-agent` en la RPi se encarga de la monitorización activa del clúster y de la gestión de energía (WoL). Puedes verificar su estado con `kgg env status`.


---

## 🧠 5. Inteligencia Artificial (Opcional)

Si tienes un nodo con GPU, puedes desplegar un LLM local con Ollama:

### 5.1. Descargar un modelo

```bash
./kgg.exe ai pull llama3
```

El CLI envía la orden al nodo GPU automáticamente.

### 5.2. Chat de prueba

```bash
./kgg.exe ai chat
```

### 5.3. Estado de modelos

```bash
./kgg.exe ai status
```

Muestra los modelos cargados y el uso de VRAM.

---

## 🚀 6. Aplicaciones

Con el clúster funcionando, puedes desplegar servicios:

```bash
./kgg.exe app deploy immich
./kgg.exe app deploy homeassistant
```

---

## 🛡 7. Operaciones Diarias

Comandos útiles para el día a día:

| Acción | Comando |
|:---|:---|
| Encender/apagar nodos remotamente | `kgg pwr on lenovo-1` / `kgg pwr off lenovo-1` |
| Backup del sistema | `kgg app backup` |
| Drenar nodo para mantenimiento | `kgg cluster drain --name lenovo-1` |
| Actualizar paquetes en todos los nodos | `kgg ops update` |
| Ver salud de un nodo (NVMe, CPU, temp) | `kgg node health 192.168.1.101` |
| Desinstalar K3s de un nodo | `kgg cluster reset --node 192.168.1.102` |

---

## 🎉 ¡Felicidades!

Tienes un **Homelab** con:
- ✅ Clúster Kubernetes en Alta Disponibilidad (3 servers)
- ✅ Almacenamiento distribuido con Longhorn
- ✅ Gestión remota completa via `kuargogo`
- ✅ IA local con GPU (si aplica)
- ✅ Control de hardware físico (si aplica)

### Documentación de referencia

| Documento | Descripción |
|:---|:---|
| [COMMANDS.md](/docs/COMMANDS) | Referencia completa de todos los comandos |
| [ARCHITECTURE.md](/docs/ARCHITECTURE) | Arquitectura del sistema |


---

## ❓ Solución de Problemas {#solucion-de-problemas}

| Problema | Solución |
|:---|:---|
| Node en estado `NotReady` | Ejecuta `kgg doctor` para ver métricas. Verifica que K3s está corriendo: `sudo systemctl status k3s`. |
| Pérdida de quórum etcd | Con 3 servers, solo puedes perder 1. Si pierdes 2+, restaura desde backup con `kgg ops backup-system`. |
| Longhorn volume `degraded` | Revisa el estado con `kgg storage status`. Asegúrate de que los nodos target están online y tienen espacio en disco. |
| `INSTALL_K3S_EXEC` errors | Verifica que swap está desactivado (`free -h`) y que los kernel modules están cargados (`lsmod | grep br_netfilter`). |
| Agente / MQTT no responde | Comprueba que Mosquitto está corriendo en la RPi: `sudo systemctl status mosquitto`. Verifica la config en `kuargogo.yaml`. |

Para más información, consulta la sección de [Solución de Problemas](#solucion-de-problemas) y la [Referencia de Comandos](/docs/COMMANDS).

---

**Anterior** ← [Guía 2: Provisioning](02-provisioning.md)  
**Inicio** → [DEPLOYMENT_GUIDE.md](/docs/DEPLOYMENT_GUIDE)
