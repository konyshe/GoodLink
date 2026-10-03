#!/usr/bin/env bash
set -euo pipefail

#set -x

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

GO_VERSION="1.26.3"
IMAGE="localhost/goodlink-builder-glibc217:go${GO_VERSION}"
BASE_IMAGE="${GLIBC217_BASE_IMAGE:-docker.m.daocloud.io/library/centos:7}"
PARENT="$(dirname "$ROOT")"

usage() {
    echo "用法: $0 {packet [debug] | docker [tag]}" >&2
    echo "  packet [debug]  多平台编译并打包（debug 仅 windows-amd64）" >&2
    echo "  docker [tag]    静态编译 linux-amd64 并构建/推送运行镜像" >&2
    exit 1
}

# 检查 docker / podman 是否可用
ensure_docker() {
    if ! command -v docker >/dev/null 2>&1; then
        echo "需要 docker 或 podman，以便在隔离环境中使用 glibc 2.17 / Go ${GO_VERSION} 编译" >&2
        exit 1
    fi
}

# 检查宿主机 upx
ensure_upx() {
    if [ ! -e "/usr/bin/upx" ]; then
        echo "请先下载 upx, 解压并保存为 /usr/bin/upx" >&2
        echo "下载地址: https://github.com/upx/upx/releases" >&2
        exit 1
    fi
}

# 确保 glibc 2.17 编译镜像存在（不存在则构建）
ensure_builder_image() {
    if docker image inspect "$IMAGE" >/dev/null 2>&1; then
        return 0
    fi
    if [ ! -x "/usr/local/go${GO_VERSION}/bin/go" ]; then
        echo "未找到 /usr/local/go${GO_VERSION}/bin/go，无法注入 glibc 2.17 编译镜像" >&2
        exit 1
    fi
    echo "构建 glibc 2.17 编译镜像: $IMAGE"
    docker build \
        --build-arg "BASE_IMAGE=${BASE_IMAGE}" \
        -t "$IMAGE" \
        -f "$ROOT/Dockerfile.glibc217" \
        "$ROOT"
}

# 在 glibc 2.17 容器中执行 make / bash（不改动宿主机工具链）
docker_make() {
    docker run --rm \
        --user "$(id -u):$(id -g)" \
        -e HOME=/tmp \
        -e GOPATH=/tmp/go \
        -e GOPROXY="${GOPROXY:-https://goproxy.cn,direct}" \
        -e GO111MODULE=on \
        -e GOTOOLCHAIN=local \
        -e GOCACHE=/tmp/go-build \
        -e GOMODCACHE=/tmp/go-mod \
        -v "$PARENT":/src \
        -v "/usr/local/go${GO_VERSION}:/usr/local/go" \
        -w /src/goodlink \
        "$IMAGE" \
        "$@"
}

# 公共前置：docker + 编译镜像 + upx
ensure_prereqs() {
    ensure_docker
    ensure_builder_image
    ensure_upx
}

