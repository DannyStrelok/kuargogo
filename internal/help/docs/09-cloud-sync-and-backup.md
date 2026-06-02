# ☁️ Cloud Sync & Disaster Recovery (S3-Native)

Este documento explica cómo mantener tu configuración de `kuargogo` sincronizada de forma segura en la nube (Cloudflare R2 o AWS S3) y cómo recuperar todo tu clúster en caso de desastre total.

## 1. Filosofía de Seguridad: E2EE

La sincronización de `kuargogo` utiliza **Cifrado de Extremo a Extremo (End-to-End Encryption - E2EE)**.
- Tus datos se cifran **localmente** en tu máquina antes de ser subidos.
- El proveedor de nube solo recibe un bloque binario cifrado e ininteligible.
- Solo tú, mediante tu **Master Passphrase**, puedes descifrar esos datos.

> [!CAUTION]
> Si pierdes tu **Master Passphrase**, no hay forma humana de recuperar tus backups en la nube. Guárdala en un gestor de contraseñas seguro.

## 2. Proveedores Soportados

`kuargogo` utiliza una arquitectura **S3-Native**, lo que significa que es compatible con cualquier proveedor que soporte la API de S3:

### A. Cloudflare R2 (Recomendado)
- **Ideal para**: Usuarios que ya usan Cloudflare.
- **Ventaja**: Puedes importar los datos de acceso directamente de la configuración de backup de tu clúster (Velero).
- **Región**: Utiliza `auto` para que Cloudflare gestione la ubicación automáticamente.

### B. AWS S3 / Minio
- **Ideal para**: Infraestructuras corporativas o auto-alojadas.
- **Configuración**: Requiere Endpoint URL, Bucket, Access Key, Secret Key y la Región específica.

## 3. Configuración Inicial

1. Navega a `☁️  Cloud Sync & Backup` -> `⚙️  Setup S3 Credentials`.
2. Introduce los datos de tu bucket.
3. Establece una **Master Passphrase** de al menos 8 caracteres. Esta clave se guardará de forma segura en el llavero de tu sistema operativo (OS Keychain).

## 4. Backups y Restauración

### Realizar un Backup (Push):
Navega a `☁️  Cloud Sync & Backup` -> `⬆️  Backup current config`.
Esto serializará todo tu `kuargogo.yaml`, lo cifrará con AES-256-GCM y lo subirá al bucket.

### Restaurar en una máquina nueva (Pull/Disaster Recovery):
1. Instala `kuargogo` en la nueva máquina.
2. Configura las credenciales S3 en `Setup S3 Credentials`.
3. Selecciona `⬇️  Restore from cloud`.
4. Introduce tu **Master Passphrase**.
5. `kuargogo` descargará la configuración y recuperará automáticamente el "Salt" de cifrado de los metadatos del objeto en la nube.

---

## 5. Detalles Técnicos

- **Algoritmo**: AES-256-GCM.
- **Derivación de Clave**: PBKDF2 con SHA-256 y 600,000 iteraciones.
- **Salt**: Se genera un salt aleatorio de 16 bytes la primera vez y se almacena como metadato S3 (`x-amz-meta-salt`) para permitir la recuperación en máquinas nuevas.
- **Objeto**: El archivo se guarda con el nombre `kuargogo-config.yaml.enc`.
