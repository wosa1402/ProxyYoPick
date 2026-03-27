# ProxyYoPick API 文档

## 启动服务

```bash
proxyyopick web [flags]
```

### 启动参数

| 参数 | 缩写 | 默认值 | 说明 |
|------|------|--------|------|
| `--addr` | | `:8080` | Web 服务监听地址 |
| `--interval` | | `10m` | 自动池定时优选间隔 |
| `--url` | | `https://socks5-proxy.github.io/` | 抓取代理的来源 URL |
| `--concurrency` | `-c` | `500` | 并发测试数 |
| `--timeout` | `-t` | `10s` | 单个代理超时时间 |
| `--target` | `-T` | `http://www.google.com/generate_204` | 测试目标 URL |
| `--ipqs-key` | | | IPQualityScore API key（或环境变量 `IPQS_KEY`） |
| `--scamalytics-user` | | | Scamalytics 用户名（或环境变量 `SCAMALYTICS_USER`） |
| `--scamalytics-key` | | | Scamalytics API key（或环境变量 `SCAMALYTICS_KEY`） |
| `--abuseipdb-key` | | | AbuseIPDB API key（或环境变量 `ABUSEIPDB_KEY`） |
| `--score-cache` | | `~/.proxyyopick/score_cache.json` | 评分缓存文件路径 |

### 启动示例

```bash
# 默认启动（端口 8080，10 分钟自动优选）
proxyyopick web

# 自定义端口和间隔
proxyyopick web --addr :9090 --interval 5m

# 带 IP 评分 API
IPQS_KEY=xxx proxyyopick web
```

---

## 通用说明

- 所有 API 返回 `Content-Type: application/json`
- `pool` 参数支持 `auto`（自动池，默认）和 `manual`（手动池）
- 基础地址：`http://localhost:8080`（取决于 `--addr` 配置）

---

## API 端点

### 1. 获取池统计信息

```
GET /api/stats?pool=auto|manual
```

返回指定代理池的统计数据。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `pool` | string | 否 | `auto`（默认）或 `manual` |

**响应示例：**

```json
{
  "total": 150,
  "success": 42,
  "fail": 108,
  "avg_ms": 320,
  "fastest_ms": 85,
  "updated_at": "2026-02-19 12:30:00",
  "running": false,
  "pool": "auto",
  "accumulated": 500,
  "live": 420,
  "dead_count": 80
}
```

**字段说明：**

| 字段 | 说明 |
|------|------|
| `total` | 本轮测试的代理总数 |
| `success` | 测试成功数 |
| `fail` | 测试失败数 |
| `avg_ms` | 成功代理的平均延迟（毫秒） |
| `fastest_ms` | 最快延迟（毫秒） |
| `updated_at` | 最后更新时间 |
| `running` | 是否正在运行测试 |
| `pool` | 池名称 |
| `accumulated` | 累计发现的代理总数 |
| `live` | 存活代理数 |
| `dead_count` | 已标记死亡的代理数 |

**curl 示例：**

```bash
# 自动池统计
curl http://localhost:8080/api/stats

# 手动池统计
curl http://localhost:8080/api/stats?pool=manual
```

---

### 2. 获取完整测试结果

```
GET /api/results?pool=auto|manual
```

返回指定池中所有代理的完整测试结果（包括成功和失败的）。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `pool` | string | 否 | `auto`（默认）或 `manual` |

**响应示例：**

```json
[
  {
    "proxy": {
      "ip": "1.2.3.4",
      "port": 1080,
      "country": "United States",
      "country_code": "US",
      "is_proxy": false,
      "is_hosting": true,
      "is_mobile": false,
      "isp": "DigitalOcean",
      "quality": "datacenter",
      "scores": {
        "ipqs": 75,
        "scamalytics": 30,
        "abuseipdb": 10
      }
    },
    "success": true,
    "latency_ms": 120,
    "tested_at": "2026-02-19T12:30:00Z"
  },
  {
    "proxy": {
      "ip": "5.6.7.8",
      "port": 1080
    },
    "success": false,
    "latency_ms": 0,
    "error": "connection timeout",
    "tested_at": "2026-02-19T12:30:00Z"
  }
]
```

**curl 示例：**

```bash
curl http://localhost:8080/api/results?pool=auto
```

---

### 3. 获取存活代理（支持国家筛选）

```
GET /api/proxies?pool=auto|manual&country=XX
```

返回指定池中**测试成功（存活）**的代理列表，支持按国家筛选。结果按延迟从低到高排序。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `pool` | string | 否 | `auto`（默认）或 `manual` |
| `country` | string | 否 | 国家代码（如 `US`、`JP`）或国家名（如 `Japan`），不区分大小写。不传则返回所有国家 |

