# Codex Proxy 代理配置指南

本指南详细说明如何在 Codex Proxy 中配置 HTTP、HTTPS 和 SOCKS5 代理。

## 支持的代理类型

### 1. HTTP 代理

用于将请求转发至 HTTP 代理服务器。

**配置格式:**
```yaml
proxy-url: "http://127.0.0.1:7890"
proxy-url: "http://user:password@127.0.0.1:7890"
```

**常见代理工具:**
- Clash
- V2Ray (with HTTP Server)
- Shadowsocks (with Privoxy)
- Mitmproxy

### 2. HTTPS 代理

用于与代理服务器建立 HTTPS 连接，然后进行 HTTP CONNECT 隧道。

**配置格式:**
```yaml
proxy-url: "https://127.0.0.1:7890"
proxy-url: "https://user:password@127.0.0.1:7890"
```

### 3. SOCKS5 代理

最通用的代理协议，支持本地 DNS 解析和远程 DNS 解析。

**配置格式:**
```yaml
# DNS 由本地解析，仅代理连接
proxy-url: "socks5://127.0.0.1:1080"
proxy-url: "socks5://user:password@127.0.0.1:1080"

# DNS 通过代理解析（避免 DNS 泄露）
proxy-url: "socks5h://127.0.0.1:1080"
proxy-url: "socks5h://user:password@127.0.0.1:1080"
```

**常见代理工具:**
- Clash (SOCKS5 Server)
- V2Ray (SOCKS Server)
- Shadowsocks (with local SOCKS5)
- Xray
- Trojan

## 配置示例

### 示例 1: 使用 Clash 的 SOCKS5 端口

```yaml
# Clash 默认在 127.0.0.1:7891 提供 SOCKS5 服务
proxy-url: "socks5://127.0.0.1:7891"
```

### 示例 2: 使用认证的 SOCKS5 代理

```yaml
# 代理需要用户名和密码
proxy-url: "socks5://username:password@proxy.example.com:1080"

# 使用远程 DNS（避免 DNS 泄露）
proxy-url: "socks5h://username:password@proxy.example.com:1080"
```

### 示例 3: 使用 HTTP 代理

```yaml
# 普通 HTTP 代理
proxy-url: "http://127.0.0.1:7890"

# 带认证的 HTTP 代理
proxy-url: "http://user:pass@proxy.example.com:8080"
```

## 性能调优提示

### 1. 连接池配置

代理会直接影响连接性能。建议调整以下参数：

```yaml
# 减少连接数，避免代理服务器过载
max-conns-per-host: 10
max-idle-conns: 20
max-idle-conns-per-host: 5

# 增加健康检查间隔，减少代理的额外压力
health-check-interval: 600
```

### 2. 超时配置

```yaml
# 上游响应超时（考虑代理延迟）
upstream-timeout-sec: 30

# 连接保活间隔（保持代理连接活跃）
keepalive-interval: 60
```

### 3. SOCKS5 vs HTTP

- **SOCKS5**: 通常性能更好，支持 UDP（未来可用）
- **HTTP**: 兼容性更好，但 CONNECT 隧道有额外开销

建议优先使用 **SOCKS5**。

## 故障排查

### 1. 代理连接超时

**症状:** 请求超时，日志显示连接延迟

**解决方案:**
- 检查代理服务器是否运行且可访问
- 增加 `upstream-timeout-sec` 值
- 确保防火墙允许出站连接

### 2. 代理认证失败

**症状:** 连接拒绝，401 或 403 错误

**解决方案:**
- 验证用户名和密码是否正确
- 对于特殊字符，使用 URL 编码（如 `%40` 代表 `@`）

### 3. DNS 泄露问题

**症状:** 代理地址无法解析

**解决方案:**
- 使用 `socks5h://` 而非 `socks5://` 强制远程 DNS 解析
- 检查代理服务器的 DNS 支持

### 4. 连接复用不充分

**症状:** 并发请求时响应缓慢

**解决方案:**
- 增加 `max-idle-conns-per-host`
- 检查代理服务器的连接限制
- 减小 `refresh-concurrency` 避免滥用代理

## 日志输出示例

当代理配置成功时，启动日志应显示：

```
代理地址: socks5://127.0.0.1:1080
```

或

```
代理地址: http://127.0.0.1:7890
```

如果代理配置有问题，会显示警告：

```
代理 URL 解析失败: ... 将忽略代理配置
SOCKS5 代理创建失败: ... 将使用直连
```

## 测试代理连接

使用 `curl` 测试代理：

```bash
# 测试 SOCKS5
curl -x socks5://127.0.0.1:1080 http://example.com

# 测试 HTTP 代理
curl -x http://127.0.0.1:7890 http://example.com

# 使用代理调用接口
curl -x socks5://127.0.0.1:1080 \
  http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-xxx"
```

## 高级用法

### 组合代理链

某些情况下需要多层代理。使用代理转发工具如 `proxychains` 或 `tun2socks`：

```bash
proxychains -f proxychains.conf ./codex-proxy -config config.yaml
```

### 路由特定主机

使用代理服务器的路由规则只代理特定请求：

**Clash 配置示例:**
```yaml
rules:
  - DOMAIN,chatgpt.com,ProxyGroup
  - DOMAIN,openai.com,ProxyGroup
  - MATCH,DIRECT
```

## 常见问题

**Q: Codex Proxy 支持代理链式配置吗?**
A: 不直接支持。使用外部工具如 proxychains 或配置多层代理。

**Q: 如何监控代理的流量?**
A: 通过代理服务器的监控工具（如 Clash Dashboard）或 tcpdump。

**Q: SOCKS5 和 HTTP 代理哪个更安全?**
A: SOCKS5 + `socks5h`（远程 DNS）更安全，避免 DNS 泄露。

## 相关资源

- [SOCKS5 RFC 1928](https://tools.ietf.org/html/rfc1928)
- [Clash 文档](https://docs.cfw.lbao.top/)
- [V2Ray 文档](https://www.v2fly.org/)
