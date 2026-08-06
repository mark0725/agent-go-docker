# agent-go-docker

Docker images for launching Claude Code container environments, a local startup script (`agent-go`), and an HTTP runner service (`runner/`).

## Directory Structure

- `Dockerfile`: Node.js 24 on Debian Bookworm, with Bookworm-provided Python plus Go and Rust toolchains.
- `Dockerfile.java*`: Java language-variant images built on top of the base image.
- `agent-go`: Local startup script that installs `agent-cc` / `agent-cc-web` / `agent-cc-tmux` commands.
- `entrypoint.sh`: Container entrypoint handling UID mapping, tmux/ttyd startup, etc.
- `runner/`: Go-based HTTP runner providing APIs to dynamically spin up agent containers per project.

## Build Instructions

### Build the Base Image Locally

```bash
docker build -t agent-go-docker:latest -f Dockerfile .
```

The Dockerfile uses the official Debian, crates.io, and npm sources by default. Override Debian with `--build-arg APT_MIRROR=...`, `--build-arg APT_SECURITY_MIRROR=...`, and npm with `--build-arg NPM_REGISTRY=...` when needed. `build.sh` passes USTC's HTTP Debian mirrors and `registry.npmmirror.com` for faster builds in China. To pull dependencies through a proxy, pass `--build-arg HTTP_PROXY=...`. The proxy variables are cleared at the end of the build:

```bash
docker build --build-arg HTTP_PROXY=http://10.1.2.12:8118 \
  -t agent-go-docker:latest -f Dockerfile .
```

### Build Language Variant Images

```bash
docker build -t agent-go-docker:java8  -f Dockerfile.java8 .
docker build -t agent-go-docker:java17 -f Dockerfile.java17 .
docker build -t agent-go-docker:java21 -f Dockerfile.java21 .
docker build -t agent-go-docker:java25 -f Dockerfile.java25 .
```

Go and Rust are included in `agent-go-docker:latest`; separate `go` and `rust` image variants are no longer required. Rust is installed globally under `/usr/local/cargo` and `/usr/local/rustup`, with its commands linked into `/usr/local/bin`, so every container user shares the same toolchain.
The legacy `agent-cc --go` and `agent-cc --rust` flags remain accepted as compatibility no-ops and use the base image.

`build.sh` publishes both amd64 and arm64 by default. For a faster local/single-architecture build, set `PLATFORM`, for example `PLATFORM=linux/amd64 ./build.sh`. Successful builds publish reusable BuildKit caches alongside each image tag.

The registry host is automatically added to `NO_PROXY`/`no_proxy`, so local image and cache pushes bypass the download proxy. Set `REGISTRY_CACHE=0` to skip remote cache import/export and push only the images, which is useful when the registry does not support BuildKit cache manifests or cache uploads are too large.

### Build the Runner Image

`runner/Dockerfile` also supports the `HTTP_PROXY` build arg and bundles Docker CLI for accessing the host Docker socket:

```bash
cd runner
docker build --build-arg HTTP_PROXY=http://10.1.2.12:8118 \
  -t agent-run:latest .
```

## Usage

### 1. Install the Startup Script

Make the script executable and install command symlinks:

```bash
chmod +x agent-go
./agent-go add
export PATH="$HOME/.local/bin:$PATH"
```

After installation the following commands are available:

- `agent-cc`: Launch the selected agent CLI interactively
- `agent-cc-web`: Launch the selected agent in a ttyd web terminal
- `agent-cc-tmux`: Launch the selected agent inside a shpool session

### 2. Basic Launch

```bash
agent-cc
```

Claude is used by default. Select Codex with `AGENT_TYPE`:

```bash
AGENT_TYPE=codex agent-cc
AGENT_TYPE=codex agent-cc-web
```

Claude starts with `--dangerously-skip-permissions`; Codex starts with `--dangerously-bypass-approvals-and-sandbox` inside the agent container.

### 3. Select an Image Variant

```bash
agent-cc --java8
agent-cc --java
agent-cc --java21
agent-cc --java25
```

`--java` is equivalent to `--java17`.

### 4. Pass Agent Arguments

```bash
agent-cc -p 'Help me review the code in the current directory'
AGENT_TYPE=codex agent-cc --search
```

### 5. Web / tmux Mode

```bash
agent-cc-web
agent-cc-tmux
```

### 6. Common Environment Variables

```bash
export AGENT_ID=default
export AGENT_TYPE=claude  # claude or codex
export AGENT_IMAGE_REGISTRY=ghcr.io/mark0725/agent-go-docker
export CLAUDE_HOME=$HOME/.claude
export CODEX_HOME=$HOME/.codex
export AGENTS_HOME=$HOME/.agents
export AGENTS_HUB=$HOME/.agents-hub
```

### 7. Run Directly with Docker

```bash
docker run -it --rm --network=host \
  --user 0 \
  -e "HOST_UID=$(id -u)" \
  -e "HOST_GID=$(id -g)" \
  -e "HOME=/home/node" \
  -e "AGENT_ID=default" \
  -e "AGENT_TYPE=claude" \
  -v node_home:/home/node \
  -v "$PWD:/workspace/$(pwd | sed 's#/#_#g')" \
  -v "$HOME/.claude:/home/node/.claude" \
  -v "$HOME/.codex:/home/node/.codex" \
  -v "$HOME/.agents:/home/node/.agents" \
  -v "$HOME/.agents-hub:/home/node/.agents-hub" \
  -w "/workspace/$(pwd | sed 's#/#_#g')" \
  ghcr.io/mark0725/agent-go-docker:latest \
  claude
```

