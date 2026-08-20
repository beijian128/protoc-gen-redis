# protoc-gen-redis 快速上手 / 使用指南

protoc-gen-redis 是一个 protoc 插件：**用 .proto 定义数据模型，自动生成 Redis Hash 存取的 Go 代码**。你不需要手写任何 Redis 命令，也不需要手写序列化代码。

它解决的核心问题：

- **字段级别的 Get/Set**：按 proto tag 存取 Hash 字段，支持按需读写单个字段
- **集合字段整体序列化**：`map` / `repeated` 与嵌套 message 一样整体走 protobuf wire format，一个 hash field 存整个集合；约定集合字段统一用 message 包一层
- **语言无关的序列化**：嵌套 message 使用**标准 protobuf wire format** 编码，任何语言拿同一份 .proto 就能解析存进 Redis 的字节

---

## 1. 环境准备

| 依赖 | 说明 |
|---|---|
| Go 1.24+ | 构建插件、使用生成代码 |
| protoc | 调用插件编译 .proto（`--plugin` 指定） |
| Redis 2.0+（建议 4.0+） | 运行环境；测试时可选（连不上会自动跳过）。生成代码只用 HSET/HGET/HMGET/HDEL（2.0+），不依赖 Lua 与 HSCAN |
| Tendis（可选） | 磁盘持久化场景替代 Redis：三个系列（存储版/混合存储版/Tendisplus）均完整可用 |

## 2. 安装插件

```bash
# 在仓库根目录构建
go build -o protoc-gen-redis.exe .
# 把 protoc-gen-redis.exe 放入 $PATH（或用 protoc 的 --plugin 参数指定路径）
```

## 3. 定义数据模型

新建 `user.proto`：

```proto
syntax = "proto3";

package example;

option go_package = "your_project/example";

enum Gender {
  GENDER_UNKNOWN = 0;
  GENDER_MALE = 1;
  GENDER_FEMALE = 2;
}

// 顶层 message：名称必须以 DB 开头（生成器强制校验）
message DBUser {
  int32 user_id = 1;          // 标量：直接存
  string name = 2;            // 字符串：直接存
  Gender gender = 3;          // 枚举：直接存整数
  bytes avatar = 4;           // 二进制：直接存
  DBFriends friends = 5;      // 集合字段：必须用 message 包起来（生成器强制校验）
  DBScores scores = 6;        // 集合字段：必须用 message 包起来（生成器强制校验）
  DBAddress address = 7;      // 嵌套 message：protobuf wire format 序列化

  // 集合字段包装 message（约定：集合统一用 message 包起来嵌套）
  message DBFriends {
    repeated string items = 1;
  }
  message DBScores {
    map<string, int32> kv = 1;
  }

  // 嵌套 message
  message DBAddress {
    string city = 1;
    string street = 2;
  }
}
```

**约定（生成期强制校验，违反时 protoc 直接报错）**：

1. 所有 message 名称必须以 `DB` 前缀开头（顶层与嵌套都要）
2. 顶层 message 的字段不能直接定义 `repeated` / `map`，集合字段必须用嵌套 message 包一层

类型映射规则：

| proto 类型 | 生成代码中的 Go 类型 | Redis 存储方式 |
|---|---|---|
| `int32/int64/uint32/uint64/float32/float64/bool` | 对应 Go 标量 | 十进制字符串 |
| `enum` | `type Gender int32` + 常量 | 整数（十进制字符串） |
| `string` | `string` | 原样 |
| `bytes` | `[]byte` | 原样 |
| 嵌套 `message` | 值类型结构体（如 `DBUser_DBFriends`、`DBUser_DBAddress`） | **protobuf wire format 二进制** |
| 包裹 message 内的 `map<K,V>` | `map[K]V`（如 `DBScores.Kv`） | **整个 map 的 protobuf wire format 二进制**（单个 hash field） |
| 包裹 message 内的 `repeated T` | `[]T`（如 `DBFriends.Items`） | **整个 repeated 的 protobuf wire format 二进制**（单个 hash field） |

> 裸 `map` / `repeated` 字段整体序列化能力仍然支持（生成 `MarshalRedisProto<Field>()` / `UnmarshalRedisProto<Field>()` 字段级方法），但按约定只出现在嵌套 message 内部，顶层 message 会被校验拦截。

## 4. 生成代码

