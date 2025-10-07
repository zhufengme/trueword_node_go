# line stop - 停止隧道

## 概述

`line stop` 命令用于停止运行中的隧道。支持停止单个隧道或所有隧道。

## 语法

```bash
# 停止单个隧道
sudo twnode line stop <隧道名>

# 停止所有隧道
sudo twnode line stop-all
```

## 参数

| 参数 | 说明 | 必需 |
|------|------|------|
| `<隧道名>` | 要停止的隧道名称（stop 命令） | 是 |

## 工作流程

### 单个隧道停止

```
1. 检查隧道是否存在
   └─ 加载配置文件

2. 检查隧道是否正在运行
   └─ ip link show <name>
   └─ 如果未运行，跳过停止

3. 停止接口
   └─ ip link set <name> down

4. 删除接口
   ├─ WireGuard: ip link del <name>
   └─ GRE: ip tunnel del <name>

5. 删除路由规则（保留配置文件）
   ├─ 删除虚拟 IP 路由: ip rule del to <remote_vip>/32 pref 80
   └─ 删除保护路由: ip rule del to <remote_ip> pref 10

6. 删除 IPsec 连接（GRE over IPsec 隧道）
   └─ 删除 XFRM state 和 policy
```

### 批量停止 (stop-all)

```
1. 加载所有隧道配置

2. 按依赖关系逆序排序
   ├─ 最深层的隧道优先停止
   ├─ 然后是中间层隧道
   └─ 最后是基于物理接口的隧道

3. 依次停止每个隧道
   └─ 使用 stop 命令逐个停止
```

## 示例

### 示例1: 停止单个隧道

```bash
$ sudo twnode line stop tunnel_hk

停止隧道: tunnel_hk

✓ 停止接口
✓ 删除接口
✓ 清理路由规则
✓ 隧道 tunnel_hk 已停止
```

### 示例2: 停止所有隧道

```bash
$ sudo twnode line stop-all

停止所有隧道...

【第三层隧道】
✓ tunnel_layer3 (基于 tunnel_layer2)

【第二层隧道】
✓ tunnel_layer2 (基于 tunnel_hk)

【第一层隧道】
✓ tunnel_hk (基于 eth0)
✓ tunnel_us (基于 eth0)

共停止 4 个隧道
✓ 所有隧道已停止
```

### 示例3: 停止未运行的隧道

```bash
$ sudo twnode line stop tunnel_hk

ℹ 隧道 tunnel_hk 未运行
```

### 示例4: 停止不存在的隧道

```bash
$ sudo twnode line stop tunnel_notexist

❌ 错误: 隧道不存在: tunnel_notexist
```

## 依赖关系处理

### 多层隧道停止顺序

```
隧道3 (tunnel_layer3) - 先停止
  ↓
隧道2 (tunnel_layer2) - 然后停止
  ↓
隧道1 (tunnel_hk) - 最后停止
  ↓
物理接口 (eth0)
```

**错误示例**（依赖隧道仍在运行）:

```bash
$ sudo twnode line stop tunnel_hk

⚠️ 警告: 以下隧道依赖 tunnel_hk，仍在运行:
  - tunnel_layer2
  - tunnel_layer3

停止 tunnel_hk 可能导致这些隧道无法正常工作。
建议先停止这些隧道。

确认停止？(yes/no):
```

**正确做法**:

```bash
# 方法1: 按逆序停止
sudo twnode line stop tunnel_layer3
sudo twnode line stop tunnel_layer2
sudo twnode line stop tunnel_hk

# 方法2: 使用 stop-all（自动处理依赖）
sudo twnode line stop-all
```

## 与 delete 的区别

| 操作 | stop | delete |
|------|------|--------|
| 删除接口 | ✓ | ✓ |
| 清理路由规则 | ✓ | ✓ |
| 删除配置文件 | ✗ | ✓ |
| 删除撤销文件 | ✗ | ✓ |
| 删除 IPsec 连接 | ✓（临时） | ✓（永久） |
| 可以重新启动 | ✓ | ✗（需重新创建） |

**总结**:
- `stop` - 临时停止，保留配置，可以重新启动
- `delete` - 永久删除，清除所有配置和文件

