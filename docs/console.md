# ACPA Console

The daemon console is implemented in React under `web/console/` and built into
a single self-contained HTML file embedded by the Go daemon.

Build and verify the console artifact:

```bash
npm install
npm run console:build
npm run console:test
```

Run browser smoke checks against a running daemon:

```bash
npm run console:smoke
```

Set `ACPA_CONSOLE_URL` when the daemon is served through a forwarded prefix or a
non-default local endpoint.
