# 🚀 protoc-gen-redis

protoc-gen-redis 是一个基于 Protocol Buffers (protoc) 生态的代码生成插件，为 Protobuf 消息自动生成与 Redis Hash 存储交互的 Go 代码，省去手写 Redis 存取样板代码。

它把 Redis 当数据库用：一个 message 对应一个 Redis Hash，业务层只有一层数据要写，不存在缓存与持久化数据库之间的双写、失效、回源等一致性问题；生产环境可换用兼容 Redis 协议、支持磁盘持久化的引擎（如腾讯 Tendis），应用层零改动。设计思路详见 [DESIGN.md](DESIGN.md)。

## ✨ 功能特性

- 🎯 **Redis Hash 存储**：一个 proto message 对应一个 Redis Hash，字段映射到 Hash field
- 🧩 **自动生成操作方法**：`GetFields()` / `SetFields()` 按需读写字段；集合字段生成元素级 `Set/Get/Del/Append` 与整体 `All` 方法
- 🏷️ **字段常量映射**：基于 proto field number 生成 `Field_<FieldName> = <tag>` 常量
- 📦 **元素级集合存储**：map / repeated 按元素拆分 hash field，单元素 O(1) 读写；整体替换走 Lua 原子操作
- 🌐 **枚举类型支持**：自动生成 Go 枚举类型与常量，命名与 protoc-gen-go 一致
- 🧱 **分片 Key 设计**：默认 `REDB#<REDBKey>:<ida>:<idb>` 多维分片，格式可经 `key_format` 参数定制
- 💾 **语言无关序列化**：嵌套 message 使用标准 protobuf wire format 编码，任何语言用同一份 .proto 即可解析
- 🔗 **跨文件引用**：支持跨 proto 文件、跨 Go 包的 message / 枚举引用
- 🛠️ **模板驱动**：基于 Go text/template，易于扩展与定制

## 🔢 版本要求

### Redis

生成代码用到的 Redis 命令及对应最低版本：

| 能力 | 用到的命令 | 最低 Redis 版本 |
|---|---|---|
| 标量字段读写、元素级集合操作 | HSET / HGET / HMGET / HDEL | 2.0 |
| Lua 批量操作（集合整体替换/删除、repeated 追加） | EVAL | 2.6 |
| 集合整体读取（`Get<Field>All` / `GetFields` 集合部分） | HSCAN + MATCH | 2.8 |
| **全功能** | 以上全部 | **2.8+** |

建议生产环境使用 **Redis 4.0+**：与 Tendis 各系列的兼容基线（Redis 4.0 / 5.0 协议）保持一致，代码可以在 Redis 与 Tendis 之间无差别切换。

### Tendis

| 系列 | 兼容的 Redis 协议 | Lua（EVAL） | 本项目可用性 |
|---|---|---|---|
| 腾讯云 Tendis 存储版 | Redis 4.0（大部分命令） | ❌ 不支持 | 标量字段与元素级操作可用；依赖 Lua 的集合批量操作不可用 |
| 腾讯云 Tendis 混合存储版 | Redis 4.0 集群版 | ✅ 支持（脚本不跨 slot） | 完整可用 |
| 开源 Tendisplus（Tencent/Tendis） | Redis 5.0 | ✅ 支持 | 完整可用 |

Lua 不可用时受影响的具体操作与注意点，见 [DESIGN.md](DESIGN.md)。

## 📦 快速开始

```bash
# 构建插件（Windows 下产出 protoc-gen-redis.exe，放入 $PATH 或用 --plugin 指定路径）
go build -o protoc-gen-redis.exe .

# 编译 proto，生成 <proto名>.redis.go
protoc \
  --plugin=./protoc-gen-redis.exe \
  --redis_out=. \
  --redis_opt=paths=source_relative \
  your_proto_file.proto
```

示例见 `gen_redis.bat`；`paths`、`key_format` 等参数说明与完整上手教程见 [USAGE.md](USAGE.md)。

> ⚠️ 与 protoc-gen-go 配合使用时，请将 `--redis_out` 输出到**独立的目录**：生成的 `.redis.go` 是自包含的（枚举、结构体独立声明），与 `.pb.go` 放到同一 Go 包会产生重复定义导致编译失败。

## 📚 文档

| 文档 | 内容 |
|---|---|
| [USAGE.md](USAGE.md) | 使用指南：环境准备、proto 定义、代码生成、Go 使用示例、跨语言读取、测试 |
| [DESIGN.md](DESIGN.md) | 设计说明：存储结构、集合字段元素级存储、Tendis 兼容性 |
