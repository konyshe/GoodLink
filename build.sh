#!/usr/bin/env bash

#set -x


#apt install shc -y

rm -rf gogo
cp -r ../gogo .

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

docker rmi konyshe/goodlink:latest -f
docker rmi konyshe/goodlink:$1 -f

docker pull golang:latest
docker pull tonistiigi/xx:golang

docker buildx build --platform linux/amd64 -t konyshe/goodlink:$1 .

rm -rf gogo upx

sed -i "/$BUILD_TIME/s/$BUILD_TIME/111111111111/g" Dockerfile

docker push konyshe/goodlink:$1
docker tag konyshe/goodlink:$1 konyshe/goodlink:latest
docker push konyshe/goodlink:latest

docker images | grep konyshe | grep goodlink
