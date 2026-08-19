# 🚀 protoc-gen-redis

protoc-gen-redis 是一个基于 Protocol Buffers (protoc) 生态的代码生成插件，用于为 Protobuf 消息自动生成与 Redis Hash 存储交互的 Go 代码。

它帮你减少手写 Redis 存取样板代码，支持：

- ✅ 基于 Protobuf 消息生成对应的 Redis 操作结构体与方法
- ✅ 字段级别的 Get/Set（读取/写入）
- ✅ Gob 序列化（复杂类型如嵌套 message、slice、map 等）
- ✅ 枚举类型（自动生成枚举常量）
- ✅ 嵌套 message / 嵌套枚举
- ✅ 跨 proto 文件、跨 Go 包的 message / 枚举引用
- ✅ 灵活、可扩展、类型安全

## ✨ 功能特性

| 特性 | 说明 |
|---|---|
| 🎯 Redis Hash 存储 | 为每个 Protobuf Message 生成对应的 Redis Hash 操作代码，字段映射到 Hash Field |
| 🧩 自动生成 Redis 方法 | 包括 `GetFields()` 和 `SetFields()`，支持按需读取/写入字段 |
| 🎨 字段常量映射 | 基于 proto field number 自动生成 `Field_<FieldName> = <tag>` 常量 |
| 📦 元素级集合存储 | map / repeated 按元素拆分为独立 hash field，单元素 O(1) 读写；message 元素单元素 Gob 序列化 |
| 🌐 枚举类型支持 | 为 Protobuf 枚举生成对应的 Go 枚举类型（如 `Gender`）及常量（如 `Gender_GENDER_UNKNOWN = 0`），命名与 protoc-gen-go 一致 |
| 🧱 分片 Key 设计 | 支持自定义业务维度 Key 与分片维度（如 `REDB#<key>:<ida>:<idb>`），格式可通过 `key_format` 参数定制 |
| 🛠️ 代码模板驱动 | 基于 Go text/template，易于扩展与定制 |
| 🧩 类型安全 | 生成的代码与 Protobuf 类型严格对应，包括基础类型、枚举、bytes、repeated、map、嵌套消息等 |

## 📦 快速开始

### 1. 编译插件

```bash
# 在仓库根目录构建
go build -o protoc-gen-redis.exe .
# 并将 protoc-gen-redis.exe 放入 $PATH（或使用 --plugin 指定路径）
```

### 2. 编译时使用插件

在调用 protoc 时，添加 `--redis_out` 参数，指定生成的 Redis 代码的输出目录：

```bash
protoc \
  --redis_out=. \
  --redis_opt=paths=source_relative \
  your_proto_file.proto
```

示例见 `gen_redis.bat`。

**输出位置**（由 `paths` 参数控制）：

- 默认（`paths=import` 或未指定）：输出到 `--redis_out` 根目录，文件名为 `<proto文件基名>.redis.go`，如 `proto/user.proto` → `user.redis.go`
- `paths=source_relative`：按 proto 文件的源路径镜像输出，如 `proto/user.proto` → `proto/user.redis.go`

**其他参数**（通过 `--redis_opt=key=value` 传入，多个参数用逗号分隔）：

- `key_format`：自定义 Redis key 的生成格式，默认 `REDB#%d:%d:%d`（依次填入 REDBKey、ida、idb）。例如 `--redis_opt=key_format=GAME#%d-%d-%d`

> ⚠️ 与 protoc-gen-go 配合使用时，请将 `--redis_out` 输出到**独立的目录**（或单独使用本插件）。生成文件是自包含的：枚举类型、message 结构体都在 `.redis.go` 中重新声明，若与 protoc-gen-go 生成的 `.pb.go` 放到同一个 Go 包，会产生重复定义导致编译失败。

## 🧪 proto 定义示例

输入：`example.proto`

```proto
syntax = "proto3";

package example;

option go_package = "example";

enum Gender {
  GENDER_UNKNOWN = 0;
  GENDER_MALE = 1;
  GENDER_FEMALE = 2;
}

message User {
  string name = 1;
  int32 age = 2;
  Gender gender = 3;
  bytes avatar = 4;
  repeated string friends = 5;   // 集合字段：元素级存储
}
```

