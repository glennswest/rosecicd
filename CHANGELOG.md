# Changelog

## [v0.2.0] — 2026-02-23

### Added
- Build queue system — one build at a time per builder, FIFO ordering
- Builder interface with pluggable backends (mkube pods, SSH remote)
- SSH builder for x86 LXC build agent (`buildx86.gw.lo`)
- MkubeBuilder adapter wrapping existing pod lifecycle
- Builders page in web UI showing status, arch, current build, queue depth
- Builder and queue position columns in builds list
- Builder name in build detail metadata
- Builder status summary on dashboard
- `BuilderConfig` in config with name, type, arch, host, SSH key path, build dir
- `Arch` field on repo config for builder routing
- SSH key volume mount in pod spec for remote builder auth
- `golang.org/x/crypto/ssh` dependency

### Changed
- Build manager routes builds to appropriate builder by architecture match
- Builds are queued instead of spawned as unbounded goroutines
- Build status now includes `queued` state with queue position

## [Unreleased]

### 2026-02-23
- **fix:** Move default config to /usr/share/rosecicd/ to avoid volume mount overlay
- **fix:** Add config file fallback in controller (checks /etc/rosecicd/ then /usr/share/rosecicd/)
- **feat:** Deploy controller to mkube — running at 192.168.200.13:8090
- **docs:** Add CLAUDE.md work plan, README.md, CHANGELOG.md
- **chore:** Update .gitignore with comprehensive ignore patterns
- **feat:** Add mkube deploy manifest (rosecicd.yaml)

## [v0.1.2] — 2026-02-23

### Added
- Controller container image built and pushed to local registry (`192.168.200.2:5000/rosecicd:edge`)

## [v0.1.1] — 2026-02-23

### Added
- Builder container image built and pushed to local registry (`192.168.200.2:5000/rosecicd-builder:edge`)
- `build.sh` script for podman-based builds with auto version bumping
- Multi-stage Dockerfiles for both controller and builder (Go compiled inside container)

## [v0.1.0] — 2026-02-23

### Added
- Initial project scaffolding (go.mod, Makefile, directory structure)
- Config loading with YAML parsing and `${ENV_VAR}` expansion
- Builder binary (`rosecicd-builder`): git clone, buildah bud, buildah push
- Build spec JSON model for passing config to builder pods via mounted volume
- Build manager: pod creation/monitoring/cleanup via mkube REST API
- Web UI with HTMX: dashboard, repos list, repo detail, builds list, build detail with log viewer
- Dark theme CSS matching fastregistry design
- GitHub polling: checks for new commits, auto-triggers builds on change
- Quick build feature: paste a GitHub URL to trigger an ad-hoc build
- Default config with 4 repos: mkube, fastregistry, microdns, mkube-console
