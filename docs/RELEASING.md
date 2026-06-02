# 🚀 kuargogo Release Guide

Este documento describe el proceso paso a paso para publicar una nueva versión de `kuargogo`.

## ✅ Pre-Requisitos

1. Asegúrate de estar en la rama `main`.
2. Asegúrate de tener los últimos cambios: `git pull origin main`.
3. Verifica que tu entorno local esté limpio de errores.

## 🛠️ Proceso de Release

### 1. Auditoría Local (Obligatorio)

Antes de crear cualquier tag, ejecuta el script de auditoría para asegurarte de que todo pasa correctamente (Linter, Tests, Seguridad).

```powershell
.\scripts\audit.ps1
```

> **Nota**: Si ves un aviso de "GCC not found", es seguro ignorarlo en Windows. Lo importante es que veas `✅ Tests Passed`.

### 2. Actualizar Versión (Opcional pero Recomendado)

Aunque GoReleaser inyecta la versión en el build, es buena práctica mantener actualizado el archivo de versión si existe (ej. `internal/version/version.go` o similar) o simplemente asegurarte de que el `CHANGELOG.md` tenga las notas de la nueva versión.

### 3. Crear el Tag

El CI dispara el release automáticamente cuando detecta un tag que empieza por `v`.

```bash
# Ejemplo para la versión 1.2.3
git tag -a v1.2.3 -m "Release v1.2.3: Breve descripción de los cambios"
```

### 4. Publicar el Release

Sube el tag a GitHub para iniciar el workflow de Actions:

```bash
git push origin v1.2.3
```

## 🤖 ¿Qué pasa después?

1. **GitHub Actions** detectará el tag.
2. Se ejecutará el workflow **Release** (`.github/workflows/release.yml`).
3. **GoReleaser**:
    - Compilará los binarios para Windows, Linux y macOS.
    - Generará una "Pre-Release" o "Release" en GitHub.
4. **Instalador de Windows**:
    - Se generará automáticamente el instalador `.exe` (usando Inno Setup) y se adjuntará al Release.

## � Comandos Útiles para Tags

Si quieres saber en qué versión estás o ver el historial de tags:

```bash
# Ver el último tag alcanzable desde el commit actual
git describe --tags --abbrev=0

# Ver el último tag y cuántos commits llevas por delante de él
git describe --tags

# Listar todos los tags existentes
git tag -l

# Ver información de un tag específico
git show v1.2.3
```

---

## �🚨 Solución de Problemas

- **Si el build falla**: Revisa la pestaña "Actions" en GitHub. Borra el tag local y remoto (`git tag -d v1.2.3`, `git push --delete origin v1.2.3`) antes de intentarlo de nuevo.
- **Permisos**: Asegúrate de que los workflows tengan permisos de escritura (ya configurado en `release.yml`).

# 1. Borrar tag local y remoto
git tag -d v1.2.3
git push --delete origin v1.2.3

# 2. Guardar el arreglo de goreleaser
git add .goreleaser.yaml
git commit -m "fix: goreleaser main package path"
git push

# 3. Volver a crear y subir el tag
git tag -a v1.2.3 -m "Release v1.2.3: Node, Provisioning and setup, Network"
git push origin v1.2.3