输出：`example.redis.go`（由 protoc-gen-redis 生成，gofmt 格式化）

该文件包含：

- 枚举类型 `Gender` 与常量：`Gender_GENDER_UNKNOWN = 0`, ...
- 字段常量：`FieldUser_Name = 1`, `FieldUser_Age = 2`, ...
- Redis 操作结构体 `User`（字段名与 protoc-gen-go 一致，camelCase，如 `user_id` → `UserId`）
- 方法：
  - `GetFields(conn redis.Conn, REDBKey uint32, ida, idb uint64, fields ...FieldUser) error`
  - `SetFields(conn redis.Conn, REDBKey uint32, ida, idb uint64, fields ...FieldUser) error`
  - 集合字段（map/repeated）的元素级方法：`SetFriends/GetFriends/DelFriends/AppendFriends` 等

## 💻 使用示例

```go
import cmddb "your_project/redis" // 生成的 <proto>.redis.go 所在包

conn, _ := redis.Dial("tcp", "127.0.0.1:6379")
defer conn.Close()

u := &cmddb.User{
    Name:    "alice",
    Age:     18,
    Gender:  cmddb.Gender_GENDER_FEMALE,
    Friends: []string{"bob", "carol"}, // 集合字段：按元素存储
}

// 整体写入（集合字段自动按元素拆分成 hash field）
u.SetFields(conn, 1, 10001, 0)

// 按需读取
got := &cmddb.User{}
got.GetFields(conn, 1, 10001, 0, cmddb.FieldUser_Name, cmddb.FieldUser_Friends)

// 元素级操作：只读写一个元素，不触碰其他元素（生产环境的热点写法）
got.SetFriends(conn, 1, 10001, 0, 2, "dave")      // 按下标写
idx, _ := got.AppendFriends(conn, 1, 10001, 0, "erin") // 追加，返回新下标
v, ok, _ := got.GetFriends(conn, 1, 10001, 0, idx)      // 按下标读
got.DelFriends(conn, 1, 10001, 0, 2)                    // 按下标删

// 集合整体读写（原子替换，可修复删除留下的下标空洞）
got.SetFriendsAll(conn, 1, 10001, 0, []string{"x", "y"})
got.GetFriendsAll(conn, 1, 10001, 0)
```

集合字段生成的方法按字段名命名：`friends` 对应 `SetFriends / GetFriends / DelFriends / AppendFriends / GetFriendsAll / SetFriendsAll / DelFriendsAll`；`map` 字段同样有单键的 `Set/Get/Del` 与整体的 `All` 版本。

## 🛠️ 生成的代码说明

**主要结构**

- `Field<User>` 常量：每个 proto 字段对应一个 `FieldUser_<FieldName>` 常量，值为 proto tag
- `User` 结构体：与 proto message 字段一一对应
- `GetFields()`：根据字段编号从 Redis Hash 中读取值，并填充到结构体；数值/枚举解析失败会返回错误（而不是静默保留零值）；Hash 中不存在的字段保持零值
- `SetFields()`：将结构体字段值存储到 Redis Hash
- Gob 支持：嵌套 message 整体、message 集合元素使用 Gob 序列化
- 集合字段元素级存储：map/repeated 按元素拆分，支持单元素 Set/Get/Del/Append（见"集合字段的元素级存储"）
- 枚举支持：自动生成枚举类型（如 `Gender`）及其常量，命名与 protoc-gen-go 完全一致（顶层枚举用枚举名做前缀，嵌套枚举用所在 message 名做前缀）

**覆盖的类型**

- 标量：`int32/int64/uint32/uint64/float32/float64/bool/string/bytes` —— 直接存储
- 枚举 —— 直接存储为整数，读取时自动转换
- 嵌套 message —— Gob 序列化为 `[]byte` 存储
- `map` / `repeated` —— **元素级存储**（见下）

**集合字段的元素级存储（方案 A）**

`map` 与 `repeated` 字段不再整块 gob 序列化，而是按元素拆成独立的 hash field（`<字段编号>:<key|下标>`），只改一个元素就只写一条命令，避免整块序列化/反序列化与并发覆盖：

