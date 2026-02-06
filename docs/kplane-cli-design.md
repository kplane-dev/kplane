# Kplane CLI Design

## Context
We need a flexible CLI that can evolve across provider and API version
differences. The architecture should mirror the benefits of Kind: a small
front-end command surface, a provider abstraction, and per-version
implementation packages. In v0 we implement:
- `kplane up`
- `kplane down`
- `kplane create cluster` (kubectl-like, creates a control plane)
- `kplane get-credentials` (update kubeconfig / set current context)

## Purpose
Provide a single, ergonomic CLI for bootstrapping a local dev environment
while remaining extensible for future providers and auth modes.

## Goals
- One command to stand up the control plane stack on a local cluster.
- Sensible defaults that mirror the operator’s managed path.
- Clear flags for images, namespaces, and auth policy.
- Provider abstraction to support non-Kind clusters later.
- Version-aware actions so future control plane or API changes can be
  implemented without breaking existing user workflows.

## Non-Goals (v0)
- Full cluster lifecycle management for all cloud providers.
- Replacing kubectl/helm for all operations.
- Managing downstream schedulers/controllers (separate repos).
- Implementing `kplane status` in the first cut.

## User-Facing UX (v0)

### `kplane up` (kind-style `create cluster`)
```
kplane up
  --provider kind
  --cluster-name kplane-management
  --namespace kplane-system
  --apiserver-image docker.io/kplanedev/apiserver:v0.0.2
  --operator-image docker.io/kplanedev/controlplane-operator:v0.0.2
  --etcd-image quay.io/coreos/etcd:v3.5.13
  --install-crds
```

### `kplane down` (kind-style `delete cluster`)
```
kplane down
  --provider kind
  --cluster-name kplane-management
```

### `kplane create cluster` (creates a ControlPlane)
```
kplane create cluster demo
  --class starter
  --endpoint https://demo.local
  --management-context kind-kplane-management
  --get-credentials
  --set-current
  --kubeconfig ~/.kube/config
```

### `kplane get-credentials` (gcloud-style kubeconfig)
```
kplane get-credentials
  --provider kind
  --cluster-name kplane-management
  --set-current
  --kubeconfig ~/.kube/config
```

### Compatibility Command Shapes
To align with Kind and GCP usage patterns, we support (or alias) these
command shapes in v0:

```
kind create cluster   -> kplane up
kind delete cluster   -> kplane down

gcloud container clusters get-credentials <cluster-name> \
  --region <cluster-region> \
  --project <project-id>
  -> kplane get-credentials <cluster-name>
```

## Default Flow (`kplane up`)
1) Create or reuse a Kind cluster (`kplane-management`).
2) Apply the embedded operator manifests (no local repo references).
3) Deploy etcd + shared apiserver + controlplane operator to `kplane-system`.
4) Install CRDs from the remote source and apply a default `ControlPlaneClass`.
5) Ensure the ingress port is available (default `8443`) and record it in the
   management cluster for later endpoint resolution.

## Provider Abstraction (Kind-like Architecture)
Kind isolates the CLI UX from cluster-specific actions through a narrow
provider interface and versioned implementation packages. We will mirror this:

### CLI Layer
- Parses flags and resolves defaults.
- Performs minimal orchestration:
  - Selects provider.
  - Selects versioned implementation.
  - Calls provider actions in order.

### Provider Layer
`Provider` interface encapsulates:
- Cluster lifecycle (create/reuse/delete) for `kplane up/down`.
- Image build and load.
- Kubeconfig retrieval and context switching.

Initial provider:
- `kind`: manages cluster lifecycle and image loading.

Future provider:
- `kubeconfig`: uses an existing cluster and skips image load.

### Versioned Implementations
Implementations are versioned by an explicit "stack version" to preserve
behavior across control plane or API changes. The CLI resolves the
implementation to run based on (in order):
1) `--stack-version` flag (explicit override)
2) A version hint in the `ControlPlaneClass` (future)
3) CLI default (e.g. latest supported)

