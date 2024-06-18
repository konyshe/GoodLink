NAME=goodlink
BINDIR=bin
# 获取源码最近一次 git commit log，包含 commit sha 值，以及 commit message
GitCommitLog=$(shell git log --pretty=oneline -n 1)
BuildTime=$(shell date +'%Y-%m-%d %H:%M:%S')
GOBUILD=GO111MODULE=on \
		GOPROXY="https://goproxy.cn,direct" \
		go build -trimpath -ldflags \
		'-X "gogo.GitCommitLog=$(GitCommitLog)" \
    	-X "gogo.GitStatus=$(GitStatus)" \
    	-X "gogo.BuildTime=$(BuildTime)" \
		-w -s -buildid='

PLATFORM_LIST = \
	linux-amd64

all: $(PLATFORM_LIST) strip

linux-amd64:
	GOARCH=amd64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

windows-amd64:
	GOARCH=amd64 GOOS=windows $(GOBUILD) -o $(BINDIR)/$(NAME)-$@.exe

strip:
	upx $(BINDIR)/*
	
clean:
	rm -rf $(BINDIR)