**响应示例：**

```json
{
  "pool": "auto",
  "total": 3,
  "proxies": [
    {
      "ip": "1.2.3.4",
      "port": 1080,
      "country": "United States",
      "country_code": "US",
      "latency_ms": 85,
      "quality": "residential",
      "isp": "Comcast"
    },
    {
      "ip": "5.6.7.8",
      "port": 1080,
      "country": "United States",
      "country_code": "US",
      "latency_ms": 150,
      "quality": "datacenter",
      "isp": "AWS"
    }
  ]
}
```

**字段说明：**

| 字段 | 说明 |
|------|------|
| `pool` | 查询的池名称 |
| `total` | 匹配的存活代理数量 |
| `proxies` | 代理列表 |
| `proxies[].ip` | 代理 IP 地址 |
| `proxies[].port` | 代理端口 |
| `proxies[].country` | 国家名称 |
| `proxies[].country_code` | ISO 3166-1 alpha-2 国家代码 |
| `proxies[].latency_ms` | 延迟（毫秒） |
| `proxies[].quality` | IP 类型：`residential`、`mobile`、`datacenter`、`proxy` |
| `proxies[].isp` | ISP 运营商 |

**curl 示例：**

```bash
# 获取自动池中所有存活代理
curl http://localhost:8080/api/proxies

# 获取自动池中美国的存活代理
curl http://localhost:8080/api/proxies?country=US

# 获取手动池中日本的存活代理
curl "http://localhost:8080/api/proxies?pool=manual&country=JP"

# 按国家名筛选（不区分大小写）
curl "http://localhost:8080/api/proxies?country=Japan"
```

---

### 4. 触发代理测试

```
POST /api/trigger?pool=auto|manual
```

手动触发一次代理抓取+测试周期。自动池会重新抓取代理源；手动池会重新测试已导入的代理。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `pool` | string | 否 | `auto`（默认）或 `manual` |

**响应示例：**

```json
// 成功启动
{"status": "started"}

// 已经在运行中
{"status": "already_running"}

// 手动池为空
{"status": "empty"}
```

**curl 示例：**

```bash
# 触发自动池测试
curl -X POST http://localhost:8080/api/trigger

# 触发手动池重新测试
curl -X POST http://localhost:8080/api/trigger?pool=manual
```

---

### 5. 导入代理到手动池

```
POST /api/import
```

导入代理到手动池。支持纯文本、文件上传、multipart 表单。导入后自动开始测试。新代理会与已有手动池代理合并去重。

**请求方式：**

#### 方式一：纯文本 Body

```bash
curl -X POST http://localhost:8080/api/import \
  -d '1.2.3.4:1080
5.6.7.8:1080
9.10.11.12:1080'
```

#### 方式二：文件上传（multipart/form-data）

```bash
curl -X POST http://localhost:8080/api/import \
  -F "file=@proxies.txt"
```

#### 方式三：文本字段（multipart/form-data）

```bash
curl -X POST http://localhost:8080/api/import \
  -F "text=1.2.3.4:1080
5.6.7.8:1080"
```

**代理格式：** 每行一个 `ip:port`

**响应示例：**

```json
// 成功
{"status": "started", "added": 10, "total": 25}

// 手动池正在测试中
{"status": "error", "error": "手动池测试正在运行中"}

// 无有效数据
{"status": "error", "error": "未提供任何代理数据"}

// 格式错误
{"status": "error", "error": "未解析到有效代理（格式: ip:port，每行一个）"}
```

**字段说明：**

| 字段 | 说明 |
|------|------|
| `status` | `started` 或 `error` |
| `added` | 本次新增的代理数（去重后） |
| `total` | 手动池当前代理总数 |

---

### 6. 清空手动池

```
POST /api/clear-manual
```

清空手动代理池中所有代理和测试结果。

**响应示例：**

```json
// 成功
{"status": "cleared"}

// 手动池正在测试中
{"status": "error", "error": "手动池测试正在运行中"}
```

**curl 示例：**

```bash
curl -X POST http://localhost:8080/api/clear-manual
```

---

## 数据持久化

代理池数据自动保存到磁盘，重启后恢复：

- 自动池：`~/.proxyyopick/pool_auto.json`
- 手动池：`~/.proxyyopick/pool_manual.json`
- 评分缓存：`~/.proxyyopick/score_cache.json`

保存时机：每次测试完成后、服务关闭时。

---

## Web 仪表盘

访问 `http://localhost:8080/` 可打开 Web 仪表盘，提供可视化界面：

- 自动池/手动池切换
- 实时统计卡片（总数、成功率、延迟等）
- 可筛选排序的结果表格
- 手动导入代理（粘贴文本或上传文件）
- 一键触发测试、清空手动池
