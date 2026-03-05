#!/usr/bin/env bash

# Wrapper script that uses Rook's github-action-helper.sh for common functions
# and provides custom functions specific to this project

set -xeEo pipefail

REPO_DIR="/tmp/rook"
ROOK_HELPER="${REPO_DIR}/tests/scripts/github-action-helper.sh"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=test/scripts/ceph/utils.sh
source "${SCRIPT_DIR}/utils.sh"

#############
# VARIABLES #
#############
: "${FUNCTION:=${1}}"

#############
# WRAPPER FUNCTIONS - Delegate to Rook's script
#############
function install_deps() {
	sudo wget https://github.com/mikefarah/yq/releases/download/v4.52.4/yq_linux_amd64 -O /usr/local/bin/yq
	sudo chmod +x /usr/local/bin/yq
}

function print_k8s_cluster_status() {
	"${ROOK_HELPER}" print_k8s_cluster_status
}

function use_local_disk() {
	"${ROOK_HELPER}" use_local_disk
}

function find_extra_block_dev() {
	"${ROOK_HELPER}" find_extra_block_dev
}

function create_bluestore_partitions() {
	"${REPO_DIR}/tests/scripts/create-bluestore-partitions.sh" "$@"
}

#############
# CUSTOM FUNCTIONS - Specific to this project
#############
function checkout_rook_release() {
	local tag="$1"
	local repo_url="https://github.com/rook/rook.git"
	echo "Cloning Rook repository with tag: ${tag}..."
	git clone \
		--branch "${tag}" \
		--single-branch \
		--depth 1 \
		"${repo_url}" \
		"${REPO_DIR}"

	# TODO: remove this once, the fix is included in a Rook release.
	# Patch find_extra_block_dev to deduplicate boot devices
	sed -i "s/awk '{print \$2}'/awk '{print \$2}' | sort -u/" \
		"${ROOK_HELPER}"
}

function deploy_rook() {
	cd "${REPO_DIR}/deploy/examples"

	yq -i '
  with(select(.kind == "ConfigMap");
    .data.CSI_ENABLE_CSIADDONS = "true" |
	.data.ROOK_CSIADDONS_IMAGE = "quay.io/csiaddons/k8s-sidecar:test" |
	.data.ROOK_CSI_CEPH_IMAGE = "quay.io/cephcsi/cephcsi:canary" |
	.data.CSI_LOG_LEVEL = "5"
  )' operator.yaml
	kubectl_retry create -f crds.yaml
	kubectl_retry create -f common.yaml
	kubectl_retry create -f operator.yaml
	kubectl_retry create -f csi-operator.yaml

}
function deploy_first_ceph_cluster() {
	DEVICE_NAME="$(find_extra_block_dev)"
	cd "${REPO_DIR}/deploy/examples"
	yq e -i 'select(.kind != "CephBlockPool")' csi/rbd/storageclass-test.yaml
	kubectl_retry create -f csi/rbd/storageclass-test.yaml

	DEVICE_NAME="$DEVICE_NAME" yq e -i '
	  with(select(.kind == "CephCluster");
	    .spec.dashboard.enabled = false |
	    .spec.storage.useAllDevices = false |
	    .spec.storage.deviceFilter = strenv(DEVICE_NAME) + "1"
	  )
	' cluster-test.yaml
	kubectl_retry create -f cluster-test.yaml
	kubectl_retry create -f toolbox.yaml
	sed -i "/resources:/,/ # priorityClassName:/d" rbdmirror.yaml
	kubectl_retry create -f rbdmirror.yaml

	wait_for_operator_pod_to_be_ready_state
	wait_for_mon rook-ceph
	wait_for_osd_pod_to_be_ready_state rook-ceph
}

