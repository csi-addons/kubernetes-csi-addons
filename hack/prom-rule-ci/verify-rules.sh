#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

CRI="${CRI:-podman}"
PROM_IMAGE="${PROM_IMAGE:-quay.io/prometheus/prometheus:v2.53.0}"
TESTS_FILE="${SCRIPT_DIR}/prom-rules-tests.yaml"

function cleanup() {
    rm -f "${target_file}"
}

function lint() {
    local target_file="${1}"
    ${CRI} run --rm --entrypoint=/bin/promtool \
        -v "${target_file}":/tmp/rules.verify:ro,Z \
        "${PROM_IMAGE}" \
        check rules /tmp/rules.verify
}

function unit_test() {
    local target_file="${1}"
    ${CRI} run --rm --entrypoint=/bin/promtool \
        -v "${TESTS_FILE}":/tmp/rules.test:ro,Z \
        -v "${target_file}":/tmp/rules.verify:ro,Z \
        "${PROM_IMAGE}" \
        test rules /tmp/rules.test
}

target_file="$(mktemp --tmpdir -u tmp.prom_rules.XXXXX)"
trap cleanup EXIT INT TERM

go run "${SCRIPT_DIR}/rule-spec-dumper.go" "${target_file}"

echo "INFO: generated rules written to ${target_file}"
lint "${target_file}"
unit_test "${target_file}"
