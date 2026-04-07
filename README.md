# agent-go-docker

Claude Code 的 Docker 镜像，支持多架构（amd64/arm64），开箱即用的开发环境。

## 镜像变体

| 变体 | 基础镜像 | 内置语言环境 |
|------|----------|--------------|
| `latest` | `node:24-slim` | Node.js 24 + Python 3 |
| `java8` | `node:24-slim` | Node.js 24 + Python 3 + Java 8 (Eclipse Temurin) + Maven 3.9.14 |
| `java17` | `node:24-slim` | Node.js 24 + Python 3 + Java 17 (Eclipse Temurin) + Maven 3.9.14 |
| `go` | `node:24-slim` | Node.js 24 + Python 3 + Go 1.26.1 + Rust (via rustup) |
| `rust` | `node:24-slim` | Node.js 24 + Python 3 + Go 1.26.1 + Rust (via rustup) |

所有镜像内置开发工具链：

```
git · curl · wget · vim · neovim · ripgrep · fd-find · jq · tree · htop
build-essential · openssh-client · ca-certificates · sudo
Python 3 + pip + venv
```

## 快速开始

### 1. 安装 `agent-cc` 脚本

```bash
curl -fsSL https://raw.githubusercontent.com/mark0725/agent-go-docker/main/agent-cc -o ~/.local/bin/agent-cc
chmod +x ~/.local/bin/agent-cc
```

### 2. 设置环境变量

```bash
export AGENT_ID="default"
```

### 3. 启动 Claude Code

```bash
# 默认 latest 镜像
agent-cc

# 使用 Java 8 环境
agent-cc --java8

# 使用 Java 17 环境
agent-cc --java

# 传递额外参数
agent-cc -p '帮我写一个 Hello World'
```

## 持久化配置

`cc` 脚本自动配置以下卷挂载：

| 宿主机路径 | 容器内路径 | 用途 |
|-----------|-----------|------|
| `node_home` (Docker 卷) | `/home/node` | 用户 home 目录（含 Maven 缓存） |
| `~/.claude` | `/home/node/.claude` | Claude Code 配置与会话状态 |
| `~/.agents` | `/home/node/.agents` | Agent 环境变量配置 |

## 手动运行

```bash
docker run -it --rm --network=host \
  --user 0 \
  -e "HOST_UID=$(id -u)" \
  -e "HOST_GID=$(id -g)" \
  -e "AGENT_ID=default" \
  -e "HOME=/home/node" \
  -v node_home:/home/node \
  -v "${HOME}/.claude:/home/node/.claude" \
  -v "${HOME}/.agents:/home/node/.agents" \
  -v "$(pwd):/workspace/$(pwd|sed 's/\//_/g')" \
  ghcr.io/mark0725/agent-go-docker:latest
```

## 目录权限映射

容器默认以 `node` 用户（UID 1000）运行。通过 `HOST_UID`/`HOST_GID` 环境变量，entrypoint 自动将容器内 UID/GID 调整为与宿主机一致，确保容器内创建的文件在宿主机上拥有正确的属主。


## CI/CD

GitHub Actions 自动构建并推送镜像到 GHCR：

| 触发条件 | 推送 tags |
|---------|----------|
| 推送至 `main` 分支 | `latest` · `java8` · `java17` · `go` · `rust` · `base` |
| 打 `v1.0.0` tag | `1.0.0-latest` · `1.0.0-java8` · `1.0.0-java17` · `1.0.0-go` · `1.0.0-rust` |
| PR | 仅构建验证，不推送 |

## 镜像地址

```
ghcr.io/mark0725/agent-go-docker:latest
ghcr.io/mark0725/agent-go-docker:java8
ghcr.io/mark0725/agent-go-docker:java17
ghcr.io/mark0725/agent-go-docker:go
ghcr.io/mark0725/agent-go-docker:rust
```

## License

MIT
