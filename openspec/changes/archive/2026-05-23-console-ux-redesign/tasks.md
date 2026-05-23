## 1. Frontend Build Foundation

- [x] 1.1 Add React console source directory and package/build configuration
- [x] 1.2 Configure build output as a single self-contained HTML artifact
- [x] 1.3 Update daemon embedding to serve the built console artifact
- [x] 1.4 Document console build command in project scripts or docs

## 2. UX Structure

- [x] 2.1 Implement mobile-first dashboard shell with compact app bar and daemon health summary
- [x] 2.2 Replace assistant table with responsive assistant cards
- [x] 2.3 Add empty, loading, error, and attention states
- [x] 2.4 Add contextual assistant actions and autostart controls

## 3. Sheet Workflows

- [x] 3.1 Implement create assistant sheet with required and advanced fields
- [x] 3.2 Implement Feishu setup sheet with QR and manual modes
- [x] 3.3 Implement QR onboarding progress display through begin and complete calls
- [x] 3.4 Implement lifecycle confirmation sheet for stop and restart actions
- [x] 3.5 Add doctor/detail sheet or panel entry point backed by existing or new diagnostic API

## 4. Desktop Compatibility

- [x] 4.1 Add desktop layout rules that keep cards as the primary view
- [x] 4.2 Add selected assistant detail panel or modal behavior on wide viewports
- [x] 4.3 Verify desktop density without nested-card layouts or table-first fallback

## 5. Visual System

- [x] 5.1 Define CSS tokens for spacing, typography, surfaces, borders, and status colors
- [x] 5.2 Apply developer-tool/operations-console visual styling
- [x] 5.3 Add visible focus states and accessible labels for sheet/modal controls
- [x] 5.4 Ensure mobile touch targets and text wrapping meet responsive requirements

## 6. Verification

- [x] 6.1 Add frontend build verification
- [x] 6.2 Add Go tests for serving the built console artifact
- [x] 6.3 Add browser smoke checks for mobile and desktop viewports
- [x] 6.4 Run `go test ./...`
- [x] 6.5 Run the console build/test commands
