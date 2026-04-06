#!/bin/bash
set -e

# 加载 ~/.agents/.env 中的环境变量（如存在）
if [ -f "${HOME}/.agents/.env" ]; then
    set -a
    source "${HOME}/.agents/.env"
    set +a
fi

# 加载 ~/.agents/<AGENT_ID>/.env 中的环境变量（如存在）
if [ -n "${AGENT_ID:-}" ] && [ -f "${HOME}/.agents/${AGENT_ID}/.env" ]; then
    set -a
    source "${HOME}/.agents/${AGENT_ID}/.env"
    set +a
fi

# 以 root 运行且设置了 HOST_UID 时，将容器内 node 用户的 UID/GID 调整为与宿主机一致
# 这样容器内创建的文件在宿主机上拥有正确的属主
if [ "$(id -u)" = "0" ] && [ -n "${HOST_UID:-}" ]; then
    if [ "$(id -u node)" != "${HOST_UID}" ]; then
        OLD_UID=$(id -u node)
        groupmod -g "${HOST_GID}" node 2>/dev/null || true
        usermod -u "${HOST_UID}" -g "${HOST_GID}" node
        # 修正构建阶段以旧 UID 创建的文件（.gitconfig 等）
        find /home/node -user "${OLD_UID}" -exec chown -h node:node {} + 2>/dev/null || true
    fi
    exec gosu node "$@"
fi

exec "$@"
