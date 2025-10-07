# 策略路由实践教程

本教程通过实际场景演示如何使用 TrueWord Node 的策略路由功能，实现灵活的流量分发和管理。

## 教程目标

完成本教程后，你将学会：

- 基于目标 CIDR 的流量分发
- 基于源地址的流量控制
- 多隧道负载均衡
- 智能故障转移配置
- 优先级管理和冲突处理

## 前置条件

- 已完成系统初始化（`twnode init`）
- 至少创建了一个隧道
- 了解基本的网络概念（IP、CIDR、路由）

## 场景1: 基于目标地址的流量分发

### 需求

将不同地区的流量路由到不同的隧道：
- 亚洲 IP 段 → 香港隧道
- 美洲 IP 段 → 美国隧道
- 欧洲 IP 段 → 德国隧道
- 其他流量 → 本地物理接口

### 前提

已创建三个隧道：
```bash
sudo twnode line list
```

输出：
```
+-------------+---------------+--------------+-----------------+----------------+-------------+
| 隧道名称    | 父接口        | 类型         | 对端IP          | 本地VIP        | 状态        |
+-------------+---------------+--------------+-----------------+----------------+-------------+
| tunnel_hk   | eth0          | WireGuard    | 203.0.113.10    | 10.0.0.1       | ✓ Active    |
| tunnel_us   | eth0          | WireGuard    | 198.51.100.20   | 10.0.1.1       | ✓ Active    |
| tunnel_de   | eth0          | WireGuard    | 192.0.2.30      | 10.0.2.1       | ✓ Active    |
+-------------+---------------+--------------+-----------------+----------------+-------------+
```

### 配置步骤

#### 1. 创建亚洲流量策略组

```bash
# 创建策略组（优先级 100 - 最高优先）
sudo twnode policy create asia_traffic tunnel_hk --priority 100

# 添加亚洲 IP 段（示例）
sudo twnode policy add-cidr asia_traffic 103.0.0.0/8      # 亚太地区
sudo twnode policy add-cidr asia_traffic 119.0.0.0/8      # 中国
sudo twnode policy add-cidr asia_traffic 220.0.0.0/8      # 东亚

# 应用策略
sudo twnode policy apply asia_traffic
```

#### 2. 创建美洲流量策略组

```bash
# 创建策略组（优先级 200）
sudo twnode policy create americas_traffic tunnel_us --priority 200

# 添加美洲 IP 段
sudo twnode policy add-cidr americas_traffic 8.0.0.0/8     # 美国
sudo twnode policy add-cidr americas_traffic 23.0.0.0/8    # 北美
sudo twnode policy add-cidr americas_traffic 107.0.0.0/8   # 美国

# 应用策略
sudo twnode policy apply americas_traffic
```

#### 3. 创建欧洲流量策略组

```bash
# 创建策略组（优先级 300）
sudo twnode policy create europe_traffic tunnel_de --priority 300

# 添加欧洲 IP 段
sudo twnode policy add-cidr europe_traffic 2.0.0.0/8       # 欧洲
sudo twnode policy add-cidr europe_traffic 31.0.0.0/8      # 欧洲
sudo twnode policy add-cidr europe_traffic 77.0.0.0/8      # 欧洲

# 应用策略
sudo twnode policy apply europe_traffic
```

#### 4. 查看策略组列表

```bash
sudo twnode policy list
```

输出：
```
+------------------+-------------+----------+-------------+-----------+
| 策略组名称       | 出口接口    | 优先级   | CIDR 数量   | 状态      |
+------------------+-------------+----------+-------------+-----------+
| asia_traffic     | tunnel_hk   | 100      | 3           | Applied   |
| americas_traffic | tunnel_us   | 200      | 3           | Applied   |
| europe_traffic   | tunnel_de   | 300      | 3           | Applied   |
+------------------+-------------+----------+-------------+-----------+

共 3 个策略组（3 个已应用，0 个未应用）
```

### 验证

