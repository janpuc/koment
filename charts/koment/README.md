# koment

![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square)

This chart runs the unified authenticated koment service. It reads immutable
GitHub commits directly through the Git data API; it needs no checkout, volume,
database, migration, or application outbox.

```sh
helm install koment oci://ghcr.io/janpuc/charts/koment \
  --namespace koment --create-namespace \
  --set repositories[0].remote=example/project
```

The default repository is public `janpuc/koment`. Private reads and reviewed
writes use an existing Secret rather than a token in Helm values:

```sh
kubectl -n koment create secret generic koment-provider \
  --from-file=github-token=./github-token

helm upgrade --install koment oci://ghcr.io/janpuc/charts/koment \
  --namespace koment --create-namespace \
  --set github.existingSecret=koment-provider
```

Agent credentials are another existing Secret. The file stores SHA-256 hashes,
never bearer plaintext:

```yaml
version: 1
tokens:
  - name: coding-agent
    sha256: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
    repositories: [koment]
    permissions: [read, write]
```

Mount it with `auth.existingSecret`. Human identity comes from an OIDC proxy;
set `auth.trustedProxies` to only that proxy's network. The default trusts only
loopback and therefore fails closed for direct cluster traffic.

The application port serves `/`, `/mcp`, `/livez`, and `/readyz`. Metrics use a
separate listener and are enabled with `metrics.enabled=true`. The chart never
places either secret in Pod environment variables or rendered manifests.

`helm test <release>` verifies liveness, readiness, and the authenticated UI
boundary with a digest-pinned client image. CI installs the chart into Kind
against the image built from the pull request before running that test.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Affinity rules for pod assignment. |
| auth.credentialsKey | string | `"credentials.yaml"` | Key inside `auth.existingSecret` that holds the credentials file. |
| auth.existingSecret | string | `""` | Name of an existing Secret holding hashed, scoped agent credentials. |
| auth.humanWrites | bool | `false` | Allow identities asserted by a trusted proxy to create reviewed annotations. |
| auth.trustedProxies | list | `["127.0.0.1/32"]` | CIDRs allowed to assert forwarded human identity. The default trusts only loopback, so direct cluster traffic fails closed. |
| github.existingSecret | string | `""` | Name of an existing Secret holding the provider token. Required for private repositories and for reviewed writes; without it the synchronizer calls GitHub unauthenticated. |
| github.tokenKey | string | `"github-token"` | Key inside `github.existingSecret` that holds the token. |
| image.digest | string | `""` | Image digest. Set it to pin the exact image a tag currently points at. |
| image.pullPolicy | string | `"IfNotPresent"` | Image pull policy. |
| image.repository | string | `"ghcr.io/janpuc/koment"` | Container image repository. |
| image.tag | string | `""` | Image tag. Defaults to the chart's appVersion. |
| ingress.annotations | object | `{}` | Annotations to add to the Ingress. |
| ingress.className | string | `""` | IngressClass name. |
| ingress.enabled | bool | `false` | Create an Ingress for the application port. It carries authentication; only the health endpoints are public. |
| ingress.hosts | list | `[]` | Ingress hosts and paths. |
| ingress.tls | list | `[]` | Ingress TLS configuration. |
| metrics.dashboard.enabled | bool | `false` | Ship the Grafana dashboard as a sidecar-discoverable ConfigMap. |
| metrics.dashboard.label | string | `"grafana_dashboard"` | Label the Grafana sidecar watches for. |
| metrics.dashboard.labelValue | string | `"1"` | Value of the label the Grafana sidecar watches for. |
| metrics.enabled | bool | `false` | Expose Prometheus metrics on their own listener, so an ingress for the application port cannot expose them with it. |
| metrics.port | int | `9090` | Port for the metrics listener. |
| metrics.serviceMonitor.enabled | bool | `false` | Create a Prometheus Operator ServiceMonitor. |
| metrics.serviceMonitor.interval | string | `"30s"` | Scrape interval. |
| metrics.serviceMonitor.labels | object | `{}` | Extra labels for the ServiceMonitor, used by Prometheus selectors. |
| metrics.serviceMonitor.scrapeTimeout | string | `"10s"` | Scrape timeout. |
| networkPolicy.enabled | bool | `false` | Restrict ingress to the pod to the rules below. |
| networkPolicy.ingress | list | `[]` | Ingress rules applied when `networkPolicy.enabled` is set. Empty denies all inbound traffic. |
| nodeSelector | object | `{}` | Node selector for pod assignment. |
| podAnnotations | object | `{}` | Annotations to add to the pod. |
| podDisruptionBudget.enabled | bool | `false` | Create a PodDisruptionBudget. |
| podDisruptionBudget.minAvailable | int | `1` | Minimum available pods during voluntary disruption. |
| podSecurityContext | object | `{"fsGroup":65532,"runAsGroup":65532,"runAsNonRoot":true,"runAsUser":65532,"seccompProfile":{"type":"RuntimeDefault"}}` | Pod-level security context. The image ships a static binary and runs as a fixed non-root user. |
| replicaCount | int | `1` | Number of service replicas. Each holds its own rebuilt snapshot. |
| repositories | list | `[{"default":true,"defaultBranch":"main","id":"koment","name":"koment","provider":"github","remote":"janpuc/koment"}]` | Repositories this service assigns identity to and serves. `id` is the stable identity in URLs and credentials; moving a repository never changes it. |
| resources | object | `{"limits":{"memory":"128Mi"},"requests":{"cpu":"10m","memory":"32Mi"}}` | Resource requests and limits. Snapshots are held in memory, so the memory limit scales with repository size rather than with request volume. |
| securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true}` | Container-level security context. |
| service.port | int | `8080` | Port serving the UI, MCP, and the health endpoints. |
| service.type | string | `"ClusterIP"` | Service type for the authenticated application port. |
| serviceAccount.annotations | object | `{}` | Annotations to add to the ServiceAccount. |
| serviceAccount.create | bool | `true` | Create a ServiceAccount for the service. |
| serviceAccount.name | string | `""` | Name of the ServiceAccount. Generated from the release when empty. |
| syncInterval | string | `"1m"` | How often each repository's branch is resolved to a new immutable commit snapshot. |
| tests.image.digest | string | `"sha256:463eaf6072688fe96ac64fa623fe73e1dbe25d8ad6c34404a669ad3ce1f104b6"` | Digest that pins the test image, so a moving tag cannot change it. |
| tests.image.pullPolicy | string | `"IfNotPresent"` | Pull policy for the test image. |
| tests.image.repository | string | `"curlimages/curl"` | Image `helm test` uses to probe the service. |
| tests.image.runAsGroup | int | `102` | Numeric group id of the pinned test image. |
| tests.image.runAsUser | int | `101` | Numeric user id of the pinned test image. Kubernetes cannot verify `runAsNonRoot` against an image that names its user instead of numbering it, so the ids are pinned with the digest they belong to. |
| tests.image.tag | string | `"8.21.0"` | Tag of the test image, recorded alongside the digest that pins it. |
| tolerations | list | `[]` | Tolerations for pod assignment. |
| topologySpreadConstraints | list | `[]` | Topology spread constraints for pod assignment. |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.14.2](https://github.com/norwoodj/helm-docs/releases/v1.14.2)
