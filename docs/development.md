# 开发指南

本文档面向希望参与 TrueWord Node 开发的贡献者，提供项目结构、开发规范、测试方法等信息。

## 项目结构

```
trueword_node_go/
├── cmd/
│   └── main.go                  # CLI 入口，命令解析
├── pkg/
│   ├── ipsec/
│   │   ├── tunnel.go           # GRE over IPsec 隧道核心逻辑
│   │   └── tunnel_manager.go   # 隧道管理（创建、删除、启动、停止）
│   ├── wireguard/
│   │   ├── tunnel.go           # WireGuard 隧道核心逻辑
│   │   └── keygen.go           # WireGuard 密钥生成
│   ├── network/
│   │   ├── interface.go        # 物理接口扫描和管理
│   │   ├── parent_interface.go # 父接口列表和管理
│   │   └── check.go            # 连通性检查
│   ├── routing/
│   │   └── policy.go           # 策略路由管理（创建、应用、撤销、故障转移）
│   └── system/
│       └── init.go             # 系统初始化
├── Makefile                     # 构建脚本
├── go.mod                       # Go 模块依赖
├── CLAUDE.md                    # Claude Code 项目指南
├── README.md                    # 项目说明
└── docs/                        # 用户文档
    ├── index.md
    ├── getting-started.md
    ├── architecture.md
    ├── commands/
    ├── tutorials/
    └── reference/
```

## 技术栈

### 语言和框架

- **Go 1.21+** - 主要编程语言
- **Cobra** - CLI 框架（github.com/spf13/cobra）
- **Netlink** - 网络接口管理（github.com/vishvananda/netlink）
- **TableWriter** - 表格显示（github.com/olekukonko/tablewriter）
- **YAML** - 配置文件解析（gopkg.in/yaml.v3）

### 系统依赖

- **iproute2** - ip 命令（网络配置）
- **iptables** - 防火墙管理
- **wireguard-tools** - wg 命令（WireGuard 配置）
- **Linux 内核** - GRE、XFRM（IPsec）、WireGuard 支持

## 开发环境搭建

### 1. 安装 Go

```bash
# Ubuntu/Debian
sudo apt install golang-go

# 或下载最新版本
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

### 2. 克隆项目

```bash
git clone https://github.com/your-org/trueword_node_go.git
cd trueword_node_go
```

### 3. 安装依赖

```bash
go mod download
```

### 4. 构建

```bash
# 开发构建（动态链接）
go build -o bin/twnode cmd/main.go

# 生产构建（静态链接，推荐）
make static
```

### 5. 验证

```bash
./bin/twnode --version
```

## 开发规范

### 代码风格

#### 语言使用

- **代码注释**: 使用简体中文
- **CLI 输出**: 使用简体中文
- **用户交互**: 使用简体中文
- **变量命名**: 使用英文（遵循 Go 命名规范）

#### 示例

```go
// 创建隧道
// Create a tunnel
func CreateTunnel(config *TunnelConfig) error {
    // 验证配置
    if config.Name == "" {
        return fmt.Errorf("隧道名称不能为空")
    }

    // 从父接口获取本地 IP
    localIP, err := getLocalIPFromParent(config.ParentInterface)
    if err != nil {
        return fmt.Errorf("无法从父接口获取 IP: %w", err)
    }

    // 创建隧道接口
    // ...
}
```

### 格式化和检查

```bash
# 格式化代码
make fmt
# 或
go fmt ./...

# 代码检查
make vet
# 或
go vet ./...
```

### 提交规范

#### Git Commit 消息格式

**简短标题**（必需）:
```
添加 WireGuard 动态 IP 容错机制
```

**详细说明**（可选）:
```
- 实现 policy sync-protection 命令
- 自动检测对端 IP 变化并更新保护路由
- 支持 Cron 定时任务
```

#### 完整示例

```
实现动态IP容错机制：智能更新保护路由

新增功能：
- 添加 policy sync-protection 命令
- 自动检测 WireGuard 对端 IP 变化
- 更新保护路由规则和配置文件

