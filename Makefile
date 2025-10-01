.PHONY: all build clean install

# 项目信息
BINARY_NAME=twnode
BUILD_DIR=bin
CMD_DIR=cmd

# Go参数
GO=go
GOFLAGS=-ldflags="-s -w"
STATIC_FLAGS=-ldflags="-s -w -extldflags '-static'"

# 检测操作系统和架构
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)

all: build

# 构建(动态链接)
build:
	@echo "构建 $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)/main.go
	@echo "构建完成: $(BUILD_DIR)/$(BINARY_NAME)"

# 静态编译(推荐生产环境)
static:
	@echo "静态编译 $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GO) build $(STATIC_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)/main.go
	@echo "静态编译完成: $(BUILD_DIR)/$(BINARY_NAME)"
	@echo "二进制文件大小:"
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME) | awk '{print $$5, $$9}'

# 清理
clean:
	@echo "清理构建文件..."
	@rm -rf $(BUILD_DIR)
	@echo "清理完成"

# 安装到系统
install: static
	@echo "安装 $(BINARY_NAME) 到 /usr/local/bin/..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "安装完成"
	@echo "现在可以直接使用 'twnode' 命令"

# 卸载
uninstall:
	@echo "卸载 $(BINARY_NAME)..."
	@sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "卸载完成"

# 运行测试
test:
	$(GO) test -v ./...

# 下载依赖
deps:
	@echo "下载依赖..."
	$(GO) mod download
	@echo "依赖下载完成"

# 更新依赖
update:
	@echo "更新依赖..."
	$(GO) get -u ./...
	$(GO) mod tidy
	@echo "依赖更新完成"

# 格式化代码
fmt:
	@echo "格式化代码..."
	$(GO) fmt ./...
	@echo "格式化完成"

# 代码检查
vet:
	@echo "检查代码..."
	$(GO) vet ./...
	@echo "检查完成"

# 显示帮助
help:
	@echo "TrueWord Node 构建系统"
	@echo ""
	@echo "可用目标:"
	@echo "  make build    - 构建可执行文件(动态链接)"
	@echo "  make static   - 静态编译(推荐,无依赖)"
	@echo "  make install  - 静态编译并安装到系统"
	@echo "  make clean    - 清理构建文件"
	@echo "  make test     - 运行测试"
	@echo "  make deps     - 下载依赖"
	@echo "  make fmt      - 格式化代码"
	@echo "  make vet      - 检查代码"
	@echo "  make help     - 显示此帮助信息"
