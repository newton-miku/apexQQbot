# ==============================================================================
# 项目基础配置
# ==============================================================================
# 项目名称
NAME = apexQQbot
# 输出目录
BINDIR = bin
# 配置目录
CONF_DIR = conf
# 静态资源目录
ASSET_DIR = asset

# 日期命令
BUILD_TIME ?= $(shell date +%Y-%m-%d\ %H:%M:%S)
DATE_CMD = date +%Y%m%d

# 版本号生成（使用跨平台日期命令）
VERSION=$(shell git describe --tags || echo "unknown version")
GOBUILD=CGO_ENABLED=0 go build -trimpath -ldflags '-X "github.com/newton-miku/apexQQbot/tools.Version=$(VERSION)" -X "github.com/newton-miku/apexQQbot/tools.BuildTime=$(BUILD_TIME)" -w -s'


# ==============================================================================
# 平台配置
# ==============================================================================
PLATFORM_LIST = \
    darwin-arm64 \
    darwin-amd64 \
    linux-amd64 \
    linux-arm64

WINDOWS_ARCH_LIST = \
	windows-386 \
	windows-amd64

all: linux-amd64 darwin-amd64 windows-amd64 # Most used

docker:
	$(GOBUILD) -o $(BINDIR)/$(NAME)-$@

darwin-arm64:
	GOARCH=arm64 GOOS=darwin $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

darwin-amd64:
	GOARCH=amd64 GOOS=darwin $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-386:
	GOARCH=386 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-amd64:
	GOARCH=amd64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-armv5:
	GOARCH=arm GOOS=linux GOARM=5 $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-armv6:
	GOARCH=arm GOOS=linux GOARM=6 $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-armv7:
	GOARCH=arm GOOS=linux GOARM=7 $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-arm64:
	GOARCH=arm64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mips64:
	GOARCH=mips64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mips64le:
	GOARCH=mips64le GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

windows-386:
	GOARCH=386 GOOS=windows $(GOBUILD) -o $(BINDIR)/$(NAME)-$@.exe

windows-amd64:
	GOARCH=amd64 GOOS=windows $(GOBUILD) -o $(BINDIR)/$(NAME)-$@.exe

gz_releases=$(addsuffix .gz, $(PLATFORM_LIST))
zip_releases=$(addsuffix .zip, $(WINDOWS_ARCH_LIST))

# 正确的压缩规则 - 包含配置和资源目录
$(gz_releases): %.gz : %
	chmod +x $(BINDIR)/$(NAME)-$(basename $@)
	# 创建临时目录用于打包
	mkdir -p $(BINDIR)/release/$(NAME)-$(basename $@)
	mv $(BINDIR)/$(NAME)-$(basename $@) $(BINDIR)/release/$(NAME)-$(basename $@)/
	cp -r $(CONF_DIR) $(BINDIR)/release/$(NAME)-$(basename $@)/
	cp -r $(ASSET_DIR) $(BINDIR)/release/$(NAME)-$(basename $@)/
	# 创建压缩包
	cd $(BINDIR)/release && tar -czvf ../$(NAME)-$(basename $@)-$(VERSION).tar.gz $(NAME)-$(basename $@)
	# 清理临时文件
	rm -rf $(BINDIR)/release

# 正确的zip规则 - 包含配置和资源目录
$(zip_releases): %.zip : %
	# 创建临时目录用于打包
	mkdir -p $(BINDIR)/release/$(NAME)-$(basename $@)
	mv $(BINDIR)/$(NAME)-$(basename $@).exe $(BINDIR)/release/$(NAME)-$(basename $@)/
	cp -r $(CONF_DIR) $(BINDIR)/release/$(NAME)-$(basename $@)/
	cp -r $(ASSET_DIR) $(BINDIR)/release/$(NAME)-$(basename $@)/
	# 创建压缩包
	cd $(BINDIR)/release && zip -r ../$(NAME)-$(basename $@)-$(VERSION).zip $(NAME)-$(basename $@)
	# 清理临时文件
	rm -rf $(BINDIR)/release

all-arch: $(PLATFORM_LIST) $(WINDOWS_ARCH_LIST)

releases: $(gz_releases) $(zip_releases)

clean:
	rm -rf $(BINDIR)/*