#!/usr/bin/env bash
set -euo pipefail

# Simple one-line deploy helper for local k3s / Kubernetes clusters.
# Usage (example):
# UI_USER=admin UI_PASS=admin SMPP_SYSTEM_ID=smpp-logger SMPP_PASSWORD=smpp-logger IMAGE=ghcr.io/overkillinc/smpp-logger:v1.0.10 DOMAIN=smpp-logger.example.net ./deploy-one-line.sh

NAMESPACE=${NAMESPACE:-smpp-logger}
IMAGE=${IMAGE:-ghcr.io/overkillinc/smpp-logger:v1.0.10}
DOMAIN=${DOMAIN:-smpp-logger.de.it-union.net}

# Dummy defaults — replace with secure credentials for production!
UI_USER=${UI_USER:-admin}
UI_PASS=${UI_PASS:-admin}
SMPP_SYSTEM_ID=${SMPP_SYSTEM_ID:-smpp-logger}
SMPP_PASSWORD=${SMPP_PASSWORD:-smpp-logger}

echo "Deploying to namespace: $NAMESPACE"
echo "Using image: $IMAGE"
echo "Ingress host: $DOMAIN"

echo "Creating/ensuring namespace $NAMESPACE"
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

echo "Creating secrets (will overwrite existing secrets)"
kubectl -n "$NAMESPACE" create secret generic smpp-logger-ui-credentials \
  --from-literal=SMPP_LOGGER_UI_USER="$UI_USER" \
  --from-literal=SMPP_LOGGER_UI_PASS="$UI_PASS" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "$NAMESPACE" create secret generic smpp-logger-auth \
  --from-literal=system-id="$SMPP_SYSTEM_ID" \
  --from-literal=password="$SMPP_PASSWORD" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "Applying service manifest"
kubectl -n "$NAMESPACE" apply -f examples/kubernetes/smpp-logger-ui-service.yaml

echo "Applying deployment manifest (will be patched to use provided image)"
kubectl -n "$NAMESPACE" apply -f examples/kubernetes/deployment.yaml || true
kubectl -n "$NAMESPACE" set image deployment/smpp-logger smpp-logger="$IMAGE" --record || true

echo "Applying ingress (production) for host: $DOMAIN"
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: smpp-logger-ingress
  namespace: $NAMESPACE
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  ingressClassName: traefik
  tls:
    - hosts:
        - $DOMAIN
      secretName: smpp-logger-tls
  rules:
    - host: $DOMAIN
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: smpp-logger-ui
                port:
                  number: 8080
EOF

echo "Waiting for deployment rollout (timeout 120s)"
kubectl -n "$NAMESPACE" rollout status deployment/smpp-logger --timeout=120s || true

cat <<EOF

Deployment applied.
Verify:
  kubectl -n $NAMESPACE get pods
  curl -u $UI_USER:$UI_PASS "http://<node-ip>:31513/"   # NodePort
  curl -u $UI_USER:$UI_PASS "https://$DOMAIN/"        # via Ingress (ensure DNS points to cluster)

To deploy in one line (example):
  UI_USER=admin UI_PASS=admin SMPP_SYSTEM_ID=smpp-logger SMPP_PASSWORD=smpp-logger IMAGE=ghcr.io/overkillinc/smpp-logger:v1.0.10 DOMAIN=smpp-logger.example.net ./deploy-one-line.sh

WARNING: Default credentials are insecure. Replace UI and SMPP secrets before exposing to public networks.
EOF
