#!/usr/bin/env bash
# Poll GHCR manifest until available, then attempt k8s deploy
set -euo pipefail
LOG=/home/copilot/smpp-logger/ci-monitor.log
GHCR_BASE="https://ghcr.io/v2/overkillinc/smpp-logger/manifests"
TAGS=("v1.0.7" "staging" "latest")
TIMEOUT=${TIMEOUT:-1800}
INTERVAL=${INTERVAL:-30}
end=$((SECONDS+TIMEOUT))
{
  echo "CI monitor started at $(date -u)" 
  echo "Timeout=${TIMEOUT}s interval=${INTERVAL}s"
} >> "$LOG"

while [ $SECONDS -lt $end ]; do
  for tag in "${TAGS[@]}"; do
    code=$(curl -s -o /dev/null -w "%{http_code}" -I "$GHCR_BASE/$tag" || echo 000)
    echo "[$(date -u '+%Y-%m-%dT%H:%M:%SZ')] check tag=$tag status=$code" >> "$LOG"
    if [ "$code" = "200" ]; then
      echo "[$(date -u)] manifest available for $tag" >> "$LOG"
      echo "IMAGE_AVAILABLE=$tag" >> "$LOG"
      # Attempt k8s deployment
      if command -v kubectl >/dev/null 2>&1; then
        echo "[$(date -u)] applying k8s manifest" >> "$LOG"
        kubectl apply -f /home/copilot/smpp-logger/examples/kubernetes/deployment.yaml >> "$LOG" 2>&1 || true
        echo "[$(date -u)] waiting for deployment to be ready" >> "$LOG"
        kubectl rollout status deployment/smpp-logger --timeout=120s >> "$LOG" 2>&1 || echo "rollout failed or timed out" >> "$LOG"
        echo "[$(date -u)] pods:" >> "$LOG"
        kubectl get pods -l app=smpp-logger -o wide >> "$LOG" 2>&1 || true
      else
        echo "[$(date -u)] kubectl not available, skipping deploy" >> "$LOG"
      fi
      exit 0
    fi
  done
  sleep "$INTERVAL"
done
echo "[$(date -u)] timeout waiting for GHCR manifest" >> "$LOG"
exit 2
