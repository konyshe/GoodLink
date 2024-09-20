# goodlink

## 介绍

无公网服务器，无配置文件，超简单的内网穿透解决方案

## 特点

1. 手写 NAT 穿透算法，未使用第三方内核，windows 端无病毒报错

2. 公网服务器无程序部署，只需 Redis 服务。可使用作者提供的免费 Redis 服务

3. 完全命令行方式运行，无需配置文件，简单明了

4. 双方无需公网 IP，传输速度上限为双方上下行带宽的最小值

## 编译

```
cd /root/go/src
git clone -b main https://gitee.com/konyshe/gogo.git
git clone https://gitee.com/konyshe/goodlink.git
cd goodlink
make clean; make
```

## 使用

### [Docker 方式点击此处](https://hub.docker.com/r/konyshe/goodlink)

### 下载发布版本

[版本发布页面](https://gitee.com/konyshe/goodlink/releases)

### 需要被远程访问的电脑（linux）

```
./goodlink-linux-amd64 --redis_addr=goodlink.kony.vip:16379 --redis_pass=goodlink --redis_id=15 --remote=127.0.0.1:80 --key=ssh_20240730
```

### 需要请求访问的本地电脑（windows）

```
.\goodlink-windows-amd64.exe --redis_addr=goodlink.kony.vip:16379 --redis_pass=goodlink --redis_id=15 --local=127.0.0.1:18080 --key=ssh_20240730
```

注：此时浏览器访问本地 PC 的 18080 端口，即可看到目标 PC 的 80 端口网页。目前仅支持 TCP 协议代理，UDP 协议还在开发中

## 选项说明

```
--gogo-restart-delay: 进程守护，如果异常退出，会自动重启。需要指定自动重启时间间隔，单位毫秒

--redis_addr: redis服务器的公网域名或IP，仅用于建立通道，不用于数据转发

--redis_pass: redis服务器的密码

--redis_id: redis服务器可用的表ID

--remote: 需要映射目标服务的IP和PORT

--local: 本地提供代理服务的IP和PORT，127.0.0.1表示只允许本机连接

--key: 如果有多个需要被访问的目标电脑，需要指定不同的key区分
```
