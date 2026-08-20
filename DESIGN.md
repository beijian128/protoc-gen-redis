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

### 集合字段的存储（整体 protobuf 序列化）

`map` 与 `repeated` 字段**不按元素拆分存储**，而是与嵌套 message 一样整体序列化：整个集合编码为 protobuf wire format 二进制，存入一个 hash field（field key 即字段编号）。读写都是整体操作，一条命令完成，不存在元素级读写。

- **约定：集合字段统一用 message 包起来嵌套**。如 `message DBFriendList { repeated string items = 1; }`，消息里声明 `DBFriendList friend_list = 22;`，字段就是普通 message 字段，走 message 序列化，语义清晰、无特殊处理
- 裸 `map` / `repeated` 字段同样支持整体序列化（通常只出现在包裹 message 内部）：每个集合字段额外生成字段级方法 `MarshalRedisProto<Field>()` / `UnmarshalRedisProto<Field>()`（GetFields/SetFields 内部使用）
- 修改集合中的单个元素需要**整体读-改-写**（读回整个集合、改动后整块写回）；业务层无需跟踪"哪个元素被改/增/删"，也就没有脏标记问题
- 存储形态：`map<K,V>` / `repeated T` 的 wire format 编码是语言无关的标准 protobuf 字节（与嵌套 message 一致），任何语言用同一份 .proto 即可解析
- 契约：集合字段无元素时回读为 nil；空集合整体写入后为 hash field 中的空字节，回读同样为 nil

### 集合字段的整体读-改-写与并发

集合字段每次写入都是整块覆盖（HSET 单个 hash field），不存在元素级操作的并发覆盖问题：
读-改-写期间其他写入方可能覆盖整个集合（最后写入者胜出），与普通 message 字段的并发语义一致，业务层按整体值看待集合即可。

## 约定校验（生成期强制）

插件在生成前校验 proto 定义是否符合约定，违反时 protoc 直接报错（编译失败，错误信息指明违规的 message / 字段）：

1. **所有 message 名称必须以 `DB` 前缀开头**（顶层与嵌套都要求，map 合成 entry 除外）
2. **顶层 message 的字段不能直接定义 `repeated` / `map`**——集合字段必须用嵌套 message 包一层

```proto
message DBUserBaseInfo {
  DBFriends friends = 8;              // ✓ 集合字段用 message 包起来
  // repeated string friends = 8;     // ✗ 违反约定：顶层字段不能直接是 repeated
  message DBFriends {
    repeated string items = 1;        // 嵌套 message 内部允许集合字段
  }
}
```

为什么这样约定：顶层 message 对应 Redis Hash（一张"表"），`DB` 前缀让数据表一眼可辨；集合必须整体序列化，直接暴露在顶层容易把"改一个元素"的诉求引向元素级操作，包裹成 message 后字段与普通嵌套 message 完全一致，读写路径唯一、行为统一。

## 生产环境：Tendis 等磁盘持久化引擎的兼容性

生成代码只使用 **HSET / HGET / HMGET / HDEL** 四条基本命令，**不依赖 Lua 脚本（EVAL）与 HSCAN**，任何 RESP 兼容引擎都完整可用：

| 引擎 | 兼容性 |
|---|---|
| 腾讯云 Tendis 存储版 | ✅ 完整可用（其不支持的 EVAL 已不再依赖） |
| 腾讯云 Tendis 混合存储版 | ✅ 完整可用 |
| 开源 Tendisplus（Tencent/Tendis） | ✅ 完整可用 |

各系列的协议兼容版本（Redis 4.0 / 5.0）与可用性总览见 README「版本要求」。集成测试只需把 `bin/config.json` 指向 Tendis 即可直接运行（作为兼容性冒烟测试）。
