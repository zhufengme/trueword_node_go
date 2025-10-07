# line show-peer - 查看对端配置

## 概述

`line show-peer` 命令用于查看 WireGuard 隧道的对端配置命令。适用于查看已保存的对端配置或忘记配置命令的情况。

**注意**: 此命令仅适用于 WireGuard 服务器模式创建的隧道。

## 语法

```bash
sudo twnode line show-peer <隧道名>
```

## 参数

| 参数 | 说明 | 必需 |
|------|------|------|
| `<隧道名>` | WireGuard 隧道名称（服务器模式） | 是 |

## 输出内容

显示完整的对端创建命令，包括：
- 完整的 `line create` 命令
- 对端私钥（Base64 编码）
- 本端公钥（Base64 编码）
- 服务器 IP 和端口
- 虚拟 IP 配置

## 示例

### 示例1: 查看对端配置

```bash
$ sudo twnode line show-peer tunnel_hk

【对端配置命令】
隧道名称: tunnel_hk (WireGuard 服务器模式)

在远程服务器上运行以下命令创建对应的隧道:

sudo twnode line create <父接口> 192.168.1.100 10.0.0.1 10.0.0.2 tunnel_ba \
  --type wireguard \
  --mode client \
  --private-key 'aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2uV3wX4yZ5aB6cD8e=' \
  --peer-pubkey 'xY9zA0bC1dE2fG3hI4jK5lM6nO7pQ8rS9tU0vW1xY2zA4=' \
  --peer-port 51820

💡 提示:
  - 将 <父接口> 替换为对端服务器的实际父接口（如 eth0）
  - 对端隧道名称可自定义（示例中为 tunnel_ba）
  - 私钥和公钥已自动生成，请勿修改

配置文件已保存: /var/lib/trueword_node/peer_configs/tunnel_hk.txt
```

### 示例2: 客户端模式隧道

```bash
$ sudo twnode line show-peer tunnel_client

❌ 错误: 隧道 tunnel_client 不是 WireGuard 服务器模式
此命令仅适用于 WireGuard 服务器模式创建的隧道。
```

### 示例3: 非 WireGuard 隧道

```bash
$ sudo twnode line show-peer tun01

❌ 错误: 隧道 tun01 不是 WireGuard 隧道
此命令仅适用于 WireGuard 隧道（服务器模式）。

提示: GRE over IPsec 隧道需要手动在两端配置相同的密钥。
```

### 示例4: 隧道不存在

```bash
$ sudo twnode line show-peer tunnel_notexist

❌ 错误: 隧道不存在: tunnel_notexist
```

## 配置文件位置

对端配置命令保存在：

```
/var/lib/trueword_node/peer_configs/<隧道名>.txt
```

可以直接查看此文件：

```bash
cat /var/lib/trueword_node/peer_configs/tunnel_hk.txt
```

## 使用场景

### 场景1: 初次配置对端

创建 WireGuard 服务器隧道后，需要在对端服务器执行配置：

```bash
# 在服务器 A（香港）
$ sudo twnode line create eth0 0.0.0.0 10.0.0.2 10.0.0.1 hk-tw \
    --type wireguard --mode server --listen-port 51820

# 输出对端配置命令...

# 在服务器 B（台湾），复制命令并替换父接口
$ sudo twnode line create eth0 192.168.1.100 10.0.0.1 10.0.0.2 tw-hk \
    --type wireguard --mode client \
    --private-key 'xxx' --peer-pubkey 'yyy' --peer-port 51820
```

### 场景2: 忘记配置命令

创建隧道后忘记复制配置命令，可以使用 `show-peer` 查看：

```bash
# 服务器 A
$ sudo twnode line show-peer hk-tw

# 显示完整的对端配置命令
# 复制到服务器 B 执行
```

### 场景3: 重新配置对端

对端服务器重装系统后，需要重新配置隧道：

```bash
# 服务器 A（保留原隧道）
$ sudo twnode line show-peer hk-tw

# 复制输出的命令到服务器 B
# 在服务器 B 重新创建隧道
```

### 场景4: 添加新的对端

**注意**: WireGuard 服务器模式通常是 1:1 的（一个服务器对应一个客户端）。

如果需要多个客户端连接同一服务器，需要：
1. 为每个客户端创建独立的服务器端隧道
2. 使用不同的监听端口
3. 使用不同的虚拟 IP

