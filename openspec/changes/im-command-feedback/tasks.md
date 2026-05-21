## 1. Command Framework

- [x] 1.1 Add structured command result and command error categories
- [x] 1.2 Normalize command response sending so each command outcome sends at most one reply
- [x] 1.3 Add user-facing text for success, failure, unknown command, and permission-denied outcomes

## 2. Permissions

- [x] 2.1 Add owner/admin detection from channel options and policy configuration
- [x] 2.2 Mark command definitions as ordinary-user or owner/admin
- [x] 2.3 Enforce owner/admin checks for `/mode`, `/mode default`, verbose skills, diagnostics, config, and memory mutation commands

## 3. User Commands

- [x] 3.1 Add `/help` output filtered by sender permission tier
- [x] 3.2 Add `/status` output for the sender's current session and connector context
- [x] 3.3 Update `/mode` replies with concise behavior and risk hints
- [x] 3.4 Add `/skills` effective skill listing
- [x] 3.5 Add `/skills verbose` source-layer and path listing for owner/admin users
- [x] 3.6 Ensure `/clear` has explicit success and failure replies

## 4. Verification

- [x] 4.1 Add runtime tests for command success, failure, unknown command, and permission-denied replies
- [x] 4.2 Add tests for mode-switch response text and policy enforcement
- [x] 4.3 Add tests for `/skills` and `/skills verbose` source reporting
- [x] 4.4 Run `go test ./...`
