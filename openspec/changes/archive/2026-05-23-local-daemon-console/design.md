## Context

ACPA currently starts assistants directly from CLI commands. `assistant start` spawns `assistant serve`, writes a PID file, and redirects logs. That works for development but does not provide a single control plane for Web setup, lazy lifecycle management, autostart, or consistent process supervision.

The next usability layer should introduce a local daemon and a local Web console. The daemon owns assistant lifecycle state; CLI and Web become clients of that local control plane.

## Goals / Non-Goals

**Goals:**

- Add a local daemon that supervises assistant processes.
- Let CLI commands lazily start the daemon when they need lifecycle or console services.
- Add explicit daemon start, stop, restart, and status commands.
- Autostart assistants marked `autostart=true`, with new assistants defaulting to autostart.
- Embed a local Web console in the daemon.
- Support assistant creation/setup and Feishu channel setup through both QR onboarding and manual existing-app credentials.
- Bind locally by default and require explicit unsafe confirmation for non-local binding.

**Non-Goals:**

- Add login, permissions, or multi-user access control to the console.
- Build detailed assistant internals management for sessions, logs, memory, skills, or permission history.
- Add systemd/launchd installation in the first version.
- Make the daemon a remote production control plane.

## Decisions

### Use daemon as the lifecycle source of truth

The daemon will supervise assistant serve processes and track configured assistants, running state, PID, last start/stop timestamps, and errors. Web and CLI lifecycle operations will call the daemon API instead of spawning assistant serve independently.

Alternative considered: let the Web console spawn assistant processes directly. That would tie assistant lifetime to the console process and make browser/console shutdown semantics unclear.

### Lazy-start the daemon from CLI clients

Commands that require daemon services will first try to connect to the local daemon socket or local HTTP endpoint. If unavailable and lazy startup is allowed, the CLI will start the daemon in the background, wait for readiness, then retry the request. Explicit daemon commands remain available.

Alternative considered: require users to run `acpa daemon start` manually. That is simpler but weakens the local appliance-style experience.

### Autostart is explicit configuration but defaults on

Assistant configuration will gain an `autostart` field. New assistants default to `autostart=true`. When the daemon starts, it launches all configured assistants with autostart enabled. Stopping an assistant does not automatically disable autostart; separate CLI/Web actions can disable it.

Alternative considered: restore "last running" assistants after daemon restart. That is convenient but makes persisted intent ambiguous.

### Embed the Web UI in the daemon

The daemon will serve both local API endpoints and static Web UI assets. `acpa console` will ensure the daemon is running and print or open the local URL.

Alternative considered: run a separate console server that talks to the daemon. That adds another process without enough benefit for v1.

### Keep bind safety simple and local-first

The daemon will bind to loopback by default. Binding to a non-loopback address requires `--insecure` and an interactive console confirmation. Because v1 is local-only by default, it will not add login or permission checks.

Alternative considered: add authentication immediately. That is appropriate for remote use but unnecessary for local-only v1 and would slow the setup flow.

### Reuse existing Feishu onboarding primitives

The Web setup flow will call the same Feishu registration and manual credential persistence logic used by CLI channel setup. QR onboarding will display the registration URL/QR state in the browser; manual mode will store provided app credentials as secret files in the assistant configspace.

Alternative considered: build separate Web-only Feishu setup code. That risks divergent behavior from CLI setup.

## Risks / Trade-offs

- [Risk] Background daemon startup differs across operating systems. -> Implement a simple pidfile/socket strategy first and keep system service installation out of v1.
- [Risk] Local HTTP without auth is unsafe if bound remotely. -> Enforce loopback binding by default and require `--insecure` plus interactive confirmation for non-local binds.
- [Risk] Two lifecycle paths may coexist during migration. -> Route new lifecycle commands through the daemon and preserve direct `assistant serve` as the supervised worker entry point.
- [Risk] Autostart may restart assistants users expected to keep stopped. -> Provide explicit disable-autostart actions and make stop semantics clear in CLI/Web text.
- [Risk] Web setup can duplicate CLI validation logic. -> Move shared assistant/channel setup logic into reusable internal packages before wiring the Web handlers.

## Migration Plan

Add daemon and console commands without removing direct `assistant serve`. Migrate `assistant start|stop|restart|status` to prefer the daemon, while keeping an explicit foreground/serve path for daemon-supervised workers and development.

## Open Questions

None for the first version.