## 验证停止状态

### 检查接口是否删除

```bash
ip link show tunnel_hk

# 应该显示:
# Device "tunnel_hk" does not exist.
```

### 检查 WireGuard 状态

```bash
sudo wg show tunnel_hk

# 应该显示:
# Unable to access interface: No such device
```

### 检查路由规则

```bash
# 虚拟 IP 路由应该被删除
ip rule show | grep tunnel_hk
# 应该没有输出

# 保护路由应该被删除
ip rule show pref 10 | grep <remote_ip>
# 应该没有输出
```

### 检查配置文件（应保留）

```bash
ls /etc/trueword_node/tunnels/tunnel_hk.yaml

# 应该显示文件仍然存在:
# /etc/trueword_node/tunnels/tunnel_hk.yaml
```

## 重新启动

停止后可以随时重新启动：

```bash
# 停止
sudo twnode line stop tunnel_hk

# 重新启动
sudo twnode line start tunnel_hk
```

## 策略路由影响

停止隧道后，使用此隧道的策略路由会失效：

```bash
$ sudo twnode line stop tunnel_hk

⚠️ 警告: 以下策略组使用此隧道作为出口:
  - asia_traffic
  - vpn_routes

停止隧道后，这些策略路由将无法正常工作。
流量可能被丢弃或走默认路由。

确认停止？(yes/no):
```

**建议**: 停止隧道前先撤销相关策略：

```bash
# 撤销策略
sudo twnode policy revoke asia_traffic

# 停止隧道
sudo twnode line stop tunnel_hk
```

或切换策略出口：

```bash
# 切换出口
sudo twnode policy create asia_traffic tunnel_backup

# 应用策略
sudo twnode policy apply asia_traffic

# 停止隧道
sudo twnode line stop tunnel_hk
```

## 强制停止

使用 `--force` 跳过所有警告和确认：

```bash
sudo twnode line stop tunnel_hk --force
```

**警告**: 强制停止会忽略所有依赖和策略路由警告，可能导致网络中断。

## 故障排查

### 问题1: 停止后接口仍然存在

**原因**: 停止过程中出错，接口未被删除。

**解决方案**:

```bash
# 手动删除接口
sudo ip link set tunnel_hk down
sudo ip link del tunnel_hk

# 或重新执行 stop
sudo twnode line stop tunnel_hk
```

### 问题2: 停止失败，提示 "Device is busy"

**原因**: 其他进程正在使用此接口。

**解决方案**:

```bash
# 查找使用接口的进程
sudo lsof | grep tunnel_hk

# 或查看路由表
ip route show dev tunnel_hk

# 清理路由后重试
sudo ip route flush dev tunnel_hk
sudo twnode line stop tunnel_hk
```

### 问题3: stop-all 卡住

**原因**: 某个隧道停止失败。

**解决方案**:

```bash
# 查看隧道状态
sudo twnode line list

# 手动停止失败的隧道
sudo twnode line stop <failed_tunnel> --force

# 继续停止其他隧道
sudo twnode line stop-all
```

## 常见问题

### Q: 停止隧道会删除配置吗？

A: 不会。`stop` 只停止运行，保留配置文件。要删除配置使用 `delete` 命令。

### Q: 停止后可以重新启动吗？

A: 可以。使用 `sudo twnode line start <name>` 重新启动。

### Q: stop 和 delete 应该用哪个？

A:
- **临时维护**: 使用 `stop`，完成后可以重新 `start`
- **永久移除**: 使用 `delete`，会清除所有配置

### Q: 停止隧道会影响策略路由吗？

A: 会。使用此隧道的策略路由会失效。建议先撤销策略或切换出口。

### Q: 如何优雅地停止所有隧道？

A: 使用 `stop-all` 命令，会自动处理依赖关系：

```bash
# 优雅停止
sudo twnode line stop-all

# 如果有警告，确认后继续
```

## 下一步

- [启动隧道](start.md) - 重新启动隧道
- [删除隧道](delete.md) - 永久删除隧道
- [列出隧道](list.md) - 查看隧道状态

---

**导航**: [← start](start.md) | [返回首页](../../index.md) | [list →](list.md)
