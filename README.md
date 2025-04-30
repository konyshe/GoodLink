<img src="https://gitee.com/konyshe/goodlink/raw/master/assert/letter-g-2.png" width="400" height="100">

由于经常外出办公, 对于市面上的远程桌面工具, 无论画面、适配等, 都不如 windows 自带的远程桌面, 但外出如何使用 windows远程桌面呢？

是否可以无需远程桌面, 直接访问公司的内网 WEB, GIT, SSH 等？

**注: 该项目仅用于学习研究, 目前无商业合作，更无恶意行为。如果未来有广告之类盈利的行为，会郑重告知大家。另外声明：严禁用于违法行为！！！**

v2版本使用更加简单，和v1版本区别较大，如需使用v1版本，[切换回1.6版文档](https://gitee.com/konyshe/goodlink/blob/v1.6/README.md)

[ **在线群聊** ](https://www.oschina.net/comment/project/74765)

# 特点

1. 两台主机之间直连！直连！直连！不经过第三方服务器, 不用担心数据泄露

2. 一条命令搞定, 无需安装、无需注册, 无需公网 IP, 无需配置文件

![原理图](https://gitee.com/konyshe/goodlink/raw/master/assert/prototype_cn.gif "原理图")

# 重点

1.  **如超过5分钟无法直连，可以找客服（电信10000,移动10086,联通10010）改NAT类型，优先NAT1>NAT2>NAT3** 

2. 本程序即支持命令行方式, 也支持 docker 方式, windows 还有UI版本（但cmd版本更稳定）, 适合新手。以下举例仅作参考, 可随意搭配

3. 两端主机运行同一个程序 / Docker, 一端使用--remote 选项(以下称 remote 端), 另一端使用--local 选项(以下称 local 端)

4. 可以在 local 端访问 remote 端, 但是反过来不可以

5. 可以无限个 local 端连接同一个 remote 端, 但一个 local 端不能同时连接多个 remote 端。通过相同的密钥(--key)确认连接关系

6. 由于Local端需要创建虚拟网卡，因此一个PC端只能运行一个 local 端，否则会互相冲突。确定右下角任务栏只能一个GoodLink图标

7. windows 自带杀毒软件, 会将所有 go 语言写的程序都默认为病毒。本程序已开源, 可放心食用

8. 以下举例说明中的密钥(--key), 请不要使用, 否则会连上别人的 remote 端, 或者被别人的 local 端连上。自己随机一个 16-24 字节长度的密钥

9. 对于有安全疑问，或者想进阶使用的同学，可以看: [使用GoodLink 是否足够安全？](https://gitee.com/konyshe/goodlink/issues/IBFKC2)

10. 该项目刚刚起步, 可能不太稳定, 欢迎提出ISSUES, 帮忙测试的同学将保证永久免费使用

 <table>
	<th>Remote端</th><th>Local端</th><th>P2P成功</th>
	<tr><td>NAT1-3</td><td>NAT1-4</td><td>YES</td></tr>
      <tr><td>NAT1-4</td><td>NAT1-3</td><td>YES</td></tr>
	<tr><td>NAT4</td><td>NAT4</td><td>由于运营商算法调整，不保证100%</td></tr>
  </table>

# 工作模式

注：以下两个模式同时存在, 无需选择

### TUN模式

    Local端会创建一个虚拟网卡, 因此需要管理员权限运行。连接成功后，界面会显示: Remote端IP

    举例: 在Local端打开 windows 远程桌面, 填写Remote端IP, 即可访问Remote端的远程桌面

### 代理模式

    代理地址端口: socket5://Remote端IP:1080

    举例: 在Local端配置socket5代理: socks5://Remote端IP:1080, 即可利用Remote端做跳板, 访问所有的网络资源

    注: 目前仅支持TCP代理，浏览器可安装插件 SwitchyOmega。其他 GIT, SVN, SSH 等, 都支持socket5代理

# 简单使用

###  **启动 remote端** 

#### windows, UI

![使用说明](https://gitee.com/konyshe/goodlink/raw/master/assert/v2/5.png "使用说明")

#### linux, Docker

```
docker rm goodlink -f; docker run -d --name=goodlink --net=host --restart=always registry.cn-shanghai.aliyuncs.com/kony/goodlink --key=AIabJpEIYHMDIA6NBgOBboYJ --remote
```

#### linux, 命令行

```
./goodlink-linux-amd64 --key=AIabJpEIYHMDIA6NBgOBboYJ --remote
```

#### windows, 命令行

```
.\goodlink-windows-amd64.exe --fork --key=AIabJpEIYHMDIA6NBgOBboYJ --remote
```

###  **启动 local端** 

#### windows, UI

![使用说明](https://gitee.com/konyshe/goodlink/raw/master/assert/v2/6.png "使用说明")

#### linux, Docker

```
由于Local端需要创建虚拟网卡，Docker中并不支持
```

#### linux, 命令行

```
./goodlink-linux-amd64 --key=AIabJpEIYHMDIA6NBgOBboYJ --local
```

#### windows, 命令行

```
.\goodlink-windows-amd64.exe --fork --key=AIabJpEIYHMDIA6NBgOBboYJ --local
```


# 感谢支持

  danshiyuan

# [问题解答](https://gitee.com/konyshe/goodlink/issues)
