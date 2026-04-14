# smpp-logger

[![CI](https://github.com/overkillinc/smpp-logger/actions/workflows/ci.yml/badge.svg)](https://github.com/overkillinc/smpp-logger/actions/workflows/ci.yml)
[![Release Workflow](https://github.com/overkillinc/smpp-logger/actions/workflows/release.yml/badge.svg)](https://github.com/overkillinc/smpp-logger/actions/workflows/release.yml)

`smpp-logger` is a small SMPP 3.4 debugging server for Kubernetes and container environments. It accepts a single credential pair, acknowledges incoming traffic, emits compact logs for message submissions, and returns synthetic delivery receipts for accepted `submit_sm` requests.


## Quickstart

Run locally (development):

```bash
go run ./cmd/smpp-logger
```

Build container (optional):

```bash
docker build -t ghcr.io/overkillinc/smpp-logger:local .
```

Run container (example):

```bash
docker run --rm -p 2775:2775 \
  -e SMPP_LOGGER_SYSTEM_ID=<SYSTEM_ID> \
  -e SMPP_LOGGER_PASSWORD=<PASSWORD> \
  ghcr.io/overkillinc/smpp-logger:latest
```

Kubernetes (one-line apply)

For a quick cluster install, apply the single-file example manifest in this repository. This will create a namespace, dummy secrets (for testing), the Deployment, Services, and an Ingress configured for Traefik and cert-manager:

```bash
kubectl apply -f https://raw.githubusercontent.com/overkillinc/smpp-logger/main/examples/kubernetes/smpp-logger-deploy.yaml
```

Replace the dummy credentials and DOMAIN in the manifest before exposing to public networks, or create Kubernetes secrets separately and use a pinned image tag for production.




## Configuration

All configuration is environment-variable based. Recommended variables:

| Variable | Default | Description |
| --- | --- | --- |
| `SMPP_LOGGER_LISTEN_ADDR` | `:2775` | TCP listen address for the SMPP server |
| `SMPP_LOGGER_SYSTEM_ID` | `smpp-logger` | Allowed SMPP login (set via secret) |
| `SMPP_LOGGER_PASSWORD` | *required* | Allowed SMPP password (set via secret) |
| `SMPP_LOGGER_LOG_FORMAT` | `text` | `text` or `json` |
| `SMPP_LOGGER_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown budget |


## Kubernetes

A single-file example manifest is available at `examples/kubernetes/smpp-logger-deploy.yaml` and includes Namespace, test-only dummy secrets, the Deployment, Services (NodePort), and an Ingress configured for Traefik + cert-manager. See Quickstart for the one-line `kubectl apply` example.

Warning: the single-file manifest contains dummy credentials and a placeholder domain. Replace the credentials and DOMAIN, or create Kubernetes secrets before applying, especially for production use.

To create Kubernetes secrets separately (recommended for production):

```bash
kubectl -n smpp-logger create secret generic smpp-logger-auth \
  --from-literal=system-id=<SYSTEM_ID> \
  --from-literal=password=<PASSWORD>

kubectl -n smpp-logger create secret generic smpp-logger-ui-credentials \
  --from-literal=SMPP_LOGGER_UI_USER=<USER> \
  --from-literal=SMPP_LOGGER_UI_PASS=<PASS>
```

If your environment requires authenticated pulls from GHCR, create an image pull secret and reference it in the Deployment:

```bash
kubectl -n smpp-logger create secret docker-registry ghcr-creds \
  --docker-server=ghcr.io --docker-username=<USER> --docker-password=<PAT> --docker-email=<EMAIL>
```

Apply the single-file manifest after replacing values or creating secrets as appropriate.



For a one-line deploy (quick testing), use the single-file manifest hosted in this repository:

```bash
kubectl apply -f https://raw.githubusercontent.com/overkillinc/smpp-logger/main/examples/kubernetes/smpp-logger-deploy.yaml
```

The single-file manifest contains dummy credentials and a placeholder domain — replace those values, or create Kubernetes secrets separately, before exposing to public networks.


## HTTP UI

The project includes a minimal HTTP UI protected by Basic Auth. The UI supports an input filter that queries the `/logs?q=` endpoint (case-insensitive substring match). For secure use:

- Do not expose the built-in HTTP UI directly to the public internet without TLS.
- Use Kubernetes Ingress + cert-manager or an external TLS terminator.
- Store UI credentials in k8s secrets and mount via env vars.


## Testing

Run unit and integration tests locally. Integration tests that exercise a running cluster are opt-in and require:

- `SMPP_TEST_TARGET_HOST` set to the target (or)
- `RUN_INTEGRATION=true` to test against localhost NodePort

Example:

```bash
export SMPP_TEST_TARGET_HOST=<node-ip-or-hostname>:30075
go test ./integration -v
```


## Contributing

Contributions are welcome. See CONTRIBUTING.md and CODE_OF_CONDUCT.md for guidance.


## License

This project is released under the MIT License. See LICENSE for details.


## Security

Do not commit secrets or credentials to this repository. If you discover a security issue, open a confidential issue or contact maintainers.


# Git hooks
Run `scripts/install-git-hooks.sh` to enable the project's pre-commit hook (requires Git >=2.9).
