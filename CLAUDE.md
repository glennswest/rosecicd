# CLAUDE.md — rosecicd Work Plan

Inherits core rules from parent `../CLAUDE.md`. This file tracks project-specific context and task progress.

---

## Work Plan

### Current Version: `v0.1.2`

### Current Sprint / Active Tasks

- [ ] Create mkube deploy manifest (rosecicd.yaml)
- [ ] Deploy controller to mkube on rose
- [ ] End-to-end test: trigger manual build via web UI
- [ ] Verify build pod lifecycle (create -> build -> push -> cleanup)
- [ ] Verify image appears in local registry after build
- [ ] Test GitHub polling auto-trigger

### In Progress

- [ ] (started 2026-02-23) Deploy to mkube — deploy manifest needed, images already pushed to local registry

### Major Changes / Milestones

- [ ] v1.0.0 — Stable build pipeline with reliable polling, log persistence, and error recovery
- [ ] Webhook support — GitHub webhook triggers instead of polling
- [ ] Build caching — Layer cache across builds for faster rebuilds
- [ ] Multi-arch builds — Support both arm64 and amd64 builds

### Completed

- [x] (2026-02-23) Project scaffolding — go.mod, Makefile, directory structure
- [x] (2026-02-23) Config loading — YAML with env var expansion
- [x] (2026-02-23) Builder binary — clone, buildah bud, buildah push, exit code
- [x] (2026-02-23) Builder container image — Dockerfile.builder with multi-stage build
- [x] (2026-02-23) Build manager — pod creation via mkube API, monitoring, cleanup
- [x] (2026-02-23) Web UI — HTMX dashboard, repos, builds, build detail with log viewer
- [x] (2026-02-23) GitHub polling — seeded SHA on startup, auto-trigger on new commits
- [x] (2026-02-23) Controller container image — Dockerfile with multi-stage build
- [x] (2026-02-23) build.sh — podman build/push script with version bumping
- [x] (2026-02-23) Both images built and pushed to 192.168.200.2:5000

### Release History

| Version | Date | Summary |
|---------|------|---------|
| v0.1.2  | 2026-02-23 | Controller image built and pushed to local registry |
| v0.1.1  | 2026-02-23 | Builder image built and pushed to local registry |
| v0.1.0  | 2026-02-23 | Initial project scaffolding and all source code |

---

## Project Context

### Tech Stack
- Language: Go 1.23
- Framework: net/http (stdlib), html/template, HTMX + Alpine.js
- Build system: podman (multi-stage Dockerfiles), build.sh
- Test framework: go test (stdlib)
- Dependencies: gopkg.in/yaml.v3

### Key Directories
```
cmd/rosecicd/              — Controller entry point
cmd/rosecicd-builder/      — Builder pod entry point
internal/config/           — YAML config loading with env var expansion
internal/builder/          — Build spec, git clone, buildah bud/push
internal/buildmgr/         — Build orchestration, pod lifecycle via mkube API
internal/poller/           — GitHub API polling for new commits
internal/server/           — HTTP server setup
internal/ui/               — HTMX web UI handlers, templates, CSS
internal/ui/templates/     — HTML templates (layout, dashboard, repos, builds)
internal/ui/static/css/    — Dark theme CSS
deploy/                    — Default config.yaml
```

### Build & Test Commands
```bash
# Build both binaries (cross-compile for linux/arm64)
./build.sh

# Build only controller image
./build.sh --controller

# Build only builder image
./build.sh --builder

# Go build/vet/test locally
go build ./...
go vet ./...
go test ./...

# Deploy to mkube
oc --server=http://api.rose1.gt.lo:8082 apply -f rosecicd.yaml
```

### Version Locations
```
VERSION              — X.Y.Z (read by build.sh)
rosecicd.yaml        — image tag in pod spec
```

### Important Patterns & Conventions

- **Two images**: `rosecicd` (controller, long-running) and `rosecicd-builder` (build pods, ephemeral)
- **No env vars in containers**: mkube/RouterOS doesn't pass env vars. Builder reads config from mounted JSON file at `/etc/rosecicd/build-spec.json`
- **buildah flags**: `--isolation chroot --storage-driver vfs` required because RouterOS containers lack privileged mode and overlayfs
- **Local registry is HTTP**: Always use `--tls-verify=false` with podman push to `192.168.200.2:5000`
- **Config uses `${ENV_VAR}` expansion**: Tokens loaded from environment, never hardcoded
- **mkube API**: REST at `http://192.168.200.2:8082`, k8s-style pod specs via `oc apply`

### Known Decisions & Context

- **Why buildah instead of docker/podman in builder?** buildah runs without a daemon and works in unprivileged containers with `--isolation chroot`
- **Why VFS storage driver?** overlayfs requires kernel support not available in RouterOS containers
- **Why mounted volume for build spec?** mkube doesn't support env vars in container specs, so config is passed via host-mounted JSON file
- **Why multi-stage Dockerfiles?** Avoids needing Go toolchain on the build host; podman builds the Go binary inside the container
- **mkube API port**: 8082 (not 8080 as in some config references — verify at deploy time)
- **Builder pod network**: Uses `gt` network for registry access

---
