# smpp-logger

`smpp-logger` is a small SMPP 3.4 debugging server for Kubernetes and container environments. It accepts a single credential pair, acknowledges incoming traffic, emits compact logs for message submissions, and returns immediate synthetic success delivery receipts for accepted `submit_sm` requests.

## Supported flow

- `bind_receiver`
- `bind_transmitter`
- `bind_transceiver`
- `enquire_link`
- `submit_sm`
- synthetic `deliver_sm` delivery receipts for `bind_transceiver` sessions
- `unbind`

`bind_transmitter` sessions can submit messages but do not receive `deliver_sm` receipts, which keeps the behavior aligned with SMPP bind semantics.

## Configuration

All configuration is environment-variable based.

| Variable | Default | Description |
| --- | --- | --- |
| `SMPP_LOGGER_LISTEN_ADDR` | `:2775` | TCP listen address for the SMPP server |
| `SMPP_LOGGER_SYSTEM_ID` | `smpp-logger` | Allowed SMPP login |
| `SMPP_LOGGER_PASSWORD` | `smpp-logger` | Allowed SMPP password |
| `SMPP_LOGGER_LOG_FORMAT` | `text` | `text` or `json` |
| `SMPP_LOGGER_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown budget |

## Local development

Run the server directly:

```bash
go run ./cmd/smpp-logger
```

Run the tests:

```bash
go test ./...
```

Default startup listens on `:2775` with the credential pair `smpp-logger` / `smpp-logger`.

## Logs

Text mode is compact by default:

```text
event=submit login="smpp-logger" sender="alice" destination="15551234567" text="hello world" message_id="msg-0000000001" client="10.0.0.15:51982" seq=3
event=receipt login="smpp-logger" sender="15551234567" destination="alice" text="hello world" message_id="msg-0000000001" client="10.0.0.15:51982" seq=1
```

Set `SMPP_LOGGER_LOG_FORMAT=json` for structured container logs.

## Container usage

Build the image locally:

```bash
docker build -t smpp-logger:local .
```

Run it:

```bash
docker run --rm -p 2775:2775 \
  -e SMPP_LOGGER_SYSTEM_ID=client \
  -e SMPP_LOGGER_PASSWORD=secret \
  ghcr.io/overkillinc/smpp-logger:latest
```

## Kubernetes

An example manifest lives at [`examples/kubernetes/deployment.yaml`](examples/kubernetes/deployment.yaml). It uses TCP socket probes on the SMPP port, which is enough for this single-purpose service.

Apply directly from GitHub (recommended for quick deploys):

```bash
kubectl apply -f https://raw.githubusercontent.com/overkillinc/smpp-logger/main/examples/kubernetes/deployment.yaml
# ensure the imagePullSecret exists in the namespace (uses GHCR token):
kubectl create secret docker-registry ghcr-creds --docker-server=ghcr.io --docker-username=<GH_USER> --docker-password=<PERSONAL_ACCESS_TOKEN> -n smpp-logger --dry-run=client -o yaml | kubectl apply -f -
```

The example exposes the SMPP port via a NodePort (30075) so the service can be reached from the host network. If a wildcard DNS like `*.de.it-union.net` is delegated to the server, the instance will be available at `<your-host>.de.it-union.net:30075`.

Quick smoke test (from this host or a machine that can reach the node):

```bash
# check TCP connectivity to nodePort
nc -zv <node-ip-or-hostname> 30075
# or
timeout 2 bash -c "</dev/tcp/<node-ip-or-hostname>/30075" && echo OK || echo FAIL
```

Integration tests (example): the repository includes integration tests that can exercise the live k3s-deployed service. Set the target host for tests and run the tests as normal (example):

```bash
export SMPP_TEST_TARGET_HOST=<node-ip-or-hostname>:30075
go test ./... -run Integration -v
```

## Release flow

Push a semantic version tag such as `v1.0.0` to trigger:

1. Go test execution
2. Multi-arch image publishing to `ghcr.io/<owner>/smpp-logger`
3. GitHub Release creation with the image digest
