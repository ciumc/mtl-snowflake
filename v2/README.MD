# mtl-snowflake

**Multi-Timeline Snowflake** — 基于多时间线改进的 Snowflake 分布式ID生成器

## 核心算法

### ID 位结构

```
┌────────┬─────────────────┬──────────┬────────┬────────────┐
│ 符号位 │     时间戳      │  机器ID  │ 时间线 │   序列号   │
│  1bit  │    41bit       │   9bit   │  1bit  │    12bit   │
│   0    │  毫秒偏移      │  0-511   │  0-1   │   0-4095   │
└────────┴─────────────────┴──────────┴────────┴────────────┘
```

| 字段 | 位数 | 范围 | 说明 |
|------|------|------|------|
| 符号位 | 1 | 固定0 | 保证ID为正整数 |
| 时间戳 | 41 | 0 ~ 2^41-1 | 相对于Epoch的毫秒偏移，可用约69年 |
| 机器ID | 9 | 0 ~ 511 | 集群节点标识，最多512节点 |
| 时间线 | 1 | 0 ~ 1 | 时间线编号，默认2条 |
| 序列号 | 12 | 0 ~ 4095 | 同一毫秒内序列，每毫秒最多4096个ID |

---

### 时间线切换算法

**核心创新**：通过多时间线机制解决 NTP 时钟回退问题。

```
初始化：
  timelineProgress[0] = 0  // 时间线0进度
  timelineProgress[1] = 0  // 时间线1进度
  curTimeline = random(0,1) // 随机选择初始时间线

生成流程：
  1. curTime = (now - Epoch) / 1ms
  2. progress = timelineProgress[curTimeline]

  3. if curTime < progress:  // 时钟回退
       if progress - curTime <= 1ms:
           等待时间追回
       else:
           切换到另一条时间线（progress < curTime）

  4. if curTime == progress:
       seq = (seq + 1) & 4095  // 位掩码循环
       if seq == 0:
           等待下一毫秒
     else:
       seq = 0  // 新毫秒重置

  5. timelineProgress[curTimeline] = curTime

  6. ID = curTime << 23 | machineID << 13 | timeline << 12 | seq
```

**时间线切换示意**：

```
时间线0: ████░░░░░░░░░░░░░░  progress=100ms（当前使用）
时间线1: ██░░░░░░░░░░░░░░░░░  progress=50ms（备用）

时钟回退到 30ms：
  → 时间线0: progress=100 > curTime=30，不可用
  → 时间线1: progress=50 > curTime=30，不可用
  → 返回 ErrNoAvailableTimeline

时钟回退到 70ms：
  → 时间线0: progress=100 > curTime=70，不可用
  → 时间线1: progress=50 < curTime=70，可用 ✓
  → 切换到时间线1，继续生成
```

---

### 序列号循环算法

**位掩码技术**：避免分支判断，实现高效循环。

```go
// 传统方式（有分支）
if seq >= 4095 {
    seq = 0
    waitNextMs()
} else {
    seq++
}

// 位掩码方式（无分支）
seq = (seq + 1) & 4095  // seqMask = 2^12 - 1
if seq == 0 {
    waitNextMs()
}
```

**原理**：`& 4095` 限制 seq 在 0-4095 范围内循环，4096 次后自动归零。

---

### 可用年限计算

**时间戳位数与可用年限对应表**：

| TimeBit | 最大毫秒数 | 可用年限 | 计算公式 |
|---------|-----------|---------|---------|
| 41 | 2^41 - 1 = 2,199,023,255,551 | **约 69.7 年** | 2^41 / (365.25 × 24 × 60 × 60 × 1000) |
| 42 | 2^42 - 1 = 4,398,046,511,103 | **约 139.4 年** | 2^42 / (365.25 × 24 × 60 × 60 × 1000) |
| 43 | 2^43 - 1 = 8,796,093,022,207 | **约 278.9 年** | 2^43 / (365.25 × 24 × 60 × 60 × 1000) |

**默认配置示例**：

```
TimeBit = 41
Epoch   = 2025-01-01 00:00:00 UTC

可用范围：
  开始时间：2025-01-01
  结束时间：2094-09-07（约69.7年后）
```

**不同 TimeBit 的推荐 Epoch**：

