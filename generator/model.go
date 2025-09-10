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
}
