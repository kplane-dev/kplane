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

## Download (Latest)

- [macOS (Apple Silicon)](https://github.com/kplane-dev/kplane/releases/latest/download/kplane-darwin-arm64)
- [macOS (Intel)](https://github.com/kplane-dev/kplane/releases/latest/download/kplane-darwin-amd64)
- [Linux (arm64)](https://github.com/kplane-dev/kplane/releases/latest/download/kplane-linux-arm64)
- [Linux (amd64)](https://github.com/kplane-dev/kplane/releases/latest/download/kplane-linux-amd64)
- [Windows (amd64)](https://github.com/kplane-dev/kplane/releases/latest/download/kplane-windows-amd64.exe)

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

Bring up the management plane (Kind):

```
./bin/kplane up
```

Bring up the management plane (k3s via k3d):

```
./bin/kplane up --provider k3s
```

Set the default provider (optional):

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