function deploy_second_ceph_cluster() {
	DEVICE_NAME="$(find_extra_block_dev)"
	cd "${REPO_DIR}/deploy/examples"
	NAMESPACE=rook-ceph-secondary envsubst <common-second-cluster.yaml | kubectl create -f -
	sed -i 's/namespace: rook-ceph/namespace: rook-ceph-secondary/g' cluster-test.yaml
	DEVICE_NAME="$DEVICE_NAME" yq e -i '
	  with(select(.kind == "CephCluster");
	    .spec.dataDirHostPath = "/var/lib/rook-external" |
	    .spec.storage.deviceFilter = strenv(DEVICE_NAME) + "2"
	  )
	' cluster-test.yaml
	kubectl_retry create -f cluster-test.yaml
	yq e -i '.metadata.namespace = "rook-ceph-secondary"' toolbox.yaml
	kubectl_retry create -f toolbox.yaml
	sed -i 's/namespace: rook-ceph/namespace: rook-ceph-secondary/g' rbdmirror.yaml
	kubectl_retry create -f rbdmirror.yaml
	wait_for_mon rook-ceph-secondary
	wait_for_osd_pod_to_be_ready_state rook-ceph-secondary
}

wait_for_osd_pod_to_be_ready_state() {
	local namespace="$1"
	timeout 200 bash -c "
		    until [ \$(kubectl get pod -l app=rook-ceph-osd -n \"$namespace\" -o custom-columns=READY:status.containerStatuses[*].ready | grep -c true) -eq 1 ]; do
		      echo \"waiting for the osd pods to be in ready state\"
			  kubectl -n \"$namespace\" get po
		      sleep 1
		    done
	"
	timeout_command_exit_code
}

wait_for_operator_pod_to_be_ready_state() {
	timeout 100 bash -c "
		    until [ \$(kubectl get pod -l app=rook-ceph-operator -n rook-ceph -o custom-columns=READY:status.containerStatuses[*].ready | grep -c true) -eq 1 ]; do
		      echo \"waiting for the operator to be in ready state\"
			  kubectl -n rook-ceph get po
		      sleep 1
		    done
	"
	timeout_command_exit_code
}

wait_for_mon() {
	local namespace="$1"
	timeout 150 bash -c "
		    until [ \$(kubectl -n \"$namespace\" get deploy -l app=rook-ceph-mon,mon_canary!=true | grep rook-ceph-mon | wc -l | awk '{print \$1}' ) -eq 1 ]; do
		      echo \"waiting for one mon deployment to exist\"
			  kubectl -n \"$namespace\" get po
		      sleep 2
		    done
	"
	timeout_command_exit_code
}

timeout_command_exit_code() {
	# timeout command return exit status 124 if command times out
	if [ $? -eq 124 ]; then
		echo "Timeout reached"
		exit 1
	fi
}

#######################################
# Enable mirrored pool on clusters
# Arguments:
#   $1 -> primary namespace (e.g. rook-ceph)
#   $2 -> secondary namespace (e.g. rook-ceph-secondary)
#######################################
enable_mirroring_cluster() {
	local PRIMARY_NS="$1"
	local SECONDARY_NS="$2"
	local POOL_NAME="replicapool"
	cd "${REPO_DIR}/deploy/examples"

	echo "Enabling mirroring on primary cluster (${PRIMARY_NS})..."

	PRIMARY_NS="$PRIMARY_NS" yq e -i '
	  .spec.mirroring.enabled = true |
	  .spec.mirroring.mode = "image" |
	  .metadata.namespace = strenv(PRIMARY_NS)
	' pool-test.yaml

	kubectl_retry create -f pool-test.yaml

	echo "Waiting for pool to become Ready on primary cluster..."
	timeout 180 sh -c "until [ \"\$(kubectl -n ${PRIMARY_NS} get cephblockpool ${POOL_NAME} -o jsonpath='{.status.phase}' | grep -c Ready)\" -eq 1 ]; do sleep 1; done"

	SECONDARY_NS="$SECONDARY_NS" yq e -i '
	  .spec.mirroring.enabled = true |
	  .spec.mirroring.mode = "image" |
	  .metadata.namespace = strenv(SECONDARY_NS)
	' pool-test.yaml

	kubectl_retry create -f pool-test.yaml

	echo "Waiting for pool to become Ready on secondary cluster..."
	timeout 180 sh -c "until [ \"\$(kubectl -n ${SECONDARY_NS} get cephblockpool ${POOL_NAME} -o jsonpath='{.status.phase}' | grep -c Ready)\" -eq 1 ]; do sleep 1; done"

	echo "Copying peer token secret to secondary cluster..."

	kubectl_retry -n "${PRIMARY_NS}" get secret pool-peer-token-${POOL_NAME} -o yaml >peer-secret.yaml

	yq e -i 'del(.metadata.ownerReferences)' peer-secret.yaml
	SECONDARY_NS="$SECONDARY_NS" yq e -i '.metadata.namespace = strenv(SECONDARY_NS)' peer-secret.yaml
	POOL_NAME="$POOL_NAME" yq e -i '.metadata.name = "pool-peer-token-" + strenv(POOL_NAME) + "-config"' peer-secret.yaml

	kubectl_retry create --namespace="${SECONDARY_NS}" -f peer-secret.yaml

	echo "Registering peer secret on secondary cluster..."

	kubectl_retry patch -n "${SECONDARY_NS}" cephblockpool ${POOL_NAME} --type merge \
		-p "{\"spec\":{\"mirroring\":{\"peers\":{\"secretNames\":[\"pool-peer-token-${POOL_NAME}-config\"]}}}}"

	echo "Copying peer token secret to primary cluster..."

	kubectl_retry -n "${SECONDARY_NS}" get secret pool-peer-token-${POOL_NAME} -o yaml >peer-secret.yaml

	yq e -i 'del(.metadata.ownerReferences)' peer-secret.yaml
	PRIMARY_NS="$PRIMARY_NS" yq e -i '.metadata.namespace = strenv(PRIMARY_NS)' peer-secret.yaml
	POOL_NAME="$POOL_NAME" yq e -i '.metadata.name = "pool-peer-token-" + strenv(POOL_NAME) + "-config"' peer-secret.yaml

	kubectl_retry create --namespace="${PRIMARY_NS}" -f peer-secret.yaml

	echo "Registering peer secret on primary cluster..."

	kubectl_retry patch -n "${PRIMARY_NS}" cephblockpool ${POOL_NAME} --type merge \
		-p "{\"spec\":{\"mirroring\":{\"peers\":{\"secretNames\":[\"pool-peer-token-${POOL_NAME}-config\"]}}}}"

	echo "Verifying mirroring health on both clusters..."
	verify_mirroring_health "${PRIMARY_NS}" "${POOL_NAME}"
	verify_mirroring_health "${SECONDARY_NS}" "${POOL_NAME}"
}

#######################################
# Verify mirroring health for a pool
# Arguments:
#   $1 -> namespace (e.g. rook-ceph or rook-ceph-secondary)
#   $2 -> pool name (e.g. replicapool)
#######################################
verify_mirroring_health() {
	local NAMESPACE="$1"
	local POOL_NAME="$2"
	local TOOLBOX_POD

	echo "Checking mirroring health in namespace ${NAMESPACE}..."

	# Get the toolbox pod name
	TOOLBOX_POD=$(kubectl -n "${NAMESPACE}" get pod -l app=rook-ceph-tools -o jsonpath='{.items[0].metadata.name}')

	if [ -z "${TOOLBOX_POD}" ]; then
		echo "ERROR: Toolbox pod not found in namespace ${NAMESPACE}"
		return 1
	fi

	echo "Using toolbox pod: ${TOOLBOX_POD}"

	# Wait for mirroring to be healthy (timeout after 180 seconds)
	timeout 180 bash -c "
		until kubectl -n \"${NAMESPACE}\" exec \"${TOOLBOX_POD}\" -- rbd mirror pool status \"${POOL_NAME}\" --format=json | jq -e '.summary.health == \"OK\"' > /dev/null 2>&1; do
			echo \"Waiting for mirroring health to be OK in ${NAMESPACE}...\"
			kubectl -n \"${NAMESPACE}\" exec \"${TOOLBOX_POD}\" -- rbd mirror pool status \"${POOL_NAME}\" || true
			sleep 5
		done
	"

	if [ $? -eq 124 ]; then
		echo "ERROR: Timeout waiting for mirroring health to be OK in ${NAMESPACE}"
		kubectl -n "${NAMESPACE}" exec "${TOOLBOX_POD}" -- rbd mirror pool status "${POOL_NAME}" || true
		return 1
	fi

	echo "Mirroring health is OK in ${NAMESPACE}"
	kubectl -n "${NAMESPACE}" exec "${TOOLBOX_POD}" -- rbd mirror pool status "${POOL_NAME}"
}

########
# MAIN #
########

FUNCTION="$1"
shift # remove function arg now that we've recorded it
# call the function with the remainder of the user-provided args
if ! $FUNCTION "$@"; then
	echo "Call to $FUNCTION was not successful" >&2
	exit 1
fi
