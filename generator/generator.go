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
				if valueKind == protoreflect.MessageKind {
					goType = fmt.Sprintf("map[%s]%s", keyType, field.Desc.MapValue().Message().Name())
				} else {
					goType = fmt.Sprintf("map[%s]%s", keyType, valueType)
				}
				isGob = true
			} else {
				if field.Desc.Kind() == protoreflect.MessageKind {
					goType = fmt.Sprintf("[]%s", field.Desc.Message().Name())
				} else {
					goType = fmt.Sprintf("[]%s", field.Desc.Kind())
				}
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

			case protoreflect.EnumKind:
				// 获取枚举类型名称（如 "Gender"）
				enumTypeName := string(field.Desc.Enum().Name())
				goType = enumTypeName // ✅ 使用枚举类型名，而不是 int32
				// 注意：此时该字段的类型是 "Gender"，是一个自定义类型（由 protoc-gen-go 生成）
				// 如果你希望生成的代码直接使用该类型，那么它应该是已知的
				isGob = false
			// ✅ Bytes：你要求不序列化，直接存 []byte
			case protoreflect.BytesKind:
				goType = "[]byte"

			case protoreflect.MessageKind:
				messageType := field.Desc.Message()
				goType = string(messageType.Name()) // 获取实际的 message 名称，如 "User"
				//goType = string(file.Desc.Name())
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
	enums := collectEnums(gen)

	for _, e := range enums {
		info.Enums = append(info.Enums, e.Name)
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
func GenerateRedisCodeHeadWithEnums(gen *protogen.Plugin) ([]byte, error) {
	// 1. 收集所有枚举
	enums := collectEnums(gen)

	// 2. 收集 imports（原有逻辑）
	info := MessageInfo{
		PackageName: string(gen.Files[0].GoPackageName), // 简化：取第一个文件的包名，或者你可以合并所有
		Imports: []string{
			"fmt",
			"github.com/gomodule/redigo/redis",
		},
	}

	for _, e := range enums {
		info.Enums = append(info.Enums, e.Name)
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
	sort.Strings(info.Imports)

	// 3. 渲染 Import 部分
	tmplHead, err := template.New("redis_code_head").Parse(codeTemplateHead)
	if err != nil {
		return nil, err
	}
	var bufHead bytes.Buffer
	if err := tmplHead.Execute(&bufHead, info); err != nil {
		return nil, err
	}

	// 4. 渲染 Enum 常量部分
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

	// 5. 合并：Import + Enum常量
	return bytes.Join([][]byte{
		bufHead.Bytes(),
		bufEnums.Bytes(),
	}, []byte("\n")), nil
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
	funcMap := template.FuncMap{
		"in": in,
	}
	tmpl, err := template.New("redis_code_head").Funcs(funcMap).Parse(codeTemplateHead)
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

func in(a string, list []string) bool {
	for _, b := range list {
		if a == b {
			return true
		}
	}
	return false
}

func collectEnums(gen *protogen.Plugin) []EnumInfo {
	var enums []EnumInfo

	for _, f := range gen.Files {
		for _, enum := range f.Enums {
			var values []EnumValueInfo
			enumName := string(enum.Desc.Name())
			for _, value := range enum.Values {
				values = append(values, EnumValueInfo{
					EnumName: enumName,
					Name:     string(value.Desc.Name()),
					Value:    int32(value.Desc.Number()),
				})
			}

			enums = append(enums, EnumInfo{
				Name:   enumName,
				Values: values,
			})
		}
	}

	// 可选：按枚举名字排序，让输出更整齐
	sort.Slice(enums, func(i, j int) bool {
		return enums[i].Name < enums[j].Name
	})

	return enums
}
