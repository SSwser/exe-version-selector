.PHONY: all clean build_dir copy_resources archive release

GO ?= go
BUILD_DIR := build

build_dir:
	@if [ ! -d $(BUILD_DIR) ]; then mkdir -p $(BUILD_DIR); fi
	@if [ ! -d $(BUILD_DIR)/resources ]; then mkdir -p $(BUILD_DIR)/resources; fi

clean:
	rm -rf $(BUILD_DIR)/*

copy_resources: build_dir
	cp -f config.yaml $(BUILD_DIR)/config.yaml
	cp -f resources/icon.ico $(BUILD_DIR)/resources/icon.ico

launcher: build_dir copy_resources launcher/main.go
ifeq ($(ENV),prod)
	$(GO) build -ldflags "-H=windowsgui" -o $(BUILD_DIR)/launcher.exe ./launcher/main.go
else
	$(GO) build -o $(BUILD_DIR)/launcher.exe ./launcher/main.go
endif

main: build_dir copy_resources core/main.go
	$(GO) build -o $(BUILD_DIR)/evs.exe ./core/main.go

all: launcher main

# archive: 打包 build 目录为 dist/evs_YYYYmmdd_HHMMSS.zip
archive: build_dir
	@if [ ! -d dist ]; then mkdir -p dist; fi
	zip_name=dist/evs_`date +%Y%m%d_%H%M%S`.zip; \
	cd build && zip -r ../$$zip_name ./*

# release: 编译所有并打包
release:
	$(MAKE) clean
	$(MAKE) ENV=prod all
	$(MAKE) archive
