# goodlink

## 介绍
两个无公网IP的PC之间，建立直连的内网穿透解决方案，无配置纯命令行，无需在公网服务端部署程序（只需要Redis服务），无病毒报错

### 特点说明

1. 手写NAT穿透算法，未使用第三方内核，不会报病毒

2. 公网服务器无程序部署，只需Redis服务，或者直接买个Redis服务就可以

3. 完全命令行方式运行，无需配置文件，简单明了

4. 只支持直连,不支持代理转发

5. 双方无需公网IP，传输速度上限为双方上下行带宽的最小值

## 编译说明

```
make clean; make
```

## 使用说明

### 直接下载发布版本

[下载地址](https://gitee.com/konyshe/goodlink/releases "下载地址")

### 需要被访问的目标电脑

```
nohup ./goodlink-linux-amd64 --gogo-restart-delay=1000 --redis_addr=1.2.3.4:6379 --redis_pass=123456 --redis_id=15 --remote=127.0.0.1:22 --key=ssh_20240730 &
```

### 需要请求访问的本机电脑

```
.\goodlink-windows-amd64.exe --redis_addr=1.2.3.4:6379 --redis_pass=123456 --redis_id=15 --local=127.0.0.1:18001 --key=ssh_20240730
```

### 选项说明

```
--gogo-restart-delay: 进程守护，如果异常退出，会自动重启。需要指定自动重启时间间隔，单位毫秒

--redis_addr: redis服务器的公网域名或IP，仅用于建立通道，不用于数据转发

--redis_pass: redis服务器的密码

--redis_id: redis服务器可用的表ID

--remote: 指向目标服务的IP和PORT

--local: 本地映射的IP和PORT，127.0.0.1表示只允许本机使用

--key: 如果有多个需要被访问的目标电脑，需要指定不同的key区分
```

## 参与贡献

1.  Fork 本仓库
2.  新建 Feat_xxx 分支
3.  提交代码
4.  新建 Pull Request


## 特技

1.  使用 Readme\_XXX.md 来支持不同的语言，例如 Readme\_en.md, Readme\_zh.md
2.  Gitee 官方博客 [blog.gitee.com](https://blog.gitee.com)
3.  你可以 [https://gitee.com/explore](https://gitee.com/explore) 这个地址来了解 Gitee 上的优秀开源项目
4.  [GVP](https://gitee.com/gvp) 全称是 Gitee 最有价值开源项目，是综合评定出的优秀开源项目
5.  Gitee 官方提供的使用手册 [https://gitee.com/help](https://gitee.com/help)
6.  Gitee 封面人物是一档用来展示 Gitee 会员风采的栏目 [https://gitee.com/gitee-stars/](https://gitee.com/gitee-stars/)