```bash
# 测试亚洲 IP（应该走 tunnel_hk）
ip route get 103.10.20.30
# 输出: 103.10.20.30 dev tunnel_hk src 10.0.0.1

# 测试美洲 IP（应该走 tunnel_us）
ip route get 8.8.8.8
# 输出: 8.8.8.8 dev tunnel_us src 10.0.1.1

# 测试欧洲 IP（应该走 tunnel_de）
ip route get 2.20.30.40
# 输出: 2.20.30.40 dev tunnel_de src 10.0.2.1

# 测试其他 IP（应该走物理接口）
ip route get 1.1.1.1
# 输出: 1.1.1.1 via 192.168.1.1 dev eth0 src 192.168.1.100
```

## 场景2: 基于源地址的流量控制

### 需求

不同的内网设备使用不同的出口：
- 服务器 `10.10.1.0/24` → 香港隧道
- 办公电脑 `10.10.2.0/24` → 美国隧道
- 访客设备 `10.10.3.0/24` → 本地物理接口

### 配置步骤

#### 1. 服务器流量策略

```bash
# 创建策略组，指定源地址限制
sudo twnode policy create server_traffic tunnel_hk \
  --priority 100 \
  --from 10.10.1.0/24

# 添加目标 CIDR（所有流量）
sudo twnode policy add-cidr server_traffic 0.0.0.0/0

# 应用策略
sudo twnode policy apply server_traffic
```

#### 2. 办公电脑流量策略

```bash
# 创建策略组
sudo twnode policy create office_traffic tunnel_us \
  --priority 200 \
  --from 10.10.2.0/24

# 添加目标 CIDR
sudo twnode policy add-cidr office_traffic 0.0.0.0/0

# 应用策略
sudo twnode policy apply office_traffic
```

#### 3. 访客设备（使用主路由表）

访客设备不配置策略，自动使用物理接口的默认路由。

### 验证

```bash
# 查看路由规则
ip rule show

# 应该看到
100:    from 10.10.1.0/24 to 0.0.0.0/0 lookup 50
200:    from 10.10.2.0/24 to 0.0.0.0/0 lookup 50
...
```

### 测试

从不同源地址测试：

```bash
# 从服务器 IP 测试（应该走 tunnel_hk）
ip route get 8.8.8.8 from 10.10.1.5
# 输出: 8.8.8.8 from 10.10.1.5 dev tunnel_hk src 10.0.0.1

# 从办公电脑 IP 测试（应该走 tunnel_us）
ip route get 8.8.8.8 from 10.10.2.5
# 输出: 8.8.8.8 from 10.10.2.5 dev tunnel_us src 10.0.1.1

# 从访客设备 IP 测试（应该走物理接口）
ip route get 8.8.8.8 from 10.10.3.5
# 输出: 8.8.8.8 from 10.10.3.5 via 192.168.1.1 dev eth0 src 192.168.1.100
```

## 场景3: 优先级分层管理

### 需求

建立清晰的流量优先级体系：
- 核心业务（优先级最高）
- 普通业务
- 备份/测试流量（优先级最低）

### 优先级规划

```
100-199: 核心业务流量
200-399: 普通业务流量
400-599: 备份和测试流量
600-899: 保留备用
```

### 配置示例

#### 1. 核心业务流量（优先级 100）

```bash
# 创建核心业务策略组
sudo twnode policy create core_business tunnel_hk --priority 100

# 添加核心业务服务器 IP 段
sudo twnode policy add-cidr core_business 203.0.113.0/24
sudo twnode policy add-cidr core_business 198.51.100.0/24

# 应用策略
sudo twnode policy apply core_business
```

#### 2. 普通业务流量（优先级 200）

```bash
# 创建普通业务策略组
sudo twnode policy create normal_business tunnel_us --priority 200

# 添加普通业务 IP 段
sudo twnode policy add-cidr normal_business 192.168.0.0/16
sudo twnode policy add-cidr normal_business 10.0.0.0/8

# 应用策略
sudo twnode policy apply normal_business
```

#### 3. 备份流量（优先级 500）

