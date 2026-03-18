# ADR-013: Plugin Manifests Sync via Git; Binaries Installed Per-Machine

**Status:** Accepted

## Context

Plugins have three components: a manifest (JSONC), user config (JSONC), and an executable (binary or script). The manifest and config are small, text-based, and platform-independent. Binaries are large, platform-specific, and bloat git history.

## Decision

Plugin manifests and configs live in `/store/plugins/{name}/` and sync via git like all other store content. Binaries are excluded from git.

**Binary resolution:** the manifest references the executable by name. Wyrd resolves it by searching `PATH` and then `~/.wyrd/bin/`. The binary is installed separately per machine via `wyrd plugin install`.

**Script plugins:** plugins implemented as scripts (Python, shell, Node) can live entirely in the plugin directory and sync normally, since scripts are platform-independent. The manifest declares `"executable_type": "script"` or `"executable_type": "binary"` to distinguish.

## Consequences

**Positive:** plugin configs travel with the store across machines. Adding a new machine requires only installing the binaries, not reconfiguring every plugin. Script-based plugins require zero per-machine setup.

**Negative:** binary plugins require manual installation on each machine. There is no automatic binary distribution mechanism; users must build from source or download a release. Version drift between machines is possible if one machine has an older binary.
