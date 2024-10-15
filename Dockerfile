FROM --platform=${BUILDPLATFORM} golang:latest AS builder

RUN echo 'Acquire::http::proxy "http://10.1.20.129:7899";' | tee -a /etc/apt/apt.conf
RUN echo 'Acquire::https::proxy "http://10.1.20.129:7899";' | tee -a /etc/apt/apt.conf
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

FROM scratch

#MAINTAINER 维护者信息
MAINTAINER kony

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /lib/x86_64-linux-gnu /lib/x86_64-linux-gnu
COPY --from=builder /lib64 /lib64
COPY --from=builder /usr/lib/x86_64-linux-gnu /usr/lib/x86_64-linux-gnu
COPY --from=builder /goodlink /home/

#WORKDIR 相当于cd
WORKDIR /home/

#ENTRYPOINT 运行命令+固定参数
ENTRYPOINT ["./goodlink", "--gogo-restart-delay=2000"]

#CMD 可变参数, 会被docker run带入的参数替换
CMD ["--h"]
