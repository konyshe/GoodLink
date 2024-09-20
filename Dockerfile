FROM --platform=${BUILDPLATFORM} golang:latest AS builder

RUN echo 'Acquire::http::proxy "http://10.1.20.129:7898";' | tee -a /etc/apt/apt.conf
RUN echo 'Acquire::https::proxy "https://10.1.20.129:7898";' | tee -a /etc/apt/apt.conf
RUN export GO111MODULE=on
RUN export GOPROXY=https://goproxy.cn,direct

RUN apt update \
    && apt upgrade -y \
    && apt install make -y \
    && apt-get -qq update \
    && apt-get -qq install -y --no-install-recommends ca-certificates

WORKDIR /go/src/goodlink
COPY --from=tonistiigi/xx:golang / /
ARG TARGETOS TARGETARCH TARGETVARIANT

RUN echo 111111111111
COPY gogo /go/src/gogo

RUN --mount=target=. \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    make clean && \
    make linux-amd64 BINDIR= ${TARGETOS}-${TARGETARCH}${TARGETVARIANT} && \
    mv /goodlink* /goodlink

COPY upx /usr/bin/
RUN upx --best /goodlink

FROM alpine:3

#MAINTAINER 维护者信息
MAINTAINER kony

COPY --from=builder /goodlink /home/

#WORKDIR 相当于cd
WORKDIR /home/

#ENTRYPOINT 运行命令+固定参数
ENTRYPOINT ["./goodlink", "--h"]