| TimeBit | 推荐 Epoch | 可用到 |
|---------|-----------|--------|
| 41 | 2025-01-01 | 2094年 |
| 41 | 2020-01-01 | 2089年 |
| 42 | 2025-01-01 | 2164年 |
| 43 | 2000-01-01 | 2279年 |

---

### 可逆字符串算法

**ToReadable**：将64bit ID转换为固定长度数字字符串。

```
结构：[时间前缀] + [合并编码后缀]

FormatYYYYMMDDHHMMSS (24位)：
  前缀 = YYYYMMDDHHMMSS (14位，本地时区)
  后缀 = 毫秒 × 4,194,304 + 机器ID × 8,192 + 时间线 × 4,096 + 序号 (10位)

编码公式：
  encodedSuffix = ms × M + machineID × S1 + timeline × S2 + seq
  其中：M = 512 × 2 × 4096 = 4,194,304
       S1 = 2 × 4096 = 8,192
       S2 = 4096

解码公式：
  seq = encodedSuffix % 4096
  encodedSuffix /= 4096
  timeline = encodedSuffix % 2
  encodedSuffix /= 2
  machineID = encodedSuffix % 512
  ms = encodedSuffix / 512
```

**示例**：

```
ID: 170770166743830528
ToReadable(FormatYYYYMMDDHHMMSS): 202604171339402130710528
                              时间前缀 ↑↑↑↑↑↑↑↑↑↑↑↑↑↑  编码后缀 ↑↑↑↑↑↑↑↑↑↑

FromReadable: 完美还原原始ID
```

---

### 机器ID获取算法

**方案一：指纹哈希**

```go
seed = hostname + "|" + MAC + "|" + IP
hash = SHA256(seed)
machineID = hash[0:8] & (2^MachineIDBit - 1)
```

**方案二：K8s Pod序号**

```go
podName = "id-generator-5"  // StatefulSet格式
ordinal = 5  // 提取最后数字
machineID = ordinal  // 零碰撞
```

---

## 算法对比

### 与 Twitter Snowflake 对比

| 对比项 | Twitter Snowflake | mtl-snowflake |
|--------|-------------------|----------------|
| **ID结构** | 1bit符号 + 41bit时间 + 10bit机器 + 12bit序列 | 1bit符号 + 41bit时间 + 9bit机器 + 1bit时间线 + 12bit序列 |
| **最大节点数** | 1024 | 512 |
| **单机QPS** | 409.6万/秒 | 409.6万/秒 |
| **集群QPS** | 42亿/秒 | 21亿/秒 |
| **时钟回退处理** | 报错/阻塞 | 自动切换时间线 |
| **时间回退容忍** | 0次 | N次（N=时间线数） |
| **时钟同步依赖** | 严格依赖NTP | 容忍NTP波动 |

### 核心差异：时间线机制

**Twitter Snowflake**：时钟回退时直接报错，需要严格的NTP配置。

```
时钟回退 → 报错/阻塞 → 业务中断
```

**mtl-snowflake**：通过时间线切换容忍时钟回退。

```
时钟回退 → 检测幅度 → 小幅等待/大幅切换时间线 → 继续生成
```

### 其他算法对比

| 算法 | 优点 | 缺点 | 适用场景 |
|------|------|------|----------|
| **UUID** | 无协调、全球唯一 | 无序、128位过长 | 非业务主键 |
| **数据库自增** | 简单、有序 | 单点瓶颈、扩展难 | 单机小规模 |
| **Redis INCR** | 高性能、有序 | 依赖Redis、单点 | 中等规模 |
| **Leaf（美团）** | 高性能、双缓冲 | 依赖Zookeeper | 大规模、稳定时钟 |
| **mtl-snowflake** | 无依赖、容忍回退 | 节点数减半 | 大规模、时钟波动环境 |

### 选择建议

```
if 集群节点数 > 512:
    选择 Twitter Snowflake 或 Leaf
elif 服务器时钟稳定（云主机、物理机）:
    选择 Twitter Snowflake
elif 服务器时钟可能波动（K8s、容器、虚拟化）:
    选择 mtl-snowflake ✓
```

---

## 场景推荐

### 场景一：单机/小规模应用

**特点**：节点数 ≤ 10，无需分布式协调。

