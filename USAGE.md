# protoc-gen-redis 快速上手 / 使用指南

protoc-gen-redis 是一个 protoc 插件：**用 .proto 定义数据模型，自动生成 Redis Hash 存取的 Go 代码**。你不需要手写任何 Redis 命令，也不需要手写序列化代码。

它解决的核心问题：

- **字段级别的 Get/Set**：按 proto tag 存取 Hash 字段，支持按需读写单个字段
- **集合字段元素级存储**：`map` / `repeated` 按元素拆成独立 Hash 字段，单元素 O(1) 读写，不用整块序列化
- **语言无关的序列化**：嵌套 message 使用**标准 protobuf wire format** 编码，任何语言拿同一份 .proto 就能解析存进 Redis 的字节

---

## 1. 环境准备

| 依赖 | 说明 |
|---|---|
| Go 1.24+ | 构建插件、使用生成代码 |
| protoc | 调用插件编译 .proto（`--plugin` 指定） |
| Redis | 运行环境；测试时可选（连不上会自动跳过） |

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

message User {
  int32 user_id = 1;          // 标量：直接存
  string name = 2;            // 字符串：直接存
  Gender gender = 3;          // 枚举：直接存整数
  bytes avatar = 4;           // 二进制：直接存
  repeated string friends = 5;      // 集合：元素级存储
  map<string, int32> scores = 6;    // 集合：元素级存储
  Address address = 7;              // 嵌套 message：protobuf wire format 序列化

  // 嵌套 message
  message Address {
    string city = 1;
    string street = 2;
  }
}
```

类型映射规则：

| proto 类型 | 生成代码中的 Go 类型 | Redis 存储方式 |
|---|---|---|
| `int32/int64/uint32/uint64/float32/float64/bool` | 对应 Go 标量 | 十进制字符串 |
| `enum` | `type Gender int32` + 常量 | 整数（十进制字符串） |
| `string` | `string` | 原样 |
| `bytes` | `[]byte` | 原样 |
| 嵌套 `message` | 值类型结构体（如 `User_Address`） | **protobuf wire format 二进制** |
| `map<K,V>` | `map[K]V` | 每个键一个 Hash 字段 `<tag>:<key>` |
| `repeated T` | `[]T` | 每个元素一个 Hash 字段 `<tag>:<index>` |

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

u := &cmddb.User{
    UserId:  1001,
    Name:    "alice",
    Gender:  cmddb.Gender_GENDER_FEMALE,
    Avatar:  []byte{0x01, 0x02},
    Friends: []string{"bob", "carol"},
    Scores:  map[string]int32{"math": 95, "english": 88},
    Address: cmddb.User_Address{City: "Shanghai", Street: "Nanjing Rd"},
}

// 整体写入（集合字段自动按元素拆分，message 字段自动 protobuf 序列化）
if err := u.SetFields(conn, 1, 10001, 0); err != nil { log.Fatal(err) }

// 按需读取：只读两个字段（不传字段则读取全部）
got := &cmddb.User{}
if err := got.GetFields(conn, 1, 10001, 0, cmddb.FieldUser_UserId, cmddb.FieldUser_Friends); err != nil {
    log.Fatal(err)
}
fmt.Println(got.UserId, got.Friends) // 1001 [bob carol]
```

字段常量：每个字段对应一个 `FieldUser_<字段名> = <proto tag>` 常量，`SetFields` / `GetFields` 按需读写时使用。Hash 中不存在的字段读取后保持零值。

### 5.2 集合字段的元素级操作（热点写法）

只读写一个元素，只发一条命令，不触碰其他元素：

```go
// repeated：按下标操作
idx, err := got.AppendFriends(conn, 1, 10001, 0, "dave") // 追加，返回新下标
v, ok, err := got.GetFriends(conn, 1, 10001, 0, idx)     // 按下标读
got.SetFriends(conn, 1, 10001, 0, 0, "bob")              // 按下标改
got.DelFriends(conn, 1, 10001, 0, 2)                     // 按下标删

// map：按键操作
got.SetScores(conn, 1, 10001, 0, "physics", 92)
s, ok, err := got.GetScores(conn, 1, 10001, 0, "math")
got.DelScores(conn, 1, 10001, 0, "english")

// 整体替换 / 读取（Lua 原子操作：先清空再写入，repeated 会重建下标 0..n-1）
got.SetFriendsAll(conn, 1, 10001, 0, []string{"x", "y"})
got.SetScoresAll(conn, 1, 10001, 0, map[string]int32{"a": 1})
got.GetFriendsAll(conn, 1, 10001, 0)
got.DelFriendsAll(conn, 1, 10001, 0)
```

