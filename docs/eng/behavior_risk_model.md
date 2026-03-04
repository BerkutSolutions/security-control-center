# Behavioral Anomaly Model (Risk-Based Step-Up)

## 1. Purpose

The model detects unusual user activity in near real time and enforces `step-up` verification:

1. Password verification.
2. Second factor (`TOTP` or `passkey`).

If step-up fails repeatedly, temporary lockout escalates:

1. 1 minute
2. 15 minutes
3. 1 hour

## 2. Features

The model uses short-window counters:

- `SensitiveViews5m`: sensitive reads during 5 minutes.
- `Exports30m`: export attempts during 30 minutes.
- `Denied10m`: authorization denials (`401/403`) during 10 minutes.
- `Mutations5m`: high-impact write operations during 5 minutes.
- `Requests1m`: total API requests per 1 minute.
- `IPNovelty`: new IP indicator (`0/1`).
- Historical baselines over 7 days for normalization.

## 3. Scoring

### 3.1 Normalization

For each feature we compute:

`z = max(0, (x - mu) / sigma)`

Where:

- `x` is current window value.
- `mu` is historical mean.
- `sigma = sqrt(mu + 0.25)`, with floor `sigma >= 0.5`.

### 3.2 Linear score

`raw = -3.4 + 0.85*z_sensitive + 1.2*z_exports + 1.4*z_denied + 0.7*z_mutations + 0.55*z_requests + 0.9*ip_novelty + 0.6*burst`

`burst = max(0, (SensitiveViews5m + Mutations5m - 12) / 6)`.

### 3.3 Probability-like value

`score = 1 / (1 + e^(-raw))`

## 4. False-positive suppression

After base score is computed, dampers are applied:

- Low recent activity (`totalRecent < 6`) -> `score *= 0.55`.
- No denies/exports and known IP -> `score *= 0.75`.
- Recently verified user (`<=24h`) -> `score *= 0.65`.
- Cold-start protection for low history and non-aggressive profile: soft cap `score <= 0.90`.

## 5. Trigger policy

Step-up is required when both conditions hold:

- `score >= 0.88`
- and one risk condition is true:
  - `Exports30m + Denied10m > 0`, or
  - `SensitiveViews5m + Mutations5m >= 14`, or
  - `Requests1m >= 120`

This keeps the model strict enough for production while avoiding noisy one-off triggers.

## 6. State and audit events

Per-user model state stores:

- `stepup_required`
- `password_verified`
- `failed_stepups`
- `locked_until`
- `last_verified_at`
- `last_risk_score`

Audit actions:

- `security.behavior.stepup_required`
- `security.behavior.stepup_password_ok`
- `security.behavior.stepup_failed`
- `security.behavior.stepup_success`

## 7. Product switch

Enable/disable in:

`Settings -> Hardening -> Enable behavioral anomaly model`

When disabled, behavior scoring and step-up are not enforced.
