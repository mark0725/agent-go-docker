FROM ghcr.io/mark0725/agent-go-docker:latest

ENV DEBIAN_FRONTEND=noninteractive

ENV GOPROXY=https://proxy.golang.com.cn,direct
ARG GO_VERSION=1.26.1
RUN GOARCH=${TARGETARCH:-$(dpkg --print-architecture)} && \
    curl -fsSL https://go.dev/dl/go${GO_VERSION}.linux-${GOARCH}.tar.gz \
    | tar -C /usr/local -xzf -
ENV PATH="/usr/local/go/bin:${PATH}"

ENV GOPATH="/home/node/go"
ENV PATH="${GOPATH}/bin:/usr/local/go/bin:${PATH}"
