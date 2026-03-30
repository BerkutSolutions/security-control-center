# DLP in Berkut SCC

Version: `1.2.2`

## Purpose

The DLP subsystem in Berkut SCC reduces data leakage risk for documents and reports:

- blocks copying actions (copy/cut/context menu, shortcuts),
- records screenshot attempts and hidden-tab/window signals,
- writes telemetry to audit and behavior risk model.

## Where to configure

`Settings -> Hardening -> DLP frame`

Available options:

- `Block copying for protected content`
- `Attempt to block screenshots`
- `Protection scope mode`:
  - `Only protected (classification/tags)`
  - `All documents/reports`
- `Protection modules` (scope):
  - `Docs`
  - `Reports`

Default values:

- copy protection = enabled,
- screenshot protection = enabled,
- mode = `protected_only`,
- scope = `docs + reports`.

## How protected content is detected

In `Only protected (classification/tags)` mode, content is protected when:

- classification level is `>= 2`, or
- at least one classification tag is present.

In `All documents/reports` mode, classification/tags check is bypassed.

## How copy blocking works

For selected modules and matching objects:

- intercepts `copy`, `cut`, and `contextmenu`,
- intercepts `Ctrl/Cmd + C/X/A`,
- applies `no-copy` mode (`user-select: none`) on editor/viewer panel,
- writes a security event on each blocked attempt.

Events:

- `doc.security.copy_blocked`
- `report.security.copy_blocked`

## How screenshot handling works

For selected modules and matching objects:

- captures `PrintScreen` (`keydown`/`keyup`),
- records `visibilitychange=hidden` as a screenshot-related signal,
- shows a privacy shield visual response,
- writes events to audit and behavior model.

Events:

- `doc.security.screenshot_attempt`
- `report.security.screenshot_attempt`

## Integration with behavior model

DLP security events are also written to `user_behavior_events` as:

- `dlp.copy_blocked`
- `dlp.screenshot_attempt`

These signatures are included in user activity analysis:

`Settings -> Hardening -> Activity analysis`

Displayed DLP metrics include:

- copy blocks in last 10 minutes,
- screenshot attempts in last 10 minutes,
- corresponding reason entries in risk `reasons`.

## Audit and traceability

DLP settings updates are audited as:

- `settings.hardening.dlp.update`

Security event audit entries:

- `doc.security.copy_blocked`
- `doc.security.screenshot_attempt`
- `report.security.copy_blocked`
- `report.security.screenshot_attempt`

## Important limitations

Browser-level DLP cannot fully block screenshots at OS/device level.  
Current implementation provides practical controls:

- prevention for common copy workflows,
- attempt detection,
- audit and risk signatures for downstream response.

## Production recommendations

- keep protections enabled by default,
- use `All documents/reports` mode for high-sensitivity environments,
- monitor DLP events in logs and activity analysis,
- combine DLP with RBAC, classification, and step-up policies.