```bash
# 创建备份策略组
sudo twnode policy create backup_traffic tunnel_bak --priority 500

# 添加备份服务器 IP 段
sudo twnode policy add-cidr backup_traffic 172.16.0.0/12

# 应用策略
sudo twnode policy apply backup_traffic
```

### 动态调整优先级

假设核心业务需要临时降低优先级：

```bash
# 调整核心业务优先级为 150
sudo twnode policy set-priority core_business 150

# 如果策略已应用，会自动重新应用
```

## 场景4: 故障转移和负载均衡

### 需求

主隧道故障时，自动切换到备用隧道。

### 配置步骤

#### 1. 创建主策略组

```bash
# 主隧道：香港
sudo twnode policy create main_traffic tunnel_hk --priority 100
sudo twnode policy add-cidr main_traffic 0.0.0.0/0
sudo twnode policy apply main_traffic
```

#### 2. 定期检查隧道连通性

```bash
# 检查所有隧道到 8.8.8.8 的连通性
sudo twnode line check tunnel_hk 8.8.8.8
sudo twnode line check tunnel_us 8.8.8.8
sudo twnode line check tunnel_bak 8.8.8.8
```

输出：
```
检查接口 tunnel_hk 到 8.8.8.8 的连通性...
✓ Ping 成功
丢包率: 0.0%, 平均延迟: 12.5 ms, 评分: 96.5 分

检查接口 tunnel_us 到 8.8.8.8 的连通性...
✓ Ping 成功
丢包率: 0.0%, 平均延迟: 45.2 ms, 评分: 85.3 分

检查接口 tunnel_bak 到 8.8.8.8 的连通性...
✓ Ping 成功
丢包率: 5.0%, 平均延迟: 150.0 ms, 评分: 62.0 分
```

#### 3. 手动故障转移

如果主隧道故障：

```bash
# 撤销主策略
sudo twnode policy revoke main_traffic

# 应用备用策略
sudo twnode policy create backup_traffic tunnel_us --priority 100
sudo twnode policy add-cidr backup_traffic 0.0.0.0/0
sudo twnode policy apply backup_traffic
```

#### 4. 自动故障转移

使用内置的 failover 功能：

```bash
# 为 main_traffic 策略组配置故障转移
# 候选出口：tunnel_hk, tunnel_us, tunnel_bak
sudo twnode policy failover main_traffic tunnel_hk,tunnel_us,tunnel_bak \
  --check-ip 8.8.8.8
```

输出：
```
【策略组故障转移】
策略组: main_traffic
候选出口: tunnel_hk, tunnel_us, tunnel_bak

检查连通性...
  tunnel_hk → 8.8.8.8: 评分 96.5
  tunnel_us → 8.8.8.8: 评分 85.3
  tunnel_bak → 8.8.8.8: 评分 62.0

最佳出口: tunnel_hk (评分 96.5)
当前出口: tunnel_hk

ℹ 出口未变化，无需切换
```

#### 5. 自动化脚本（Cron）

创建定时任务，每 5 分钟检查一次：

```bash
# 编辑 crontab
sudo crontab -e

# 添加定时任务
*/5 * * * * /usr/local/bin/twnode policy failover main_traffic tunnel_hk,tunnel_us,tunnel_bak --check-ip 8.8.8.8 >/dev/null 2>&1
```

## 场景5: 默认路由 + 例外策略

### 需求

- 默认所有流量走 VPN 隧道
- 特定 IP 段走物理接口（例外）

### 配置步骤

#### 1. 设置默认路由

```bash
# 所有流量默认走 VPN
sudo twnode policy set-default tunnel_vpn
```

#### 2. 添加例外策略（更高优先级）

```bash
# 本地网络走物理接口（优先级 100，高于默认路由的 900）
sudo twnode policy create local_traffic eth0 --priority 100
sudo twnode policy add-cidr local_traffic 192.168.0.0/16
sudo twnode policy add-cidr local_traffic 10.0.0.0/8
sudo twnode policy add-cidr local_traffic 172.16.0.0/12
sudo twnode policy apply local_traffic
```

### 流量走向

