FROM scratch

#MAINTAINER 维护者信息
LABEL maintainer="kony"

ENV TZ=Asia/Shanghai

COPY bin/goodlink-linux-amd64 /home/goodlink

#WORKDIR 相当于cd
WORKDIR /home/

#ENTRYPOINT 运行命令+固定参数
ENTRYPOINT ["./goodlink"]

#CMD 可变参数, 会被docker run带入的参数替换
#CMD ["--h"]