```go
// 推荐配置：最大化序列号
settings := mtl.Settings{
    TimeBit:      41,
    MachineIDBit: 6,   // 64节点足够
    TimelineBit:  0,   // 禁用时间线（单机时钟稳定）
    SeqBit:       15,  // 32768/ms，高吞吐
    Epoch:        mtl.DefaultEpoch,
}
gen, _ := mtl.NewGeneratorWithSettings(0, settings)

// 单机可达 3276万 ID/秒
```

---

### 场景二：物理机集群部署

**特点**：固定IP、时钟相对稳定、节点数中等。

```go
// 推荐配置：默认配置
gen, _ := mtl.NewGenerator(0)  // 使用 DefaultSettings

// 自动获取机器ID（指纹哈希）
machineID, _ := mtl.GetMachineID(9)  // 9bit = 512节点
gen, _ := mtl.NewGenerator(machineID)

// 512节点集群，每节点 409.6万/秒，总计 2亿/秒
```

**注意事项**：
- 指纹哈希在小规模集群碰撞概率低（10节点 < 1%）
- 使用 chrony 配合 NTP，减少时钟波动

---

### 场景三：K8s StatefulSet 部署（推荐）

**特点**：Pod序号天然唯一、零碰撞、时钟可能波动。

```yaml
# StatefulSet 配置
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: id-generator
spec:
  replicas: 100           # ≤ 512
  serviceName: id-generator
  template:
    spec:
      containers:
      - name: id-generator
        image: id-generator:latest
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
```

```go
// 零碰撞机器ID获取
machineID, _ := mtl.GetMachineIDFromPodName(9)
gen, _ := mtl.NewGenerator(machineID)

// Pod名称: id-generator-0 → machineID=0
// Pod名称: id-generator-5 → machineID=5
```

**优势**：
- ✅ 零碰撞：序号由K8s保证唯一
- ✅ 无外部依赖：仅用 downward API
- ✅ 自动分配：无需手动配置
- ✅ 容忍时钟回退：时间线切换机制

---

### 场景四：云环境/虚拟化部署

**特点**：时钟波动大、NTP同步频繁、需要高容错。

```go
// 推荐配置：增加时间线容错
settings := mtl.Settings{
    TimeBit:      41,
    MachineIDBit: 7,   // 128节点
    TimelineBit:  2,   // 4条时间线，容忍3次连续回退
    SeqBit:       12,  // 4096/ms
    Epoch:        mtl.DefaultEpoch,
}
gen, _ := mtl.NewGeneratorWithSettings(machineID, settings)

// 可容忍连续3次时钟回退
// 回退检测 → 切换时间线1 → 再回退 → 切换时间线2 → ...
```

---

### 场景五：极端时钟波动环境

**特点**：频繁时钟调整、需要最大容错。

```go
// 推荐配置：最大化时间线
settings := mtl.Settings{
    TimeBit:      41,
    MachineIDBit: 6,   // 64节点
    TimelineBit:  3,   // 8条时间线，容忍7次连续回退
    SeqBit:       12,  // 4096/ms
    Epoch:        mtl.DefaultEpoch,
}

// 警告：时间线越多，节点数越少
// TimelineBit=3 时，MachineIDBit=6，仅支持64节点
```

---

### 场景六：订单号/业务流水号

**特点**：需要可读格式、便于客户查询。

```go
// 生成ID
id, _ := gen.Generate()

// 转换为可读格式（24位数字）
readable := gen.ToReadableWithFormat(id, mtl.FormatYYYYMMDDHHMMSS)
// 输出: 202604171339402130710528

// 可读格式特点：
// - 前缀14位：年月日时分秒，便于按时间查询
// - 后缀10位：编码信息，可逆还原原始ID

// 从可读格式还原
originalID, _ := gen.FromReadable(readable, mtl.FormatYYYYMMDDHHMMSS)
// originalID == id ✓
```

**格式选择**：

| 格式 | 长度 | 前缀 | 适用场景 |
|------|------|------|----------|
| FormatYYMMDD | 21位 | 6位年月日 | 当天订单查询 |
| FormatYYYYMMDD | 23位 | 8位年月日 | 按日期查询 |
| FormatYYMMDDHHMMSS | 22位 | 12位年月日时分秒 | 精确时间查询 |
| FormatYYYYMMDDHHMMSS | 24位 | 14位年月日时分秒 | 完整时间显示 |

