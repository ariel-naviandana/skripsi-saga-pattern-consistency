#!/usr/bin/env bash
set -euo pipefail

SCENARIO="${1:-S1}"
APPROACH="${2:-choreography}"
RUNS="${3:-30}"

if [[ "$APPROACH" != "choreography" && "$APPROACH" != "orchestration" ]]; then
  echo "approach must be choreography|orchestration" >&2
  exit 1
fi

OUT_DIR="docs/runs/${SCENARIO}/${APPROACH}"
mkdir -p "$OUT_DIR"

configure_fault() {
  case "$SCENARIO" in
    S1|S7) FAIL_AT_STEP=""  FAIL_AT_ATTEMPT="0" DELAY_MS="0" FAIL_ON_COMPENSATE="false" DROP_EVENT="" DROP_RESPONSE_AT_STEP="" SELECTIVE_COMPENSATE="false" ;;
    S2)    FAIL_AT_STEP="shipping" FAIL_AT_ATTEMPT="1" DELAY_MS="0" FAIL_ON_COMPENSATE="false" DROP_EVENT="" DROP_RESPONSE_AT_STEP="" SELECTIVE_COMPENSATE="false" ;;
    S3)    FAIL_AT_STEP="inventory" FAIL_AT_ATTEMPT="1" DELAY_MS="0" FAIL_ON_COMPENSATE="false" DROP_EVENT="" DROP_RESPONSE_AT_STEP="" SELECTIVE_COMPENSATE="false" ;;
    S6)    FAIL_AT_STEP="inventory" FAIL_AT_ATTEMPT="1" DELAY_MS="0" FAIL_ON_COMPENSATE="true" DROP_EVENT="" DROP_RESPONSE_AT_STEP="" SELECTIVE_COMPENSATE="false" ;;
    S8)    FAIL_AT_STEP="" FAIL_AT_ATTEMPT="0" DELAY_MS="0" FAIL_ON_COMPENSATE="false" DROP_EVENT="saga.order.created" DROP_RESPONSE_AT_STEP="" SELECTIVE_COMPENSATE="false" ;;
    S9)    FAIL_AT_STEP="" FAIL_AT_ATTEMPT="0" DELAY_MS="0" FAIL_ON_COMPENSATE="false" DROP_EVENT="" DROP_RESPONSE_AT_STEP="inventory" SELECTIVE_COMPENSATE="false" ;;
    *)     echo "unsupported scenario $SCENARIO" >&2; exit 1 ;;
  esac
}

configure_count() {
  case "$SCENARIO" in
    S7) COUNT=500 ;;
    *)  COUNT=1 ;;
  esac
}

echo "Scenario: $SCENARIO / $APPROACH / $RUNS runs"
configure_fault
configure_count
echo "Fault config: FAIL_AT_STEP=$FAIL_AT_STEP FAIL_AT_ATTEMPT=$FAIL_AT_ATTEMPT FAIL_ON_COMPENSATE=$FAIL_ON_COMPENSATE DROP_EVENT=$DROP_EVENT DROP_RESPONSE_AT_STEP=$DROP_RESPONSE_AT_STEP SELECTIVE_COMPENSATE=$SELECTIVE_COMPENSATE"

export APPROACH FAIL_AT_STEP FAIL_AT_ATTEMPT DELAY_MS FAIL_ON_COMPENSATE DROP_EVENT DROP_RESPONSE_AT_STEP SELECTIVE_COMPENSATE

echo "Starting services with approach=$APPROACH..."
docker compose up -d order-service payment-service inventory-service shipping-service orchestrator > /dev/null
sleep 5

for i in $(seq 1 "$RUNS"); do
  bash scripts/reset.sh > /dev/null 2>&1 || true
  OUT="${OUT_DIR}/run-${i}.json"
  go run ./cmd/workload-generator \
    -approach "$APPROACH" \
    -count "$COUNT" \
    -out "$OUT" > /dev/null 2>&1 || echo "  run $i failed" >&2
  echo "  run $i done -> $OUT"
done

echo "Done. Results in $OUT_DIR"