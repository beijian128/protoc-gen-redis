package generator

// FieldInfo 描述 proto 中的一个字段
type FieldInfo struct {
	Name     string // 字段的 Go 名（camelCase），如 "UserId"
	ProtoTag int    // proto tag，如 1
	GoType   string // Go 类型，如 "uint64", "string", "Gender", "map[string]string"
	IsGob    bool   // 复杂类型（repeated/map/嵌套 message）是否用 gob 序列化存储
	IsEnum   bool   // 是否为枚举字段（GetFields 需按整数解析并做类型转换）
}

// MessageInfo 描述一个 proto message
type MessageInfo struct {
	PackageName string
	MessageName string
	Fields      []FieldInfo
	Imports     []string // 动态生成的 import 列表，如 []string{"bytes", "encoding/gob", ...}
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
