# protoc-gen-redis 设计说明

## 设计背景：为什么直接用 Redis 当数据库

游戏开发场景下，传统方案是缓存（Redis）+ 持久化数据库（MySQL/Mongo）两级存储：业务层需要自己维护缓存与数据库之间的一致性（双写、失效、回源、补偿），逻辑复杂且容易出并发问题。

本项目的思路是**把 Redis 当数据库用，绕开两级一致性问题**：

- 用 Protobuf 定义数据结构，一个 message 对应一个 Redis Hash，字段即 Hash field
- 业务层只有一层数据要写，不存在缓存与持久化的一致性维护
- 内存 Redis 成本昂贵，生产环境可选用**兼容 Redis 协议、支持磁盘持久化的数据库**（如腾讯 Tendis），应用层零改动

## 存储结构

### Redis Key 格式

默认采用如下格式存储用户数据：

```
REDB#<REDBKey>:<ida>:<idb>
```

三个维度由业务自定义（插件只按 key_format 顺序填入），游戏开发中常见的划分：

- `REDBKey`：业务维度，通常是系统 ID（如家园系统=1、好友系统=2…），uint32
- `ida`：玩家 UID，uint64
- `idb`：二级区分 ID（赛季、角色等，不需要填 0），uint64

以家园系统为例：`REDB#1:123456:3` 表示家园系统、玩家 UID 123456、赛季 3。

可通过 `--redis_opt=key_format=...` 修改格式，例如 `--redis_opt=key_format=USER#%d#%d#%d`。

### 字段存储结构

每个 proto message 对应一个 Redis Hash，其中：

- Field：即 proto 字段编号（如 1, 2, 3...），对应 Hash 中的 field key
- Value：字段值（string / int / []byte / protobuf wire format 编码的二进制）

### 集合字段的元素级存储（方案 A）

`map` 与 `repeated` 字段不整块序列化，而是按元素拆成独立的 hash field（`<字段编号>:<key|下标>`），只改一个元素就只写一条命令，避免整块序列化/反序列化与并发覆盖：

- `map<K,V>`：`Set<Field>(conn, key, k, v)` / `Get<Field>(conn, key, k)` / `Del<Field>(conn, key, k)`，单键操作
- `repeated`：`Set<Field>(conn, key, i, v)` / `Get<Field>(conn, key, i)` / `Del<Field>(conn, key, i)` / `Append<Field>(conn, key, v)`（返回新下标）
- 整体读写：`Get<Field>All` / `Set<Field>All` / `Del<Field>All`——整体替换用 **Lua 脚本原子完成**（先清空旧元素再写入，repeated 重建下标 0..n-1 可修复删除空洞）
- 元素值规则：标量/枚举/bytes 直接存储，message 元素单元素 protobuf wire format 序列化
- `GetFields`/`SetFields` 对集合字段自动走元素级存储整体读写，两种方式数据互通
- 契约：集合字段无元素时回读为 nil；`Del<Field>(i)` 删除后不压缩下标，`Append` 从最大下标 + 1 继续；需要紧凑序列用 `Set<Field>All` 重建

## 生产环境：Tendis 等磁盘持久化引擎的兼容性

生成代码的命令选型与 Tendis 各版本的兼容情况：

| Tendis 版本 | Lua 脚本（EVAL） | 对本项目的影响 |
|---|---|---|
| 腾讯云 Tendis 存储版 | ❌ 不支持（`eval`/`evalsha` 在不支持命令清单中） | 集合字段的批量操作不可用 |
| 腾讯云 Tendis 混合存储版 | ✅ 支持（要求脚本不跨 slot） | 本项目脚本只操作 `KEYS[1]` 单 key，天然满足 |
| 开源 Tendisplus（Tencent/Tendis） | ✅ 完整支持 | 无影响 |

各系列的协议兼容版本（Redis 4.0 / 5.0）与可用性总览见 README「版本要求」。

**依赖 Lua 脚本（EVAL）的操作**——仅在支持 Lua 的引擎上可用：

- `SetFields` 写入集合字段（内部走 `Set<Field>All`）
- `Set<Field>All`（整体替换）、`Del<Field>All`（整体删除）
- `Append<Field>`（repeated 追加，靠脚本找最大下标）

**纯命令实现、任何 RESP 兼容引擎可用**：

- `GetFields` / `SetFields` 的标量字段（HMGET / HSET）
- 元素级 `Set<Field>` / `Get<Field>` / `Del<Field>`（HSET / HGET / HDEL）
- `Get<Field>All`（HSCAN + MATCH，Tendis 支持 Scan 族命令）

注意：Tendis 这类磁盘引擎上 HSCAN 是全量遍历，`Get<Field>All` 的性能比内存 Redis 差，Hash 较大时需按业务评估。集成测试只需把 `bin/config.json` 指向 Tendis 即可直接运行（存储版会挂掉全部 Lua 用例，可当兼容性冒烟测试）。
