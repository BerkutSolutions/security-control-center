# Compat — Quick Cheat Sheet

## Fast path
1) After login, open the Compatibility wizard (it appears automatically if action is needed).
2) For a module status:
   - `needs_attention` → run **Partial adapt** first
   - `needs_reinit` → run **Full reset** (destructive)
   - `broken` → inspect logs/DB first, then decide
3) Before Full reset: **make a backup**.
4) Start the job and wait for `finished` (progress 100%).

## Partial adapt vs Full reset
- Partial adapt: safe, business data must remain.
- Full reset: deletes module data and restores defaults.

## Where to check progress
- Wizard UI polls automatically.
- API:
  - `GET /api/app/jobs/:id`

