package generator

import (
	"bytes"
	"fmt"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
	"sort"
	"strings"
	"text/template"
)

func GenerateRedisCode(gen *protogen.Plugin, file *protogen.File, msg *protogen.Message) ([]byte, error) {
	var fields []FieldInfo

	for _, field := range msg.Fields {
		var goType string
		var isGob bool
		if field.Desc.Cardinality() == protoreflect.Repeated {
			if field.Desc.IsMap() {
				keyKind := field.Desc.MapKey().Kind()
				valueKind := field.Desc.MapValue().Kind()

				keyType := mapKeyType(keyKind)
				valueType := mapValueType(valueKind)

				goType = fmt.Sprintf("map[%s]%s", keyType, valueType)
				isGob = true
			} else {
				goType = fmt.Sprintf("[]%s", field.Desc.Kind())
				isGob = true
			}

		} else {
			switch field.Desc.Kind() {
			// ✅ 基础类型：直接存取，不使用 gob
			case protoreflect.Uint32Kind:
				goType = "uint32"
			case protoreflect.Uint64Kind:
				goType = "uint64"
			case protoreflect.Int32Kind:
				goType = "int32"
			case protoreflect.Int64Kind:
				goType = "int64"
			case protoreflect.FloatKind:
				goType = "float32"
			case protoreflect.DoubleKind:
				goType = "float64"
			case protoreflect.StringKind:
				goType = "string"
			case protoreflect.BoolKind:
				goType = "bool"

			// ✅ Enum：底层是 int32，你要求直接存为 int32，不序列化
			case protoreflect.EnumKind:
				goType = "int32"

			// ✅ Bytes：你要求不序列化，直接存 []byte
			case protoreflect.BytesKind:
				goType = "[]byte"

			case protoreflect.MessageKind:
				goType = string(file.Desc.Name())
				goType = strings.ToUpper(goType[:1]) + goType[1:]
				isGob = true
			default:

			}
		}

		name := string(field.Desc.Name())
		name = strings.ToUpper(name[:1]) + name[1:]
		fields = append(fields, FieldInfo{
			Name:     name,
			ProtoTag: int(field.Desc.Number()),
			GoType:   goType,
			IsGob:    isGob,
		})
	}

	info := MessageInfo{
		PackageName: string(file.GoPackageName),
		MessageName: string(msg.Desc.Name()),
		Fields:      fields,
	}

	tmpl, err := template.New("redis_code").Funcs(template.FuncMap{
		"in": in,
	}).Parse(codeTemplate)

	if err != nil {
		return nil, fmt.Errorf("解析模板失败: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, info); err != nil {
		return nil, fmt.Errorf("渲染模板失败: %v", err)
	}

	return buf.Bytes(), nil
}

func GenerateRedisCodeHead(gen *protogen.Plugin, file *protogen.File) ([]byte, error) {

	info := MessageInfo{
		PackageName: string(file.GoPackageName),
		Imports: []string{
			//"bytes",
			//"encoding/gob",
			"fmt",
			//"strconv",
			"github.com/gomodule/redigo/redis",
		},
	}
	var needGob, needStrconv bool
	for _, f := range gen.Files {
		for _, msg := range f.Messages {
			for _, field := range msg.Fields {
				if field.Desc.Cardinality() == protoreflect.Repeated {
					needGob = true

				} else {
					switch field.Desc.Kind() {
					// ✅ 基础类型：直接存取，不使用 gob
					case protoreflect.Uint32Kind, protoreflect.Uint64Kind, protoreflect.Int32Kind, protoreflect.Int64Kind, protoreflect.FloatKind, protoreflect.DoubleKind:
						needStrconv = true
					case protoreflect.BoolKind:
						//needStrconv = true

					case protoreflect.EnumKind:
						needStrconv = true

					case protoreflect.BytesKind:

					case protoreflect.MessageKind:
						needGob = true
					default:

					}
				}

			}

		}
	}

	if needStrconv {
		info.Imports = append(info.Imports, "strconv")
	}
	if needGob {
		info.Imports = append(info.Imports, "encoding/gob")
		info.Imports = append(info.Imports, "bytes")
	}
	sort.Slice(info.Imports, func(i, j int) bool {
		return info.Imports[i] < info.Imports[j]
	})
	tmpl, err := template.New("redis_code_head").Parse(codeTemplateHead)
	if err != nil {
		return nil, fmt.Errorf("解析模板失败: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, info); err != nil {
		return nil, fmt.Errorf("渲染模板失败: %v", err)
	}

	return buf.Bytes(), nil
}

func mapKeyType(k protoreflect.Kind) string {
	switch k {
	case protoreflect.StringKind:
		return "string"
	case protoreflect.Int32Kind:
		return "int32"
	case protoreflect.Int64Kind:
		return "int64"
	case protoreflect.Uint32Kind:
		return "uint32"
	case protoreflect.Uint64Kind:
		return "uint64"
	default:
		return k.String()
	}
}

func mapValueType(v protoreflect.Kind) string {
	switch v {
	case protoreflect.StringKind:
		return "string"
	case protoreflect.Int32Kind:
		return "int32"
	case protoreflect.Int64Kind:
		return "int64"
	case protoreflect.Uint32Kind:
		return "uint32"
	case protoreflect.Uint64Kind:
		return "uint64"
	case protoreflect.FloatKind:
		return "float32"
	case protoreflect.DoubleKind:
		return "float64"
	case protoreflect.BoolKind:
		return "bool"
	default:
		return v.String()
	}
}

func in(a int, list []int) bool {
	for _, b := range list {
		if a == b {
			return true
		}
	}
	return false
}
