The mission is to provide a docker image to host in k8s as a simple way to debug smpp clients.

1. Github public repo overkillinc/smpp-logger
2. Golang implementation
3. Ok reply to correct smpp requests
4. Single set of login/password
5. Compact, easy to read container logs: login, sender, address, text for each incoming request
6. Unit test everything,
7. Integration tests
8. Build a multi-arch docker image, use github releases and gh images storage (I don't remember how it's called)

# Progress update (automated)
- Rotated UI credentials and backed them to /tmp/smpp-logger-ui-password.txt
- Removed temporary placeholder UI backend and manual ingress
- Deployed release v1.0.8 and updated deployment to use the release image
- Fixed deployment env var duplication; ensured UI reads SMPP_LOGGER_UI_USER and SMPP_LOGGER_UI_PASS from secret


# Staging deployment (automated)
- Created namespace smpp-logger-staging and copied relevant secrets (ghcr-creds, smpp-logger-ui-credentials, smpp-logger-auth when present).
- Duplicated deployment and service into the staging namespace and created an ingress at smpp-logger-staging.de.it-union.net for testing.
- Collected staging UI root at /tmp/ui_root_staging.html and /tmp/ui_logs_staging.txt for debugging.


# Staging routing fix
- Created ClusterIP service 'smpp-logger' in smpp-logger-staging to back the ingress (selector app=smpp-logger).
- Verified endpoints and performed in-cluster curl to service; tested node-level Host-header curl before and after service creation.
- Patched ingress to set spec.ingressClassName: traefik to ensure Traefik picks up routes if needed.


# Routing fixes applied
- Created smpp-logger-ui Service in smpp-logger namespace so the production ingress has a valid backend.
- Confirmed staging ingress now routes to the smpp-logger service in smpp-logger-staging and serves the UI (401 when unauthenticated).
- Verified original host responds and accepts basic auth.

