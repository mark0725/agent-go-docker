# Dockerfile.full
FROM node:24-slim

ARG HTTP_PROXY=
ARG USER_UID=1000
ARG USER_GID=1000
ARG TARGETARCH

ENV DEBIAN_FRONTEND=noninteractive
ENV SHELL=/bin/bash

# Set http proxy
RUN if [ -n "$HTTP_PROXY" ]; then \
        echo "Configuring proxy..."; \
        export http_proxy=$HTTP_PROXY; \
        export https_proxy=$HTTP_PROXY; \
        export proxy_url=$HTTP_PROXY; \
    fi

# ===== 基础工具 =====
RUN apt-get update && apt-get install -y \
    git \
    curl \
    wget \
    vim \
    neovim \
    ripgrep \
    fd-find \
    jq \
    tree \
    htop \
    build-essential \
    openssh-client \
    ca-certificates \
    sudo \
    locales \
    && rm -rf /var/lib/apt/lists/*

# 设置 locale
RUN sed -i '/en_US.UTF-8/s/^# //g' /etc/locale.gen && locale-gen
ENV LANG=en_US.UTF-8
ENV LANGUAGE=en_US:en
ENV LC_ALL=en_US.UTF-8

# ===== Python =====
RUN apt-get update && apt-get install -y \
    python3 \
    python3-pip \
    python3-venv \
    && rm -rf /var/lib/apt/lists/*

# ===== Go (可选) =====
ENV GOPROXY=https://proxy.golang.com.cn,direct
ARG GO_VERSION=1.26.1
RUN GOARCH=${TARGETARCH:-$(dpkg --print-architecture)} && \
    curl -fsSL https://go.dev/dl/go${GO_VERSION}.linux-${GOARCH}.tar.gz \
    | tar -C /usr/local -xzf -
ENV PATH="/usr/local/go/bin:${PATH}"

# ===== Rust (可选) =====
# RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
# ENV PATH="/root/.cargo/bin:${PATH}"

# ===== Docker CLI (可选，用于 DinD 场景) =====
# RUN curl -fsSL https://get.docker.com | sh

# ===== 安装 Claude Code =====
RUN npm install -g @anthropic-ai/claude-code

# ===== 安装 Codex =====
RUN npm i -g @openai/codex

# Unset http proxy
RUN if [ -n "$HTTP_PROXY" ]; then \
        unset http_proxy; \
        unset https_proxy; \
        unset proxy_url; \
    fi

# ===== UID 映射: gosu + entrypoint =====
RUN ARCH=${TARGETARCH:-$(dpkg --print-architecture)} && \
    curl -fsSL "https://github.com/tianon/gosu/releases/download/1.17/gosu-${ARCH}" -o /usr/local/bin/gosu && \
    chmod +x /usr/local/bin/gosu
COPY entrypoint.sh /entrypoint.sh

# 以 root 创建 node 用户的配置文件，运行时 entrypoint 会修正属主
WORKDIR /home/node
ENV GOPATH="/home/node/go"
ENV PATH="${GOPATH}/bin:/usr/local/go/bin:${PATH}"
RUN git config --global init.defaultBranch main \
    && chown -R node:node /home/node

ENTRYPOINT ["/entrypoint.sh"]
CMD ["claude"]
