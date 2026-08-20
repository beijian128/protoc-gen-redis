package generator

// FieldKind 字段存储形态
type FieldKind string

const (
	// FieldPlain 单值字段（标量/枚举/bytes/嵌套 message）
	FieldPlain FieldKind = "plain"
	// FieldMap map 字段，整体 protobuf wire format 序列化，存单个 hash field
	FieldMap FieldKind = "map"
	// FieldSlice repeated 字段，整体 protobuf wire format 序列化，存单个 hash field
	FieldSlice FieldKind = "slice"
)

// FieldInfo 描述 proto 中的一个字段
type FieldInfo struct {
	Name     string // 字段的 Go 名（camelCase），如 "UserId"
	ProtoTag int    // proto tag，如 1
	GoType   string // Go 类型，如 "uint64", "string", "Gender", "map[string]string"
	Kind     FieldKind

	IsMsg  bool // plain 字段：嵌套 message，整块 protobuf wire format 序列化
	IsEnum bool // plain 字段：是否为枚举（GetFields 需按整数解析并转换）

	// 集合字段（map/slice）的元素信息（整体序列化时仍需要，用于编码/解码）
	KeyType    string // map 键类型
	ElemType   string // 集合元素类型
	ElemIsMsg  bool   // 元素为 message，单元素 protobuf wire format 序列化
	ElemIsEnum bool   // 元素为枚举
}

// MessageInfo 描述一个 proto message
type MessageInfo struct {
	PackageName string
	MessageName string
	FieldType   string // 字段编号类型名（默认 Field<MessageName>，命名冲突时带 X 后缀）
	Fields      []FieldInfo
	Imports     []string // 动态生成的 import 列表，如 []string{"math", "strconv", ...}
	KeyFormat   string   // 生成 Redis key 用的 fmt.Sprintf 格式，如 "REDB#%d:%d:%d"
}

type EnumInfo struct {
	Name   string // 枚举类型的 Go 名，如 "Gender"、"ExtraMsg_State"
	Values []EnumValueInfo
}

type EnumValueInfo struct {
	Name  string // 枚举值完整的常量名，如 "Gender_GENDER_MALE"
	Value int32  // 枚举值，如 1
}
