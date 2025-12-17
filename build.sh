#!/usr/bin/env bash

go install github.com/akavel/rsrc@latest

set -x

rm -rf go2 goodlink2 proxy
cp -r ../go2 .
cp -r ../goodlink2 .
cp -r ../proxy .

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


docker pull golang:bookworm
docker pull tonistiigi/xx:golang

docker rmi dev/goodlink:latest -f
docker buildx build --platform linux/amd64 -t dev/goodlink:latest .

rm -rf go2 goodlink2 proxy upx

sed -i "/$BUILD_TIME/s/$BUILD_TIME/111111111111/g" Dockerfile

if [ $# -eq 1 ]; then
    #docker rmi konyshe/goodlink:$1 -f
    #docker tag dev/goodlink:latest konyshe/goodlink:$1
    #docker push konyshe/goodlink:$1

    #docker rmi konyshe/goodlink:latest -f
    #docker tag dev/goodlink:latest konyshe/goodlink:latest
    #docker push konyshe/goodlink:latest

    docker rmi registry.cn-shanghai.aliyuncs.com/kony/goodlink:$1 -f
    docker tag dev/goodlink:latest registry.cn-shanghai.aliyuncs.com/kony/goodlink:$1
    docker push registry.cn-shanghai.aliyuncs.com/kony/goodlink:$1

    docker rm registry.cn-shanghai.aliyuncs.com/kony/goodlink:latest -f
    docker tag dev/goodlink:latest registry.cn-shanghai.aliyuncs.com/kony/goodlink:latest
    docker push registry.cn-shanghai.aliyuncs.com/kony/goodlink:latest
fi

docker images | grep goodlink
