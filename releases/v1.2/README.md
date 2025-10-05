# TrueWord Node v1.2 发布文件

## 📦 包含文件

- `twnode-v1.2-linux-amd64`：适用于 x86_64 架构 Linux 系统
- `twnode-v1.2-linux-arm64`：适用于 ARM64 架构 Linux 系统
- `twnode-v1.2-linux-armv7`：适用于 ARMv7 架构 Linux 系统（如树莓派）
- `twnode-v1.2-checksums.txt`：SHA256 校验和文件

## ✅ 验证文件完整性

下载文件后，请使用 SHA256 校验和验证文件完整性：

```bash
sha256sum -c twnode-v1.2-checksums.txt
```

或单独验证某个文件：

```bash
sha256sum twnode-v1.2-linux-amd64
# 应与 checksums.txt 中的值一致
```

## 🚀 安装说明

### 1. 下载对应架构的二进制文件

根据您的系统架构选择合适的文件：

```bash
# x86_64 系统
wget https://[下载地址]/twnode-v1.2-linux-amd64

# ARM64 系统
wget https://[下载地址]/twnode-v1.2-linux-arm64

# ARMv7 系统（树莓派等）
wget https://[下载地址]/twnode-v1.2-linux-armv7
```

### 2. 安装到系统

```bash
# 赋予执行权限
chmod +x twnode-v1.2-linux-amd64

# 安装到系统路径
sudo mv twnode-v1.2-linux-amd64 /usr/local/bin/twnode

# 验证安装
twnode version
```

### 3. 初始化（首次使用）

```bash
sudo twnode init
```

## 📋 系统要求

- **操作系统**：Linux（内核 3.10+）
- **权限**：需要 root 权限运行
- **依赖**：无（静态编译，包含所有依赖）
- **工具**：需要系统已安装 `ip`, `iptables`, `ping`, `sysctl` 命令

## 🔄 从旧版本升级

### 备份配置

```bash
sudo cp -r /etc/trueword_node /etc/trueword_node.backup
```

### 替换二进制

```bash
sudo cp twnode-v1.2-linux-amd64 /usr/local/bin/twnode
sudo chmod +x /usr/local/bin/twnode
```

### 验证升级

```bash
twnode version
# 应显示: TrueWord Node v1.2
```

配置文件完全兼容，无需任何修改。

## 📖 完整发布说明

请查看项目根目录的 `RELEASE_NOTES_v1.2.md` 获取完整的版本更新内容。

## 🔐 安全说明

- 所有二进制文件均为官方编译，未经第三方修改
- 建议通过 SHA256 校验和验证文件完整性
- 仅从官方渠道下载二进制文件

## 📞 支持

如有问题，请查阅项目文档或提交 Issue。
