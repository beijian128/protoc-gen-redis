🚀 proto-gen-redis

proto-gen-redis 是一个基于 https://grpc.io/docs/languages/go/quickstart/  生态的 Protocol Buffers 代码生成插件，用于为 Protobuf 消息自动生成与 Redis Hash 存储交互的 Go 代码。

它帮你减少手写 Redis 存取样板代码，支持：

• ✅ 基于 Protobuf 消息生成对应的 Redis 操作结构体与方法

• ✅ 支持 字段级别的 Get/Set（读取/写入）

• ✅ 支持 Gob 序列化（复杂类型如嵌套 message、slice 等）

• ✅ 支持 枚举类型（自动生成枚举常量）

• ✅ 灵活、可扩展、类型安全

✨ 功能特性

特性 说明

🎯 Redis Hash 存储 为每个 Protobuf Message 生成对应的 Redis Hash 操作代码，字段映射到 Hash Field

🧩 自动生成 Redis 方法 包括 GetFields() 和 SetFields()，支持按需读取/写入字段

🎨 字段常量映射 基于 proto field number 自动生成 Field_<FieldName> = <tag> 常量

📦 Gob 序列化支持 自动识别嵌套 Message、Slice 等复杂类型，使用 Gob 序列化存取

🌐 枚举类型支持 为 Protobuf 枚举生成对应的 Go 枚举类型（如 Gender）及常量（如 Gender_GENDER_UNKNOWN = 0）

🧱 分片 Key 设计 支持自定义业务维度 Key 与分片维度（如 REDB#<key>:<ida>:<idb>）

🛠️ 代码模板驱动 基于 Go text/template，易于扩展与定制

🧩 类型安全 生成的代码与 Protobuf 类型严格对应，包括基础类型、枚举、bytes 等

📦 快速开始

1. 安装 protoc-gen-redis（待发布）

当前为示例，假设你已经将本项目编译为 protoc-gen-redis 可执行文件，并放置在 $PATH 中。

# 待你编译后安装到 PATH，例如：
go install ./cmd/protoc-gen-redis@latest


2. 编译时使用插件

在调用 protoc 时，添加 --redis_out 参数，指定生成的 Redis 代码的输出目录：
protoc \
--go_out=. \
--go_opt=paths=source_relative \
--redis_out=. \
--redis_opt=paths=source_relative \
your_proto_file.proto

示例见 gen_redis.bat

🧪 proto定义示例

输入：example.proto

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


输出：example_redis.gen.go（由 proto-gen-redis 生成）

该文件包含：

• Redis 操作结构体 User

• 字段常量：FieldUser_Name = 1, FieldUser_Age = 2, ...

• 方法：

• GetFields(conn redis.Conn, REDBKey uint32, ida, idb uint64, fields ...FieldUser) error

• SetFields(conn redis.Conn, REDBKey uint32, ida, idb uint64, fields ...FieldUser) error

• 枚举类型 Gender 与常量：Gender_GENDER_UNKNOWN = 0, ...

🛠️ 生成的代码说明

主要结构

• Field<User> 常量：每个 proto 字段对应一个 FieldUser_<FieldName> 常量，值为 proto tag

• User 结构体：与 proto message 字段一一对应

• GetFields()：根据字段编号，从 Redis Hash 中读取值，并填充到结构体

• SetFields()：将结构体字段值存储到 Redis Hash

• Gob 支持：嵌套 message / slice 等类型自动进行 Gob 序列化

• 枚举支持：自动生成枚举类型（如 Gender）及其常量

🧠 设计说明

Redis Key 格式

默认采用如下格式存储用户数据：

REDB#<REDBKey>:<ida>:<idb>


• REDBKey：业务维度 key（如用户ID、租户ID等）

• ida, idb：用于分片的两个维度（如 shard1, shard2），均为 uint64

你可以根据需求在代码中修改 key 的生成逻辑。

字段存储结构

每个 proto message 对应一个 Redis Hash，其中：

• Field：即 proto 字段编号（如 1, 2, 3...），对应 Hash 中的 field key

• Value：字段值（string / int / []byte / gob-encoded 二进制）

🏗️ 安装与使用（开发者向）

1. 克隆项目

git clone <your-repo-url>
cd proto-gen-redis


2. 编译插件

go build -o protoc-gen-redis ./cmd/protoc-gen-redis


3. 安装到 $PATH（可选）

sudo mv protoc-gen-redis /usr/local/bin/


4. 使用 protoc 调用

确保你的 protoc 命令中包含：
--redis_out=. \
--redis_opt=paths=source_relative


⚠️ 注意事项

• 本生成器默认将 嵌套 message、slice、map 等复杂类型使用 Gob 序列化存储为 []byte

• 枚举类型会被生成为 type Gender int32 以及一组常量，Get/Set 时会做类型转换（字符串 → int → 枚举）

• 目前 Key 设计为业务自定义（REDB#...），如需更高级 Key 管理，可扩展模板

• 生成的代码需要依赖 github.com/gomodule/redigo/redis，请确保你的项目引入该包

• 若你的 proto 文件使用了自定义选项，目前暂未支持解析，但可扩展


