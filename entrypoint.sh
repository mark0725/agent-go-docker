#!/bin/bash
set -e

# 确保 /usr/local/bin 在 PATH 中
export PATH="/usr/local/bin:$PATH"

# source agents-hub 环境变量。用 _AGENT_ENV_LOADED 作 guard：本脚本会以 root 跑一遍、
# 再以 node 身份 re-exec 跑第二遍，guard 防止 .env 被重复 source（否则 PATH 之类会被追加两次）。
if [ -z "${_AGENT_ENV_LOADED:-}" ]; then
    if [ -f "/home/node/.agents-hub/agents/.env" ]; then
        set -a
        source "/home/node/.agents-hub/agents/.env"
        set +a
    fi
    if [ -n "${AGENT_ID:-}" ] && [ -f "/home/node/.agents-hub/agents/${AGENT_ID}/.env" ]; then
        set -a
        source "/home/node/.agents-hub/agents/${AGENT_ID}/.env"
        set +a
    fi
    export _AGENT_ENV_LOADED=1
fi

# 以 root 运行且设置了 HOST_UID 时，将容器内 node 用户的 UID/GID 调整为与宿主机一致，
# 这样容器内创建的文件在宿主机上拥有正确的属主。
# 随后以 node 身份重新进入本脚本（re-exec），使 AGENT_INIT_FILE 与 agent 本体都以 node 身份执行。
if [ "$(id -u)" = "0" ] && [ -n "${HOST_UID:-}" ]; then
    if [ "$(id -u node)" != "${HOST_UID}" ]; then
        OLD_UID=$(id -u node)
        groupmod -g "${HOST_GID}" node 2>/dev/null || true
        usermod -u "${HOST_UID}" -g "${HOST_GID}" node
        # 修正构建阶段以旧 UID 创建的文件（.gitconfig 等）
        find /home/node -user "${OLD_UID}" -exec chown -h node:node {} + 2>/dev/null || true
    fi
    exec gosu node "$0" "$@"
fi

# 可选的 agent 初始化脚本：AGENT_INIT_FILE 有值时执行它（source 方式，其 export 的变量会传给 agent）。
# 值可来自 docker -e / runner ExtraEnv，或上面 source 的 agents-hub .env。
# 文件缺失视为配置错误，直接报错退出（避免「以为跑了其实没跑」）。
if [ -n "${AGENT_INIT_FILE:-}" ]; then
    if [ -f "${AGENT_INIT_FILE}" ]; then
        echo "[entrypoint] running AGENT_INIT_FILE: ${AGENT_INIT_FILE}"
        # shellcheck source=/dev/null
        . "${AGENT_INIT_FILE}"
    else
        echo "[entrypoint] ERROR: AGENT_INIT_FILE set but not found: ${AGENT_INIT_FILE}" >&2
        exit 1
    fi
fi

# 清理内部标记，避免污染 agent 环境
unset _AGENT_ENV_LOADED
exec "$@"