```
192.168.1.5 → 匹配优先级 100（local_traffic） → eth0
8.8.8.8 → 匹配优先级 900（默认路由） → tunnel_vpn
```

## 场景6: 成本（Cost）机制

### 需求

使用 Cost 值影响故障转移的选择，优先选择低成本隧道。

### 配置步骤

#### 1. 创建策略组并设置 Cost

```bash
# 主隧道（低成本）
sudo twnode policy create main_traffic tunnel_hk --priority 100
# Cost 默认为 0

# 备用隧道1（中等成本）
sudo twnode policy create backup1_traffic tunnel_us --priority 200
# 手动编辑配置文件设置 cost: 5

# 备用隧道2（高成本）
sudo twnode policy create backup2_traffic tunnel_bak --priority 300
# 手动编辑配置文件设置 cost: 10
```

#### 2. 编辑配置文件设置 Cost

```bash
# 编辑 backup1 配置
sudo nano /etc/trueword_node/policies/backup1_traffic.json
```

修改为：
```json
{
  "name": "backup1_traffic",
  "exit_interface": "tunnel_us",
  "priority": 200,
  "from_source": "",
  "cidrs": ["0.0.0.0/0"],
  "cost": 5
}
```

同样设置 backup2_traffic 的 cost 为 10。

#### 3. 执行故障转移

```bash
sudo twnode policy failover main_traffic tunnel_hk,tunnel_us,tunnel_bak \
  --check-ip 8.8.8.8
```

**评分算法**：
```
最终评分 = 基础评分 - (Cost × 0.5)

假设连通性评分都是 90 分：
- tunnel_hk: 90 - (0 × 0.5) = 90.0
- tunnel_us: 90 - (5 × 0.5) = 87.5
- tunnel_bak: 90 - (10 × 0.5) = 85.0

选择 tunnel_hk（最高分）
```

## 优先级管理最佳实践

### 1. 规划优先级范围

提前规划好优先级分层，避免后期冲突：

```
100-199: 核心业务（高优先）
200-299: 普通业务
300-499: 低优先业务
500-699: 备份和测试
700-899: 保留备用
900: 默认路由
```

### 2. 预留优先级间隔

创建策略组时留出间隔，方便后续插入：

```bash
# 不推荐（没有间隔）
priority: 100, 101, 102, 103, ...

# 推荐（有间隔）
priority: 100, 110, 120, 130, ...
```

### 3. 检查优先级冲突

创建新策略组前先查看已有优先级：

```bash
# 查看所有策略组的优先级
sudo twnode policy list

# 选择未使用的优先级创建
sudo twnode policy create new_traffic tunnel_new --priority 150
```

### 4. 使用自动分配（简单场景）

如果不需要精细控制，使用自动分配：

```bash
# 不指定优先级，系统自动分配
sudo twnode policy create auto_traffic tunnel_auto
```

## 常见问题

### Q: 如何查看当前所有的路由规则？

```bash
ip rule show
```

### Q: 如何查看某个路由表的具体路由？

```bash
ip route show table 50
```

### Q: 策略路由不生效怎么办？

**检查步骤**：
1. 确认策略组已应用：`sudo twnode policy list`
2. 检查路由规则：`ip rule show`
3. 测试路由：`ip route get <目标IP>`
4. 重新应用策略：`sudo twnode policy apply <group_name>`

### Q: 如何临时禁用策略组？

```bash
# 撤销策略（保留配置）
sudo twnode policy revoke <group_name>

# 重新启用
sudo twnode policy apply <group_name>
```

### Q: 如何批量管理策略组？

```bash
# 撤销所有策略组
sudo twnode policy revoke

# 重新应用所有策略组
sudo twnode policy apply
```

## 下一步

- [故障转移配置](failover-setup.md) - 深入学习故障转移
- [嵌套隧道](nested-tunnels.md) - 多层隧道架构
- [路由表参考](../reference/routing-tables.md) - 路由表详细说明

---

**导航**: [← GRE over IPsec 配置](gre-ipsec-setup.md) | [返回首页](../index.md) | [故障转移配置 →](failover-setup.md)
