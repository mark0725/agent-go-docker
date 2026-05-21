# agent-go-docker

Docker images for launching Claude Code container environments, a local startup script (`agent-go`), and an HTTP runner service (`runner/`).

## Directory Structure

- `Dockerfile` / `Dockerfile.<variant>`: Base image and language-variant images.
- `agent-go`: Local startup script that installs `agent-cc` / `agent-cc-web` / `agent-cc-tmux` commands.
- `entrypoint.sh`: Container entrypoint handling UID mapping, tmux/ttyd startup, etc.
- `runner/`: Go-based HTTP runner providing APIs to dynamically spin up agent containers per project.

## Build Instructions

### Build the Base Image Locally

```bash
docker build -t agent-go-docker:latest -f Dockerfile .
```

To pull dependencies through a proxy, pass `--build-arg HTTP_PROXY=...`. The proxy variables are cleared at the end of the build:

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
docker build -t agent-go-docker:go     -f Dockerfile.go .
docker build -t agent-go-docker:rust   -f Dockerfile.rust .
```

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

- `agent-cc`: Launch Claude Code interactive CLI
- `agent-cc-web`: Launch ttyd web terminal + tmux
- `agent-cc-tmux`: Launch Claude Code inside tmux

### 2. Basic Launch

```bash
agent-cc
```

### 3. Select an Image Variant

```bash
agent-cc --java8
agent-cc --java
agent-cc --java21
agent-cc --java25
agent-cc --go
agent-cc --rust
```

`--java` is equivalent to `--java17`.

### 4. Pass Claude Arguments

```bash
agent-cc -p 'Help me review the code in the current directory'
```

### 5. Web / tmux Mode

```bash
agent-cc-web
agent-cc-tmux
```

### 6. Common Environment Variables

```bash
export AGENT_ID=default
export AGENT_IMAGE_REGISTRY=ghcr.io/mark0725/agent-go-docker
export CLAUDE_HOME=$HOME/.claude
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
  -v node_home:/home/node \
  -v "$PWD:/workspace/$(pwd | sed 's#/#_#g')" \
  -v "$HOME/.claude:/home/node/.claude" \
  -v "$HOME/.agents:/home/node/.agents" \
  -v "$HOME/.agents-hub:/home/node/.agents-hub" \
  -w "/workspace/$(pwd | sed 's#/#_#g')" \
  ghcr.io/mark0725/agent-go-docker:latest \
  claude
```

## Runner Service

`runner/` is an HTTP service written in Go that listens on `:8080`. It dynamically starts and manages agent containers per request, assigning ttyd web terminal ports from the `7681-7780` range per project.

### Startup

It is recommended to run the runner on the host network so that agents share the same network, keeping the reverse proxy path short:

```bash
docker run -d --name agent-run \
  --network=host \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /data/work:/data/work \
  -v ${HOME}/.agents-hub:/data/hub \
  -e "HOST_UID=$(id -u)" \
  -e "HOST_GID=$(id -g)" \
  -e "AGENT_ID=default" \
  -e "CLAUDE_HOME=${HOME}/.claude" \
  -e "CODEX_HOME=${HOME}/.codex" \
  -e "AGENTS_HUB=${HOME}/.agents-hub" \
  -e "AGENTS_HOME=${HOME}/.agents" \
  -e "AGENT_IMAGE_REGISTRY=ghcr.io/mark0725/agent-go-docker" \
  -e "AGENT_IMAGE_TAG=latest" \
  ghcr.io/mark0725/agent-run:latest
```

Agent containers are always started with `--network=host`. ttyd binds directly to host ports `7681-7780`, so no `-p` mapping is needed.

If the runner must run on a bridge network, ensure it can reverse-proxy to agent ttyd on the host:

```bash
docker run -d --name agent-run \
  -p 8080:8080 \
  --add-host=host.docker.internal:host-gateway \
  -e "RUNNER_PROXY_HOST=host.docker.internal" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  ...
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
- `HOST_UID` / `HOST_GID`: Host UID/GID passed through to agent containers, preventing workspace files from being owned by root. Recommended: `$(id -u)` / `$(id -g)`.
- `AGENT_IMAGE_REGISTRY` / `AGENT_IMAGE_TAG`: Agent image and tag.
- `RUNNER_PROXY_HOST`: Hostname used when the runner reverse-proxies agent ttyd, default `127.0.0.1` (requires runner and agents to share the host network). For bridge networking, set to `host.docker.internal` and add `--add-host=host.docker.internal:host-gateway`.
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
- `PORT_RANGE_START` / `PORT_RANGE_END`: Assignable port range, default `7681-7780`.
- `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_BASE_URL`, `HTTP_PROXY`, `HTTPS_PROXY`, `PROXY_URL`: Passed through to agent containers.

### Form Field Directory Sources

The create/edit agent form provides autocomplete suggestions for the following fields:

- **AGENT_ID**: Lists directory names under `/data/hub/agents` on the host.
- **Project ID**: Lists directory names under `PROJECT_ROOT` (default `/data/work`) on the host.
- **Workspace ID**: Lists directory names under `PROJECT_ROOT/{projectId}` on the host; updated dynamically as Project ID changes.

All three fields also accept free-text input.
