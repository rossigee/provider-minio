# Webhook Timeout Fix

## Root Cause
The webhook timeout issues were caused by a Cilium Network Policy that blocked ingress traffic to providers on port 9443.

## Problem
The `crossplane-providers-ingress` CiliumNetworkPolicy in crossplane-system namespace only allowed ports 8080 and 9090, but Crossplane provider webhooks run on port 9443.

## Fix Applied
Added port 9443 to the ingress policy:

```bash
kubectl patch ciliumnetworkpolicy crossplane-providers-ingress -n crossplane-system --type='json' -p='[{"op": "add", "path": "/spec/ingress/0/toPorts/0/ports/-", "value": {"port": "9443", "protocol": "TCP"}}]'
```

## Verification
- ✅ Webhook validation now works without timeouts
- ✅ ServiceAccount controller can connect to MinIO
- ✅ Network path API server → webhook functions correctly

## Note
This fix should be applied to the GitOps configuration that manages the crossplane-system CiliumNetworkPolicies to ensure it persists across deployments.