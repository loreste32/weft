#!/usr/bin/env bash
set -euo pipefail

if [[ "${WEFT_LIVE_REQUIRED:-0}" != "1" ]]; then
  echo "telecom live validation requires WEFT_LIVE_REQUIRED=1" >&2
  exit 2
fi

: "${FREESWITCH_HOST:?set FREESWITCH_HOST}"
: "${FREESWITCH_ESL_PASSWORD:?set FREESWITCH_ESL_PASSWORD}"
: "${ASTERISK_HOST:?set ASTERISK_HOST}"
: "${ASTERISK_ARI_USER:?set ASTERISK_ARI_USER}"
: "${ASTERISK_ARI_PASSWORD:?set ASTERISK_ARI_PASSWORD}"
: "${SIPP_TARGET:?set SIPP_TARGET to the SIPp remote target}"
: "${SIPP_SCENARIO:?set SIPP_SCENARIO to a checked-in or runner-provided XML scenario}"

if [[ ! -f "$SIPP_SCENARIO" ]]; then
  echo "SIPp scenario does not exist: $SIPP_SCENARIO" >&2
  exit 1
fi
command -v sipp >/dev/null 2>&1 || {
  echo "SIPp is required on the telecom self-hosted runner" >&2
  exit 1
}

root_dir=$(cd "$(dirname "$0")/.." && pwd)
weft_bin="${RUNNER_TEMP:-/tmp}/weft-telecom-live"
go build -o "$weft_bin" "$root_dir/cmd/weft"

FREESWITCH_HOST="$FREESWITCH_HOST" \
FREESWITCH_ESL_PORT="${FREESWITCH_ESL_PORT:-8021}" \
FREESWITCH_ESL_PASSWORD="$FREESWITCH_ESL_PASSWORD" \
ASTERISK_HOST="$ASTERISK_HOST" \
ASTERISK_ARI_PORT="${ASTERISK_ARI_PORT:-8088}" \
ASTERISK_ARI_USER="$ASTERISK_ARI_USER" \
ASTERISK_ARI_PASSWORD="$ASTERISK_ARI_PASSWORD" \
ASTERISK_ARI_APP="${ASTERISK_ARI_APP:-weft-live}" \
  "$weft_bin" run "$root_dir/packages/telecom/live_smoke.weft"

sipp -sf "$SIPP_SCENARIO" "$SIPP_TARGET" \
  -m "${SIPP_CALLS:-1}" \
  -timeout "${SIPP_TIMEOUT:-30}" \
  -trace_err
