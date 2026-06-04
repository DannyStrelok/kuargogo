# 🛠️ Guía 2: Provisioning con kuargogo

> [!TIP]
> **Hoja de Ruta**: Esta guía corresponde a la **Fase 1 (Foundation)** de la [Hoja de Ruta](00-workflow-roadmap.md). Asegúrate de seguir el orden estratégico para evitar errores de almacenamiento.

> **Tiempo estimado**: 30-45 minutos  
> **Prerequisito**: [Guía 1: Preparación del Hardware](01-hardware-preparation.md) completada

Esta guía cubre la instalación del CLI, la configuración inicial, el acceso SSH y el aprovisionamiento automatizado de los nodos. Al finalizar, todos tus nodos estarán preparados para formar el clúster Kubernetes.

---

## 📦 1. Instalar kuargogo

### Opción A: Descarga del binario (Recomendado)

Descarga el binario precompilado para tu sistema desde la página de [Releases](https://github.com/DannyStrelok/kuargogo/releases):

- **Windows**: `kuargogo_windows_amd64.exe` → renombra a `kgg.exe`
- **Linux**: `kuargogo_linux_amd64`
- **macOS**: `kuargogo_darwin_amd64` o `kuargogo_darwin_arm64`

### Opción B: Build desde fuente

```bash
git clone https://github.com/DannyStrelok/kuargogo.git
cd kuargogo
go mod tidy
go build -o kgg.exe ./cmd/kgg
```

### Verificar instalación

```bash
./kgg.exe help
```

Deberías ver la lista de comandos disponibles. Para referencia completa, consulta la [Referencia de Comandos](/docs/COMMANDS).

> [!IMPORTANT]
> Los comandos de provisioning y cluster (`kgg prep`, `kgg cluster init`, `kgg site`, etc.) requieren dependencias en tu PC Administrador (Ansible, K9s). Puedes instalarlas de forma automática ejecutando:
> ```bash
> kgg setup
> ```
> *(En Windows, instalará Python vía winget si es necesario y luego `pip install ansible`. En macOS usará `brew` y en Linux `apt/pacman`).*

---

## ⚙️ 2. Configuración Inicial (`kgg init`)

Ejecuta el asistente de configuración:

```bash
./kgg.exe init
```

El wizard interactivo te guiará para crear el archivo `kuargogo.yaml`. Si ya existe, usa `--force` para volver a ejecutarlo:

```bash
./kgg.exe init --force
```

### Configuración para Alta Disponibilidad

Para un clúster K3s HA necesitas **al menos 3 nodos con rol `master`**. Edita tu `kuargogo.yaml` asegurándote de que la configuración refleje tu infraestructura:

```yaml
nodes:
  - name: "hp-prodesk"
    ip: "192.168.1.101"
    user: "debian"
    role: "master"          # ← Los 3 nodos deben ser "master" para HA
    arch: "amd64"
    mac: "98:90:96:aa:bb:cc"

  - name: "lenovo-1"
    ip: "192.168.1.102"
    user: "debian"
    role: "master"
    arch: "amd64"
    mac: "xx:xx:xx:xx:xx:xx"

  - name: "lenovo-2"
    ip: "192.168.1.103"
    user: "debian"
    role: "master"
    arch: "amd64"
    mac: "yy:yy:yy:yy:yy:yy"
    labels:
      gpu: "nvidia"         # ← Marca el nodo con GPU si aplica

ssh:
  private_key_path: "~/.ssh/kgg_cluster_id"  # Generado por 'kgg init' según nombre del cluster
  port: 22

k3s:
  token: ""                 # Se rellenará automáticamente tras 'kgg cluster init'
```

> [!IMPORTANT]
> Los campos `name` deben coincidir con los hostnames que configuraste durante la instalación de Debian. Los campos `mac` son necesarios para Wake-on-LAN (`kgg pwr on`).

### Verificar la configuración

Puedes comprobar que la configuración es válida con:

```bash
./kgg.exe env
```

Y listar los nodos registrados:

```bash
./kgg.exe inventory
```

Para una vista visual del rack:

```bash
./kgg.exe inventory -v
```

> [!TIP]
> Si prefieres no editar el YAML manualmente, puedes usar `kgg discover` para escanear tu red y añadir nodos automáticamente a la configuración.

---

## 🔑 3. Acceso SSH

`kuargogo` necesita acceso SSH sin contraseña a todos los nodos. Hay dos formas de configurarlo:

### Opción A: Paso a paso (Control total)

**3.1. Generar la clave del clúster:**

```bash
./kgg.exe ssh-keygen
```

Esto genera un par de claves Ed25519 en `~/.ssh/kgg_<cluster>_id` (la ruta configurada en `kuargogo.yaml` durante `kgg init`).

**3.2. Distribuir la clave a cada nodo:**

```bash
./kgg.exe ssh-copy --node 192.168.1.101 --user debian
./kgg.exe ssh-copy --node 192.168.1.102 --user debian
./kgg.exe ssh-copy --node 192.168.1.103 --user debian
```

Te pedirá la contraseña del usuario `debian` de cada nodo. Tras esto, `kuargogo` podrá conectarse sin contraseña.

> [!CAUTION]
> **Paso Obligatorio**: No intentes ejecutar comandos de provisioning (`kgg prep`, `kgg site`, etc.) sin haber completado este paso. Ansible fallará inmediatamente si no encuentra la clave privada configurada en tu `kuargogo.yaml`.

> [!NOTE]
> Si la clave no existe al ejecutar `ssh-copy`, se generará automáticamente (no necesitas ejecutar `ssh-keygen` por separado).

### Opción B: Bootstrap (Todo en uno)

El comando `bootstrap` combina keygen + ssh-copy + provisioning en un solo paso:

```bash
./kgg.exe bootstrap --node 192.168.1.101 --user debian
```

Flags útiles:
- `--skip-provision` — Solo configura SSH, sin provisionar
- `--create-user` — Crea el usuario `kgg-admin` con sudo
- `--tags firewall,kernel` — Ejecuta solo tags específicos de Ansible
- `--password <pass>` — Proporciona la contraseña directamente (si no, te la pedirá)

> [!TIP]
> `kgg bootstrap` es ideal si quieres hacer todo de golpe nodo a nodo. Si prefieres primero configurar SSH en todos y luego provisionar todos, usa la **Opción A** seguida del paso 4.

---

- Creación de usuario `kgg-admin` (opcional)

> [!IMPORTANT]
> **Prerrequisito de Seguridad**: Antes de ejecutar `kgg prep`, debes haber generado e instalado las llaves SSH en el nodo de destino (ver [Sección 3: Acceso SSH](#-3-acceso-ssh)). Si usas Windows, el CLI se encargará de sincronizar estas llaves automáticamente con WSL.

### Ejecutar provisioning

```bash
./kgg.exe prep --node 192.168.1.101
./kgg.exe prep --node 192.168.1.102
./kgg.exe prep --node 192.168.1.103
```

### Flags disponibles

| Flag | Descripción |
|:---|:---|
| `--node <IP>` | IP del nodo a provisionar (obligatorio) |
| `--create-user` | Crea usuario `kgg-admin` con privilegios sudo |
| `--tags <tags>` | Tags Ansible separados por comas (ej: `firewall,kernel`) |

> [!NOTE]
> Internamente, `kgg prep` ejecuta un pre-flight check de SSH antes de lanzar el playbook de Ansible. Si falla, te indicará el problema (clave no encontrada, acceso denegado, etc.).

### Verificación post-provisioning

Tras el provisioning, verás el mensaje:

```
✅ Node is ready for K3s HA cluster with Longhorn storage.
📝 Note: A reboot may be required for kernel modules to fully load.
```

Reinicia cada nodo si es necesario:

```bash
ssh debian@192.168.1.101 "sudo reboot"
```

---

## 🎮 5. GPU Setup (Opcional)

Si uno de tus nodos tiene una **GPU Nvidia**, instala los drivers y el NVIDIA Container Toolkit:

```bash
./kgg.exe setup-gpu --node lenovo-2
```

Esto automatiza:
- Instalación de drivers NVIDIA
- Instalación del NVIDIA Container Toolkit
- Configuración del runtime de contenedores para K3s

> [!IMPORTANT]
> Ejecuta este paso **antes** de unir el nodo GPU al clúster, para que K3s detecte la GPU al arrancar.

---

## 💾 6. Almacenamiento Auxiliar (Opcional)

Si tus nodos tienen un **disco secundario** (NVMe, SSD SATA) que quieres usar para almacenamiento distribuido (Longhorn):

```bash
./kgg.exe storage mount --node hp-master --disk /dev/sdb --mount /mnt/data
```

Deberás indicar:
- **Device Path**: Ruta del disco (ej: `/dev/nvme0n1`, `/dev/sdb`)
- **Mount Point**: Directorio de montaje (ej: `/mnt/data` o `/var/lib/longhorn`)

> [!CAUTION]
> Asegúrate de seleccionar el disco correcto. Este proceso **formatea** el disco indicado. No selecciones la partición del sistema operativo.

---

## 🚀 Atajo: Despliegue Express (`kgg site`)

Si prefieres automatizar todo el proceso (provisioning + GPU + K3s + NFS) en un solo comando:

```bash
./kgg.exe site
```

Esto ejecuta el playbook maestro `site.yml` que orquesta:
1. Provisioning de todos los nodos
2. Setup de GPU en nodos compatibles
3. Inicialización de K3s en el primer master
4. Join del resto de nodos al clúster
5. Configuración de NFS

Se puede filtrar por fases con `--tags`:

```bash
./kgg.exe site --tags k3s,init
```

> [!WARNING]
> Si tienes discos secundarios para Longhorn, ejecuta `kgg storage mount` **antes** de `kgg site`.

---

## ✅ Checklist de Verificación

Antes de continuar con la [Guía 3: Clúster y Servicios](03-cluster-and-services.md), asegúrate de:

- [ ] `kuargogo` instalado y `kgg env` funciona correctamente
- [ ] `kuargogo.yaml` configurado con al menos 3 nodos `master`
- [ ] Acceso SSH sin contraseña a todos los nodos
- [ ] `kgg prep` ejecutado en cada nodo sin errores
- [ ] GPU setup completado (si aplica)
- [ ] Discos secundarios montados (si aplica)

---

## ❓ Solución de Problemas {#solucion-de-problemas}

| Problema | Solución |
|:---|:---|
| `Permission denied (publickey)` | Verifica que la clave SSH se copió correctamente con `kgg ssh-copy`. Comprueba permisos del archivo `~/.ssh/authorized_keys` en el nodo (debe ser `600`). |
| `ansible: command not found` | Instala Ansible: `pip install ansible` (ver sección 1). |
| `Pre-flight check failed` | Ejecuta `kgg node health <IP>` para diagnosticar la conectividad SSH. |
| `Host key verification failed` | Usa `kgg ssh-copy` que acepta automáticamente el host key (TOFU). |

Para más información, consulta la sección de [Solución de Problemas](#solucion-de-problemas).

---

**Anterior** ← [Guía 1: Preparación del Hardware](01-hardware-preparation.md)  
**Siguiente** → [Guía 3: Clúster y Servicios](03-cluster-and-services.md)