每个集合字段生成的方法：`Set<Field> / Get<Field> / Del<Field>`（单元素）、`Append<Field>`（repeated 追加）、`Set<Field>All / Get<Field>All / Del<Field>All`（整体）。message 元素（如 `repeated Weapon`）的单元素操作内部自动做 protobuf 编解码。

### 5.3 直接使用 protobuf 序列化方法

每个 message 都生成 `MarshalRedisProto() ([]byte, error)` 和 `UnmarshalRedisProto([]byte) error`，按 proto3 语义编解码（零值标量跳过、未知字段跳过、兼容其他实现输出的 packed repeated）。可以脱离 Redis 单独用于数据交换：

```go
u := &cmddb.User{Name: "alice", UserId: 1001}
data, err := u.MarshalRedisProto() // 标准 protobuf wire format 字节
other := &cmddb.User{}
other.UnmarshalRedisProto(data)
```

## 6. 跨语言读取（语言无关序列化）

message 字段（以及 message 集合元素）存进 Redis 的是**标准 protobuf wire format** 字节。其他语言只要使用同一份 .proto 生成自己的 protobuf 代码，就能直接解析——这就是"语言无关"的含义。

以 Python 为例，读一个 Go 写入的用户地址：

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
addr = user_pb2.User.Address()
addr.ParseFromString(raw)
print(addr.city)            # Shanghai
```

其他语言读取时需要知道的存储约定：

| 数据 | Hash 字段名 | Value |
|---|---|---|
| 标量 / 枚举 / string / bytes / message | proto tag（如 `"1"`、`"7"`） | 标量为十进制字符串；message 为 protobuf 字节 |
| `map<K,V>` | `"<tag>:<key>"`（如 `"6:math"`） | 元素值（message 元素为 protobuf 字节） |
| `repeated T` | `"<tag>:<index>"`（如 `"5:0"`） | 元素值（message 元素为 protobuf 字节） |

## 7. 测试与演示

```bash
# 单元测试：golden 对比 + 类型映射回归 + protobuf wire 一致性（不依赖外部服务）
go test ./...

# 集成测试：需要本机 Redis（默认 127.0.0.1:6379，可用 bin/config.json 覆盖；连不上自动跳过）
go test ./Test

# 演示程序：真实读写 Redis（全字段 + 元素级操作）
go run ./Test -config bin/config.json
```

## 8. 注意事项

- **输出到独立目录**：生成文件是自包含的（枚举、结构体、序列化方法都重新声明），与 protoc-gen-go 的 `.pb.go` 放同一包会重复定义
- **老数据迁移**：序列化格式从旧版 gob 改为 protobuf wire format 后不兼容，升级需对存量数据一次性迁移
- **跨文件引用**：字段引用其他 .proto 文件的 message 时，被引用的文件也需用本插件生成（生成代码会调用其 `MarshalRedisProto` / `UnmarshalRedisProto`）；`google.protobuf.Timestamp` 等 well-known 类型暂不支持
- **集合字段契约**：无元素时整体读回为 nil；`Del<Field>(i)` 删除后不压缩下标，`Append` 从"最大下标 + 1"继续，需要紧凑序列用 `Set<Field>All` 重建
- **Redis key**：默认 `REDB#<REDBKey>:<ida>:<idb>`，`REDBKey` 为业务维度 key（游戏系统 ID，如家园系统=1），`ida` 为玩家 UID，`idb` 为赛季 ID 等二级维度；可用 `key_format` 定制。以家园系统为例：`REDB#1:123456:3` = 家园系统、玩家 UID 123456、赛季 3
- 生成代码依赖 `github.com/gomodule/redigo/redis`，使用方项目需要引入
