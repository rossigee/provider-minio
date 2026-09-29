#!/bin/bash

set -eo pipefail

endpoint=${1}
bucket_name=${2}
file_path=${3}
secret_name=${4}
secret_namespace=${5}

access_key=$(kubectl -n "${secret_namespace}" get secret "${secret_name}" -o jsonpath='{.data.AWS_ACCESS_KEY_ID}' | base64 -d)
secret_key=$(kubectl -n "${secret_namespace}" get secret "${secret_name}" -o jsonpath='{.data.AWS_SECRET_ACCESS_KEY}' | base64 -d)
export MC_HOST_minio=http://${access_key}:${secret_key}@${endpoint}

echo "Uploading object to bucket: ${bucket_name}"
echo "File path: ${file_path}"
echo "Endpoint: ${endpoint}"
echo "Access key: ${access_key}"
echo "Secret key: ${secret_key}"

# mc is installed to the suite's tool directory by the $(mc_bin) prerequisite of
# test-e2e, which exports GOBIN only for its own recipe. kuttl does not inherit
# that, so an unset GOBIN would expand to "/mc". Resolve it from the checkout
# root instead, which is where kuttl is invoked from, and fall back to PATH.
repo_root=$(git rev-parse --show-toplevel 2>/dev/null || echo "${PWD}/../../..")
mc="${MC_BIN:-${repo_root}/_output/bin/mc}"
if [ ! -x "${mc}" ]; then
  mc=$(command -v mc)
fi

"${mc}" cp --quiet --debug "${file_path}" "minio/${bucket_name}"