技术改进：
- 使用 wg show endpoints 获取运行时对端 IP
- 清理僵尸保护路由规则
- 支持 Cron 定时任务
```

## 核心功能开发

### 添加新的隧道类型

1. **在 `pkg/` 下创建新包**:
   ```
   pkg/
   └── newtunnel/
       ├── tunnel.go
       └── keygen.go
   ```

2. **实现核心接口**:
   ```go
   // pkg/newtunnel/tunnel.go
   package newtunnel

   type Tunnel struct {
       Name       string
       LocalIP    string
       RemoteIP   string
       LocalVIP   string
       RemoteVIP  string
   }

   func (t *Tunnel) Create() error {
       // 创建隧道逻辑
   }

   func (t *Tunnel) Delete() error {
       // 删除隧道逻辑
   }

   func (t *Tunnel) Start() error {
       // 启动隧道逻辑
   }

   func (t *Tunnel) Stop() error {
       // 停止隧道逻辑
   }
   ```

3. **集成到隧道管理器**:
   ```go
   // pkg/ipsec/tunnel_manager.go
   func CreateTunnelByType(config *TunnelConfig) error {
       switch config.TunnelType {
       case "gre":
           return createGRETunnel(config)
       case "wireguard":
           return createWireGuardTunnel(config)
       case "newtunnel":  // 新增
           return createNewTunnel(config)
       default:
           return fmt.Errorf("不支持的隧道类型: %s", config.TunnelType)
       }
   }
   ```

4. **添加 CLI 命令**:
   ```go
   // cmd/main.go
   // 在 line create 命令中添加新类型支持
   ```

5. **记录撤销命令**:
   ```go
   // pkg/newtunnel/tunnel.go
   func (t *Tunnel) recordRevCommands() error {
       revFile := filepath.Join("/var/lib/trueword_node/rev", t.Name+".rev")
       commands := []string{
           "ip link set " + t.Name + " down",
           "ip link del " + t.Name,
           // ...
       }
       return ioutil.WriteFile(revFile, []byte(strings.Join(commands, "\n")), 0644)
   }
   ```

### 添加新的策略路由功能

1. **在 `pkg/routing/policy.go` 中添加函数**:
   ```go
   // 新功能示例：按时间段应用策略
   func ApplyGroupBySchedule(groupName string, schedule string) error {
       // 解析时间段
       // 检查当前时间是否在时间段内
       // 如果在，应用策略
       // 如果不在，撤销策略
   }
   ```

2. **注意策略规则管理原则**:
   ```go
   // ✅ 正确：先添加后清理
   func updatePolicyRule(priority int, cidr string) error {
       // 1. 添加新规则
       exec.Command("ip", "rule", "add", "to", cidr, "lookup", "50", "pref", strconv.Itoa(priority)).Run()

       // 2. 循环清理重复规则
       for {
           cmd := exec.Command("bash", "-c", fmt.Sprintf("ip rule show pref %d | wc -l", priority))
           output, _ := cmd.Output()
           count, _ := strconv.Atoi(strings.TrimSpace(string(output)))
           if count <= 1 {
               break
           }
           exec.Command("ip", "rule", "del", "pref", strconv.Itoa(priority)).Run()
           time.Sleep(100 * time.Millisecond)
       }

       // 3. 验证规则存在
       cmd := exec.Command("bash", "-c", fmt.Sprintf("ip rule show pref %d | wc -l", priority))
       output, _ := cmd.Output()
       count, _ := strconv.Atoi(strings.TrimSpace(string(output)))
       if count == 0 {
           // 规则被意外删除，重新添加
           exec.Command("ip", "rule", "add", "to", cidr, "lookup", "50", "pref", strconv.Itoa(priority)).Run()
       }

       return nil
   }
   ```

3. **添加 CLI 命令**:
   ```go
   // cmd/main.go
   var policyScheduleCmd = &cobra.Command{
       Use:   "apply-schedule <group_name> <schedule>",
       Short: "按时间段应用策略",
       Run: func(cmd *cobra.Command, args []string) {
           // ...
       },
   }
   ```

### 使用网络操作库

**优先使用 netlink 库**，而不是系统命令：

```go
import "github.com/vishvananda/netlink"

// ✅ 推荐：使用 netlink 库
func addGRETunnel(name, local, remote string) error {
    link := &netlink.Gretun{
        LinkAttrs: netlink.LinkAttrs{Name: name},
        Local:     net.ParseIP(local),
        Remote:    net.ParseIP(remote),
    }
    return netlink.LinkAdd(link)
}

// ❌ 不推荐：使用系统命令（仅在 netlink 不支持时使用）
func addGRETunnel(name, local, remote string) error {
    cmd := exec.Command("ip", "tunnel", "add", name, "mode", "gre",
                       "local", local, "remote", remote)
    return cmd.Run()
}
```

**何时使用系统命令**:
- `ip xfrm` - IPsec 配置（netlink 支持有限）
- `wg` - WireGuard 配置（需要 wg-tools）
- `ping` - 连通性测试

## 测试

### 单元测试

```bash
# 运行所有测试
make test
# 或
go test ./...

# 运行特定包的测试
go test ./pkg/routing

# 运行特定测试
go test ./pkg/routing -run TestApplyGroup

# 查看覆盖率
go test -cover ./...
```

### 集成测试

**重要**: 集成测试需要 root 权限和实际的网络环境。

```bash
# 创建测试环境
sudo twnode init

# 运行集成测试
sudo go test ./test/integration -v
```

### 手动测试

```bash
# 1. 编译
make static

# 2. 安装
sudo make install

# 3. 初始化
sudo twnode init

# 4. 创建测试隧道
sudo twnode line create eth0 203.0.113.50 10.0.0.2 10.0.0.1 test_tunnel

# 5. 启动隧道
sudo twnode line start test_tunnel

# 6. 测试连通性
ping 10.0.0.2

