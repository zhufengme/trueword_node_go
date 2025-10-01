# 使用示例

## 初始化系统

```bash
# 首次运行，初始化系统环境
sudo ./bin/twnode init
```

初始化过程会:
1. 检查系统软件
2. 启用IP转发
3. 配置iptables
4. **扫描并保存物理接口配置**

## 创建隧道 - 交互式(推荐)

```bash
sudo ./bin/twnode line create
```

系统会:
1. **列出可用的父接口**(物理接口 + 已创建的隧道)
2. **让你选择父接口**(不需要输入本地IP!)
3. 输入: remote_ip, remote_vip, local_vip
4. **本地IP自动从父接口获取**

## 创建隧道 - 命令行模式

```bash
# 格式: twnode line create <parent_interface> <remote_ip> <remote_vip> <local_vip> [tunnel_name]

# 在物理接口eth0上创建隧道
sudo ./bin/twnode line create eth0 1.2.3.4 10.0.0.2 10.0.0.1 tun01 \
  --auth-key mykey --enc-key mykey

# 在隧道tun01上创建嵌套隧道
sudo ./bin/twnode line create tun01 10.0.0.2 10.1.0.2 10.1.0.1 tun02 \
  --auth-key mykey2 --enc-key mykey2
```

## 关键改进

✅ **不再需要输入本地IP** - 自动从父接口获取  
✅ **自动选择父接口** - 物理接口或隧道  
✅ **自动配置策略路由** - 使用父接口的网关  
✅ **支持多层嵌套** - 隧道上可以再建隧道  
✅ **GRE Key自动生成** - 从auth密钥生成  
✅ **路由表80** - 虚拟IP路由管理  
