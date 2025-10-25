#!/bin/bash

# 不使用set -e，允许单个平台编译失败
# set -e

# 获取版本号
VERSION=$(grep -oP 'Version = "\K[^"]+' cmd/main.go)
if [ -z "$VERSION" ]; then
    echo "❌ 无法获取版本号"
    exit 1
fi

echo "=========================================="
echo "  TrueWord Node Release Builder"
echo "  Version: v${VERSION}"
echo "=========================================="
echo

# 创建发布目录
RELEASE_DIR="bin/release"
mkdir -p "$RELEASE_DIR"

# 清理旧文件
rm -f "$RELEASE_DIR"/twnode-v${VERSION}-*.tar.gz

# 定义目标平台
# 格式: "操作系统/架构"
# 注意：TrueWord Node 仅支持 Linux 平台（依赖 netlink 等 Linux 特定功能）
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "linux/386"
    "linux/arm"
    # Darwin 和 Windows 平台不支持（netlink/netns 等是 Linux 特定的）
    # "darwin/amd64"
    # "darwin/arm64"
    # "windows/amd64"
    # "windows/386"
)

# 编译函数
build_platform() {
    local os=$1
    local arch=$2
    local output_name="twnode"

    # Windows平台需要.exe后缀
    if [ "$os" = "windows" ]; then
        output_name="twnode.exe"
    fi

    local archive_name="twnode-v${VERSION}-${os}-${arch}.tar.gz"

    echo "📦 编译 ${os}/${arch}..."

    # 编译
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build \
        -ldflags="-s -w -extldflags '-static'" \
        -o "$output_name" \
        cmd/main.go

    if [ $? -ne 0 ]; then
        echo "❌ 编译 ${os}/${arch} 失败"
        return 1
    fi

    # 打包
    tar -czf "${RELEASE_DIR}/${archive_name}" "$output_name"

    if [ $? -eq 0 ]; then
        local size=$(du -h "${RELEASE_DIR}/${archive_name}" | cut -f1)
        echo "   ✓ ${archive_name} (${size})"
    else
        echo "   ❌ 打包失败"
        return 1
    fi

    # 清理临时文件
    rm -f "$output_name"
}

# 编译所有平台
echo "开始编译..."
echo

success_count=0
fail_count=0

for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -r os arch <<< "$platform"

    if build_platform "$os" "$arch"; then
        ((success_count++))
    else
        ((fail_count++))
    fi
    echo
done

# 显示结果
echo "=========================================="
echo "编译完成"
echo "=========================================="
echo "✓ 成功: ${success_count}"
echo "✗ 失败: ${fail_count}"
echo
echo "发布文件保存在: ${RELEASE_DIR}/"
echo

# 列出生成的文件
if [ $success_count -gt 0 ]; then
    echo "生成的文件:"
    ls -lh "${RELEASE_DIR}"/twnode-v${VERSION}-*.tar.gz | awk '{printf "  %s  %s\n", $5, $9}'
fi

echo
echo "完成！"
