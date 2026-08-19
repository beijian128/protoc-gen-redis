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
| 📦 Gob 序列化支持 | 自动识别 nested message、repeated、map 等复杂类型，使用 Gob 序列化存取 |
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

## 🛠️ 生成的代码说明

**主要结构**

- `Field<User>` 常量：每个 proto 字段对应一个 `FieldUser_<FieldName>` 常量，值为 proto tag
- `User` 结构体：与 proto message 字段一一对应
- `GetFields()`：根据字段编号从 Redis Hash 中读取值，并填充到结构体；数值/枚举解析失败会返回错误（而不是静默保留零值）；Hash 中不存在的字段保持零值
- `SetFields()`：将结构体字段值存储到 Redis Hash
- Gob 支持：nested message / repeated / map 等类型自动进行 Gob 序列化
- 枚举支持：自动生成枚举类型（如 `Gender`）及其常量，命名与 protoc-gen-go 完全一致（顶层枚举用枚举名做前缀，嵌套枚举用所在 message 名做前缀）

**覆盖的类型**

- 标量：`int32/int64/uint32/uint64/float32/float64/bool/string/bytes` —— 直接存储
- 枚举 —— 直接存储为整数，读取时自动转换
- `repeated`、`map`、嵌套 message —— Gob 序列化为 `[]byte` 存储

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
go test ./...   # 包含 golden 测试（对比 generated/user.redis.go）与类型映射回归测试
```

## ⚠️ 注意事项

- 本生成器默认将 嵌套 message、repeated、map 等复杂类型使用 Gob 序列化存储为 `[]byte`
- 枚举类型会被生成为 `type Gender int32` 以及一组常量，Get/Set 时会做类型转换（字符串 → int → 枚举）
- **生成的 `.redis.go` 是自包含的**（枚举、结构体独立声明），与 protoc-gen-go 的 `.pb.go` 放到同一 Go 包会导致重复定义，请输出到独立目录
- 生成的代码需要依赖 `github.com/gomodule/redigo/redis`，请确保你的项目引入该包
- 若你的 proto 文件使用了自定义选项，目前暂未支持解析，但可扩展
- 更新模板后可用 `UPDATE_GOLDEN=1 go test ./...` 刷新 `generated/user.redis.go` 基准文件