- `map<K,V>`：`Set<Field>(conn, key, k, v)` / `Get<Field>(conn, key, k)` / `Del<Field>(conn, key, k)`，单键操作
- `repeated`：`Set<Field>(conn, key, i, v)` / `Get<Field>(conn, key, i)` / `Del<Field>(conn, key, i)` / `Append<Field>(conn, key, v)`（返回新下标）
- 整体读写：`Get<Field>All` / `Set<Field>All` / `Del<Field>All`——整体替换用 **Lua 脚本原子完成**（先清空旧元素再写入，repeated 重建下标 0..n-1 可修复删除空洞）
- 元素值规则：标量/枚举/bytes 直接存储，message 元素单元素 gob 序列化
- `GetFields`/`SetFields` 对集合字段自动走元素级存储整体读写，两种方式数据互通
- 契约：集合字段无元素时回读为 nil；`Del<Field>(i)` 删除后不压缩下标，`Append` 从最大下标 + 1 继续

## 🧠 设计说明

### Redis Key 格式

默认采用如下格式存储用户数据：

```
REDB#<REDBKey>:<ida>:<idb>
```

- `REDBKey`：业务维度 key（如用户ID、租户ID等），uint32
- `ida, idb`：用于分片的两个维度（如 shard1, shard2），均为 uint64

可通过 `--redis_opt=key_format=...` 修改格式，例如 `--redis_opt=key_format=USER#%d#%d#%d`。

### 字段存储结构

每个 proto message 对应一个 Redis Hash，其中：

- Field：即 proto 字段编号（如 1, 2, 3...），对应 Hash 中的 field key
- Value：字段值（string / int / []byte / gob-encoded 二进制）

## 🏗️ 安装与使用（开发者向）

1. 克隆项目

```bash
git clone <your-repo-url>
cd protoc-gen-redis
```

2. 编译插件

```bash
go build -o protoc-gen-redis.exe .
```

3. 安装到 $PATH（可选）

```bash
mv protoc-gen-redis.exe /usr/local/bin/   # Windows 下放入 PATH 目录即可
# 或直接安装为全局命令：
go install .
```

4. 使用 protoc 调用

```bash
protoc \
  --plugin=./protoc-gen-redis.exe \
  --redis_out=. \
  --redis_opt=paths=source_relative \
  your_proto_file.proto
```

5. 运行测试

```bash
# 单元测试：golden 对比（generated/user.redis.go）+ 类型映射回归，不依赖外部服务
go test ./...

# 集成测试：需要本机 Redis（默认 127.0.0.1:6379 无密码，
# 可用 bin/config.json 覆盖地址/密码，参考 bin/config.example.json；
# 连不上时集成测试自动跳过）
go test ./Test

# 演示程序：真实读写 Redis（全字段 + 元素级操作）
go run ./Test -config bin/config.json

# 修改模板后刷新 golden 基准文件
UPDATE_GOLDEN=1 go test -run TestGenerateUserProtoGolden .
```

## ⚠️ 注意事项

- 嵌套 message 整体、message 集合元素使用 Gob 序列化存储为 `[]byte`；标量/枚举/bytes 直接存储
- 枚举类型会被生成为 `type Gender int32` 以及一组常量，Get/Set 时会做类型转换（字符串 → int → 枚举）
- **生成的 `.redis.go` 是自包含的**（枚举、结构体独立声明），与 protoc-gen-go 的 `.pb.go` 放到同一 Go 包会导致重复定义，请输出到独立目录
- 集合字段（map/repeated）的元素级存储契约：
  - 无元素时整体读回为 nil；`Del<Field>(i)` 删除后不压缩下标，`Append` 从最大下标 + 1 继续，需要紧凑序列用 `Set<Field>All` 重建
  - 整体替换（`Set<Field>All` 与 `SetFields`）是 Lua 原子操作
  - 元素级存储与旧版整块 gob 格式不兼容，老数据需一次性迁移
- 生成的代码需要依赖 `github.com/gomodule/redigo/redis`，请确保你的项目引入该包
- 若你的 proto 文件使用了自定义选项，目前暂未支持解析，但可扩展
- 更新模板后可用 `UPDATE_GOLDEN=1 go test ./...` 刷新 `generated/user.redis.go` 基准文件