cmd_packet() {
    local DEBUG=0
    case "${1:-}" in
        debug) DEBUG=1 ;;
        "") ;;
        *)
            echo "用法: $0 packet [debug]" >&2
            echo "  debug  只编译并打包 windows-amd64" >&2
            exit 1
            ;;
    esac

    make clean
    make ui

    # windows-amd64 需要 goversioninfo 生成 nac.syso
    docker_make bash -c \
        'export PATH="/tmp/go/bin:/usr/local/go/bin:$PATH"; go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest; make windows86'

    if [ "$DEBUG" -eq 1 ]; then
        rm -rf /mnt/windows/bin/goodlink-windows-amd64.exe; mv bin/goodlink-windows-amd64.exe /mnt/windows/bin/
        exit 0
    fi

    docker_make make packet

    if command -v objdump >/dev/null 2>&1 && [ -f bin/goodlink-linux-amd64 ]; then
        echo "goodlink-linux-amd64 依赖的 GLIBC 符号:"
        objdump -T bin/goodlink-linux-amd64 | grep -oE 'GLIBC_[0-9.]+' | sort -Vu || true
    fi

    local DEST_DIR="/mnt/windows/packet"
    rm -rf ${DEST_DIR}
    mkdir -p ${DEST_DIR}

    # 打包阶段：忽略单个架构的错误，不要中途结束
    set +eo pipefail

    cd bin
    wget https://gitee.com/konyshe/goodlink_conf/raw/master/wintun.dll
    md5sum goodlink-windows-amd64.exe wintun.dll > md5sum.txt; zip ${DEST_DIR}/goodlink-windows-amd64.zip goodlink-windows-amd64.exe wintun.dll md5sum.txt
    md5sum goodlink-windows-arm.exe > md5sum.txt; zip ${DEST_DIR}/goodlink-windows-arm.zip goodlink-windows-arm.exe md5sum.txt
    md5sum goodlink-windows-arm64.exe > md5sum.txt; zip ${DEST_DIR}/goodlink-windows-arm64.zip goodlink-windows-arm64.exe md5sum.txt
    md5sum goodlink-windows-386.exe > md5sum.txt; zip ${DEST_DIR}/goodlink-windows-386.zip goodlink-windows-386.exe md5sum.txt

    upx -9 goodlink-linux-amd64; md5sum goodlink-linux-amd64 > md5sum.txt; zip ${DEST_DIR}/goodlink-linux-amd64.zip goodlink-linux-amd64 md5sum.txt
    upx -9 goodlink-linux-arm64; md5sum goodlink-linux-arm64 > md5sum.txt; zip ${DEST_DIR}/goodlink-linux-arm64.zip goodlink-linux-arm64 md5sum.txt
    upx -9 goodlink-linux-386; md5sum goodlink-linux-386 > md5sum.txt; zip ${DEST_DIR}/goodlink-linux-386.zip goodlink-linux-386 md5sum.txt
    upx -9 goodlink-linux-arm; md5sum goodlink-linux-arm > md5sum.txt; zip ${DEST_DIR}/goodlink-linux-arm.zip goodlink-linux-arm md5sum.txt
    upx -9 goodlink-linux-armv6l; md5sum goodlink-linux-armv6l > md5sum.txt; zip ${DEST_DIR}/goodlink-linux-armv6l.zip goodlink-linux-armv6l md5sum.txt
    upx -9 goodlink-linux-loong64; md5sum goodlink-linux-loong64 > md5sum.txt; zip ${DEST_DIR}/goodlink-linux-loong64.zip goodlink-linux-loong64 md5sum.txt
    upx -9 goodlink-linux-mips; md5sum goodlink-linux-mips > md5sum.txt; zip ${DEST_DIR}/goodlink-linux-mips.zip goodlink-linux-mips md5sum.txt
    upx -9 goodlink-linux-mipsle; md5sum goodlink-linux-mipsle > md5sum.txt; zip ${DEST_DIR}/goodlink-linux-mipsle.zip goodlink-linux-mipsle md5sum.txt
    upx -9 goodlink-linux-mips64; md5sum goodlink-linux-mips64 > md5sum.txt; zip ${DEST_DIR}/goodlink-linux-mips64.zip goodlink-linux-mips64 md5sum.txt
    upx -9 goodlink-linux-mips64le; md5sum goodlink-linux-mips64le > md5sum.txt; zip ${DEST_DIR}/goodlink-linux-mips64le.zip goodlink-linux-mips64le md5sum.txt
    upx -9 goodlink-linux-riscv64; md5sum goodlink-linux-riscv64 > md5sum.txt; zip ${DEST_DIR}/goodlink-linux-riscv64.zip goodlink-linux-riscv64 md5sum.txt

    upx -9 goodlink-darwin-amd64; md5sum goodlink-darwin-amd64 > md5sum.txt; zip ${DEST_DIR}/goodlink-darwin-amd64.zip goodlink-darwin-amd64 md5sum.txt
    upx -9 goodlink-darwin-arm64; md5sum goodlink-darwin-arm64 > md5sum.txt; zip ${DEST_DIR}/goodlink-darwin-arm64.zip goodlink-darwin-arm64 md5sum.txt

    rm -rf md5sum.txt
    cd ..

    make clean

    sync

    exit 0
}

cmd_docker() {
    local TAG="${1:-}"

    make clean
    make ui

    docker_make bash -c \
        'export PATH="/usr/local/go/bin:$PATH"; make linux-amd64'
    upx -9 bin/goodlink-linux-amd64

    docker rmi dev/goodlink:latest -f
    docker buildx build --platform linux/amd64 -f "$ROOT/Dockerfile.runtime" -t dev/goodlink:latest .

    if [ -n "$TAG" ]; then
        docker rmi registry.cn-shanghai.aliyuncs.com/kony/goodlink:$TAG -f
        docker tag dev/goodlink:latest registry.cn-shanghai.aliyuncs.com/kony/goodlink:$TAG
        docker push registry.cn-shanghai.aliyuncs.com/kony/goodlink:$TAG

        #docker rmi registry.cn-shanghai.aliyuncs.com/kony/goodlink:latest -f
        #docker tag dev/goodlink:latest registry.cn-shanghai.aliyuncs.com/kony/goodlink:latest
        #docker push registry.cn-shanghai.aliyuncs.com/kony/goodlink:latest

        docker rmi dev/goodlink:latest -f
    fi

    docker images | grep goodlink
}

# ---- main ----
ensure_prereqs

CMD="${1:-}"
shift || true
case "$CMD" in
    packet) cmd_packet "$@" ;;
    docker) cmd_docker "$@" ;;
    *) usage ;;
esac
