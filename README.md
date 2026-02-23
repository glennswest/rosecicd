# rosecicd

Local CI/CD build controller for mkube. Builds container images from GitHub repos directly on ARM64 hardware, pushing to both the local registry and GHCR.

## Architecture

```
+-----------------------+
|  rosecicd controller  |
|  (Go, web UI :8090)   |
+-----------+-----------+
            |
  mkube REST API (POST /api/v1/.../pods)
            |
+-----------+-----------+
|  Build Pod (ephemeral) |
|  buildah + git         |
|  restartPolicy: Never  |
+-----------+-----------+
            |
+-----------+-----------+
|  clone -> build -> push |
|  -> cleanup -> exit     |
+-----------+-----------+
            |
  +---------+---------+
  v                   v
192.168.200.2:5000   ghcr.io/glennswest/*
(local registry)     (GitHub registry)
```

## Components

| Component | Description | Image |
|-----------|-------------|-------|
| `rosecicd` | Controller with web UI (long-running) | `192.168.200.2:5000/rosecicd:edge` |
| `rosecicd-builder` | Build pod with buildah + git (ephemeral) | `192.168.200.2:5000/rosecicd-builder:edge` |

## Build Flow

1. **Trigger**: Manual via web UI, quick build URL, or automatic GitHub polling
2. Controller creates a build pod via mkube API
3. Build pod reads config from mounted JSON file (`/etc/rosecicd/build-spec.json`)
4. Build pod executes: `git clone` -> `buildah bud` -> `buildah push` -> exit
5. Controller monitors pod status, collects logs, cleans up on completion
6. mkube's image watcher detects new digest and rolling-restarts affected pods

## Quick Start

### Prerequisites

- mkube running on rose with REST API accessible
- `podman` installed on build machine (macOS or Linux)
- `GHCR_TOKEN` environment variable set (GitHub PAT with `packages:write` + repo read)

### Build Images

```bash
# Build and push both images to local registry
./build.sh

# Build only controller
./build.sh --controller

# Build only builder
./build.sh --builder
```

### Deploy to mkube

```bash
oc --server=http://api.rose1.gt.lo:8082 apply -f rosecicd.yaml
```

### Configuration

Edit `deploy/config.yaml` or mount a custom config at `/etc/rosecicd/config.yaml`:

```yaml
server:
  addr: ":8090"

github:
  user: "glennswest"
  token: "${GHCR_TOKEN}"

registry:
  local: "192.168.200.2:5000"
  ghcr: "ghcr.io/glennswest"

mkube:
  apiURL: "http://192.168.200.2:8082"

build:
  builderImage: "192.168.200.2:5000/rosecicd-builder:edge"
  network: "gt"
  cacheDir: "/data/rosecicd/cache"

repos:
  - name: mkube
    url: https://github.com/glennswest/mkube
    branch: main
    dockerfile: Dockerfile
    tags: [edge]
    poll: 5m
```

## Web UI

Access at `http://<host>:8090`

| Page | Description |
|------|-------------|
| Dashboard | Repo cards with last build status, quick build URL input |
| Repos | List of configured repos with build buttons |
| Repo Detail | Build history, config display, manual trigger |
| Builds | All builds sorted by time with status badges |
| Build Detail | Full log viewer with live polling, rebuild button |

## Technical Notes

- **`--isolation chroot`**: RouterOS containers don't support privileged mode or user namespaces. Chroot isolation runs Dockerfile RUN commands in a simple chroot.
- **`--storage-driver vfs`**: No overlayfs support in RouterOS containers. VFS copies directories (slower but works everywhere).
- **No env vars**: mkube/RouterOS doesn't pass env vars to containers. Config is passed via mounted JSON file.
- **Local registry is HTTP**: Use `--tls-verify=false` when pushing to `192.168.200.2:5000`.

## Project Structure

```
rosecicd/
├── cmd/
│   ├── rosecicd/main.go           # Controller entry point
│   └── rosecicd-builder/main.go   # Builder entry point
├── internal/
│   ├── config/config.go           # YAML config with env expansion
│   ├── builder/                   # Clone, buildah, spec model
│   ├── buildmgr/                  # Build orchestration via mkube API
│   ├── poller/poller.go           # GitHub commit polling
│   ├── server/server.go           # HTTP server
│   └── ui/                        # HTMX templates, handlers, CSS
├── deploy/config.yaml             # Default configuration
├── rosecicd.yaml                  # mkube deploy manifest
├── Dockerfile                     # Controller image (multi-stage)
├── Dockerfile.builder             # Builder image (multi-stage)
├── build.sh                       # Build and push script
├── VERSION                        # Current version
└── go.mod
```