Each version package defines:
- Default images (apiserver/operator/etcd).
- CRDs and manifests to apply.
- Feature flags (auth modes, issuer templates, etc.).

This allows future changes (e.g. CRD or auth defaults) without breaking
existing workflows or requiring new commands.

## Kubeconfig Switching
`kplane get-credentials` updates kubeconfig to make the target cluster context
the current default. Behavior:
- If the name matches a Kind cluster, it uses `kind get kubeconfig`.
- Otherwise it treats the name as a VCP, waits for readiness, fetches the
  `apiserver-kubeconfig` secret from the management cluster, and rewrites the
  server address to the ingress endpoint.
- Merges or writes to `--kubeconfig` (default `~/.kube/config`).
- If `--set-current`, sets the current context to the cluster’s context.

This command is intentionally small and focused so users can idiomatically
switch between clusters with a single Kplane command.

## Create Cluster (ControlPlane Creation)
`kplane create cluster` is a kubectl-like command that creates a `ControlPlane`
resource in the target management cluster. It should:
- Use the selected `ControlPlaneClass` (default starter).
- Set the endpoint if provided (otherwise defaults to the ingress endpoint
  `https://127.0.0.1:<port>/clusters/<name>/control-plane`).
- Remain provider-agnostic (pure API call once kubeconfig is active).

## Auth Defaults
Use ManagedIssuer by default, derived from the external endpoint:
- Default `ControlPlaneClass` sets `auth.policy=ManagedIssuer`.
- `issuerTemplate=https://{externalHost}`.

## Default Manifests
Apply a minimal stack:
- Namespace: `kplane-system`
- etcd deployment + service
- apiserver deployment + service
- controlplane operator deployment
- `ControlPlaneClass` (managed defaults)

## Configuration Storage
We should support persistent, user-editable config with sensible defaults.

### Location
Follow XDG and common CLI conventions:
- Primary: `${XDG_CONFIG_HOME:-~/.config}/kplane/config.yaml`
- Fallback: `~/.kplane/config.yaml` (if XDG is not set)

### Precedence
Resolution order (highest to lowest):
1) CLI flags
2) Environment variables (e.g. `KPLANE_*`)
3) Config file
4) Built-in defaults

### Shape (Draft)
```
apiVersion: config.kplane.io/v1alpha1
kind: KplaneConfig
currentProfile: default
profiles:
  default:
    provider: kind
    clusterName: kplane-management
    namespace: kplane-system
    kubeconfigPath: ~/.kube/config
    stackVersion: latest
    images:
      apiserver: docker.io/kplanedev/apiserver:v0.0.2
      operator: docker.io/kplanedev/controlplane-operator:v0.0.2
      etcd: quay.io/coreos/etcd:v3.5.13
    auth:
      policy: managed
      issuerTemplate: https://{externalHost}
    kind:
      nodeImage: kindest/node:v1.29.2
      configPath: ~/.config/kind/kplane-management.yaml
      ingressPort: 8443
```

Notes:
- Profiles allow multiple environments and quick switching.
- Provider-specific config is nested under the provider key.
- Versioned stack values live alongside defaults so upgrades are explicit.

## CLI Wiring (High Level)
- `cmd/kplane`:
  - Root command, subcommands: `up`, `down`, `create cluster`, `get-credentials`.
  - Resolves provider + stack version.
- `pkg/providers`:
  - `Provider` interface.
  - `kind` implementation.
- `pkg/stack`:
  - Single `latest` implementation for now.
  - Future versions can be introduced without changing the CLI surface.

## Open Questions
- Should `kplane up` create a sample `ControlPlaneEndpoint` + `ControlPlane`?
- Should auth keys be generated by the CLI or solely by the operator?
- Do we need a `--use-existing` flag to avoid rebuilding images?
