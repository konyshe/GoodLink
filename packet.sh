#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

DEBUG=0
case "${1:-}" in
    debug) DEBUG=1 ;;
    "") ;;
    *)
        echo "用法: $0 [debug]" >&2
        echo "  debug  只编译并打包 windows-amd64" >&2
        exit 1
        ;;
esac

make clean
make ui

GO_VERSION="1.25.14"

# Windows / Linux 均在 glibc 2.17 容器中用固定 Go 编译，不改动宿主机工具链
if ! command -v docker >/dev/null 2>&1; then
    echo "需要 docker 或 podman，以便在隔离环境中使用 glibc 2.17 / Go ${GO_VERSION} 编译" >&2
    exit 1
fi
IMAGE="localhost/goodlink-builder-glibc217:go${GO_VERSION}"
BASE_IMAGE="${GLIBC217_BASE_IMAGE:-docker.m.daocloud.io/library/centos:7}"

if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
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
fi

PARENT="$(dirname "$ROOT")"

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

cd bin
wget https://gitee.com/konyshe/goodlink_conf/raw/master/wintun.dll
md5sum goodlink-windows-amd64.exe wintun.dll > md5sum.txt; zip ../goodlink-windows-amd64.zip goodlink-windows-amd64.exe wintun.dll md5sum.txt
md5sum goodlink-linux-amd64 > md5sum.txt; zip goodlink-linux-amd64.zip goodlink-linux-amd64 md5sum.txt; rm -rf goodlink-linux-amd64 md5sum.txt
md5sum goodlink-linux-arm64 > md5sum.txt; zip goodlink-linux-arm64.zip goodlink-linux-arm64 md5sum.txt; rm -rf goodlink-linux-arm64 md5sum.txt
md5sum goodlink-linux-386 > md5sum.txt; zip goodlink-linux-386.zip goodlink-linux-386 md5sum.txt; rm -rf goodlink-linux-386 md5sum.txt
md5sum goodlink-linux-arm > md5sum.txt; zip goodlink-linux-arm.zip goodlink-linux-arm md5sum.txt; rm -rf goodlink-linux-arm md5sum.txt
md5sum goodlink-linux-armv6l > md5sum.txt; zip goodlink-linux-armv6l.zip goodlink-linux-armv6l md5sum.txt; rm -rf goodlink-linux-armv6l md5sum.txt
md5sum goodlink-linux-loong64 > md5sum.txt; zip goodlink-linux-loong64.zip goodlink-linux-loong64 md5sum.txt; rm -rf goodlink-linux-loong64 md5sum.txt
md5sum goodlink-linux-mips > md5sum.txt; zip goodlink-linux-mips.zip goodlink-linux-mips md5sum.txt; rm -rf goodlink-linux-mips md5sum.txt
md5sum goodlink-linux-mipsle > md5sum.txt; zip goodlink-linux-mipsle.zip goodlink-linux-mipsle md5sum.txt; rm -rf goodlink-linux-mipsle md5sum.txt
md5sum goodlink-linux-mips64 > md5sum.txt; zip goodlink-linux-mips64.zip goodlink-linux-mips64 md5sum.txt; rm -rf goodlink-linux-mips64 md5sum.txt
md5sum goodlink-linux-mips64le > md5sum.txt; zip goodlink-linux-mips64le.zip goodlink-linux-mips64le md5sum.txt; rm -rf goodlink-linux-mips64le md5sum.txt
md5sum goodlink-linux-riscv64 > md5sum.txt; zip goodlink-linux-riscv64.zip goodlink-linux-riscv64 md5sum.txt; rm -rf goodlink-linux-riscv64 md5sum.txt
md5sum goodlink-darwin-amd64 > md5sum.txt; zip goodlink-darwin-amd64.zip goodlink-darwin-amd64 md5sum.txt; rm -rf goodlink-darwin-amd64 md5sum.txt
md5sum goodlink-darwin-arm64 > md5sum.txt; zip goodlink-darwin-arm64.zip goodlink-darwin-arm64 md5sum.txt; rm -rf goodlink-darwin-arm64 md5sum.txt
md5sum goodlink-windows-arm.exe > md5sum.txt; zip goodlink-windows-arm.zip goodlink-windows-arm.exe md5sum.txt; rm -rf goodlink-windows-arm.exe md5sum.txt
md5sum goodlink-windows-arm64.exe > md5sum.txt; zip goodlink-windows-arm64.zip goodlink-windows-arm64.exe md5sum.txt; rm -rf goodlink-windows-arm64.exe md5sum.txt
md5sum goodlink-windows-386.exe > md5sum.txt; zip goodlink-windows-386.zip goodlink-windows-386.exe md5sum.txt; rm -rf goodlink-windows-386.exe md5sum.txt
cd ..

sync
rm -rf /mnt/windows/packet; mv bin /mnt/windows/packet
