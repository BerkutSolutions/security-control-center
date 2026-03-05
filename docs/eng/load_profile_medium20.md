# Minimal Load Profile (Medium Company)

Goal: safe local/pilot profile without overloading a workstation.

## Medium-20 profile

- Monitors: up to 20 active HTTP/TCP monitors.
- Check interval: 30s.
- Timeout: 20-24s.
- Retries: 2.
- Retry interval: 30s.
- Max concurrent checks: 4.
- Task boards: 2-3 boards, 200-500 tasks total.
- Incidents: up to 200 open/closed in 30 days.

## Recommended settings

- `BERKUT_MONITORING_STATS_LOG_INTERVAL_SECONDS=60`
- `BERKUT_MONITORING_JITTER_PERCENT=15`
- `BERKUT_MONITORING_JITTER_MAX_SECONDS=10`
- In UI: Monitoring settings -> `max_concurrent_checks = 4`
- In dev/home, avoid heavy recurring reports and frequent bulk operations.

## Healthy signals

- Average CPU < 35-45% on local machine.
- Short peaks up to 70% are acceptable during concurrent timeouts.
- RAM usage remains stable without continuous growth.

## Quick smoke for this profile

1. Create 20 monitors with 30s interval.
2. Run for 15 minutes.
3. Ensure Monitoring/Tasks/Incidents tabs stay responsive.
4. Ensure no runaway growth in retry/scheduled checks.

## Limits

- Not intended for high-cardinality event loads (thousands of monitors).
- Enterprise scale requires dedicated capacity tests and sizing.
