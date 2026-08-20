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

// mapEntry 构造 map 字段必需的合成 entry message（map_entry=true）。
func mapEntry(name string, keyTyp, valTyp descriptorpb.FieldDescriptorProto_Type, valTypeName string) *descriptorpb.DescriptorProto {
	e := &descriptorpb.DescriptorProto{
		Name:    proto.String(name),
		Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
		Field: []*descriptorpb.FieldDescriptorProto{
			field("key", 1, keyTyp, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("value", 2, valTyp, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, valTypeName),
		},
	}
	return e
}

// userFileDescriptor 与 proto/user.proto 一一对应（遵守约定：DB 前缀 + 集合字段 message 包裹）。
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
		Name: proto.String("DBUserBaseInfo"),
		Field: []*descriptorpb.FieldDescriptorProto{
			field("user_id", 1, descriptorpb.FieldDescriptorProto_TYPE_INT32, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("username", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("avatar_url", 3, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("gender", 4, descriptorpb.FieldDescriptorProto_TYPE_ENUM, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".user.Gender"),
			field("level", 5, descriptorpb.FieldDescriptorProto_TYPE_INT32, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("exp", 6, descriptorpb.FieldDescriptorProto_TYPE_INT64, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("balance", 7, descriptorpb.FieldDescriptorProto_TYPE_FLOAT, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("friends", 8, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".user.DBUserBaseInfo.DBFriends"),
			field("settings", 9, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".user.DBUserBaseInfo.DBSettings"),
			field("login_source", 10, descriptorpb.FieldDescriptorProto_TYPE_ENUM, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".user.LoginSource"),
			field("int32_list", 11, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".user.DBUserBaseInfo.DBInt32List"),
			field("weapons", 12, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".user.DBUserBaseInfo.DBWeapons"),
			field("weapon", 13, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".user.DBWeapon"),
			field("weapon_map", 14, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".user.DBUserBaseInfo.DBWeaponMap"),
			field("coin", 15, descriptorpb.FieldDescriptorProto_TYPE_UINT32, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("gem", 16, descriptorpb.FieldDescriptorProto_TYPE_UINT64, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("vip", 17, descriptorpb.FieldDescriptorProto_TYPE_BOOL, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("score", 18, descriptorpb.FieldDescriptorProto_TYPE_DOUBLE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("token", 19, descriptorpb.FieldDescriptorProto_TYPE_BYTES, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("profile", 20, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".user.DBUserBaseInfo.DBProfile"),
			field("vip_level", 21, descriptorpb.FieldDescriptorProto_TYPE_ENUM, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".user.DBUserBaseInfo.VipLevel"),
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
				Name: proto.String("DBFriends"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("items", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ""),
				},
			},
			{
				Name: proto.String("DBSettings"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("kv", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ".user.DBUserBaseInfo.DBSettings.KvEntry"),
				},
				NestedType: []*descriptorpb.DescriptorProto{
					mapEntry("KvEntry", descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""),
				},
			},
			{
				Name: proto.String("DBInt32List"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("items", 1, descriptorpb.FieldDescriptorProto_TYPE_INT32, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ""),
				},
			},
			{
				Name: proto.String("DBWeapons"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("items", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ".user.DBWeapon"),
				},
			},
			{
				Name: proto.String("DBWeaponMap"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("items", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ".user.DBUserBaseInfo.DBWeaponMap.ItemsEntry"),
				},
				NestedType: []*descriptorpb.DescriptorProto{
					mapEntry("ItemsEntry", descriptorpb.FieldDescriptorProto_TYPE_INT32, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".user.DBWeapon"),
				},
			},
			{
				Name: proto.String("DBProfile"),
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
		Name: proto.String("DBWeapon"),
		Field: []*descriptorpb.FieldDescriptorProto{
			field("name", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("damage", 2, descriptorpb.FieldDescriptorProto_TYPE_INT32, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			field("element", 3, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
		},
	}
}

// extraFileDescriptor 覆盖此前类型映射的漏洞场景：
// 跨包引用、嵌套 message、嵌套枚举、repeated 枚举/bytes、map 值为枚举、
// 字段与嵌套 message 同名时的类型名 X 消歧（db_inner 字段 + DBInner）。
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
				Name: proto.String("DBExtraMsg"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("kind", 1, descriptorpb.FieldDescriptorProto_TYPE_ENUM, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".extra.ExtraKind"),
					field("kinds", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".extra.DBExtraMsg.DBExtraKinds"),
					field("blobs", 3, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".extra.DBExtraMsg.DBExtraBlobs"),
					field("kind_map", 4, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".extra.DBExtraMsg.DBExtraKindMap"),
					field("inner", 5, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".extra.DBExtraMsg.DBInner"),
					field("dBInner", 6, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".extra.DBExtraMsg.DBInner"),
					field("inners", 7, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".extra.DBExtraMsg.DBExtraInners"),
					field("state", 8, descriptorpb.FieldDescriptorProto_TYPE_ENUM, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".extra.DBExtraMsg.State"),
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
						Name: proto.String("DBExtraKinds"),
						Field: []*descriptorpb.FieldDescriptorProto{
							field("items", 1, descriptorpb.FieldDescriptorProto_TYPE_ENUM, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ".extra.ExtraKind"),
						},
					},
					{
						Name: proto.String("DBExtraBlobs"),
						Field: []*descriptorpb.FieldDescriptorProto{
							field("items", 1, descriptorpb.FieldDescriptorProto_TYPE_BYTES, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ""),
						},
					},
					{
						Name: proto.String("DBExtraKindMap"),
						Field: []*descriptorpb.FieldDescriptorProto{
							field("kv", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ".extra.DBExtraMsg.DBExtraKindMap.KvEntry"),
						},
						NestedType: []*descriptorpb.DescriptorProto{
							mapEntry("KvEntry", descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_TYPE_ENUM, ".extra.ExtraKind"),
						},
					},
					{
						Name: proto.String("DBExtraInners"),
						Field: []*descriptorpb.FieldDescriptorProto{
							field("items", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ".extra.DBExtraMsg.DBInner"),
						},
					},
					{
						Name: proto.String("DBInner"),
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
		field("extra_ref", 22, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".extra.DBExtraMsg"))
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

// pluginError 运行插件并返回错误文本（校验失败等场景用），无错误时返回 ""。
func pluginError(t *testing.T, files []*descriptorpb.FileDescriptorProto) string {
	t.Helper()
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.GetName())
	}
	req := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: names,
		Parameter:      proto.String(""),
		ProtoFile:      files,
	}
	gen, err := protogen.Options{}.New(req)
	if err != nil {
		return err.Error()
	}
	if err := run(gen, generator.DefaultKeyFormat); err != nil {
		return err.Error()
	}
	return gen.Response().GetError()
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

// ---------- 约定校验测试 ----------

// TestValidateConventions 校验约定：message 必须 DB 前缀、顶层 message 不得直接定义 repeated/map。
// 合规描述符（userFileDescriptor）能正常生成，由其余测试覆盖。
func TestValidateConventions(t *testing.T) {
	// 违规 1：message 缺少 DB 前缀
	f := proto.Clone(userFileDescriptor()).(*descriptorpb.FileDescriptorProto)
	f.MessageType[0].Name = proto.String("UserBaseInfo")
	err := pluginError(t, []*descriptorpb.FileDescriptorProto{f})
	if !strings.Contains(err, "UserBaseInfo") || !strings.Contains(err, "DB") || !strings.Contains(err, "DBUserBaseInfo") {
		t.Errorf("缺少 DB 前缀应报错并给出建议名, got %q", err)
	}

	// 违规 2：顶层 message 直接定义 repeated 字段
	f2 := proto.Clone(userFileDescriptor()).(*descriptorpb.FileDescriptorProto)
	f2.MessageType[0].Field = append(f2.MessageType[0].Field,
		field("bad_list", 30, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ""))
	err = pluginError(t, []*descriptorpb.FileDescriptorProto{f2})
	if !strings.Contains(err, "bad_list") || !strings.Contains(err, "repeated/map") {
		t.Errorf("顶层 repeated 字段应报错并指明字段名, got %q", err)
	}

	// 违规 3：顶层 message 直接定义 map 字段
	f3 := proto.Clone(userFileDescriptor()).(*descriptorpb.FileDescriptorProto)
	f3.MessageType[0].NestedType = append(f3.MessageType[0].NestedType,
		mapEntry("BadMapEntry", descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""))
	f3.MessageType[0].Field = append(f3.MessageType[0].Field,
		field("bad_map", 31, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ".user.DBUserBaseInfo.BadMapEntry"))
	err = pluginError(t, []*descriptorpb.FileDescriptorProto{f3})
	if !strings.Contains(err, "bad_map") || !strings.Contains(err, "repeated/map") {
		t.Errorf("顶层 map 字段应报错并指明字段名, got %q", err)
	}

	// 嵌套 message 内的集合字段不违规（包裹 message 是约定的写法）
	if err := pluginError(t, []*descriptorpb.FileDescriptorProto{userFileDescriptor()}); err != "" {
		t.Errorf("合规描述符不应报错, got %q", err)
	}
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
		"FieldDBUserBaseInfo_UserId FieldDBUserBaseInfo = 1", // user_id -> UserId
		"UserId int32",
		"AvatarUrl string",
		"LoginSource LoginSource",
		"Friends DBUserBaseInfo_DBFriends",   // 集合字段：message 包裹
		"Settings DBUserBaseInfo_DBSettings", // 集合字段：message 包裹
		"Int32List DBUserBaseInfo_DBInt32List",
		"Weapons DBUserBaseInfo_DBWeapons",
		"WeaponMap DBUserBaseInfo_DBWeaponMap",
		"Coin uint32",
		"Gem uint64",
		"Vip bool",
		"Score float64",
		"Token []byte",
		"Profile DBUserBaseInfo_DBProfile",
		"VipLevel DBUserBaseInfo_VipLevel",
		"DBUserBaseInfo_VIP_1 DBUserBaseInfo_VipLevel", // 嵌套枚举常量（message 名前缀）
		"type DBUserBaseInfo_DBProfile struct",         // 嵌套 message 生成代码
		"Nickname string",
		"FieldDBUserBaseInfo_Profile FieldDBUserBaseInfo = 20",
		"type DBUserBaseInfo_DBFriends struct", // 集合字段包裹 message
		"Items []string",
		"Kv map[string]string",
		"Items map[int32]DBWeapon",
		"type DBWeapon struct",
		// protobuf wire format 序列化（语言无关）
		"func (p *DBUserBaseInfo) MarshalRedisProto() ([]byte, error)",
		"func (p *DBUserBaseInfo) UnmarshalRedisProto(b []byte) error",
		"func (p *DBUserBaseInfo_DBProfile) MarshalRedisProto() ([]byte, error)",
		"func (p *DBWeapon) UnmarshalRedisProto(b []byte) error",
		"redisProtoAppendTag(buf, 7, 5)",             // float32 -> fixed32
		"redisProtoAppendTag(buf, 18, 1)",            // float64 -> fixed64
		"redisProtoAppendVarint(buf, uint64(p.Gem))", // uint64 varint
		`redisProtoAppendLen(entry, []byte(k))`,      // map 键 -> length-delimited
		"p.Kv[k] = val",
		// 包裹 message 内裸集合字段的字段级序列化方法
		"func (p *DBUserBaseInfo_DBSettings) MarshalRedisProtoKv() ([]byte, error)",
		"func (p *DBUserBaseInfo_DBFriends) UnmarshalRedisProtoItems(b []byte) error",
		"p.Settings.UnmarshalRedisProto(val)", // GetFields message 字段整体反序列化
		"p.Weapons.MarshalRedisProto()",       // SetFields message 字段整体序列化
		`fmt.Sprintf("REDB#%d:%d:%d", REDBKey, ida, idb)`,
	} {
		if !containsCode(content, want) {
			t.Errorf("生成内容缺少 %q", want)
		}
	}
	for _, banned := range []string{"EVAL", "HSCAN", "AppendFriends", "SetSettingsAll"} {
		if containsCode(content, banned) {
			t.Errorf("生成内容不应包含 %q（元素级操作已移除）", banned)
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
// repeated 枚举/bytes、map 值为枚举、字段与嵌套 message 同名时的 X 消歧。
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
		"ExtraRef extra.DBExtraMsg",
		"extra.DBExtraMsg",
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
		"type DBExtraMsg_DBInner struct",         // 嵌套 message 生成代码
		"Inners DBExtraMsg_DBExtraInners",        // repeated 嵌套 message（包裹）
		"type DBExtraMsg_State int32",            // 嵌套枚举声明
		"DBExtraMsg_STATE_NONE DBExtraMsg_State", // 嵌套枚举常量（message 名前缀）
		"Items []ExtraKind",                      // repeated 枚举（此前会生成 []enum）
		"Items [][]byte",                         // repeated bytes（此前会生成 []bytes）
		"Kv map[string]ExtraKind",                // map 值为枚举（此前会生成 map[string]enum）
		"State DBExtraMsg_State",
		"DBInner DBExtraMsg_DBInner", // dBInner 字段
		// 字段 dBInner 与嵌套 message DBInner 同名：类型名加 X 消歧
		"type FieldDBExtraMsg_DBInnerX uint32",
		"FieldDBExtraMsg_DBInnerX_X FieldDBExtraMsg_DBInnerX = 1",
		"FieldDBExtraMsg_DBInner FieldDBExtraMsg = 6",
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
