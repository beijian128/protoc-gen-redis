package main

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/beijian128/protoc-gen-redis/generator"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// ---------- 描述符构造 ----------

func field(name string, number int32, typ descriptorpb.FieldDescriptorProto_Type, label descriptorpb.FieldDescriptorProto_Label, typeName string) *descriptorpb.FieldDescriptorProto {
	f := &descriptorpb.FieldDescriptorProto{
		Name:   proto.String(name),
		Number: proto.Int32(number),
		Type:   typ.Enum(),
		Label:  label.Enum(),
	}
	if typeName != "" {
		f.TypeName = proto.String(typeName)
	}
	return f
}

// userFileDescriptor 与 proto/user.proto 一一对应。
func userFileDescriptor() *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("proto/user.proto"),
		Package: proto.String("user"),
		Syntax:  proto.String("proto3"),
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String("github.com/beijian128/protoc-gen-redis/cmddb"),
		},
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{
				Name: proto.String("Gender"),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: proto.String("GENDER_UNKNOWN"), Number: proto.Int32(0)},
					{Name: proto.String("GENDER_MALE"), Number: proto.Int32(1)},
					{Name: proto.String("GENDER_FEMALE"), Number: proto.Int32(2)},
				},
			},
			{
				Name: proto.String("LoginSource"),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: proto.String("SOURCE_UNKNOWN"), Number: proto.Int32(0)},
					{Name: proto.String("SOURCE_APP"), Number: proto.Int32(1)},
					{Name: proto.String("SOURCE_H5"), Number: proto.Int32(2)},
					{Name: proto.String("SOURCE_MINI_PROGRAM"), Number: proto.Int32(3)},
				},
			},
		},
		MessageType: []*descriptorpb.DescriptorProto{
			userBaseInfoDescriptor(),
			weaponDescriptor(),
		},
	}
}

