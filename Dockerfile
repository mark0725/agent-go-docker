# syntax=docker/dockerfile:1

ARG RUST_VERSION=1.97.1

# Keep the builder on the same Debian release as the runtime image. Otherwise
# shpool may link against a newer glibc and fail to start on Bookworm.
FROM rust:${RUST_VERSION}-slim-bookworm AS shpool-builder

ARG HTTP_PROXY
ARG APT_MIRROR=
ARG APT_SECURITY_MIRROR=
ARG CARGO_REGISTRIES_CRATES_IO_INDEX=sparse+https://index.crates.io/
ARG CARGO_REGISTRIES_CRATES_IO_PROTOCOL=sparse

ENV HTTP_PROXY=${HTTP_PROXY}
ENV HTTPS_PROXY=${HTTP_PROXY}
ENV PROXY_URL=${HTTP_PROXY}
ENV DEBIAN_FRONTEND=noninteractive

RUN if [ -n "${APT_SECURITY_MIRROR}" ]; then \
        sed -i \
            -e "s|^URIs: http://deb.debian.org/debian-security$|URIs: ${APT_SECURITY_MIRROR}|" \
            /etc/apt/sources.list.d/debian.sources; \
    fi && \
    if [ -n "${APT_MIRROR}" ]; then \
        sed -i \
            -e "s|^URIs: http://deb.debian.org/debian$|URIs: ${APT_MIRROR}|" \
            /etc/apt/sources.list.d/debian.sources; \
    fi && \
    apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    pkg-config \
    libssl-dev \
    && rm -rf /var/lib/apt/lists/*

RUN --mount=type=cache,target=/usr/local/cargo/registry,sharing=locked \
    --mount=type=cache,target=/usr/local/cargo/git,sharing=locked \
    cargo install --locked shpool --root /usr/local && \
    chmod +x /usr/local/bin/shpool

FROM node:24-bookworm-slim

ARG USER_UID=1000
ARG USER_GID=1000
ARG TARGETARCH
ARG DOCKER_VERSION=27.1.0
ARG TTYD_VERSION="1.7.7"
ARG GIT_VERSION=2.49.1
ARG GO_VERSION=1.26.5
ARG APT_MIRROR=
ARG APT_SECURITY_MIRROR=
ARG NPM_REGISTRY=https://registry.npmjs.org

ARG HTTP_PROXY
ENV HTTP_PROXY=${HTTP_PROXY}
ENV HTTPS_PROXY=${HTTP_PROXY}
ENV PROXY_URL=${HTTP_PROXY}

ENV DEBIAN_FRONTEND=noninteractive
ENV SHELL=/bin/bash

RUN if [ -n "${APT_SECURITY_MIRROR}" ]; then \
        sed -i \
            -e "s|^URIs: http://deb.debian.org/debian-security$|URIs: ${APT_SECURITY_MIRROR}|" \
            /etc/apt/sources.list.d/debian.sources; \
    fi && \
    if [ -n "${APT_MIRROR}" ]; then \
        sed -i \
            -e "s|^URIs: http://deb.debian.org/debian$|URIs: ${APT_MIRROR}|" \
            /etc/apt/sources.list.d/debian.sources; \
    fi && \
    apt-get update && apt-get install -y --no-install-recommends \
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
    tzdata \
    locales \
    zlib1g-dev \
    libssl-dev \
    libcurl4-openssl-dev \
    libexpat1-dev \
    gettext \
    tcl \
    python3 \
    python-is-python3 \
    python3-pip \
    python3-venv \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fL --http1.1 --retry 8 --retry-delay 3 --retry-all-errors --connect-timeout 20 \
    "https://www.kernel.org/pub/software/scm/git/git-${GIT_VERSION}.tar.gz" -o /tmp/git.tar.gz \
    && tar -xzf /tmp/git.tar.gz -C /tmp \
    && make -C "/tmp/git-${GIT_VERSION}" prefix=/usr/local all \
    && make -C "/tmp/git-${GIT_VERSION}" prefix=/usr/local install \
    && rm -rf "/tmp/git-${GIT_VERSION}" /tmp/git.tar.gz


RUN ARCH=$(uname -m) && \
    case ${ARCH} in \
        x86_64)  TTYD_ARCH="x86_64" ;; \
        aarch64) TTYD_ARCH="aarch64"  ;; \
        *)       echo "Unsupported architecture: ${ARCH}"; exit 1 ;; \
    esac && \
    TTYD_URL="https://github.com/tsl0922/ttyd/releases/download/${TTYD_VERSION}/ttyd.${TTYD_ARCH}" && \
    wget --tries=8 --timeout=20 -O /usr/local/bin/ttyd ${TTYD_URL} && \
    chmod +x /usr/local/bin/ttyd

ENV PATH="/usr/local/bin:${PATH}"

# 设置 locale
RUN sed -i '/en_US.UTF-8/s/^# //g' /etc/locale.gen && locale-gen
ENV LANG=en_US.UTF-8
ENV LANGUAGE=en_US:en
ENV LC_ALL=en_US.UTF-8

# ===== Docker CLI (可选，用于 DinD 场景) =====
RUN curl -fL --http1.1 \
        --retry 8 --retry-delay 3 --retry-all-errors --connect-timeout 20 \
        -o /tmp/docker.tgz \
        "https://download.docker.com/linux/static/stable/$(uname -m)/docker-${DOCKER_VERSION}.tgz" && \
    tar -xzf /tmp/docker.tgz -C /usr/local/bin --strip-components=1 docker/docker && \
    chmod +x /usr/local/bin/docker && \
    rm -f /tmp/docker.tgz

RUN groupadd --gid 999 docker && usermod -aG docker node

COPY --from=shpool-builder /usr/local/bin/shpool /usr/local/bin/shpool
RUN shpool version

# shpool 重连(刷 ttyd 页面)默认 session_restore_mode = "screen",只回放一屏。
# output_spool_lines 只是内存缓冲上限,真正决定回放多少的是 session_restore_mode。
# 改成 { lines = 20000 } 才能在刷新后看到 20000 行历史;lines 不能超过 output_spool_lines。
RUN mkdir -p /etc/shpool && \
    printf 'output_spool_lines = 20000\nsession_restore_mode = { lines = 20000 }\n' > /etc/shpool/config.toml && \
    chmod 0644 /etc/shpool/config.toml

# ===== 安装 Claude Code =====
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    npm install -g --registry="${NPM_REGISTRY}" @anthropic-ai/claude-code

# ===== 安装 Codex =====
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    npm install -g --registry="${NPM_REGISTRY}" @openai/codex

# Verify Node.js 24 and the Debian Bookworm-provided Python runtime before
# adding the separately managed Go and Rust toolchains below.
RUN node --version && npm --version && python --version && python3 --version && pip3 --version

# ===== Go =====
ENV GOPROXY=https://proxy.golang.com.cn,direct
ENV GOPATH=/home/node/go
RUN GOARCH=${TARGETARCH:-$(dpkg --print-architecture)} && \
    curl -fL --retry 8 --retry-delay 3 --retry-all-errors --connect-timeout 20 \
        -o /tmp/go.tgz "https://go.dev/dl/go${GO_VERSION}.linux-${GOARCH}.tar.gz" && \
    tar -C /usr/local -xzf /tmp/go.tgz && \
    rm -f /tmp/go.tgz && \
    ln -sf /usr/local/go/bin/go /usr/local/bin/go && \
    ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt && \
    printf '%s\n' \
        'export GOPATH="/home/node/go"' \
        'export PATH="/home/node/go/bin:/usr/local/go/bin:/usr/local/bin:$PATH"' \
        > /etc/profile.d/go-path.sh && \
    chmod 0644 /etc/profile.d/go-path.sh
ENV PATH="${GOPATH}/bin:/usr/local/go/bin:/usr/local/bin:${PATH}"

# ===== Rust =====
ENV CARGO_HOME=/usr/local/cargo
ENV RUSTUP_HOME=/usr/local/rustup
RUN mkdir -p "${CARGO_HOME}" "${RUSTUP_HOME}" && \
    curl --proto '=https' --tlsv1.2 -fL \
        --retry 8 --retry-delay 3 --retry-all-errors --connect-timeout 20 \
        -o /tmp/rustup.sh https://sh.rustup.rs && \
    sh /tmp/rustup.sh -y --no-modify-path && \
    rm -f /tmp/rustup.sh && \
    chmod -R a+rwX "${CARGO_HOME}" "${RUSTUP_HOME}" && \
    for binary in cargo cargo-clippy cargo-fmt clippy-driver rustc rustdoc rustfmt rustup; do \
        ln -sf "${CARGO_HOME}/bin/${binary}" "/usr/local/bin/${binary}"; \
    done && \
    printf '%s\n' \
        'export CARGO_HOME="/usr/local/cargo"' \
        'export RUSTUP_HOME="/usr/local/rustup"' \
        'export PATH="/usr/local/cargo/bin:/usr/local/bin:$PATH"' \
        > /etc/profile.d/rust-path.sh && \
    chmod 0644 /etc/profile.d/rust-path.sh && \
    rustc --version && \
    cargo --version
ENV PATH="${CARGO_HOME}/bin:${PATH}"

# ===== UID 映射: gosu + entrypoint =====
RUN ARCH=${TARGETARCH:-$(dpkg --print-architecture)} && \
    curl -fL --http1.1 --retry 8 --retry-delay 3 --retry-all-errors --connect-timeout 20 \
        "https://github.com/tianon/gosu/releases/download/1.17/gosu-${ARCH}" -o /usr/local/bin/gosu && \
    chmod +x /usr/local/bin/gosu

RUN curl -fL --retry 8 --retry-delay 3 --retry-all-errors --connect-timeout 20 \
        -o /tmp/uv-install.sh https://astral.sh/uv/install.sh && \
    env UV_INSTALL_DIR=/usr/local/bin sh /tmp/uv-install.sh && \
    rm -f /tmp/uv-install.sh

# Unset http proxy
ENV HTTP_PROXY=
ENV HTTPS_PROXY=
ENV PROXY_URL=

COPY entrypoint.sh /entrypoint.sh

# 以 root 创建 node 用户的配置文件，运行时 entrypoint 会修正属主
WORKDIR /home/node
RUN git config --global init.defaultBranch main \
    && chown -R node:node /home/node

EXPOSE 7681

ENTRYPOINT ["/entrypoint.sh"]
CMD ["claude"]
