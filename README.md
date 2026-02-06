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
- Docker (for Kind)
- `kind` in your PATH
- `kubectl` in your PATH

## Getting Started

Build the CLI:

```
go build -o ./bin/kplane ./cmd/kplane
```

Bring up the management plane:

```
./bin/kplane up
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

- `kplane up` creates (or reuses) a Kind cluster and installs the management
  plane stack (etcd, shared apiserver, controlplane-operator, CRDs).
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

## Common Commands

```
./bin/kplane up
./bin/kplane down
./bin/kplane create cluster demo
./bin/kplane get clusters
./bin/kplane get-credentials demo
./bin/kplane config use-context kplane-demo
```