```bash
# 服务器端创建多个隧道
sudo twnode line create eth0 0.0.0.0 10.0.0.2 10.0.0.1 client1 \
  --type wireguard --mode server --listen-port 51820

sudo twnode line create eth0 0.0.0.0 10.0.1.2 10.0.1.1 client2 \
  --type wireguard --mode server --listen-port 51821

# 查看各自的对端配置
sudo twnode line show-peer client1
sudo twnode line show-peer client2
```

## 安全注意事项

### 私钥保护

对端配置命令中包含**对端私钥**，必须安全传输：

```bash
# ✅ 安全方式
# 1. 使用加密通道（SSH、加密邮件）
# 2. 通过安全的内部网络传输
# 3. 使用密码管理器

# ❌ 不安全方式
# 1. 明文邮件
# 2. 公共聊天工具
# 3. 未加密的文件共享
```

### 私钥泄露处理

如果怀疑私钥泄露：

```bash
# 1. 停止隧道
sudo twnode line stop hk-tw

# 2. 删除隧道
sudo twnode line delete hk-tw

# 3. 重新创建隧道（生成新密钥）
sudo twnode line create eth0 0.0.0.0 10.0.0.2 10.0.0.1 hk-tw \
  --type wireguard --mode server --listen-port 51820

# 4. 通知对端使用新配置
sudo twnode line show-peer hk-tw
```

## 导出配置

### 导出到文件

```bash
# 导出为文本文件
sudo twnode line show-peer hk-tw > peer_config.txt

# 发送给对端管理员
scp peer_config.txt admin@remote-server:/tmp/
```

### 批量导出

导出所有 WireGuard 服务器隧道的对端配置：

```bash
#!/bin/bash
# export_all_peer_configs.sh

mkdir -p peer_configs_export

for tunnel in $(sudo twnode line list --type wireguard --json | \
                jq -r '.tunnels[] | select(.mode=="server") | .name'); do
    sudo twnode line show-peer $tunnel > "peer_configs_export/${tunnel}.txt"
    echo "已导出: ${tunnel}"
done

tar -czf peer_configs_$(date +%Y%m%d).tar.gz peer_configs_export/
echo "导出完成: peer_configs_$(date +%Y%m%d).tar.gz"
```

## 常见问题

### Q: 可以修改对端配置命令中的参数吗？

A: 部分参数可以修改：
- ✅ **父接口**: 必须替换为对端实际接口
- ✅ **隧道名称**: 可以自定义
- ❌ **私钥和公钥**: 不能修改（必须匹配）
- ❌ **虚拟 IP**: 不能修改（必须匹配）
- ❌ **服务器 IP 和端口**: 不能修改（必须匹配）

### Q: 对端配置文件丢失怎么办？

A: 使用 `show-peer` 命令重新生成：

```bash
sudo twnode line show-peer tunnel_hk
```

配置文件从隧道配置中读取，只要隧道配置存在就可以重新生成。

### Q: 可以在创建隧道时不保存对端配置吗？

A: 不建议。对端配置文件很小，保存它不会占用多少空间，而且方便后续查看。

如果确实需要禁用，可以修改源码或在创建后删除文件：

```bash
# 创建后删除对端配置文件（不推荐）
sudo rm /var/lib/trueword_node/peer_configs/tunnel_hk.txt
```

### Q: show-peer 显示的命令与创建时输出的有什么区别？

A: 完全相同。`show-peer` 从保存的对端配置文件中读取，内容与创建时输出的一致。

### Q: 如何自动发送对端配置给管理员？

A: 可以使用邮件或 webhook：

```bash
#!/bin/bash
# auto_send_peer_config.sh

TUNNEL=$1
ADMIN_EMAIL="admin@example.com"

CONFIG=$(sudo twnode line show-peer $TUNNEL)

echo "$CONFIG" | mail -s "WireGuard Peer Config: $TUNNEL" $ADMIN_EMAIL

echo "对端配置已发送到 $ADMIN_EMAIL"
```

## 下一步

- [创建隧道](create.md) - 创建 WireGuard 隧道
- [WireGuard 完整教程](../../tutorials/wireguard-setup.md) - 完整配置流程
- [列出隧道](list.md) - 查看所有隧道

---

**导航**: [← check](check.md) | [返回首页](../../index.md) | [line 命令](index.md)