---

### 配置参数速查表

| 场景 | MachineIDBit | TimelineBit | SeqBit | 节点数 | 回退容忍 |
|------|--------------|-------------|--------|--------|----------|
| 单机高吞吐 | 6 | 0 | 15 | 64 | 0 |
| 物理机集群 | 9 | 1 | 12 | 512 | 1 |
| K8s StatefulSet | 9 | 1 | 12 | 512 | 1 |
| 云环境高容错 | 7 | 2 | 12 | 128 | 3 |
| 极端时钟波动 | 6 | 3 | 12 | 64 | 7 |

---

### 不同配置结果对比

**完整配置参数表**（Epoch=2025-01-01）：

| 配置 | TimeBit | MachineIDBit | TimelineBit | SeqBit | 节点数 | 回退容忍 | 可用年限 | 到期时间 | 单机QPS | 集群QPS |
|------|---------|--------------|-------------|--------|--------|----------|----------|----------|---------|---------|
| **默认配置** | 41 | 9 | 1 | 12 | 512 | 1次 | 69.7年 | 2094-09 | 409.6万/秒 | 20.9亿/秒 |
| **高吞吐** | 41 | 6 | 0 | 16 | 64 | 0次 | 69.7年 | 2094-09 | 655.3万/秒 | 4.1亿/秒 |
| **高吞吐(极端)** | 41 | 0 | 0 | 22 | 1 | 0次 | 69.7年 | 2094-09 | 419.4万/秒 | 419.4万/秒 |
| **高容错** | 41 | 8 | 2 | 12 | 256 | 3次 | 69.7年 | 2094-09 | 409.6万/秒 | 10.5亿/秒 |
| **超高容错** | 41 | 6 | 3 | 13 | 64 | 7次 | 69.7年 | 2094-09 | 819.2万/秒 | 5.2亿/秒 |
| **多节点** | 41 | 10 | 0 | 12 | 1024 | 0次 | 69.7年 | 2094-09 | 409.6万/秒 | 41.9亿/秒 |
| **长时间跨度** | 42 | 8 | 1 | 12 | 256 | 1次 | 139.4年 | 2164-05 | 409.6万/秒 | 10.5亿/秒 |
| **超长时间** | 43 | 6 | 1 | 12 | 64 | 1次 | 278.9年 | 2303-11 | 409.6万/秒 | 2.6亿/秒 |

**关键指标说明**：

```
单机QPS = SeqBit容量 / 毫秒 = 2^SeqBit × 1000
集群QPS = 单机QPS × 节点数 = 2^SeqBit × 1000 × 2^MachineIDBit
节点数   = 2^MachineIDBit
回退容忍 = 时间线数 - 1 = 2^TimelineBit - 1
可用年限 = 2^TimeBit / (365.25 × 24 × 60 × 60 × 1000)
到期时间 = Epoch + 可用年限
```

**ID数值范围对比**：

| 配置 | ID最小值 | ID最大值 | ID示例 |
|------|---------|---------|--------|
| 默认配置 | 0 | 2^63 - 1 ≈ 9.22×10^18 | 170784104755761152 |
| 高吞吐 | 0 | 2^63 - 1 | 170784104755984128 |
| 高容错 | 0 | 2^63 - 1 | 170784104755959552 |

**位数分配可视化**：

```
默认配置 (41+9+1+12=63):
┌────────┬─────────────────┬──────────┬────────┬────────────┐
│  1bit  │    41bit       │   9bit   │  1bit  │    12bit   │
│ 符号位 │    时间戳      │  机器ID  │ 时间线 │   序列号   │
└────────┴─────────────────┴──────────┴────────┴────────────┘

高吞吐配置 (41+6+0+16=63):
┌────────┬─────────────────┬──────────┬────────────────────┐
│  1bit  │    41bit       │   6bit   │       16bit        │
│ 符号位 │    时间戳      │  机器ID  │      序列号        │
└────────┴─────────────────┴──────────┴────────────────────┘
注意：TimelineBit=0 时禁用时间线切换机制

高容错配置 (41+8+2+12=63):
┌────────┬─────────────────┬──────────┬────────┬────────────┐
│  1bit  │    41bit       │   8bit   │  2bit  │    12bit   │
│ 符号位 │    时间戳      │  机器ID  │ 时间线 │   序列号   │
└────────┴─────────────────┴──────────┴────────┴────────────┘
4条时间线，可容忍3次时钟回退
```

