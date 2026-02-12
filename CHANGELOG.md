# Changelog

## 1.0.1 - 2026-02-12

### Added
- App version metadata (`1.0.1`) exposed via backend meta/settings APIs.
- Deployment Mode configuration with `Enterprise` and `Home` options in Settings.
- Home mode UI warning and runtime enforcement (`HOME` behavior in app metadata/footer).
- Update-check toggle in Settings with manual check action.
- Safe GitHub latest-release checker (timeout, cache, rate-limit backoff, version comparison only).
- Global app metadata endpoint for footer/version/update visibility.
- Automated i18n coverage test for RU/EN key parity and UI key usage validation.
- `CHANGELOG.md` and `Zero-Trust Audit Report`.
- Settings tab `Грифы` (`Classifications`) for managing document classification labels and hierarchy slots.

### Changed
- Monitoring home toolbar layout refactored into compact two-column structure.
- Monitoring empty-state behavior now shows dedicated “no monitors” message and hides detail/events center when list is empty.
- Settings page structure cleaned and normalized (tabs/panels, sources panel, runtime panel, about version block).
- README updated with docker-compose quick deploy section and environment guidance.
- Config defaults/documentation include explicit `deployment_mode`.
- Archive system UX improved (localized archive open action in Tasks archive list).
- HTTPS settings UX aligned with deployment mode behavior.
- Routing fixes applied for task comment mutation endpoints.
- Docker deployment guidance expanded for GHCR quick deploy and persistent SQLite volume usage.
- Accounts role details now render permissions as tag-like pills instead of comma-separated plain text.
- Docs sidebar width and action-button layout tuned so `Новый (MD)`, `Импорт`, `Шаблоны` stay on one line without jumping.
- Accounts top tabs (`Главная`, `Группы`, `Пользователи`) resized to match Reports tab sizing.
- Log range inputs (`С`/`По`) expanded for better date-time usability.
- Custom classification hierarchy now supports moving user-defined levels between built-in levels.
- Default visible classification order changed to: `Конфиденциальный` -> `Внутренний` -> `ДСП` -> `Публичный`.

### Fixed
- Tasks Home visual spacing between “Spaces” and “Task Templates” cards using existing stylesheet.
- Missing archive translation key `common.open` added in RU/EN.
- Added missing RU/EN keys used by new Settings/Monitoring UI states.
- Task comment mutation routes now require `tasks.comment` instead of `tasks.view`.
- Removed malformed duplicated fragments from Settings HTML layout.
- Settings runtime update checkbox alignment adjusted to match deployment mode controls.
- Update-check status rendering now accepts both checked_at and checkedAt fields.
- Monitoring home toolbar layout stabilized as requested: left column (search/refresh, status, activity), right column (tags).
- Logs details view now hides noisy numeric counters for list-only actions.
- Logs date filters (from/to) are aligned in one horizontal row on desktop.
- Update-check backend no longer crashes when GitHub API is unavailable (safe fallback result instead of nil dereference).
- `Check now` in settings now persists enabled update checks before manual check, preventing UI/server toggle desync.
- Tasks board layout spacing adjusted so a lower wide board does not push first-row boards apart.
- Monitoring tags selector height reduced for cleaner toolbar alignment.
- Approvals toolbar layout fixed: refresh button stays to the right of status filter with proper spacing.
- Docs header actions kept in one row on desktop so `Templates` stays to the right of `Import`.
- Docs viewer API now routes to internal document tabs, ensuring view/edit open in the built-in tab system instead of modal flow.
- Removed hardcoded `Секретно` / `Совершенно секретно` / `Особой важности` labels from active UI selections by default; these levels are now optional custom slots managed from `Грифы`.
- Document and incident classification selectors now read active values from the shared classifications directory.
- Accounts clearance selectors now sync with the active classifications directory to keep document access hierarchy consistent.
- Removed legacy modal takeover on document open path: document view/edit now stays in internal tabs without overlay modal.
- Reports home card `Список отчетов` top offset removed.
- Role permission chips in Accounts details now use rounded pill corners.
- Added localization for update-check audit action in Logs (`settings.updates.check`) and human-readable details mapping.

### Security
- Update checker performs version metadata fetch only (no code download/execution).
- Update-check actions are audited: enable/disable and check events.
- Home mode keeps zero-trust controls (session + RBAC + ACL deny-by-default unchanged).
- Confirmed route-level protection remains enforced through middleware/permission wrappers.
- Tightened task comment write/delete endpoint permissions to prevent view-only bypass.