## Runner Service

`runner/` is an HTTP service written in Go that listens on `:8080`. It dynamically starts and manages agent containers per request. All agent containers are attached to a user-defined bridge network called `agents-net`, which the runner creates on startup if it does not exist. Each agent's ttyd listens on a single fixed container port (7681); the port is **not** published to the host — every ttyd request is reverse-proxied by the runner over `agents-net`.

### Startup

On startup the runner ensures `agents-net` exists (`docker network create agents-net` if missing) and then attaches every new agent to it. The runner container must also be on `agents-net` so its reverse-proxy can resolve agent IDs through Docker's embedded DNS:

```bash
docker network create agents-net   # only if not already created by a previous runner start

docker run -d --name agent-run \
  --network=agents-net \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /data/work:/data/work \
  -v ${HOME}/.agents-hub:/data/hub \
  -e "HOST_UID=$(id -u)" \
  -e "HOST_GID=$(id -g)" \
  -e "AGENT_ID=default" \
  -e "CLAUDE_CONFIG=${HOME}/.claude.json" \
  -e "CLAUDE_HOME=${HOME}/.claude" \
  -e "CODEX_HOME=${HOME}/.codex" \
  -e "AGENTS_HUB=${HOME}/.agents-hub" \
  -e "AGENTS_HOME=${HOME}/.agents" \
  -e "AGENT_IMAGE_REGISTRY=ghcr.io/mark0725/agent-go-docker" \
  -e "AGENT_IMAGE_TAG=latest" \
  ghcr.io/mark0725/agent-run:latest
```

Because all ttyd traffic is reverse-proxied over `agents-net`, no host port range needs to be reserved for agents. The runner's own port (`8080` in the example) is the only one you need to publish.

If the runner was started on a different network, you can attach it without restarting:

```bash
docker network connect agents-net agent-run
```

### Authentication

By default there is no authentication — expose the runner port only to trusted networks. Set `RUNNER_AUTH_TOKEN` to enable token-based authentication:

```bash
-e "RUNNER_AUTH_TOKEN=$(openssl rand -hex 24)"
```

Once enabled, all `/api/*`, `/proxy/*`, and UI endpoints require authentication. Clients can authenticate via any of:

- `Authorization: Bearer <token>` — suitable for API/curl usage.
- Cookie `runner_token=<token>` — for persistent browser access.
- URL `?token=<token>` — on first successful match, the runner sets an HttpOnly cookie; subsequent page refreshes and iframe ttyd reverse-proxy requests carry the cookie automatically.

`/health` is always unauthenticated for health checks.

### Common Environment Variables

- `LISTEN_ADDR`: HTTP listen address, default `:8080`.
- `DOCKER_SOCK`: Host Docker socket, default `/var/run/docker.sock`.
- `AGENT_ID`: Default `AGENT_ID` injected into agent containers; used when the creation form field is left blank, default `default`.
- `AGENT_TYPE`: Per-container agent selected by the runner form/API. Supported values are `claude` (default) and `codex`.
- `HOST_UID` / `HOST_GID`: Host UID/GID passed through to agent containers, preventing workspace files from being owned by root. Recommended: `$(id -u)` / `$(id -g)`.
- `AGENT_IMAGE_REGISTRY` / `AGENT_IMAGE_TAG`: Agent image and tag.
- `RUNNER_AUTH_TOKEN`: Shared token for accessing runner pages and APIs, default empty (no authentication). See "Authentication" above.
- `PROJECT_ROOT`: Project workspace root directory, default `/data/work`.
- `PROJECT_HOME`: Overrides `${PROJECT_ROOT}/${PROJECT_ID}`, forcing the use of a single workspace directory.

### Workspace Directory Structure

```
/data/work/
  └── {PROJECT_ID}/              # One directory per project
        ├── main/                 # Default workspace (WORKSPACE_ID=main)
        └── {WORKSPACE_ID}/      # Git worktree, each a separate workspace
              └── ...             # Project source files
```

Each agent container mounts `/data/work/{PROJECT_ID}/{WORKSPACE_ID}` as its working directory (`-w`). `WORKSPACE_ID` defaults to `main`; additional workspaces correspond to git worktrees within the same project.
- `CLAUDE_HOME` / `CODEX_HOME` / `AGENTS_HOME` / `AGENTS_HUB`: Host-side directories mounted into agent containers at `/home/node/.{claude,codex,agents,agents-hub}`. Defaults are based on the runner user's `$HOME`.
- `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_BASE_URL`, `HTTP_PROXY`, `HTTPS_PROXY`, `PROXY_URL`: Passed through to agent containers.

### Form Field Directory Sources

The create/edit agent form provides selectors for the following fields:

- **AGENT_TYPE**: Selects Claude or Codex. Claude starts with `--dangerously-skip-permissions`; Codex starts with `--dangerously-bypass-approvals-and-sandbox` inside the agent container.
- **AGENT_ID**: Supports free-text input and suggestions from directory names under `/data/hub/agents` on the host.
- **Project ID**: Searchable selector populated from directory names under `PROJECT_ROOT` (default `/data/work`).
- **Workspace ID**: Selector populated from directory names under `PROJECT_ROOT/{projectId}`; updated dynamically as Project ID changes.

The selected type is stored in the container as the `AGENT_TYPE` environment variable and the `agent-go-runner.agent-type` Docker label. Existing runner containers without this label are treated as Claude agents.
