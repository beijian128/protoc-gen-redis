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

// GenerateRedisCode 为一个 message 生成 Redis 存取代码。
func GenerateRedisCode(gen *protogen.Plugin, file *protogen.File, msg *protogen.Message, g *protogen.GeneratedFile, keyFormat string) ([]byte, error) {
	fields := make([]FieldInfo, 0, len(msg.Fields))
	for _, field := range msg.Fields {
		goType, isGob, isEnum := typeForField(gen, g, field)
		fields = append(fields, FieldInfo{
			Name:     field.GoName,
			ProtoTag: int(field.Desc.Number()),
			GoType:   goType,
			IsGob:    isGob,
			IsEnum:   isEnum,
		})
	}

	info := MessageInfo{
		PackageName: string(file.GoPackageName),
		MessageName: string(msg.GoIdent.GoName),
		Fields:      fields,
		KeyFormat:   keyFormat,
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

// GenerateRedisCodeHeadWithEnums 生成文件头（package、imports）以及本文件内声明的全部枚举。
// 枚举只取当前 proto 文件声明的（含嵌套在 message 里的），
// 引用其他文件的枚举时不会重复声明，而是带包前缀直接引用（见 typeForField）。
func GenerateRedisCodeHeadWithEnums(file *protogen.File) ([]byte, error) {
	enums := collectFileEnums(file)

	needGob, needStrconv := scanImports(file)
	imports := []string{
		"fmt",
		"github.com/gomodule/redigo/redis",
	}
	if needStrconv {
		imports = append(imports, "strconv")
	}
	if needGob {
		imports = append(imports, "bytes", "encoding/gob")
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

	return bytes.Join([][]byte{
		bufHead.Bytes(),
		bufEnums.Bytes(),
	}, []byte("\n")), nil
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

// typeForField 计算字段在生成代码中的 Go 类型。
// message/enum 类型一律经 protogen 解析：同包直接用名字，
// 跨包自动带包前缀并登记 import（由 protogen 在生成文件时统一插入）。
func typeForField(gen *protogen.Plugin, g *protogen.GeneratedFile, f *protogen.Field) (goType string, isGob, isEnum bool) {
	if f.Desc.Cardinality() == protoreflect.Repeated {
		if f.Desc.IsMap() {
			// map 整体用 gob 序列化
			keyType := mapKeyType(f.Desc.MapKey().Kind())
			valueType := typeForDescriptor(gen, g, f.Desc.MapValue())
			return fmt.Sprintf("map[%s]%s", keyType, valueType), true, false
		}
		switch f.Desc.Kind() {
		case protoreflect.MessageKind:
			return "[]" + g.QualifiedGoIdent(f.Message.GoIdent), true, false
		case protoreflect.EnumKind:
			return "[]" + g.QualifiedGoIdent(f.Enum.GoIdent), true, false
		case protoreflect.BytesKind:
			return "[][]byte", true, false
		default:
			return "[]" + scalarGoType(f.Desc.Kind()), true, false
		}
	}

	switch f.Desc.Kind() {
	case protoreflect.MessageKind:
		return g.QualifiedGoIdent(f.Message.GoIdent), true, false
	case protoreflect.EnumKind:
		return g.QualifiedGoIdent(f.Enum.GoIdent), false, true
	case protoreflect.BytesKind:
		return "[]byte", false, false
	default:
		return scalarGoType(f.Desc.Kind()), false, false
	}
}

// typeForDescriptor 解析 map 值字段的 Go 类型。
// protogen 的 Field 不暴露 map 键值类型，这里直接按描述符解析。
func typeForDescriptor(gen *protogen.Plugin, g *protogen.GeneratedFile, desc protoreflect.FieldDescriptor) string {
	switch desc.Kind() {
	case protoreflect.MessageKind:
		return g.QualifiedGoIdent(goIdentOf(gen, desc.Message()))
	case protoreflect.EnumKind:
		return g.QualifiedGoIdent(goIdentOf(gen, desc.Enum()))
	case protoreflect.BytesKind:
		return "[]byte"
	default:
		return scalarGoType(desc.Kind())
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
// 判断生成代码是否需要 strconv / gob + bytes 这两个 import。
func scanImports(file *protogen.File) (needGob, needStrconv bool) {
	walkMessages(file.Messages, func(m *protogen.Message) {
		if m.Desc.IsMapEntry() {
			return
		}
		for _, field := range m.Fields {
			if field.Desc.Cardinality() == protoreflect.Repeated {
				needGob = true
				continue
			}
			switch field.Desc.Kind() {
			case protoreflect.MessageKind:
				needGob = true
			case protoreflect.Uint32Kind, protoreflect.Uint64Kind, protoreflect.Int32Kind, protoreflect.Int64Kind,
				protoreflect.FloatKind, protoreflect.DoubleKind, protoreflect.EnumKind:
				needStrconv = true
			}
		}
	})
	return needGob, needStrconv
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
