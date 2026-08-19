package generator

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// DefaultKeyFormat 是生成 Redis key 的默认格式，依次填入 REDBKey、ida、idb。
// 可通过 --redis_opt=key_format=... 覆盖。
const DefaultKeyFormat = "REDB#%d:%d:%d"

// 集合字段元素级操作使用的 Lua 脚本。
// 约定：KEYS[1] 为消息的 hash key，ARGV[1] 为字段编号（元素 hash field 形如 "<tag>:<key>"）。
const luaReplaceScript = `local prefix = ARGV[1]..":"
local cursor = "0"
repeat
  local r = redis.call("HSCAN", KEYS[1], cursor, "MATCH", prefix.."*", "COUNT", 512)
  cursor = r[1]
  local entries = r[2]
  for i = 1, #entries, 2 do
    redis.call("HDEL", KEYS[1], entries[i])
  end
until cursor == "0"
for i = 2, #ARGV, 2 do
  redis.call("HSET", KEYS[1], prefix..ARGV[i], ARGV[i+1])
end
return 1`

const luaDeleteScript = `local prefix = ARGV[1]..":"
local cursor = "0"
repeat
  local r = redis.call("HSCAN", KEYS[1], cursor, "MATCH", prefix.."*", "COUNT", 512)
  cursor = r[1]
  local entries = r[2]
  for i = 1, #entries, 2 do
    redis.call("HDEL", KEYS[1], entries[i])
  end
until cursor == "0"
return 1`

const luaAppendScript = `local prefix = ARGV[1]..":"
local max = -1
local cursor = "0"
repeat
  local r = redis.call("HSCAN", KEYS[1], cursor, "MATCH", prefix.."*", "COUNT", 512)
  cursor = r[1]
  local entries = r[2]
  for i = 1, #entries, 2 do
    local idx = tonumber(string.sub(entries[i], #prefix + 1))
    if idx and idx > max then max = idx end
  end
until cursor == "0"
redis.call("HSET", KEYS[1], prefix..(max + 1), ARGV[2])
return max + 1`

// luaSliceReplaceScript 与 luaReplaceScript 的区别：repeated 只传元素值（无键），
// 清空后按下标 0..n-1 重建，可顺带修复删除留下的下标空洞。
const luaSliceReplaceScript = `local prefix = ARGV[1]..":"
local cursor = "0"
repeat
  local r = redis.call("HSCAN", KEYS[1], cursor, "MATCH", prefix.."*", "COUNT", 512)
  cursor = r[1]
  local entries = r[2]
  for i = 1, #entries, 2 do
    redis.call("HDEL", KEYS[1], entries[i])
  end
until cursor == "0"
for i = 2, #ARGV do
  redis.call("HSET", KEYS[1], prefix..(i - 2), ARGV[i])
end
return 1`

