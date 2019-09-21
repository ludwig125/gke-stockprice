#!/bin/bash
# Exit on any error
set -e

cd `dirname $0`

IMAGE_NAME="$1"
echo $IMAGE_NAME
../../../kustomize edit set image gcr.io/gke-stockprice/gke-stockprice:"${IMAGE_NAME}"
