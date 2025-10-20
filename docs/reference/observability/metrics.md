# Crossplane Metrics

## XR Circuit Breaker Counters

Crossplane exposes counters that let operators observe the XR watch circuit breaker. Following metrics are scraped from the manager's `/metrics` endpoint:

- `crossplane_circuit_breaker_opens_total{controller}` – counts transitions from a closed circuit breaker to open.
- `crossplane_circuit_breaker_closes_total{controller}` – counts transitions from open back to closed.
- `crossplane_circuit_breaker_events_total{controller,result}` – counts each watched event that reaches the XR controller. `result` is one of:
  - `allowed` – the event was processed while the circuit was closed.
  - `dropped` – the event was discarded because the circuit remained open.
  - `halfopen_allowed` – the event was processed while probing a half-open circuit.

The `controller` label identifies the XR controller using the format `composite/<plural>.<group>`, for example `composite/myapps.example.org`. Per-object labels are intentionally avoided to keep cardinality bounded.

When the breaker is open, Crossplane also sets the composite resource's `Responsive` condition to `False` with reason `WatchCircuitOpen`. Use the condition for per-resource investigations and the metrics for fleet-level alerting and dashboards.

## Example Queries

### Estimated Open Circuits

```
sum by (controller) (
  rate(crossplane_circuit_breaker_opens_total[5m])
) - sum by (controller) (
  rate(crossplane_circuit_breaker_closes_total[5m])
)
```

### Dropped Event Alert

```
sum by (controller) (
  rate(crossplane_circuit_breaker_events_total{result="dropped"}[5m])
) > 0
```

### Thrash Detection

```
sum(
  rate(crossplane_circuit_breaker_opens_total[5m])
) > 0
```

Dashboards can chart the open/close rates alongside dropped event rates to visualize controllers that are under watch pressure.
