#!/bin/bash
# Verify ServiceAccount CRD installation and functionality

set -e

echo "=== MinIO Provider ServiceAccount Verification ==="
echo

# Check if CRD is installed
echo "1. Checking if ServiceAccount CRD is installed..."
if kubectl get crd serviceaccounts.minio.crossplane.io &>/dev/null; then
    echo "✓ ServiceAccount CRD is installed"
    kubectl get crd serviceaccounts.minio.crossplane.io -o jsonpath='{.spec.versions[*].name}' | tr ' ' '\n'
else
    echo "✗ ServiceAccount CRD not found"
    exit 1
fi
echo

# Check provider status
echo "2. Checking MinIO provider status..."
PROVIDER_NAME="provider-minio"
if kubectl get provider $PROVIDER_NAME &>/dev/null; then
    echo "✓ Provider found"
    kubectl get provider $PROVIDER_NAME -o jsonpath='{.status.currentRevision}'
    echo
    kubectl get provider $PROVIDER_NAME -o jsonpath='{.spec.package}'
    echo
else
    echo "✗ Provider not found"
    exit 1
fi
echo

# Check if provider is healthy
echo "3. Checking provider health..."
HEALTHY=$(kubectl get provider $PROVIDER_NAME -o jsonpath='{.status.conditions[?(@.type=="Healthy")].status}')
if [ "$HEALTHY" == "True" ]; then
    echo "✓ Provider is healthy"
else
    echo "✗ Provider is not healthy"
    kubectl get provider $PROVIDER_NAME -o jsonpath='{.status.conditions[*]}' | jq
fi
echo

# List existing ServiceAccounts
echo "4. Listing existing ServiceAccounts..."
kubectl get serviceaccounts.minio.crossplane.io -A 2>/dev/null || echo "No ServiceAccounts found yet"
echo

# Check provider logs for ServiceAccount controller
echo "5. Checking provider logs for ServiceAccount controller..."
POD=$(kubectl get pods -n crossplane-system -l pkg.crossplane.io/provider=$PROVIDER_NAME -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$POD" ]; then
    echo "Provider pod: $POD"
    echo "Recent ServiceAccount controller logs:"
    kubectl logs -n crossplane-system $POD --tail=20 | grep -i serviceaccount || echo "No ServiceAccount logs found"
else
    echo "✗ Provider pod not found"
fi
echo

echo "=== Verification Complete ==="
echo
echo "To create a test ServiceAccount, use:"
echo "kubectl apply -f examples/minio.crossplane.io_serviceaccount.yaml"