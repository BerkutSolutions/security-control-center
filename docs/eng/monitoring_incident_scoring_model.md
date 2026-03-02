# Incident Scoring (adaptive incident formation) — model, method, and experiment

This document explains how SCC computes a numeric `incident_score ∈ [0..1]` from a stream of monitoring observations and how it is used to open/close incidents adaptively. It also describes a reproducible computational experiment and parameter fitting via an explicit loss function.

## 1. Why this exists

A naive rule **“DOWN → open incident”** often produces:

- **False incidents** during short network flaps / transient errors.
- **Late detection** of service degradation (latency spikes, HTTP 5xx) while the monitor is still “UP”.

The goal is to replace a binary trigger with an **adaptive policy** driven by a probability-like score, reducing noise without missing real outages.

## 2. Observations and hidden state

Each check yields observable telemetry:

- UP/DOWN (`OK`)
- `latency_ms`
- HTTP `status_code`
- `error_kind` (timeout/dns/connect/…)
- operational suppressions (`paused` / `maintenance`)

We assume the monitored object has a hidden state:

- **N (Normal)**
- **D (Degraded)**
- **O (Outage)**

The state is not directly observed; we infer it from the observation stream.

## 3. Mathematical model: 3-state HMM

We use a Hidden Markov Model (HMM):

- hidden state `S_t ∈ {N, D, O}`
- discrete observation `X_t ∈ Ω`

### 3.1. Discretizing telemetry into observations

Telemetry is mapped to a single discrete symbol:

- `ok`
- `latency_high`, `latency_very_high`
- `http_5xx`
- `timeout`, `dns`, `connect`, `connection_refused`
- `other_down`

This keeps the model explainable and stable for replay/simulation.

### 3.2. HMM parameters

The model parameters are:

- prior `π = P(S_0)`
- transitions `A_{ij} = P(S_t=j | S_{t-1}=i)`
- emissions `B_j(x) = P(X_t=x | S_t=j)`

Rows of `A` and `B` are normalized.

### 3.3. Numerical method: online posterior filtering

We compute the posterior distribution online:

1) prediction:
\[
\hat{p}_t(j) = \sum_i p_{t-1}(i)\,A_{ij}
\]
2) correction:
\[
\tilde{p}_t(j) = \hat{p}_t(j)\,B_j(X_t)
\]
3) normalization:
\[
p_t(j) = \frac{\tilde{p}_t(j)}{\sum_k \tilde{p}_t(k)}
\]

The posterior `p_t = (P(N), P(D), P(O))` is stored in `monitor_state` for incremental updates.

## 4. Turning posterior into `incident_score`

We compute a scalar score as:

\[
score = clamp\Big(P(O) + 0.5 \cdot P(D)\Big)
\]

This makes outages dominant, while degradation contributes as an early warning signal.

## 5. Adaptive auto-incident policy

Scoring does not open incidents by itself. Policy:

- open when `score ≥ open_threshold` with `confirmations` (consecutive non-UP steps)
- close when `score ≤ close_threshold` (hysteresis) or on UP if `auto_incident_close_on_up` is enabled

Hysteresis prevents chatter near the threshold.

## 6. Loss function (explicit optimization target)

To strengthen the “numerical methods” part, we introduce:

\[
Loss = W_{false}\cdot FalseOpens + W_{miss}\cdot Misses + W_{delay}\cdot DelaySumSec + W_{noise}\cdot Actions
\]

Where:
- `FalseOpens`: false incident openings
- `Misses`: real outages not opened
- `DelaySumSec`: total time-to-open over real outages
- `Actions = opens + closes`: noise / processing load
- `W_*`: weights

## 7. Parameter fitting via grid search

We run a grid search over:
- `open_threshold`, `close_threshold` (only `close < open`)
- `confirmations`
- (HMM3 only) `hmm3_diag_boost`: “stickiness” scaling for the diagonal of `A`

Each candidate is evaluated on the same observation stream; the best one minimizes `Loss`.

### Replay vs simulate

- **simulate**: the scenario generator provides ground truth.
- **replay**: ground truth is unknown; a duration-based heuristic marks long outages as “likely real”.

## 8. Reproducible CLI experiment

- Run policies:
  - `net-tool experiment --monitor-id 123 --since 2026-01-01 --mode replay --out result.json`
  - `net-tool experiment --monitor-id 123 --since 2026-01-01 --mode simulate --scenario mixed --seed 1 --out result.json`

- Fit parameters:
  - `net-tool experiment-fit --monitor-id 123 --since 2026-01-01 --mode replay --out fit.json`
  - `net-tool experiment-fit --monitor-id 123 --since 2026-01-01 --mode simulate --scenario mixed --seed 1 --out fit.csv`

## 9. Code pointers

- HMM3: `core/monitoring/incidenthmm/`
- HMM score: `core/monitoring/incident_score_hmm3.go`
- Policy: `core/monitoring/incidents.go`
- Experiment: `core/monitoring/experiment/`
- Loss + fitting: `core/monitoring/experiment/fit.go`

