# Makefile for exe-version-selector (launcher/evs 架构)
# 用法：
#   make all          # 编译所有二进制
#   make launcher     # 只编译托盘进程
#   make main         # 只编译业务进程
#   make clean        # 清理生成文件

GO ?= go
BUILD_DIR := build

all: launcher main

launcher: build_dir copy_resources launcher.go
	$(GO) build -o $(BUILD_DIR)/launcher.exe launcher.go

main: build_dir copy_resources main.go
	$(GO) build -o $(BUILD_DIR)/evs.exe main.go

clean:
	rm -rf $(BUILD_DIR)/*.exe $(BUILD_DIR)/config.yaml $(BUILD_DIR)/resources

build_dir:
	@if [ ! -d $(BUILD_DIR) ]; then mkdir -p $(BUILD_DIR); fi
	@if [ ! -d $(BUILD_DIR)/resources ]; then mkdir -p $(BUILD_DIR)/resources; fi

copy_resources: build_dir
	cp -f config.yaml $(BUILD_DIR)/config.yaml
	cp -f resources/icon.ico $(BUILD_DIR)/resources/icon.ico

.PHONY: all tray console clean build_dir copy_resources archive release

# archive: 打包 build 目录为 dist/evs_YYYYmmdd_HHMMSS.zip
archive: build_dir
	@if [ ! -d dist ]; then mkdir -p dist; fi
	zip_name=dist/evs_`date +%Y%m%d_%H%M%S`.zip; \
	cd build && zip -r ../$$zip_name ./*

# release: 编译所有并打包
release: all archive