func userBaseInfoDescriptor() *descriptorpb.DescriptorProto {
	return &descriptorpb.DescriptorProto{
		Name: proto.String("UserBaseInfo"),
		Field: []*descriptorpb.FieldDescriptorProto{
			field("user_id", 1, descriptorpb.FieldDescriptorProto_TYPE_INT32, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("username", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("avatar_url", 3, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("gender", 4, descriptorpb.FieldDescriptorProto_TYPE_ENUM, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".user.Gender"),
			field("level", 5, descriptorpb.FieldDescriptorProto_TYPE_INT32, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("exp", 6, descriptorpb.FieldDescriptorProto_TYPE_INT64, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("balance", 7, descriptorpb.FieldDescriptorProto_TYPE_FLOAT, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("friends", 8, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ""),
			field("settings", 9, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ".user.UserBaseInfo.SettingsEntry"),
			field("login_source", 10, descriptorpb.FieldDescriptorProto_TYPE_ENUM, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".user.LoginSource"),
			field("listint32", 11, descriptorpb.FieldDescriptorProto_TYPE_INT32, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ""),
			field("weapons", 12, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ".user.Weapon"),
			field("weapon", 13, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".user.Weapon"),
			field("weaponMap", 14, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ".user.UserBaseInfo.WeaponMapEntry"),
			field("coin", 15, descriptorpb.FieldDescriptorProto_TYPE_UINT32, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("gem", 16, descriptorpb.FieldDescriptorProto_TYPE_UINT64, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("vip", 17, descriptorpb.FieldDescriptorProto_TYPE_BOOL, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("score", 18, descriptorpb.FieldDescriptorProto_TYPE_DOUBLE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("token", 19, descriptorpb.FieldDescriptorProto_TYPE_BYTES, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("profile", 20, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".user.UserBaseInfo.Profile"),
			field("vip_level", 21, descriptorpb.FieldDescriptorProto_TYPE_ENUM, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".user.UserBaseInfo.VipLevel"),
		},
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{
				Name: proto.String("VipLevel"),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: proto.String("VIP_NONE"), Number: proto.Int32(0)},
					{Name: proto.String("VIP_1"), Number: proto.Int32(1)},
					{Name: proto.String("VIP_2"), Number: proto.Int32(2)},
				},
			},
		},
		NestedType: []*descriptorpb.DescriptorProto{
			{
				Name:    proto.String("SettingsEntry"),
				Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
				Field: []*descriptorpb.FieldDescriptorProto{
					field("key", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
					field("value", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
				},
			},
			{
				Name:    proto.String("WeaponMapEntry"),
				Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
				Field: []*descriptorpb.FieldDescriptorProto{
					field("key", 1, descriptorpb.FieldDescriptorProto_TYPE_INT32, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
					field("value", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".user.Weapon"),
				},
			},
			{
				Name: proto.String("Profile"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("nickname", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
					field("age", 2, descriptorpb.FieldDescriptorProto_TYPE_INT32, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
				},
			},
		},
	}
}

func weaponDescriptor() *descriptorpb.DescriptorProto {
	return &descriptorpb.DescriptorProto{
		Name: proto.String("Weapon"),
		Field: []*descriptorpb.FieldDescriptorProto{
			field("name", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("damage", 2, descriptorpb.FieldDescriptorProto_TYPE_INT32, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("element", 3, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
		},
	}
}

// extraFileDescriptor 覆盖此前类型映射的漏洞场景：
// 跨包引用、嵌套 message、嵌套枚举、repeated 枚举/bytes、map 值为枚举。
func extraFileDescriptor() *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("extra.proto"),
		Package: proto.String("extra"),
		Syntax:  proto.String("proto3"),
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String("github.com/beijian128/protoc-gen-redis/extra"),
		},
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{
				Name: proto.String("ExtraKind"),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: proto.String("KIND_UNKNOWN"), Number: proto.Int32(0)},
					{Name: proto.String("KIND_A"), Number: proto.Int32(1)},
				},
			},
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("ExtraMsg"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("kind", 1, descriptorpb.FieldDescriptorProto_TYPE_ENUM, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".extra.ExtraKind"),
					field("kinds", 2, descriptorpb.FieldDescriptorProto_TYPE_ENUM, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ".extra.ExtraKind"),
					field("blobs", 3, descriptorpb.FieldDescriptorProto_TYPE_BYTES, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ""),
					field("kind_map", 4, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ".extra.ExtraMsg.KindMapEntry"),
					field("inner", 5, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".extra.ExtraMsg.Inner"),
					field("inners", 6, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ".extra.ExtraMsg.Inner"),
					field("state", 7, descriptorpb.FieldDescriptorProto_TYPE_ENUM, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".extra.ExtraMsg.State"),
				},
				EnumType: []*descriptorpb.EnumDescriptorProto{
					{
						Name: proto.String("State"),
						Value: []*descriptorpb.EnumValueDescriptorProto{
							{Name: proto.String("STATE_NONE"), Number: proto.Int32(0)},
						},
					},
				},
				NestedType: []*descriptorpb.DescriptorProto{
					{
						Name:    proto.String("KindMapEntry"),
						Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
						Field: []*descriptorpb.FieldDescriptorProto{
							field("key", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
							field("value", 2, descriptorpb.FieldDescriptorProto_TYPE_ENUM, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".extra.ExtraKind"),
						},
					},
					{
						Name: proto.String("Inner"),
						Field: []*descriptorpb.FieldDescriptorProto{
							field("x", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
						},
					},
				},
			},
		},
	}
}

// userFileWithExtraRef 在 user.proto 上追加一个引用 extra 包 message 的字段。
func userFileWithExtraRef() *descriptorpb.FileDescriptorProto {
	f := proto.Clone(userFileDescriptor()).(*descriptorpb.FileDescriptorProto)
	f.Dependency = []string{"extra.proto"}
	f.MessageType[0].Field = append(f.MessageType[0].Field,
		field("extra_ref", 22, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".extra.ExtraMsg"))
	return f
}

// ---------- 测试辅助 ----------

func runPlugin(t *testing.T, files []*descriptorpb.FileDescriptorProto, parameter, keyFormat string) *pluginpb.CodeGeneratorResponse {
	t.Helper()
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.GetName())
	}
	req := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: names,
		Parameter:      proto.String(parameter),
		ProtoFile:      files,
	}
	gen, err := protogen.Options{}.New(req)
	if err != nil {
		t.Fatalf("protogen.New: %v", err)
	}
	if err := run(gen, keyFormat); err != nil {
		t.Fatalf("run: %v", err)
	}
	resp := gen.Response()
	if resp.GetError() != "" {
		t.Fatalf("plugin error: %s", resp.GetError())
	}
	return resp
}

func fileByName(t *testing.T, resp *pluginpb.CodeGeneratorResponse, name string) string {
	t.Helper()
	for _, f := range resp.GetFile() {
		if f.GetName() == name {
			return f.GetContent()
		}
	}
	t.Fatalf("响应中没有文件 %q，实际为 %v", name, resp.GetFile())
	return ""
}

func assertParseable(t *testing.T, name, content string) {
	t.Helper()
	if _, err := parser.ParseFile(token.NewFileSet(), name, content, parser.AllErrors); err != nil {
		t.Fatalf("%s 不是合法的 Go 源码: %v\n%s", name, err, content)
	}
}

// containsCode 判断生成内容中是否包含指定片段，忽略 gofmt 对齐产生的空白差异。
func containsCode(content, want string) bool {
	return strings.Contains(strings.Join(strings.Fields(content), " "), want)
}

// ---------- 测试用例 ----------

// TestGenerateUserProtoGolden 用与 proto/user.proto 等价的描述符生成代码，
// 与仓库里提交的 generated/user.redis.go 对比（可用 UPDATE_GOLDEN=1 刷新）。
func TestGenerateUserProtoGolden(t *testing.T) {
	resp := runPlugin(t, []*descriptorpb.FileDescriptorProto{userFileDescriptor()}, "", generator.DefaultKeyFormat)
	if len(resp.GetFile()) != 1 {
		t.Fatalf("生成了 %d 个文件，期望 1 个", len(resp.GetFile()))
	}
	f := resp.GetFile()[0]
	if f.GetName() != "user.redis.go" {
		t.Errorf("默认模式下文件名为 %q，期望 user.redis.go", f.GetName())
	}
	content := f.GetContent()
	assertParseable(t, f.GetName(), content)

	// 抽查命名与关键逻辑
	for _, want := range []string{
		"type Gender int32",
		"Gender_GENDER_MALE Gender",
		"FieldUserBaseInfo_UserId FieldUserBaseInfo = 1", // user_id -> UserId
		"UserId int32",
		"AvatarUrl string",
		"LoginSource LoginSource",
		"Settings map[string]string",
		"Weapons []Weapon",
		"WeaponMap map[int32]Weapon",
		"Coin uint32",
		"Gem uint64",
		"Vip bool",
		"Score float64",
		"Token []byte",
		"Profile UserBaseInfo_Profile",
		"VipLevel UserBaseInfo_VipLevel",
		"UserBaseInfo_VIP_1 UserBaseInfo_VipLevel", // 嵌套枚举常量（message 名前缀）
		"type UserBaseInfo_Profile struct",         // 嵌套 message 生成代码
		"Nickname string",
		"FieldUserBaseInfo_Profile FieldUserBaseInfo = 20", // 父字段常量名保持不变
		"type FieldUserBaseInfo_ProfileX uint32",           // 冲突的嵌套类型名加 X 消歧
		"FieldUserBaseInfo_ProfileX_Nickname FieldUserBaseInfo_ProfileX = 1",
		// 集合字段元素级方法（方案 A）
		"func (p *UserBaseInfo) SetSettings(conn redis.Conn, REDBKey uint32, ida, idb uint64, k string, v string) error",
		"func (p *UserBaseInfo) GetSettings(conn redis.Conn, REDBKey uint32, ida, idb uint64, k string) (string, bool, error)",
		"func (p *UserBaseInfo) DelSettings(conn redis.Conn, REDBKey uint32, ida, idb uint64, k string) (bool, error)",
		"func (p *UserBaseInfo) GetSettingsAll(conn redis.Conn, REDBKey uint32, ida, idb uint64) error",
		"func (p *UserBaseInfo) AppendFriends(conn redis.Conn, REDBKey uint32, ida, idb uint64, v string) (int, error)",
		"func (p *UserBaseInfo) SetWeaponMap(conn redis.Conn, REDBKey uint32, ida, idb uint64, k int32, v Weapon) error",
		`"EVAL", script, 1, key, 8`,
		`fmt.Sprintf("REDB#%d:%d:%d", REDBKey, ida, idb)`,
	} {
		if !containsCode(content, want) {
			t.Errorf("生成内容缺少 %q", want)
		}
	}

	const goldenPath = "generated/user.redis.go"
	got := []byte(content)
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("已刷新 %s", goldenPath)
		return
	}
	if !bytes.Equal(got, want) {
		t.Errorf("生成结果与 %s 不一致（用 UPDATE_GOLDEN=1 刷新）", goldenPath)
	}
}

// TestNestedCrossPackageAndTypeMapping 覆盖：
// 嵌套 message/枚举生成代码、跨包引用自动加包前缀并登记 import、
// repeated 枚举/bytes、map 值为枚举等此前会生成非法代码的场景。
func TestNestedCrossPackageAndTypeMapping(t *testing.T) {
	// 依赖文件必须先于引用它的文件（拓扑序），与 protoc 的请求一致
	files := []*descriptorpb.FileDescriptorProto{extraFileDescriptor(), userFileWithExtraRef()}
	resp := runPlugin(t, files, "", generator.DefaultKeyFormat)

	user := fileByName(t, resp, "user.redis.go")
	extra := fileByName(t, resp, "extra.redis.go")
	assertParseable(t, "user.redis.go", user)
	assertParseable(t, "extra.redis.go", extra)

	// 跨包引用：类型带包前缀，且只 import 不重复声明枚举
	for _, want := range []string{
		"ExtraRef extra.ExtraMsg",
		"extra.ExtraMsg",
		`"github.com/beijian128/protoc-gen-redis/extra"`,
		"type Gender int32", // 本文件的枚举正常声明
	} {
		if !containsCode(user, want) {
			t.Errorf("user.redis.go 缺少 %q", want)
		}
	}
	if containsCode(user, "type ExtraKind int32") {
		t.Error("user.redis.go 不应重复声明 extra 包的枚举 ExtraKind")
	}

	// 嵌套 message / 嵌套枚举 / 类型映射修复点
	for _, want := range []string{
		"type ExtraMsg_Inner struct",         // 嵌套 message 生成代码
		"Inners []ExtraMsg_Inner",            // repeated 嵌套 message
		"type ExtraMsg_State int32",          // 嵌套枚举声明
		"ExtraMsg_STATE_NONE ExtraMsg_State", // 嵌套枚举常量（message 名前缀）
		"Kinds []ExtraKind",                  // repeated 枚举（此前会生成 []enum）
		"Blobs [][]byte",                     // repeated bytes（此前会生成 []bytes）
		"KindMap map[string]ExtraKind",       // map 值为枚举（此前会生成 map[string]enum）
		"State ExtraMsg_State",
	} {
		if !containsCode(extra, want) {
			t.Errorf("extra.redis.go 缺少 %q", want)
		}
	}
}

// TestKeyFormatParam 验证 --redis_opt=key_format=... 生效。
func TestKeyFormatParam(t *testing.T) {
	resp := runPlugin(t, []*descriptorpb.FileDescriptorProto{userFileDescriptor()}, "key_format=GAME#%d-%d-%d", "GAME#%d-%d-%d")
	content := fileByName(t, resp, "user.redis.go")
	if !containsCode(content, `fmt.Sprintf("GAME#%d-%d-%d", REDBKey, ida, idb)`) {
		t.Error("key_format 参数未生效")
	}
}

// TestPathsSourceRelative 验证 paths=source_relative 时按源路径镜像输出。
func TestPathsSourceRelative(t *testing.T) {
	resp := runPlugin(t, []*descriptorpb.FileDescriptorProto{userFileDescriptor()}, "paths=source_relative", generator.DefaultKeyFormat)
	if len(resp.GetFile()) != 1 || resp.GetFile()[0].GetName() != "proto/user.redis.go" {
		t.Errorf("source_relative 模式下文件名为 %v，期望 proto/user.redis.go", resp.GetFile())
	}
	assertParseable(t, resp.GetFile()[0].GetName(), resp.GetFile()[0].GetContent())
}
