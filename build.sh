#!/usr/bin/env bash

set -x

rm -rf gotools goodlink2
cp -r ../gotools .
cp -r ../goodlink2 .

if [ -e "/usr/bin/upx" ]; then
    cp /usr/bin/upx .
else
    echo "请先下载 upx, 解压并保存为 /usr/bin/upx"
    echo "下载地址: https://github.com/upx/upx/releases"
    exit
fi

make clean

BUILD_TIME=$(date +'%Y%m%d%H%M')
sed -i "/111111111111/s/111111111111/$BUILD_TIME/g" Dockerfile

docker rmi dev/goodlink:latest -f

docker pull golang:latest
docker pull tonistiigi/xx:golang

docker buildx build --platform linux/amd64 -t dev/goodlink:latest .

rm -rf gotools goodlink2 upx

sed -i "/$BUILD_TIME/s/$BUILD_TIME/111111111111/g" Dockerfile

if [ $# -eq 1 ]; then
    docker rmi konyshe/goodlink:$1 -f
    docker tag dev/goodlink:latest konyshe/goodlink:$1
    docker push konyshe/goodlink:$1

    docker tag dev/goodlink:latest konyshe/goodlink:latest
    docker push konyshe/goodlink:latest

    docker tag dev/goodlink:latest registry.cn-shanghai.aliyuncs.com/kony/goodlink:$1
    docker push registry.cn-shanghai.aliyuncs.com/kony/goodlink:$1

    docker tag dev/goodlink:latest registry.cn-shanghai.aliyuncs.com/kony/goodlink:latest
    docker push registry.cn-shanghai.aliyuncs.com/kony/goodlink:latest

    docker rmi dev/goodlink:latest -f
fi

docker images | grep goodlink
