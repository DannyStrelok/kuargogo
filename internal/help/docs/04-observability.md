# 📊 Guía 4: Observabilidad y Métricas (Multi-Proyecto)

> [!TIP]
> **Hoja de Ruta**: Esta guía corresponde a la **Fase 3 (Ecosistema)** de la [Hoja de Ruta Profesional](00-workflow-roadmap.md).

> **Tiempo estimado**: 10 minutos
> **Prerequisito**: [Guía 3: Clúster y Servicios](03-cluster-and-services.md) completada

Con tu clúster K3s funcionando, el siguiente paso crítico en cualquier entorno profesional es tener **visibilidad total**. `kuargogo` facilita el despliegue de un stack completo de observabilidad basado en **LGTM** (Loki, Grafana, Tempo, Mimir/Prometheus) configurado automáticamente para entornos multi-proyecto.

---

## 🚀 1. Despliegue del Stack

El stack de observabilidad incluye:
- **Prometheus**: Recolecta métricas (Time-series).
- **Grafana**: Visualización dinámica y dashboards.
- **AlertManager**: Gestión y enrutamiento de alertas.
- **Loki & Promtail**: Agregación de logs centralizada.

Para desplegar o actualizar el stack, ejecuta desde tu equipo administrador:

```bash
kgg ops observability
```

Este comando instala el Helm Chart oficial de `kube-prometheus-stack` y `loki-stack` en el namespace `observability`, autoconfigurando Prometheus para descubrir servicios automáticamente.

---

## 🛠️ 2. Integración Multi-Proyecto (Desarrolladores)

Una característica clave de la versión v0.6.0+ es que Prometheus está preconfigurado para **auto-descubrir métricas de cualquier proyecto** sin importar el namespace en el que se despliegue.

Si estás desplegando una aplicación propia o de terceros (por ejemplo, una API en Node.js, Python o Go) y quieres ver sus métricas en Grafana, solo necesitas crear un recurso **`ServiceMonitor`** o **`PodMonitor`**.

### Ejemplo: Añadir métricas de tu proyecto

Imagina que tienes una aplicación llamada `mi-api` desplegada en el namespace `proyectos-web` que expone métricas de Prometheus en el puerto `8080` (ruta `/metrics`).

1. Asegúrate de que tu aplicación tiene un `Service` en Kubernetes que apunte al pod.
2. Crea el siguiente archivo `servicemonitor.yaml` junto a tu proyecto:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: mi-api-monitor
  namespace: proyectos-web  # Puede estar en el namespace de tu app
  labels:
    proyecto: mi-api
spec:
  selector:
    matchLabels:
      app: mi-api # Debe coincidir con las labels del Service de tu app
  endpoints:
  - port: metrics # Nombre del puerto en tu Service
    interval: 15s
    path: /metrics
```

3. Aplícalo en el clúster:
```bash
kubectl apply -f servicemonitor.yaml
```

**Resultado**: ¡Ya está! Prometheus Operator en el namespace `observability` detectará automáticamente este recurso (gracias a la configuración wildcard aplicada por `kgg ops observability`) y comenzará a recolectar (scrape) las métricas cada 15 segundos sin configuración manual adicional.

## 🔗 3. Trazas Distribuidas (Tempo / OTLP)

Además de métricas y logs, el clúster expone un endpoint del protocolo **OpenTelemetry (OTLP)** gracias a Grafana Tempo. Esto te permite recoger *trazas distribuidas* completas de las peticiones que saltan entre múltiples contenedores o servicios.

Si tu aplicación utiliza librerías OTel (OpenTelemetry) u OTLP, puedes enviar las trazas (spans) directamente al servicio interno de Tempo:

- **Endpoint gRPC**: `http://tempo.observability.svc.cluster.local:4317`
- **Endpoint HTTP**: `http://tempo.observability.svc.cluster.local:4318/v1/traces`

Por ejemplo, si utilizas un **OTel Collector** en tu proyecto (como en `clandestino`), puedes configurarlo como exportador:

```yaml
exporters:
  otlp/tempo:
    endpoint: "tempo.observability.svc.cluster.local:4317"
    tls:
      insecure: true
```

Tus trazas se enviarán automáticamente y estarán entrelazadas con los logs en Grafana.

---

## 📈 4. Acceso a Grafana

Grafana viene preinstalado con dashboards útiles para analizar la salud del clúster K3s, uso de red, y estado de los discos.

### Port-Forward (Acceso Local)

Para acceder a Grafana de manera segura desde tu equipo administrador, utiliza `kubectl`:

```bash
kubectl port-forward svc/kube-prometheus-stack-grafana -n observability 3000:80
```

1. Abre tu navegador: [http://localhost:3000](http://localhost:3000)
2. **Usuario**: `admin`
3. **Contraseña**: (Definida en las variables de entorno de Ansible o usando `prom-operator` por defecto; consulta tu configuración si ha sido modificada).

### Dashboards Destacados

- **Kubernetes / Compute Resources / Node**: Muestra CPU, Memoria y Red de cada nodo (HP, Lenovo).
- **Loki**: En la sección de "Explore", selecciona a "Loki" como Data Source para consultar los logs unificados de *cualquier pod* en el sistema usando consultas directas tipo `{app="mi-api"}`.

---

## 🔮 Siguientes pasos

Como parte del **Roadmap v0.6.0+**, se espera que los comandos propios del `kuargogo` (por ejemplo, `kgg node health`) consulten directamente la API de este Prometheus central en lugar de realizar conexiones SSH individuales, unificando los flujos de alerta a través del *Brain* (Raspberry Pi).

Para más detalles, visita o contribuye en el **[ROADMAP.md](../../../docs/ROADMAP.md)**.

---

**Anterior** ← [Guía 3: Clúster y Servicios](03-cluster-and-services.md)  
**Siguiente** → [Guía 5: Configuración de Telegram Bot](05-telegram-setup.md)
