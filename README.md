# goodlink

## 介绍

超简单的内网穿透，无需注册，无需公网服务器，无需配置文件

## 特点

1. 无需注册，无需公网服务器，无需配置文件

2. 手写 NAT 穿透算法，无第三方内核，windows 端无病毒报错

3. 只需租个 Redis 服务，也可使用作者提供的免费 Redis 服务（使用说明中）

4. 数据传输走 QUIC，高性能，已加密，安全可靠

## 编译

```
cd /root/go/src
git clone -b main https://gitee.com/konyshe/gogo.git
git clone https://gitee.com/konyshe/goodlink.git
cd goodlink
make clean; make
```

## 使用说明

### 场景说明

将家里 NAS（需要被远程访问的电脑）的 9999 端口，穿透到公司内网电脑（需要请求访问的本地电脑）的 18080 端口。隧道建立成功后，在公司内网电脑，打开浏览器，访问本地 http://127.0.0.1:18080，即可访问到家里NAS的管理页面（9999端口）。

注：目前仅支持穿透 TCP 协议，UDP 协议还在开发中

### 需要被远程访问的电脑 (linux，Docker)

下载国内镜像源：registry.cn-shanghai.aliyuncs.com/kony/goodlink

```
docker rm goodlink -f; docker run -d --name=goodlink --net=host --restart=always registry.cn-shanghai.aliyuncs.com/kony/goodlink --redis_addr=goodlink.kony.vip:16379 --redis_pass=goodlink --redis_id=15 --remote=127.0.0.1:80 --key=ssh_20240730
```

### 需要请求访问的本地电脑 (windows, 命令行)

[下载程序](https://gitee.com/konyshe/goodlink/releases)

```
.\goodlink-windows-amd64.exe --redis_addr=goodlink.kony.vip:16379 --redis_pass=goodlink --redis_id=15 --local=127.0.0.1:18080 --key=ssh_20240730
```

注：该程序既支持命令行方式，也支持 Docker 方式，可随意切换。以上仅作两种方式的举例。

## 选项说明

```
--gogo-restart-delay: 进程守护，如果异常退出，会自动重启。需要指定自动重启时间间隔，单位毫秒

--redis_addr: redis服务器的公网域名或IP，仅用于建立通道，不用于数据转发

--redis_pass: redis服务器的密码

--redis_id: redis服务器可用的表ID

--remote: 需要映射目标服务的IP和PORT

--local: 本地提供代理服务的IP和PORT，127.0.0.1表示只允许本机连接

--key: 一个key只能对应一个需要被远程访问的电脑端口，多个端口需要自定义不同的key
```
