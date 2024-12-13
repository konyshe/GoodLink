## 特点介绍

1. 一条命令即可，无需安装、无需注册，无需公网IP，无需中转服务器，无需配置文件

2. 自己写的 NAT 穿透，无病毒报错

3. 需要公网 Redis 服务，可使用作者提供的免费 Redis 服务（参考使用说明）

4. 数据传输走 QUIC，高性能，已加密

## 使用说明

### 工作模式 - 介绍

#### P2P代理模式

	客户端需要指定本地监听端口，以提供socks5代理服务，通过该代理，可访问服务端所处网络中的任意主机端口

	该模式简单粗暴，一劳永逸，服务端所处网络的任意主机端口均可访问

#### P2P转发模式

	服务端需要指定所处网络中的某一个主机端口，客户端也需要指定本地监听端口。访问客户端指定的本地监听端口，等于访问服务端指定的主机端口

	该模式更加安全，只允许访问服务端所处网络的某一个主机端口，其他不能访问

### P2P代理模式 - 举例

客户端运行在公司的电脑，服务端运行在家里的NAS。

在公司电脑上配置代理地址：socks5://127.0.0.1:18080，便可访问家里包括NAS在内的所有主机端口。

### 家里的NAS ( linux，Docker )

下载镜像：registry.cn-shanghai.aliyuncs.com/kony/goodlink

```
docker rm goodlink -f; docker run -d --name=goodlink --net=host --restart=always registry.cn-shanghai.aliyuncs.com/kony/goodlink --redis_addr=goodlink.kony.vip:16379 --redis_pass=goodlink --redis_id=15 --key= nas_20240730
```

### 公司的电脑  ( windows, 命令行 )

[下载程序](https://gitee.com/konyshe/goodlink/releases)

```
.\goodlink-windows-amd64.exe --redis_addr=goodlink.kony.vip:16379 --redis_pass=goodlink --redis_id=15 --local=0.0.0.0:18080 --key=nas_20240730
```

注：服务端和客户端均支持命令行/ Docker 方式，以上仅作两种方式的举例。

### P2P转发模式 - 举例

客户端运行在公司的电脑，服务端运行在家里的NAS。

在公司访问 http://127.0.0.1:9999 , 等于访问家里的NAS管理页面http://192.168.3.2:9999

### 需要被远程访问的电脑 (linux，Docker)

下载镜像：registry.cn-shanghai.aliyuncs.com/kony/goodlink

```
docker rm goodlink -f; docker run -d --name=goodlink --net=host --restart=always registry.cn-shanghai.aliyuncs.com/kony/goodlink --redis_addr=goodlink.kony.vip:16379 --redis_pass=goodlink --redis_id=15 --remote=127.0.0.1:9999 --key=nas_20240730
```

### 需要请求访问的本地电脑 (windows, 命令行)

[下载程序](https://gitee.com/konyshe/goodlink/releases)

```
.\goodlink-windows-amd64.exe --redis_addr=goodlink.kony.vip:16379 --redis_pass=goodlink --redis_id=15 --local=0.0.0.0:9999 --key=nas_20240730
```

注：服务端和客户端均支持命令行/ Docker 方式，以上仅作两种方式的举例。

## 选项说明

```
--gogo-restart-delay: 进程守护，如果异常退出，会自动重启。需要指定自动重启时间间隔，单位毫秒

--redis_addr: Redis服务的公网域名或IP，仅用于帮助客户端和服务端之间建立P2P直连，不用于数据转发

--redis_pass: Redis服务的密码

--redis_id: Redis服务的表ID

--remote: 用于服务端指定所处网络中的某一个主机端口。如果不指定，则工作在P2P代理模式

--local: 用于客户端指定本地监听端口，127.0.0.1表示只能本机访问

--key: 客户端和服务端，需要使用同一个key，才能建立连接。最好16-32个字节长度，以避免和其他人冲突。
```

注：P2P转发模式仅支持 TCP 协议，如果服务端需要转发多个 TCP端口，需同时执行多个命令或启动多个 Docker（--key不能重复）

## 自己如何编译

```
cd /root/go/src
git clone -b main https://gitee.com/konyshe/gogo.git
git clone https://gitee.com/konyshe/goodlink.git
cd goodlink
make clean; make
```