```bash
protoc \
  --plugin=./protoc-gen-redis.exe \
  --redis_out=. \
  --redis_opt=paths=source_relative \
  user.proto
```

输出文件与参数：

- 默认输出 `user.redis.go`（放在 `--redis_out` 根目录）；`paths=source_relative` 时按 .proto 的源路径镜像输出（如 `proto/user.proto` → `proto/user.redis.go`）
- `--redis_opt=key_format=...`：自定义 Redis key 格式，默认 `REDB#%d:%d:%d`（依次填入 REDBKey、ida、idb）。例如 `--redis_opt=key_format=GAME#%d-%d-%d`
- 生成文件**自包含**（枚举、结构体、序列化方法全部重新声明），建议输出到独立目录，不要与 protoc-gen-go 的 `.pb.go` 放同一个包

## 5. 在 Go 项目中使用

把生成的包引入项目（示例中 `go_package` 为 `your_project/example`）：

```go
import cmddb "your_project/example" // user.redis.go 所在包
```

### 5.1 整体写入 / 读取

```go
conn, err := redis.Dial("tcp", "127.0.0.1:6379")
if err != nil { log.Fatal(err) }
defer conn.Close()

u := &cmddb.DBUer{
    UserId:  1001,
    Name:    "alice",
    Gender:  cmddb.Gender_GENDER_FEMALE,
    Avatar:  []byte{0x01, 0x02},
    Friends: cmddb.DBUer_DBFriends{Items: []string{"bob", "carol"}},
    Scores:  cmddb.DBUer_DBScores{Kv: map[string]int32{"math": 95, "english": 88}},
    Address: cmddb.DBUer_DBAddress{City: "Shanghai", Street: "Nanjing Rd"},
}

// 整体写入（message 字段自动 protobuf 序列化）
if err := u.SetFields(conn, 1, 10001, 0); err != nil { log.Fatal(err) }

// 按需读取：只读两个字段（不传字段则读取全部）
got := &cmddb.DBUer{}
if err := got.GetFields(conn, 1, 10001, 0, cmddb.FieldDBUer_UserId, cmddb.FieldDBUer_Friends); err != nil {
    log.Fatal(err)
}
fmt.Println(got.UserId, got.Friends.Items) // 1001 [bob carol]
```

字段常量：每个字段对应一个 `FieldDBUer_<字段名> = <proto tag>` 常量，`SetFields` / `GetFields` 按需读写时使用。Hash 中不存在的字段读取后保持零值；数值/枚举字段解析失败会返回错误（而不是静默保留零值）。

### 5.2 集合字段（message 包裹）整体读写

集合字段与普通 message 字段一样整体存取：一个 hash field 存整个集合的 protobuf 字节，一条命令完成。修改集合中的单个元素需要先读回整个集合、改动后再整块写回（整体读-改-写）：

```go
// 整体写入
u.Friends = cmddb.DBUer_DBFriends{Items: []string{"bob", "carol"}}
u.Scores = cmddb.DBUer_DBScores{Kv: map[string]int32{"math": 95}}
if err := u.SetFields(conn, 1, 10001, 0, cmddb.FieldDBUer_Friends, cmddb.FieldDBUer_Scores); err != nil {
    log.Fatal(err)
}

// 整体读回（未写入过 / 空集合的 Items/Kv 回读为 nil）
got := &cmddb.DBUer{}
if err := got.GetFields(conn, 1, 10001, 0, cmddb.FieldDBUer_Friends, cmddb.FieldDBUer_Scores); err != nil {
    log.Fatal(err)
}
fmt.Println(got.Friends.Items, got.Scores.Kv) // [bob carol] map[math:95]

// 修改单个元素：读-改-写
if err := got.GetFields(conn, 1, 10001, 0, cmddb.FieldDBUer_Friends); err != nil { log.Fatal(err) }
got.Friends.Items = append(got.Friends.Items, "dave")
got.Friends.Items = got.Friends.Items[1:] // 删除第一个
if err := got.SetFields(conn, 1, 10001, 0, cmddb.FieldDBUer_Friends); err != nil { log.Fatal(err) }
```

集合字段**没有元素级操作**：不需要业务层声明"改了哪个元素/增删了哪个"，也就没有脏标记维护负担；代价是单元素修改要整块读-改-写（并发下是整体覆盖语义，与普通 message 字段一致）。

### 5.3 直接使用 protobuf 序列化方法

