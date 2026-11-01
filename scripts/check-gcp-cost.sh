#!/usr/bin/env bash
# scripts/check-gcp-cost.sh
# Checks Google Cloud Artifact Registry storage and Cloud Run cost status for taite-board.

set -euo pipefail

PROJECT_ID="${GCP_PROJECT:-taite-board}"
REGION="${GCP_REGION:-asia-northeast1}"
REPO_NAME="taite"
SERVICE_NAME="taite-parser"

export CLOUDSDK_PYTHON="${CLOUDSDK_PYTHON:-/opt/homebrew/Caskroom/miniconda/base/bin/python3.13}"

echo "=========================================================="
echo " Google Cloud Cost & Resource Status Check"
echo " Project: ${PROJECT_ID} | Region: ${REGION}"
echo "=========================================================="
echo ""

echo "--- [1] Artifact Registry: ${REPO_NAME} ---"
if raw_output=$(gcloud artifacts repositories describe "${REPO_NAME}" --location="${REGION}" --project="${PROJECT_ID}" 2>&1); then
  size_str=$(echo "${raw_output}" | grep -i "Repository Size:" | awk '{print $3}' || echo "0MB")
  size_num=$(echo "${size_str}" | sed -E 's/[^0-9.]//g')
  if [[ -z "${size_num}" ]]; then
    size_num="0"
  fi
  
  echo "Repository URI:       asia-northeast1-docker.pkg.dev/${PROJECT_ID}/${REPO_NAME}"
  echo "Current Stored Size:  ${size_str}"
  echo "Free Tier Allowance:  500.00 MB / month (0.5 GB across project)"
  
  pct=$(awk "BEGIN {printf \"%.2f\", (${size_num} / 500.0) * 100}")
  echo "Free Tier Usage:      ${pct}% of monthly allowance"
  
  if (( $(echo "${size_num} <= 500" | bc -l) )); then
    echo "Estimated Cost:       \$0.00 / month (Fully covered by Free Tier)"
  else
    billable_mb=$(awk "BEGIN {printf \"%.2f\", ${size_num} - 500}")
    est_cost=$(awk "BEGIN {printf \"%.4f\", (${billable_mb} / 1024.0) * 0.10}")
    echo "Estimated Cost:       \$${est_cost} / month (${billable_mb} MB over free tier at \$0.10/GB)"
  fi
  
  echo ""
  echo "Active Cleanup Policies:"
  echo "${raw_output}" | grep -E "id: |action: |keepCount: |olderThan: " | sed 's/^/  /' || true
else
  echo "Warning: Could not fetch Artifact Registry details for ${REPO_NAME}."
fi

echo ""
echo "--- [2] Cloud Run: ${SERVICE_NAME} ---"
if cr_desc=$(gcloud run services describe "${SERVICE_NAME}" --region="${REGION}" --project="${PROJECT_ID}" --format=json 2>/dev/null); then
  min_inst=$(echo "${cr_desc}" | jq -r '.spec.template.metadata.annotations["autoscaling.knative.dev/minScale"] // "0"')
  max_inst=$(echo "${cr_desc}" | jq -r '.spec.template.metadata.annotations["autoscaling.knative.dev/maxScale"] // "default"')
  memory=$(echo "${cr_desc}" | jq -r '.spec.template.spec.containers[0].resources.limits.memory // "512Mi"')
  cpu=$(echo "${cr_desc}" | jq -r '.spec.template.spec.containers[0].resources.limits.cpu // "1000m"')
  url=$(echo "${cr_desc}" | jq -r '.status.url // "N/A"')
  
  echo "Endpoint:             ${url}"
  echo "Min Instances:        ${min_inst} (Scale-to-zero active: \$0 when idle)"
  echo "Max Instances:        ${max_inst} (Budget protection upper bound)"
  echo "Resources per Pod:    CPU: ${cpu}, Memory: ${memory}"
  echo "Free Tier Allowance:  2,000,000 requests/month, 360,000 GB-seconds/month"
  echo "Idle Cost:            \$0.00"
else
  echo "Warning: Could not fetch Cloud Run service details for ${SERVICE_NAME}."
fi

echo ""
echo "=========================================================="
echo " Official Pricing References:"
echo " - Artifact Registry: https://cloud.google.com/artifact-registry/pricing"
echo " - Cloud Run:         https://cloud.google.com/run/pricing"
echo "=========================================================="
