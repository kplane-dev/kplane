```
  _  __      _                  
 | |/ /     | |                 
 | ' / _ __ | | __ _ _ __   ___ 
 |  < | '_ \| |/ _` | '_ \ / _ \
 | . \| |_) | | (_| | | | |  __/
 |_|\_\ .__/|_|\__,_|_| |_|\___|
      | |                       
      |_|                       
```
# kplane [Experimental]

Kplane is a CLI for running a local management plane and creating virtual
control planes (VCPs). Each VCP is a logical Kubernetes control plane backed
by a shared API server, isolated by request path
(`.../clusters/<name>/control-plane`).

## Why This Matters

Traditional Kubernetes control planes and clusters scale out by duplicating
controllers, apiservers, and supporting infrastructure. That means per-cluster
CPU and memory overhead just to keep the lights on. The
[multicluster-runtime](https://github.com/kubernetes-sigs/multicluster-runtime)
project showed that controllers can be made multi-cluster aware and share
resources effectively. The apiserver side has had many proposals, but nothing
has landed upstream.

**Baseline kplane virtual control planes (VCPs) use about 0.8MB of memory per control plane.**

Kplane explores a practical path forward: try to keep compatibility with upstream API
contracts while virtualizing the apiserver, scheduler, and controller manager.
([kplane-dev/apiserver](https://github.com/kplane-dev/apiserver)). The
goal is to understand upstream limitations, solve them in a compatible way, and
advocate the right upstream approaches.

Highlights from the apiserver design:

- Single store per resource with cluster-scoped key rewriting and one watchcache.
- Server-owned cluster labels to enforce isolation and cache keying. (temporarily due to upstream limitations)
- Per-cluster admission, webhook, and namespace lifecycle environments.
- Shared client and informer pools to reduce per-cluster overhead.
- CEL runtime caching and storage diagnostics for safer operations.

## Install (Homebrew)

```
brew tap kplane-dev/tap
brew install kplane
```

## Install (Manual)

Install to your PATH:

```
curl -fsSL https://raw.githubusercontent.com/kplane-dev/kplane/main/scripts/install.sh | sh
```

The installer adds `alias kp="kplane"` to your shell rc file. Restart your
shell (or run `source ~/.zshrc` / `source ~/.bashrc`) to use `kp`.

Or download manually and make it executable:

```
chmod +x ./kplane-<os>-<arch>
```

macOS Gatekeeper (temporary): unsigned binaries may be blocked. If you see a
warning, run:

```
xattr -d com.apple.quarantine ./kplane-<os>-<arch>
```

## Prereqs

- Go 1.22+ (to build the CLI)
- `kubectl` in your PATH
- Docker (for local providers)
- One of:
  - `kind` in your PATH (default)
  - `k3d` in your PATH (for k3s-in-docker)

## Getting Started

Build the CLI:

```
go build -o ./bin/kplane ./cmd/kplane
```

Bring up the management plane (default provider: Kind):

```
./bin/kplane up
```

Or bring up the management plane with k3s (via k3d):

```
./bin/kplane up --provider k3s
```

If you want future commands to target a specific provider like k3s:

```
./bin/kplane config set-provider k3s
```

Create a virtual control plane:

```
./bin/kplane create cluster demo
```

Fetch credentials and switch context:

```
./bin/kplane get-credentials demo
./bin/kplane config use-context kplane-demo
```

Verify:

```
kubectl get ns
```

## How It Works

- `kplane up` creates (or reuses) a local management cluster (Kind or k3s via
  k3d) and installs the management plane stack (etcd, shared apiserver,
  controlplane-operator, CRDs).
- `kplane create cluster <name>` creates a `ControlPlane` and a
  `ControlPlaneEndpoint` in the management cluster.
- Each VCP is served by the shared apiserver, isolated by path:
  `https://127.0.0.1:<port>/clusters/<name>/control-plane`.
- The CLI stores the chosen ingress port in the management cluster so all
  kubeconfigs resolve to the correct endpoint.

## Roadmap

Planned for later releases:
- Multi-cluster scheduler
- Multi-cluster controller manager
- Worker node management (join/leave)

## Commands

- `kplane up` — creates or reuses the local management cluster (Kind or k3s
  via k3d) and installs the management plane stack (etcd, shared apiserver,
  controlplane-operator, CRDs).
- `kplane down` — deletes the management cluster.
- `kplane create cluster <name>` — creates a `ControlPlane` and
  `ControlPlaneEndpoint` and writes a VCP kubeconfig context.
- `kplane cc <name>` — alias for `kplane create cluster <name>`.
- `kplane get clusters` — lists the management cluster and existing VCPs.
- `kplane get-credentials <name>` — writes kubeconfig for a local management
  cluster or VCP and optionally switches the current context.
- `kplane config use-context <name>` — switches your kubeconfig context (aliasing
  `kubectl config use-context`).
