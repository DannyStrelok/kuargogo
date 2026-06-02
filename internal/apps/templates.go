package apps

func GetTemplate(appName string) string {
	switch appName {
	case "immich":
		return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: immich
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: immich
  template:
    metadata:
      labels:
        app: immich
    spec:
      containers:
      - name: immich
        image: ghcr.io/immich-app/immich-server:release
        ports:
        - containerPort: 3001
        volumeMounts:
        - mountPath: /usr/src/app/upload
          name: storage
      volumes:
      - name: storage
        hostPath:
          path: /mnt/storage/immich
---
apiVersion: v1
kind: Service
metadata:
  name: immich-svc
spec:
  type: LoadBalancer
  ports:
  - port: 80
    targetPort: 3001
  selector:
    app: immich`

	case "ollama":
		return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ollama
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ollama
  template:
    metadata:
      labels:
        app: ollama
    spec:
      runtimeClassName: nvidia
      nodeSelector:
        gpu: nvidia
      containers:
      - name: ollama
        image: ollama/ollama:latest
        ports:
        - containerPort: 11434
        resources:
          limits:
            nvidia.com/gpu: 1
---
apiVersion: v1
kind: Service
metadata:
  name: ollama-svc
spec:
  type: LoadBalancer
  ports:
  - port: 11434
    targetPort: 11434
  selector:
    app: ollama`

	case "homeassistant":
		return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: homeassistant
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: homeassistant
  template:
    metadata:
      labels:
        app: homeassistant
    spec:
      hostNetwork: true
      containers:
      - name: homeassistant
        image: ghcr.io/home-assistant/home-assistant:stable
        ports:
        - containerPort: 8123
---
apiVersion: v1
kind: Service
metadata:
  name: ha-svc
spec:
  type: ClusterIP
  ports:
  - port: 8123
    targetPort: 8123
  selector:
    app: homeassistant`

	default:
		return ""
	}
}
