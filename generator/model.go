package generator

// FieldKind 字段存储形态
type FieldKind string

const (
	// FieldPlain 单值字段（标量/枚举/bytes/嵌套 message），整体存一个 hash field
	FieldPlain FieldKind = "plain"
	// FieldMap map 字段，元素级存储：每个键一个 hash field（<tag>:<key>）
	FieldMap FieldKind = "map"
	// FieldSlice repeated 字段，元素级存储：每个下标一个 hash field（<tag>:<index>）
	FieldSlice FieldKind = "slice"
)

// FieldInfo 描述 proto 中的一个字段
type FieldInfo struct {
	Name     string // 字段的 Go 名（camelCase），如 "UserId"
	ProtoTag int    // proto tag，如 1
	GoType   string // Go 类型，如 "uint64", "string", "Gender", "map[string]string"
	Kind     FieldKind

	IsGob  bool // plain 字段：复杂类型（嵌套 message）整块 gob 序列化
	IsEnum bool // plain 字段：是否为枚举（GetFields 需按整数解析并转换）

	// 集合字段（map/slice）的元素信息
	KeyType    string // map 键类型
	ElemType   string // 集合元素类型
	ElemIsGob  bool   // 元素为 message，单元素 gob 序列化
	ElemIsEnum bool   // 元素为枚举

	// MethodPrefix 元素级方法名的前缀（默认等于 Name，命名冲突时带 X 后缀），
	// 如 map 字段 settings -> SetSettings / GetSettings / DelSettings / ...
	MethodPrefix string
}

// MessageInfo 描述一个 proto message
type MessageInfo struct {
	PackageName string
	MessageName string
	FieldType   string // 字段编号类型名（默认 Field<MessageName>，命名冲突时带 X 后缀）
	Fields      []FieldInfo
	Imports     []string // 动态生成的 import 列表，如 []string{"bytes", "encoding/gob", ...}
	KeyFormat   string   // 生成 Redis key 用的 fmt.Sprintf 格式，如 "REDB#%d:%d:%d"

	// 集合字段元素级操作使用的 Lua 脚本（函数内局部常量，通过模板 %q 输出）
	LuaReplaceScript      string // map 整体替换：清空 <tag>:* 前缀的旧元素后写入键值对
	LuaSliceReplaceScript string // repeated 整体替换：清空后按下标 0..n-1 重建
	LuaDeleteScript       string // 清空 <tag>:* 前缀的全部元素
	LuaAppendScript       string // repeated 追加：最大下标 + 1 后写入
}

type EnumInfo struct {
	Name   string // 枚举类型的 Go 名，如 "Gender"、"ExtraMsg_State"
	Values []EnumValueInfo
}

type EnumValueInfo struct {
	Name  string // 枚举值完整的常量名，如 "Gender_GENDER_MALE"
	Value int32  // 枚举值，如 1
}
