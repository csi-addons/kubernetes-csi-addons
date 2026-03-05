#!/usr/bin/env bash

# Collect debug information from Ceph clusters for troubleshooting

set -eo pipefail

echo "=== Capturing debug information ==="

echo "=== Pods in rook-ceph namespace ==="
kubectl get po -n rook-ceph -o wide || true
kubectl get po -n rook-ceph -show-labels || true

echo "=== Pods in rook-ceph-secondary namespace ==="
kubectl get po -n rook-ceph-secondary -o wide || true

echo "=== Pod descriptions in rook-ceph namespace ==="
for pod in $(kubectl get po -n rook-ceph -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || true); do
	echo "--- Describing pod: $pod ---"
	kubectl describe pod "$pod" -n rook-ceph || true
done

echo "=== Pod descriptions in rook-ceph-secondary namespace ==="
for pod in $(kubectl get po -n rook-ceph-secondary -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || true); do
	echo "--- Describing pod: $pod ---"
	kubectl describe pod "$pod" -n rook-ceph-secondary || true
done

echo "=== Pod logs in rook-ceph namespace ==="
for pod in $(kubectl get po -n rook-ceph -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || true); do
	echo "--- Logs for pod: $pod ---"
	kubectl logs "$pod" -n rook-ceph --all-containers=true || true
done

echo "=== Pod logs in rook-ceph-secondary namespace ==="
for pod in $(kubectl get po -n rook-ceph-secondary -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || true); do
	echo "--- Logs for pod: $pod ---"
	kubectl logs "$pod" -n rook-ceph-secondary --all-containers=true || true
done

echo "=== CSI logs in rook-ceph namespace ==="
for pod in $(kubectl get po -n rook-ceph -l app=csi-rbdplugin -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || true); do
	echo "--- CSI RBD plugin logs for pod: $pod ---"
	kubectl logs "$pod" -n rook-ceph --all-containers=true || true
	echo "--- CSI RBD plugin container logs for pod: $pod ---"
	kubectl logs "$pod" -n rook-ceph -c csi-rbdplugin || true
done

for pod in $(kubectl get po -n rook-ceph -l app=csi-cephfsplugin -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || true); do
	echo "--- CSI CephFS plugin logs for pod: $pod ---"
	kubectl logs "$pod" -n rook-ceph --all-containers=true || true
done

for pod in $(kubectl get po -n rook-ceph -l app=rook-ceph.rbd.csi.ceph.com-ctrlplugin -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || true); do
	echo "--- CSI RBD provisioner logs for pod: $pod ---"
	kubectl logs "$pod" -n rook-ceph --all-containers=true || true
	echo "--- CSI RBD plugin container logs for pod: $pod ---"
	kubectl logs "$pod" -n rook-ceph -c csi-rbdplugin || true
done

for pod in $(kubectl get po -n rook-ceph -l app=csi-cephfsplugin-provisioner -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || true); do
	echo "--- CSI CephFS provisioner logs for pod: $pod ---"
	kubectl logs "$pod" -n rook-ceph --all-containers=true || true
done

echo "--- csi addons resources ---"
kubectl get encryptionkeyrotationjobs,encryptionkeyrotationcronjobs,reclaimspacecronjobs,reclaimspacejobs,networkfences,networkfenceclasses -A -oyaml

echo "--- csi addons controller logs ---"
for pod in $(kubectl get po -n csi-addons-system -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || true); do
	echo "--- Logs for pod: $pod ---"
	kubectl logs "$pod" -n csi-addons-system --all-containers=true || true
done

echo "=== RBD Pool Status - rook-ceph namespace ==="
TOOLBOX_POD=$(kubectl get po -n rook-ceph -l app=rook-ceph-tools -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -n "$TOOLBOX_POD" ]; then
	echo "--- Ceph status ---"
	kubectl exec -n rook-ceph "$TOOLBOX_POD" -- ceph status || true
	echo "--- RBD pool status ---"
	kubectl exec -n rook-ceph "$TOOLBOX_POD" -- ceph osd pool ls detail || true
	echo "--- RBD mirror pool status ---"
	kubectl exec -n rook-ceph "$TOOLBOX_POD" -- rbd mirror pool status replicapool --verbose || true
	echo "--- RBD images in pool ---"
	pool=replicapool
	echo "--- Images in pool: $pool ---"
	kubectl exec -n rook-ceph "$TOOLBOX_POD" -- rbd ls "$pool" || true
	echo "--- RBD mirror image status for pool: $pool ---"
	for image in $(kubectl exec -n rook-ceph "$TOOLBOX_POD" -- rbd ls "$pool" 2>/dev/null || true); do
		echo "--- Mirror status for image: $pool/$image ---"
		kubectl exec -n rook-ceph "$TOOLBOX_POD" -- rbd mirror image status "$pool/$image" || true
	done
else
	echo "Toolbox pod not found in rook-ceph namespace"
fi

echo "=== RBD Pool Status - rook-ceph-secondary namespace ==="
TOOLBOX_POD_SECONDARY=$(kubectl get po -n rook-ceph-secondary -l app=rook-ceph-tools -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -n "$TOOLBOX_POD_SECONDARY" ]; then
	echo "--- Ceph status ---"
	kubectl exec -n rook-ceph-secondary "$TOOLBOX_POD_SECONDARY" -- ceph status || true
	echo "--- RBD pool status ---"
	kubectl exec -n rook-ceph-secondary "$TOOLBOX_POD_SECONDARY" -- ceph osd pool ls detail || true
	echo "--- RBD mirror pool status ---"
	kubectl exec -n rook-ceph-secondary "$TOOLBOX_POD_SECONDARY" -- rbd mirror pool status replicapool --verbose || true
	echo "--- RBD images in pool ---"
	pool=replicapool
	echo "--- Images in pool: $pool ---"
	kubectl exec -n rook-ceph-secondary "$TOOLBOX_POD_SECONDARY" -- rbd ls "$pool" || true
	echo "--- RBD mirror image status for pool: $pool ---"
	for image in $(kubectl exec -n rook-ceph-secondary "$TOOLBOX_POD_SECONDARY" -- rbd ls "$pool" 2>/dev/null || true); do
		echo "--- Mirror status for image: $pool/$image ---"
		kubectl exec -n rook-ceph-secondary "$TOOLBOX_POD_SECONDARY" -- rbd mirror image status "$pool/$image" || true
	done
else
	echo "Toolbox pod not found in rook-ceph-secondary namespace"
fi

echo "=== Debug information capture complete ==="