**配置选择决策树**：

```
                    开始选择配置
                         │
                         ▼
              ┌─────────────────────┐
              │  需要多少节点？      │
              └─────────────────────┘
                    │
        ┌───────────┼───────────┬─────────────┐
        ▼           ▼           ▼             ▼
     ≤64节点     ≤128节点    ≤512节点     ≤1024节点
        │           │           │             │
        ▼           ▼           ▼             ▼
   MachineIDBit=6  MachineIDBit=7 MachineIDBit=9 MachineIDBit=10
        │           │           │             │
        ▼           ▼           ▼             ▼
   ┌────────┐   ┌────────┐  ┌────────┐    ┌────────┐
   │时钟稳定？│   │时钟稳定？│  │时钟稳定？│    │时钟稳定？│
   └────────┘   └────────┘  └────────┘    └────────┘
        │           │           │             │
    ┌───┴───┐   ┌───┴───┐   ┌───┴───┐     ┌───┴───┐
    ▼       ▼   ▼       ▼   ▼       ▼     ▼       ▼
   稳定    波动  稳定    波动  稳定    波动   稳定    波动
    │       │   │       │   │       │     │       │
    ▼       ▼   ▼       ▼   ▼       ▼     ▼       ▼
 TimelineBit TimelineBit TimelineBit TimelineBit TimelineBit TimelineBit
    =0        =3        =1        =2    =1        =1     =0       =0
    │         │         │         │     │         │      │        │
    ▼         ▼         ▼         ▼     ▼         ▼      ▼        ▼
 SeqBit=16  SeqBit=12  SeqBit=12 SeqBit=12 SeqBit=12 SeqBit=12 SeqBit=12 SeqBit=12
```

---

## 性能优化

### 预计算字段

```go
type Settings struct {
    // 用户配置
    TimeBit, MachineIDBit, TimelineBit, SeqBit uint64
    Epoch int64

    // 预计算（初始化时一次性计算）
    timeMax      int64   // 2^TimeBit - 1
    machineIDMax int64   // 2^MachineIDBit - 1
    seqMask      int64   // 序列号掩码
    timeShift    uint64  // 时间戳左移位数
    ...
}
```

### 零分配写入

```go
// 传统方式（5次分配）
prefix := strconv.Itoa(year) + strconv.Itoa(month) + ...
suffix := strconv.FormatInt(encodedSuffix, 10)
return prefix + suffix

// 零分配方式（1次分配）
var buf [24]byte
writeInt4(&buf, year)      // 直接写入字符
writeInt2(&buf, month)     // '0' + n/10, '0' + n%10
...
return string(buf[:24])    // 仅一次字符串分配
```

---

## 性能基准

| 方法 | 性能 | 内存 |
|------|------|------|
| Generate | **249 ns/op** | **0 allocs** |
| ToReadable | 61 ns/op | 24 B, 1 alloc |
| FromReadable | **28 ns/op** | **0 allocs** |

单机吞吐：**400万 ID/秒**，集群吞吐：**2亿 ID/秒**（512节点）

---

## 快速开始

```go
import mtl "github.com/ciumc/mtl-snowflake/v2"

// 创建生成器
gen, _ := mtl.NewGenerator(0)

// 生成ID
id, _ := gen.Generate()

// 转换可读格式
readable := gen.ToReadableWithFormat(id, mtl.FormatYYYYMMDDHHMMSS)
// 输出: 202604171339402130710528 (24位)

// 解析还原
originalID, _ := gen.FromReadable(readable, mtl.FormatYYYYMMDDHHMMSS)
// originalID == id ✓
```

---

## 错误处理

```go
id, err := gen.Generate()
if errors.Is(err, mtl.ErrTimeBackwardTooFar) {
    // 时钟严重回退
}
if errors.Is(err, mtl.ErrNoAvailableTimeline) {
    // 时间线耗尽
}
```

---

## License

MIT