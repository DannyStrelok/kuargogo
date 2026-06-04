# 🤖 Guía 5: Configuración de Telegram Bot

> **Tiempo estimado**: 5 minutos
> **Prerequisito**: Tener una cuenta de Telegram

Para que el **Infrastructure Manager** (Raspberry Pi) pueda enviarte alertas sobre la salud del clúster, temperatura y permitirte controlar los ventiladores de forma remota, necesitas configurar un Bot de Telegram.

---

## 🛠️ 1. Crear el Bot (@BotFather)

El primer paso es registrar un nuevo bot en la plataforma de Telegram para obtener tu `telegram_token`.

1. Abre Telegram y busca al usuario **@BotFather**.
2. Envía el comando `/newbot`.
3. Sigue las instrucciones:
   - Elige un **nombre** para tu bot (ej. `Mi Rack Bot`).
   - Elige un **username** único que termine en `bot` (ej. `mi_rack_pro_bot`).
4. **Copia el API Token**: BotFather te dará un token largo (ej. `123456789:ABCdefGHIjklMNOpqrsTUVwxyZ`). **Este es tu `telegram_token`**.

---

## 🆔 2. Obtener tu ID de Usuario (@userinfobot)

Por seguridad, el bot solo responderá a tus comandos. Necesitas tu ID numérico personal para configurarlo como `telegram_admin_id`.

1. Busca al usuario **@userinfobot** en Telegram.
2. Envía cualquier mensaje o simplemente dale a `/start`.
3. El bot te responderá con tu información. **Copia el número que aparece en `Id`** (ej. `987654321`). **Este es tu `telegram_admin_id`**.

---

## ⚙️ 3. Configurar en kuargogo

Una vez tengas ambos valores, hay dos formas de aplicarlos:

### A. A través del archivo de configuración
Añádelos a tu archivo `kuargogo.yaml` (usualmente en `~/.kuargogo/kuargogo.yaml` o en el directorio raíz del proyecto):

```yaml
telegram:
  bot_token: "TU_TOKEN_AQUI"
  admin_id: 987654321
```

### B. Durante la inicialización (TUI/CLI)
Cuando ejecutes `kgg infra init`, el sistema te solicitará estos valores si no están presentes en la configuración.

---

## 🚀 4. Probar el Bot

1. Una vez desplegada la infraestructura con `kgg infra init`, busca tu bot por su username y dale a `/start`.
2. Prueba los comandos disponibles:
   - `/status`: Ver temperatura y estado actual.
   - `/fan auto`: Poner ventiladores en modo automático.
   - `/incidents`: (Nuevo) Ver los últimos problemas registrados.

> [!TIP]
> Si el bot no responde, asegúrate de que la Raspberry Pi tiene acceso a internet y que el servicio `kgg-agent` está corriendo (`sudo systemctl status kgg-agent`).

---

**Anterior** ← [Guía 4: Observabilidad](04-observability.md)  
**Inicio** → [DEPLOYMENT_GUIDE.md](/docs/DEPLOYMENT_GUIDE)
