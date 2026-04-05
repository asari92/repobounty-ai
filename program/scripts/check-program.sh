#!/usr/bin/env bash
set -euo pipefail

cd /app

ARTIFACTS_DIR="${PROGRAM_ARTIFACTS_DIR:-/artifacts}"
SOLANA_RPC_URL="${SOLANA_RPC_URL:-https://api.devnet.solana.com}"
WALLET_PATH="${ANCHOR_WALLET:-/wallet/id.json}"
LOG_PATH="${ARTIFACTS_DIR}/check.log"
VALIDATOR_LOG_PATH="${ARTIFACTS_DIR}/validator.log"
LOCALNET_RPC_URL="${LOCALNET_RPC_URL:-http://127.0.0.1:8899}"
SOLANA_CONFIG_DIR="${SOLANA_CONFIG_DIR:-/root/.config/solana}"
DEFAULT_WALLET_PATH="${SOLANA_CONFIG_DIR}/id.json"
validator_pid=""

mkdir -p "${ARTIFACTS_DIR}"
: > "${LOG_PATH}"

log() {
  echo "$1" | tee -a "${LOG_PATH}"
}

run_and_log() {
  log ""
  log "\$ $*"
  "$@" 2>&1 | tee -a "${LOG_PATH}"
}

cleanup() {
  if [ -n "${validator_pid}" ]; then
    kill "${validator_pid}" >/dev/null 2>&1 || true
    wait "${validator_pid}" >/dev/null 2>&1 || true
  fi
}

trap cleanup EXIT

if [ ! -f "${WALLET_PATH}" ]; then
  log "Missing wallet file at ${WALLET_PATH}."
  log "Set SOLANA_DEPLOY_WALLET to the admin wallet directory containing id.json before running solana-check."
  exit 1
fi

mkdir -p "${SOLANA_CONFIG_DIR}"
ln -sfn "${WALLET_PATH}" "${DEFAULT_WALLET_PATH}"

run_and_log npm ci
run_and_log anchor build

log ""
log "\$ solana-test-validator --reset --ledger /tmp/test-ledger"
solana-test-validator --reset --ledger /tmp/test-ledger >"${VALIDATOR_LOG_PATH}" 2>&1 &
validator_pid=$!

for _ in $(seq 1 30); do
  if solana cluster-version --url "${LOCALNET_RPC_URL}" >/dev/null 2>&1; then
    break
  fi
  if ! kill -0 "${validator_pid}" >/dev/null 2>&1; then
    log "solana-test-validator exited before becoming ready."
    log "Validator log:"
    cat "${VALIDATOR_LOG_PATH}" | tee -a "${LOG_PATH}"
    exit 1
  fi
  sleep 1
done

if ! solana cluster-version --url "${LOCALNET_RPC_URL}" >/dev/null 2>&1; then
  log "solana-test-validator did not become ready at ${LOCALNET_RPC_URL}."
  log "Validator log:"
  cat "${VALIDATOR_LOG_PATH}" | tee -a "${LOG_PATH}"
  exit 1
fi

run_and_log anchor deploy --provider.cluster localnet
run_and_log env ANCHOR_PROVIDER_URL="${LOCALNET_RPC_URL}" ANCHOR_WALLET="${WALLET_PATH}" npm exec -- ts-mocha -p ./tsconfig.json -t 1000000 tests/**/*.ts

log ""
log "Program build and tests completed successfully."
log "Log saved to ${LOG_PATH}"