// GenerateRedisCode 为一个 message 生成 Redis 存取代码。
func GenerateRedisCode(gen *protogen.Plugin, file *protogen.File, msg *protogen.Message, g *protogen.GeneratedFile, keyFormat string) ([]byte, error) {
	fieldTypes := resolveFieldTypeNames(CollectMessages(file))

	fields := make([]FieldInfo, 0, len(msg.Fields))
	for _, field := range msg.Fields {
		ft := fieldTypeFor(gen, g, field)
		fields = append(fields, FieldInfo{
			Name:         field.GoName,
			ProtoTag:     int(field.Desc.Number()),
			GoType:       ft.goType,
			Kind:         ft.kind,
			KeyType:      ft.keyType,
			ElemType:     ft.elemType,
			ElemIsMsg:    ft.elemIsMsg,
			ElemIsEnum:   ft.elemIsEnum,
			IsMsg:        ft.wholeMsg,
			IsEnum:       ft.isEnum,
			MethodPrefix: field.GoName,
		})
	}
	resolveMethodPrefixes(fields)

	info := MessageInfo{
		PackageName:           string(file.GoPackageName),
		MessageName:           string(msg.GoIdent.GoName),
		FieldType:             fieldTypes[msg],
		Fields:                fields,
		KeyFormat:             keyFormat,
		LuaReplaceScript:      luaReplaceScript,
		LuaSliceReplaceScript: luaSliceReplaceScript,
		LuaDeleteScript:       luaDeleteScript,
		LuaAppendScript:       luaAppendScript,
	}

	tmpl, err := template.New("redis_code").Parse(codeTemplate)
	if err != nil {
		return nil, fmt.Errorf("解析模板失败: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, info); err != nil {
		return nil, fmt.Errorf("渲染模板失败: %v", err)
	}

	return buf.Bytes(), nil
}

// resolveMethodPrefixes 为集合字段确定元素级方法名的前缀。
// 元素方法（Get<F>/Set<F>/Del<F>/Append<F>/Get<F>All/...）与既有方法 GetFields/SetFields
// 以及同消息其他集合字段的方法共享命名空间，冲突时按 protoc-gen-go 惯例追加 X 消歧。
func resolveMethodPrefixes(fields []FieldInfo) {
	taken := map[string]bool{"GetFields": true, "SetFields": true}
	for i := range fields {
		if fields[i].Kind == FieldPlain {
			continue
		}
		prefix := fields[i].Name
		for {
			names := []string{
				"Get" + prefix, "Set" + prefix, "Del" + prefix,
				"Get" + prefix + "All", "Set" + prefix + "All", "Del" + prefix + "All",
			}
			if fields[i].Kind == FieldSlice {
				names = append(names, "Append"+prefix)
			}
			conflict := false
			for _, n := range names {
				if taken[n] {
					conflict = true
					break
				}
			}
			if !conflict {
				for _, n := range names {
					taken[n] = true
				}
				fields[i].MethodPrefix = prefix
				break
			}
			prefix += "X"
		}
	}
}

// resolveFieldTypeNames 为每个 message 确定"字段编号类型"的名字（默认 Field<MessageName>）。
// 该类型名与字段常量（Field<MessageName>_<FieldName>）共用 Field 前缀，
// 当某个 message 存在与字段同名的嵌套类型时（如字段 profile + 嵌套 message Profile，
// 常量 FieldUserBaseInfo_Profile 与类型 FieldUserBaseInfo_Profile 冲突），
// 按 protoc-gen-go 惯例给类型名追加 X 后缀消歧，保证生成代码可编译。
func resolveFieldTypeNames(messages []*protogen.Message) map[*protogen.Message]string {
	names := make(map[*protogen.Message]string, len(messages))
	for _, m := range messages {
		names[m] = "Field" + string(m.GoIdent.GoName)
	}
	for iter := 0; iter < 10; iter++ {
		// 全文件所有类型名 / IDs 变量名 -> 归属 message
		owner := make(map[string]*protogen.Message, len(names)*2)
		for _, m := range messages {
			owner[names[m]] = m
			owner[names[m]+"IDs"] = m
		}
		changed := false
		for _, m := range messages {
			t := names[m]
			for _, f := range m.Fields {
				// 常量撞上了某个 message 的类型/IDs 名 → 重命名类型的归属者
				if victim := owner[t+"_"+f.GoName]; victim != nil {
					names[victim] = names[victim] + "X"
					changed = true
					break
				}
			}
		}
		if !changed {
			return names
		}
	}
	return names
}

// GenerateRedisCodeHeadWithEnums 生成文件头（package、imports、protobuf wire 辅助函数）
// 以及本文件内声明的全部枚举。
// 枚举只取当前 proto 文件声明的（含嵌套在 message 里的），
// 引用其他文件的枚举时不会重复声明，而是带包前缀直接引用（见 typeForField）。
func GenerateRedisCodeHeadWithEnums(file *protogen.File) ([]byte, error) {
	enums := collectFileEnums(file)

	needProto, needMath, needStrconv, needSort := scanImports(file)
	imports := []string{
		"fmt",
		"github.com/gomodule/redigo/redis",
	}
	if needStrconv {
		imports = append(imports, "strconv")
	}
	if needMath {
		imports = append(imports, "math")
	}
	if needSort {
		imports = append(imports, "sort")
	}
	sort.Strings(imports)

	info := MessageInfo{
		PackageName: string(file.GoPackageName),
		Imports:     imports,
	}

	tmplHead, err := template.New("redis_code_head").Parse(codeTemplateHead)
	if err != nil {
		return nil, err
	}
	var bufHead bytes.Buffer
	if err := tmplHead.Execute(&bufHead, info); err != nil {
		return nil, err
	}

	tmplEnums, err := template.New("redis_enum_consts").Parse(codeTemplateEnums)
	if err != nil {
		return nil, err
	}
	var bufEnums bytes.Buffer
	if err := tmplEnums.Execute(&bufEnums, struct {
		Enums []EnumInfo
	}{
		Enums: enums,
	}); err != nil {
		return nil, err
	}

	parts := [][]byte{bufHead.Bytes(), bufEnums.Bytes()}
	if needProto {
		tmplHelpers, err := template.New("redis_proto_helpers").Parse(codeTemplateProtoHelpers)
		if err != nil {
			return nil, err
		}
		var bufHelpers bytes.Buffer
		if err := tmplHelpers.Execute(&bufHelpers, nil); err != nil {
			return nil, err
		}
		parts = append(parts, bufHelpers.Bytes())
	}

	return bytes.Join(parts, []byte("\n")), nil
}

// CollectMessages 返回文件中所有需要生成代码的 message（顶层 + 嵌套，按声明顺序），
// 跳过 map 的合成 entry message。
func CollectMessages(file *protogen.File) []*protogen.Message {
	var messages []*protogen.Message
	var walk func(m *protogen.Message)
	walk = func(m *protogen.Message) {
		if m.Desc.IsMapEntry() {
			return
		}
		messages = append(messages, m)
		for _, nested := range m.Messages {
			walk(nested)
		}
	}
	for _, m := range file.Messages {
		walk(m)
	}
	return messages
}

// fieldType 描述一个字段的存储类型信息
type fieldType struct {
	goType     string // 结构体字段的 Go 类型
	kind       FieldKind
	keyType    string // map 键类型
	elemType   string // 集合元素类型
	elemIsMsg  bool   // 元素为 message，单元素 protobuf wire format 序列化
	elemIsEnum bool   // 元素为枚举
	wholeMsg   bool   // plain 字段整块 protobuf wire format 序列化
	isEnum     bool   // plain 字段为枚举
}

// fieldTypeFor 计算字段的存储类型信息。
// message/enum 类型一律经 protogen 解析：同包直接用名字，
// 跨包自动带包前缀并登记 import（由 protogen 在生成文件时统一插入）。
// 集合字段（map/repeated）按元素级存储设计：元素类型单独解析，
// 标量/枚举/bytes 元素直接存储，message 元素单元素 protobuf 序列化。
func fieldTypeFor(gen *protogen.Plugin, g *protogen.GeneratedFile, f *protogen.Field) fieldType {
	if f.Desc.Cardinality() == protoreflect.Repeated {
		if f.Desc.IsMap() {
			keyType := mapKeyType(f.Desc.MapKey().Kind())
			elemType, elemIsMsg, elemIsEnum := elemTypeFor(gen, g, f.Desc.MapValue())
			return fieldType{
				goType:     fmt.Sprintf("map[%s]%s", keyType, elemType),
				kind:       FieldMap,
				keyType:    keyType,
				elemType:   elemType,
				elemIsMsg:  elemIsMsg,
				elemIsEnum: elemIsEnum,
			}
		}
		elemType, elemIsMsg, elemIsEnum := elemTypeFor(gen, g, f.Desc)
		return fieldType{
			goType:     "[]" + elemType,
			kind:       FieldSlice,
			elemType:   elemType,
			elemIsMsg:  elemIsMsg,
			elemIsEnum: elemIsEnum,
		}
	}

	switch f.Desc.Kind() {
	case protoreflect.MessageKind:
		return fieldType{
			goType:   g.QualifiedGoIdent(f.Message.GoIdent),
			kind:     FieldPlain,
			wholeMsg: true,
		}
	case protoreflect.EnumKind:
		return fieldType{
			goType: g.QualifiedGoIdent(f.Enum.GoIdent),
			kind:   FieldPlain,
			isEnum: true,
		}
	case protoreflect.BytesKind:
		return fieldType{goType: "[]byte", kind: FieldPlain}
	default:
		return fieldType{goType: scalarGoType(f.Desc.Kind()), kind: FieldPlain}
	}
}

// elemTypeFor 解析集合元素类型：标量/枚举/bytes 直接存储，message 单元素 protobuf 序列化。
func elemTypeFor(gen *protogen.Plugin, g *protogen.GeneratedFile, desc protoreflect.FieldDescriptor) (elemType string, isMsg, isEnum bool) {
	switch desc.Kind() {
	case protoreflect.MessageKind:
		return g.QualifiedGoIdent(goIdentOf(gen, desc.Message())), true, false
	case protoreflect.EnumKind:
		return g.QualifiedGoIdent(goIdentOf(gen, desc.Enum())), false, true
	case protoreflect.BytesKind:
		return "[]byte", false, false
	default:
		return scalarGoType(desc.Kind()), false, false
	}
}

// goIdentOf 按 protogen 的命名规则（newGoIdent）为描述符计算 Go 标识符，
// 用于 protogen 未直接暴露的 map 值类型。
func goIdentOf(gen *protogen.Plugin, desc protoreflect.Descriptor) protogen.GoIdent {
	file := gen.FilesByPath[string(desc.ParentFile().Path())]
	if file == nil {
		// 正常不会发生：能被字段引用的类型必然在本请求的文件集合中。
		return protogen.GoIdent{GoName: goCamelCase(string(desc.Name()))}
	}
	name := strings.TrimPrefix(string(desc.FullName()), string(file.Desc.Package())+".")
	return protogen.GoIdent{
		GoName:       goCamelCase(name),
		GoImportPath: file.GoImportPath,
	}
}

func scalarGoType(k protoreflect.Kind) string {
	switch k {
	case protoreflect.Uint32Kind:
		return "uint32"
	case protoreflect.Uint64Kind:
		return "uint64"
	case protoreflect.Int32Kind:
		return "int32"
	case protoreflect.Int64Kind:
		return "int64"
	case protoreflect.FloatKind:
		return "float32"
	case protoreflect.DoubleKind:
		return "float64"
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BoolKind:
		return "bool"
	default:
		return k.String()
	}
}

func mapKeyType(k protoreflect.Kind) string {
	switch k {
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.Int32Kind:
		return "int32"
	case protoreflect.Int64Kind:
		return "int64"
	case protoreflect.Uint32Kind:
		return "uint32"
	case protoreflect.Uint64Kind:
		return "uint64"
	default:
		// proto 规定 map 键只能是整型、bool、string，走到这里说明描述符异常
		return goCamelCase(k.String())
	}
}

// scanImports 扫描文件中全部字段（含嵌套 message，跳过 map entry），
// 判断生成代码需要哪些 stdlib import。
// needProto：存在 message 类型字段（plain / map 值 / repeated 元素），
// 需要生成 protobuf wire 辅助函数与每个 message 的 Marshal/Unmarshal 方法；
// needMath：存在 float 字段且文件需要生成 marshaler（编码需要 math.Float32bits/Float64bits）；
// strconv：集合下标、非 string map 键、数值/枚举直存字段的解析；sort：repeated 整体读回按下标排序。
func scanImports(file *protogen.File) (needProto, needMath, needStrconv, needSort bool) {
	walkMessages(file.Messages, func(m *protogen.Message) {
		if m.Desc.IsMapEntry() {
			return
		}
		for _, field := range m.Fields {
			if field.Desc.Cardinality() == protoreflect.Repeated {
				if field.Desc.IsMap() {
					needProto = needProto || field.Desc.MapValue().Kind() == protoreflect.MessageKind
					needMath = needMath || field.Desc.MapValue().Kind() == protoreflect.FloatKind ||
						field.Desc.MapValue().Kind() == protoreflect.DoubleKind
					needStrconv = needStrconv || field.Desc.MapKey().Kind() != protoreflect.StringKind
				} else {
					needProto = needProto || field.Desc.Kind() == protoreflect.MessageKind
					needMath = needMath || field.Desc.Kind() == protoreflect.FloatKind ||
						field.Desc.Kind() == protoreflect.DoubleKind
					needStrconv = true
					needSort = true
				}
				continue
			}
			switch field.Desc.Kind() {
			case protoreflect.MessageKind:
				needProto = true
			case protoreflect.Uint32Kind, protoreflect.Uint64Kind, protoreflect.Int32Kind, protoreflect.Int64Kind,
				protoreflect.FloatKind, protoreflect.DoubleKind, protoreflect.EnumKind:
				needStrconv = true
				needMath = needMath || field.Desc.Kind() == protoreflect.FloatKind ||
					field.Desc.Kind() == protoreflect.DoubleKind
			}
		}
	})
	// 没有 message 字段就不会生成 marshaler，math 只随 marshaler 一起引入
	return needProto, needProto && needMath, needStrconv, needSort
}

// collectFileEnums 收集本文件声明的全部枚举（含嵌套在 message 里的），按名字排序。
func collectFileEnums(file *protogen.File) []EnumInfo {
	var enums []EnumInfo
	add := func(e *protogen.Enum) {
		values := make([]EnumValueInfo, 0, len(e.Values))
		for _, v := range e.Values {
			values = append(values, EnumValueInfo{
				Name:  string(v.GoIdent.GoName),
				Value: int32(v.Desc.Number()),
			})
		}
		enums = append(enums, EnumInfo{Name: string(e.GoIdent.GoName), Values: values})
	}
	for _, e := range file.Enums {
		add(e)
	}
	walkMessages(file.Messages, func(m *protogen.Message) {
		for _, e := range m.Enums {
			add(e)
		}
	})
	sort.Slice(enums, func(i, j int) bool {
		return enums[i].Name < enums[j].Name
	})
	return enums
}

func walkMessages(messages []*protogen.Message, fn func(*protogen.Message)) {
	for _, m := range messages {
		fn(m)
		walkMessages(m.Messages, fn)
	}
}

// goCamelCase 与 protoc-gen-go 使用的 strs.GoCamelCase 行为一致：
// 把 proto 标识符转换为 Go 导出标识符（如 "user_id" -> "UserId"、"ExtraMsg.Inner" -> "ExtraMsg_Inner"）。
func goCamelCase(s string) string {
	// 不变式：如果下一个字母是小写，它必须被转为大写。
	// 即逐"词"处理，词以下划线或大写字母分隔，数字也单独成词。
	var b []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '.' && i+1 < len(s) && isASCIILower(s[i+1]):
			// 跳过 ".lowercase" 中的点
		case c == '.':
			b = append(b, '_') // 点转换为下划线
		case c == '_' && (i == 0 || s[i-1] == '.'):
			// 开头或点后的下划线换成 X，保证以大写字母开头
			b = append(b, 'X')
		case c == '_' && i+1 < len(s) && isASCIILower(s[i+1]):
			// 跳过 "_lowercase" 中的下划线
		case isASCIIDigit(c):
			b = append(b, c)
		default:
			// 词首字母大写，词内小写保持原样
			if isASCIILower(c) {
				c -= 'a' - 'A'
			}
			b = append(b, c)
			for ; i+1 < len(s) && isASCIILower(s[i+1]); i++ {
				b = append(b, s[i+1])
			}
		}
	}
	return string(b)
}

func isASCIILower(c byte) bool {
	return 'a' <= c && c <= 'z'
}

func isASCIIUpper(c byte) bool {
	return 'A' <= c && c <= 'Z'
}

func isASCIIDigit(c byte) bool {
	return '0' <= c && c <= '9'
}
