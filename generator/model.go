package generator

// FieldInfo 描述 proto 中的一个字段
type FieldInfo struct {
	Name     string // 字段名，如 "Id"
	ProtoTag int    // proto tag，如 1
	GoType   string // Go 类型，如 "uint64", "string", "uint32"
	IsGob    bool
}

// MessageInfo 描述一个 proto message
type MessageInfo struct {
	PackageName string
	MessageName string
	Fields      []FieldInfo
	Imports     []string // 动态生成的 import 列表，如 []string{"bytes", "encoding/gob", ...}
	Enums       []string // 新增：记录所有自定义枚举类型名，如 ["Gender", "LoginSource"]
}

type EnumInfo struct {
	Name   string // 枚举类型名，如 "UserRole"
	Values []EnumValueInfo
}

type EnumValueInfo struct {
	EnumName string // 枚举类型名，如 "UserRole"
	Name     string // 枚举值名，如 "ROLE_ADMIN"
	Value    int32  // 枚举值，如 1
}