# 7. 清理
sudo twnode line delete test_tunnel
```

## 构建和发布

### 构建选项

```bash
# 开发构建（动态链接）
go build -o bin/twnode cmd/main.go

# 静态构建（推荐，适用任何 Linux 系统）
make static

# 验证静态链接
file bin/twnode
# 应显示: statically linked

ldd bin/twnode
# 应显示: not a dynamic executable
```

### 交叉编译

```bash
# 编译 ARM64 版本
GOARCH=arm64 make static

# 编译 386 版本
GOARCH=386 make static
```

### 版本管理

版本号在 `cmd/main.go` 中定义：

```go
const version = "v1.2.0"
```

发布新版本时：

1. 更新版本号
2. 更新 `CLAUDE.md` 中的版本历史
3. 创建 Git tag
4. 构建并发布

```bash
# 更新版本号
# 编辑 cmd/main.go

# 提交更改
git add .
git commit -m "发布 v1.2.0"

# 创建 tag
git tag -a v1.2.0 -m "版本 1.2.0"

# 推送
git push origin main
git push origin v1.2.0

# 构建发布版本
make static
```

## UI 和输出规范

### 命令执行输出

- **正常执行**: 只显示结果，不显示具体命令
- **出错时**: 显示命令和错误输出

```go
func execCommand(name string, args ...string) error {
    cmd := exec.Command(name, args...)
    output, err := cmd.CombinedOutput()
    if err != nil {
        // 仅在出错时显示命令和输出
        fmt.Printf("❌ 命令执行失败: %s %s\n", name, strings.Join(args, " "))
        fmt.Printf("输出: %s\n", string(output))
        return err
    }
    return nil
}
```

### 表格显示

**始终使用 tablewriter 库**处理表格：

```go
import "github.com/olekukonko/tablewriter"

func displayTunnels(tunnels []Tunnel) {
    table := tablewriter.NewWriter(os.Stdout)
    table.SetHeader([]string{"隧道名", "类型", "本地VIP", "对端VIP", "状态"})

    for _, t := range tunnels {
        table.Append([]string{t.Name, t.Type, t.LocalVIP, t.RemoteVIP, t.Status})
    }

    table.Render()
}
```

### 分组标记

使用中文分组标记增强可读性：

```go
fmt.Println("\n【配置信息】")
fmt.Printf("  父接口: %s\n", config.ParentInterface)
fmt.Printf("  本地 IP: %s (自动获取)\n", localIP)

fmt.Println("\n【建立连接】")
fmt.Println("  ✓ 创建 WireGuard 接口")
fmt.Println("  ✓ 配置虚拟 IP")
```

## 贡献流程

### 1. Fork 项目

在 GitHub 上 Fork 项目到你的账户。

### 2. 创建分支

```bash
git checkout -b feature/your-feature-name
```

### 3. 开发和测试

```bash
# 开发
# ...

# 格式化
make fmt

# 代码检查
make vet

# 测试
make test
```

### 4. 提交

```bash
git add .
git commit -m "添加新功能: 你的功能描述"
```

### 5. 推送

```bash
git push origin feature/your-feature-name
```

### 6. 创建 Pull Request

在 GitHub 上创建 Pull Request，描述你的更改。

### PR 描述模板

```markdown
## 更改说明

简要描述你的更改。

## 更改类型

- [ ] 新功能
- [ ] Bug 修复
- [ ] 文档更新
- [ ] 代码重构
- [ ] 性能优化

## 测试

描述你如何测试这些更改。

## 相关 Issue

关联的 Issue 编号（如果有）。
```

## 常见开发任务

### 添加新的 CLI 命令

参见 `cmd/main.go`，使用 Cobra 框架：

```go
var newCmd = &cobra.Command{
    Use:   "new <args>",
    Short: "新命令的简短描述",
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        // 命令逻辑
    },
}

func init() {
    rootCmd.AddCommand(newCmd)
    newCmd.Flags().String("option", "", "选项说明")
}
```

### 修改配置文件格式

1. 更新结构体（如 `pkg/network/tunnel_config.go`）
2. 更新配置文件读写逻辑
3. 更新文档（`docs/reference/config-files.md`）
4. 考虑向后兼容性

### 调试技巧

```go
import "fmt"

// 临时调试输出
fmt.Printf("DEBUG: localIP = %s\n", localIP)

// 或使用 log
import "log"
log.Printf("DEBUG: config = %+v\n", config)
```

生产代码中移除所有调试输出。

## 文档更新

修改功能时，同步更新相关文档：

- `CLAUDE.md` - 项目指南（面向 Claude Code）
- `docs/` - 用户文档
- `README.md` - 项目说明

## 获取帮助

- **Issues**: 在 GitHub 上提交 Issue
- **Discussions**: 在 GitHub Discussions 讨论

## 许可证

本项目采用开源许可证，详见 LICENSE 文件。

---

**导航**: [← 返回首页](index.md) | [架构设计](architecture.md) | [配置文件详解](reference/config-files.md)