每个 message 都生成 `MarshalRedisProto() ([]byte, error)` 和 `UnmarshalRedisProto([]byte) error`，按 proto3 语义编解码（零值标量跳过、未知字段跳过、兼容其他实现输出的 packed repeated）。可以脱离 Redis 单独用于数据交换：

```go
u := &cmddb.DBUer{Name: "alice", UserId: 1001}
data, err := u.MarshalRedisProto() // 标准 protobuf wire format 字节
other := &cmddb.DBUer{}
other.UnmarshalRedisProto(data)
```

## 6. 跨语言读取（语言无关序列化）

message 字段、集合字段（包裹 message 整体）存进 Redis 的都是**标准 protobuf wire format** 字节。其他语言只要使用同一份 .proto 生成自己的 protobuf 代码，就能直接解析——这就是"语言无关"的含义。

以 Python 为例，读一个 Go 写入的用户地址与好友列表：

```python
import redis
import user_pb2  # 同一份 user.proto 用 protoc --python_out 生成

r = redis.Redis(host="127.0.0.1", port=6379)
key = "REDB#1:10001:0"      # 与 Go 侧 key_format 保持一致

user_id = r.hget(key, 1)    # 标量字段：Hash 字段名就是 proto tag
name = r.hget(key, 2)
avatar = r.hget(key, 4)     # bytes 原样

# message 字段（tag 7）：字节就是 protobuf wire format，直接解析
raw = r.hget(key, 7)
addr = user_pb2.DBUer.DBAddress()
addr.ParseFromString(raw)
print(addr.city)            # Shanghai

# 集合字段（tag 5，message 包裹的约定写法）：字节就是 DBFriends 的 protobuf 编码
raw = r.hget(key, 5)
friends = user_pb2.DBUer.DBFriends()
friends.ParseFromString(raw)
print(friends.items)        # ['bob', 'carol']
```

其他语言读取时需要知道的存储约定：

| 数据 | Hash 字段名 | Value |
|---|---|---|
| 标量 / 枚举 / string / bytes / message | proto tag（如 `"1"`、`"7"`） | 标量为十进制字符串；message 为 protobuf 字节 |
| 包裹 message 内的 `map<K,V>` | proto tag（如 `"6"`） | 整个包裹 message 的 protobuf 字节（内含 map entry 子消息） |
| 包裹 message 内的 `repeated T` | proto tag（如 `"5"`） | 整个包裹 message 的 protobuf 字节（内含 repeated 元素） |

## 7. 测试与演示

```bash
# 单元测试：golden 对比 + 类型映射回归 + protobuf wire 一致性（不依赖外部服务）
go test ./...

# 集成测试：需要本机 Redis（默认 127.0.0.1:6379，可用 bin/config.json 覆盖；连不上自动跳过）
go test ./Test

# 演示程序：真实读写 Redis（全字段 + 集合字段整体读写）
go run ./Test -config bin/config.json

# 修改模板后刷新 golden 基准文件（单元测试基准对比用）
UPDATE_GOLDEN=1 go test -run TestGenerateUserProtoGolden .
```

## 8. 注意事项

- **输出到独立目录**：生成文件是自包含的（枚举、结构体、序列化方法都重新声明），与 protoc-gen-go 的 `.pb.go` 放同一包会重复定义
- **跨文件引用**：字段引用其他 .proto 文件的 message 时，被引用的文件也需用本插件生成（生成代码会调用其 `MarshalRedisProto` / `UnmarshalRedisProto`）；`google.protobuf.Timestamp` 等 well-known 类型暂不支持
- **message 命名与结构约定（生成期强制校验）**：所有 message 名称必须以 `DB` 前缀开头；顶层 message 的字段不能直接定义 `repeated` / `map`，集合字段必须用嵌套 message 包一层。违反约定时 protoc 生成直接报错
- **集合字段行为**：集合字段（包裹 message）整体 protobuf 序列化，存单个 hash field，没有元素级操作，修改单个元素需整体读-改-写；包裹 message 内的集合无元素时回读为 nil
- 生成代码依赖 `github.com/gomodule/redigo/redis`，使用方项目需要引入
- 若 proto 文件使用了自定义选项，目前暂未支持解析，但可扩展
- Redis key 格式、集合字段整体序列化、约定校验、Tendis 兼容性等设计细节见 [DESIGN.md](DESIGN.md)
