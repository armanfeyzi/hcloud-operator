#!/usr/bin/env bash
# Assert kro expanded a HetznerWebStack into expected HKIC child CRs (no Hetzner API).
set -euo pipefail

STACK_NAME="${1:-demo}"
NS="${2:-default}"

echo "Checking HetznerWebStack child CRs for stack=${STACK_NAME} namespace=${NS} ..."

kubectl get hcloudnetwork "${STACK_NAME}-net" -n "${NS}" -o jsonpath='{.spec.ipRange}{"\n"}' | grep -q '10\.90'
kubectl get hcloudserver "${STACK_NAME}-srv" -n "${NS}" -o jsonpath='{.spec.serverType}{"\n"}' | grep -q 'cx23'
kubectl get hcloudserver "${STACK_NAME}-srv" -n "${NS}" -o jsonpath='{.spec.networkRef.name}{"\n"}' | grep -q "${STACK_NAME}-net"
kubectl get hcloudvolume "${STACK_NAME}-vol" -n "${NS}" -o jsonpath='{.spec.size}{"\n"}' | grep -q '20'
kubectl get hcloudloadbalancer "${STACK_NAME}-lb" -n "${NS}" -o jsonpath='{.spec.loadBalancerType}{"\n"}' | grep -q 'lb11'

echo "OK: expected child CRs present with propagated spec fields."
