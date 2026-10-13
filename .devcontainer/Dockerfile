FROM mcr.microsoft.com/devcontainers/go:1.26-bookworm

# socat：容器内 127.0.0.1:6789 中转依赖它（Copilot 的 BYOK 把 base URL 存成这个
# 地址，配置在 Windows 侧、remoteEnv 注入不进去，详见 devcontainer.json 的说明
# 与 start-dev.sh 的中转段）。
#
# 基础镜像当前恰好带着 socat 1.7.4.4，但那是上游的偶然，不是本项目的保证 ——
# 显式安装，免得基础镜像升级后中转静默失效。
RUN apt-get update \
    && apt-get install -y --no-install-recommends socat \
    && rm -rf /var/lib/apt/lists/*
