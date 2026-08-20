package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	cmddb "github.com/beijian128/protoc-gen-redis/generated"
	"github.com/gomodule/redigo/redis"
)

// 集成测试：需要本机 Redis（默认 127.0.0.1:6379，可用 bin/config.json 覆盖）。
// 连不上时自动跳过，不影响 go test ./... 在无 Redis 环境下的运行。

const testREDBKey uint32 = 424242

func dialRedis(t *testing.T) redis.Conn {
	t.Helper()
	addr := "127.0.0.1:6379"
	password := ""
	if data, err := os.ReadFile("../bin/config.json"); err == nil {
		var cfg struct {
			RedisCfg struct {
				Address  string `json:"address"`
				Password string `json:"password"`
			} `json:"redis"`
		}
		if json.Unmarshal(data, &cfg) == nil && cfg.RedisCfg.Address != "" {
			addr = cfg.RedisCfg.Address
			password = cfg.RedisCfg.Password
		}
	}
	conn, err := redis.Dial("tcp", addr, redis.DialPassword(password))
	if err != nil {
		t.Skipf("Redis 不可用，跳过集成测试: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// newTestUser 返回一个全字段填满的测试数据，覆盖所有类型映射与边界值。
func newTestUser() *cmddb.DBUserBaseInfo {
	return &cmddb.DBUserBaseInfo{
		UserId:      -2147483648, // int32 最小值
		Username:    "测试用户-中文",
		AvatarUrl:   "https://example.com/a.png?x=1&y=2",
		Gender:      cmddb.Gender_GENDER_FEMALE,
		Level:       99,
		Exp:         9223372036854775807, // int64 最大值
		Balance:     3.5,
		Friends:     cmddb.DBUserBaseInfo_DBFriends{Items: []string{"alice", "bob", ""}},
		Settings:    cmddb.DBUserBaseInfo_DBSettings{Kv: map[string]string{"sound": "80", "lang": "zh-CN"}},
		LoginSource: cmddb.LoginSource_SOURCE_MINI_PROGRAM,
		Int32List:   cmddb.DBUserBaseInfo_DBInt32List{Items: []int32{1, -2, 3}},
		Weapons: cmddb.DBUserBaseInfo_DBWeapons{Items: []cmddb.DBWeapon{
			{Name: "sword", Damage: 10, Element: "fire"},
			{Name: "bow", Damage: 8, Element: "ice"},
		}},
		Weapon: cmddb.DBWeapon{Name: "knife", Damage: 5, Element: "poison"},
		WeaponMap: cmddb.DBUserBaseInfo_DBWeaponMap{Items: map[int32]cmddb.DBWeapon{
			1: {Name: "w1", Damage: 1, Element: "e1"},
			2: {Name: "w2", Damage: 2, Element: "e2"},
		}},
		Coin:     4294967295,           // uint32 最大值
		Gem:      18446744073709551615, // uint64 最大值
		Vip:      true,
		Score:    3.25,
		Token:    []byte{0x00, 0x01, 0xFF}, // 含不可打印字节
		Profile:  cmddb.DBUserBaseInfo_DBProfile{Nickname: "nick", Age: 30},
		VipLevel: cmddb.DBUserBaseInfo_VIP_2,
	}
}

// TestFullRoundTrip 全字段写入 + 全字段读取，逐一字段比对。
func TestFullRoundTrip(t *testing.T) {
	conn := dialRedis(t)
	t.Cleanup(func() { conn.Do("DEL", fmt.Sprintf("REDB#%d:1:0", testREDBKey)) })

	want := newTestUser()
	if err := want.SetFields(conn, testREDBKey, 1, 0); err != nil {
		t.Fatalf("SetFields(全字段): %v", err)
	}

	got := &cmddb.DBUserBaseInfo{}
	if err := got.GetFields(conn, testREDBKey, 1, 0); err != nil {
		t.Fatalf("GetFields(全字段): %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("全字段回读不一致:\n got = %#v\nwant = %#v", got, want)
	}
}

// TestPartialFields 只写部分字段，只读部分字段；未写字段保持零值。
func TestPartialFields(t *testing.T) {
	conn := dialRedis(t)
	t.Cleanup(func() { conn.Do("DEL", fmt.Sprintf("REDB#%d:2:0", testREDBKey)) })

	u := &cmddb.DBUserBaseInfo{UserId: 7, Username: "partial", Vip: true}
	if err := u.SetFields(conn, testREDBKey, 2, 0,
		cmddb.FieldDBUserBaseInfo_UserId, cmddb.FieldDBUserBaseInfo_Username, cmddb.FieldDBUserBaseInfo_Vip); err != nil {
		t.Fatalf("SetFields(局部): %v", err)
	}

	got := &cmddb.DBUserBaseInfo{}
	if err := got.GetFields(conn, testREDBKey, 2, 0,
		cmddb.FieldDBUserBaseInfo_UserId, cmddb.FieldDBUserBaseInfo_Username, cmddb.FieldDBUserBaseInfo_Vip,
		cmddb.FieldDBUserBaseInfo_Level, // 顺带读一个从未写入的字段
	); err != nil {
		t.Fatalf("GetFields(局部): %v", err)
	}
	if got.UserId != 7 || got.Username != "partial" || !got.Vip {
		t.Errorf("局部字段回读错误: %#v", got)
	}
	if got.Level != 0 {
		t.Errorf("未写入字段 Level 应为零值, got %d", got.Level)
	}
}

// TestSingleField 单字段写入与读取。
func TestSingleField(t *testing.T) {
	conn := dialRedis(t)
	t.Cleanup(func() { conn.Do("DEL", fmt.Sprintf("REDB#%d:3:0", testREDBKey)) })

	u := &cmddb.DBUserBaseInfo{Exp: 1234567890123}
	if err := u.SetFields(conn, testREDBKey, 3, 0, cmddb.FieldDBUserBaseInfo_Exp); err != nil {
		t.Fatalf("SetFields(单字段): %v", err)
	}

	got := &cmddb.DBUserBaseInfo{}
	if err := got.GetFields(conn, testREDBKey, 3, 0, cmddb.FieldDBUserBaseInfo_Exp); err != nil {
		t.Fatalf("GetFields(单字段): %v", err)
	}
	if got.Exp != 1234567890123 {
		t.Errorf("Exp = %d, want 1234567890123", got.Exp)
	}
}

// TestMissingFieldIsZero 读一个从未写入的键/字段，应得到零值而非报错。
func TestMissingFieldIsZero(t *testing.T) {
	conn := dialRedis(t)
	t.Cleanup(func() { conn.Do("DEL", fmt.Sprintf("REDB#%d:4:0", testREDBKey)) })

	got := &cmddb.DBUserBaseInfo{}
	if err := got.GetFields(conn, testREDBKey, 4, 0); err != nil {
		t.Fatalf("GetFields(空键): %v", err)
	}
	want := &cmddb.DBUserBaseInfo{}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("空键读取应全零: %#v", got)
	}
}

// TestKeyIsolation 不同 ida/idb 分片之间互不影响。
func TestKeyIsolation(t *testing.T) {
	conn := dialRedis(t)
	t.Cleanup(func() { conn.Do("DEL", fmt.Sprintf("REDB#%d:5:1", testREDBKey)) })

	u := &cmddb.DBUserBaseInfo{UserId: 100}
	if err := u.SetFields(conn, testREDBKey, 5, 1); err != nil {
		t.Fatalf("SetFields: %v", err)
	}

	got := &cmddb.DBUserBaseInfo{}
	if err := got.GetFields(conn, testREDBKey, 5, 2); err != nil {
		t.Fatalf("GetFields(其他分片): %v", err)
	}
	if got.UserId != 0 {
		t.Errorf("其他分片不应读到数据, got %#v", got)
	}
}

// TestOverwritePreservesOthers 只更新一个字段，其余字段保持原值。
func TestOverwritePreservesOthers(t *testing.T) {
	conn := dialRedis(t)
	t.Cleanup(func() { conn.Do("DEL", fmt.Sprintf("REDB#%d:6:0", testREDBKey)) })

	u := newTestUser()
	if err := u.SetFields(conn, testREDBKey, 6, 0); err != nil {
		t.Fatalf("SetFields(全字段): %v", err)
	}

	u.Username = "renamed"
	if err := u.SetFields(conn, testREDBKey, 6, 0, cmddb.FieldDBUserBaseInfo_Username); err != nil {
		t.Fatalf("SetFields(单字段覆盖): %v", err)
	}

	got := &cmddb.DBUserBaseInfo{}
	if err := got.GetFields(conn, testREDBKey, 6, 0); err != nil {
		t.Fatalf("GetFields: %v", err)
	}
	if got.Username != "renamed" {
		t.Errorf("Username = %q, want renamed", got.Username)
	}
	if got.UserId != u.UserId || got.Gem != u.Gem || !reflect.DeepEqual(got.WeaponMap, u.WeaponMap) {
		t.Errorf("覆盖后其他字段应保持原值:\n got = %#v", got)
	}
}

// TestUnknownFieldError 未知字段编号应返回错误（Get 与 Set 两侧）。
func TestUnknownFieldError(t *testing.T) {
	conn := dialRedis(t)
	t.Cleanup(func() { conn.Do("DEL", fmt.Sprintf("REDB#%d:7:0", testREDBKey)) })

	u := &cmddb.DBUserBaseInfo{}
	if err := u.SetFields(conn, testREDBKey, 7, 0, cmddb.FieldDBUserBaseInfo(999)); err == nil {
		t.Error("SetFields 未知字段应返回错误")
	}
	if err := u.GetFields(conn, testREDBKey, 7, 0, cmddb.FieldDBUserBaseInfo(999)); err == nil {
		t.Error("GetFields 未知字段应返回错误")
	}
}

// TestInvalidValueReturnsError 数值/枚举字段被写入脏数据时，GetFields 应返回错误而非静默零值。
func TestInvalidValueReturnsError(t *testing.T) {
	conn := dialRedis(t)
	t.Cleanup(func() { conn.Do("DEL", fmt.Sprintf("REDB#%d:8:0", testREDBKey)) })

	key := fmt.Sprintf("REDB#%d:8:0", testREDBKey)
	if _, err := conn.Do("HSET", key, cmddb.FieldDBUserBaseInfo_UserId, "not-a-number"); err != nil {
		t.Fatalf("HSET 脏数据: %v", err)
	}
	if _, err := conn.Do("HSET", key, cmddb.FieldDBUserBaseInfo_Gender, "abc"); err != nil {
		t.Fatalf("HSET 脏数据: %v", err)
	}
	// 集合字段（message 包裹）：hash field 就是字段编号，值为整个包裹 message 的 wire format 字节
	if _, err := conn.Do("HSET", key, cmddb.FieldDBUserBaseInfo_Weapons, []byte{0xDE, 0xAD}); err != nil {
		t.Fatalf("HSET 脏数据: %v", err)
	}

	u := &cmddb.DBUserBaseInfo{}
	if err := u.GetFields(conn, testREDBKey, 8, 0, cmddb.FieldDBUserBaseInfo_UserId); err == nil {
		t.Error("int32 字段脏数据应返回错误")
	}
	if err := u.GetFields(conn, testREDBKey, 8, 0, cmddb.FieldDBUserBaseInfo_Gender); err == nil {
		t.Error("枚举字段脏数据应返回错误")
	}
	if err := u.GetFields(conn, testREDBKey, 8, 0, cmddb.FieldDBUserBaseInfo_Weapons); err == nil {
		t.Error("protobuf 字段脏数据应返回错误")
	}
}

// TestZeroValueRoundTrip 零值全字段的写入与读取。
// 契约：包裹 message 内的集合无元素即回读为 nil（如 Friends.Items 为 nil）；
// bytes 字段经 redigo 直存，nil 写回后是空 []byte{}。
func TestZeroValueRoundTrip(t *testing.T) {
	conn := dialRedis(t)
	t.Cleanup(func() { conn.Do("DEL", fmt.Sprintf("REDB#%d:9:0", testREDBKey)) })

	u := &cmddb.DBUserBaseInfo{} // 全零值
	if err := u.SetFields(conn, testREDBKey, 9, 0); err != nil {
		t.Fatalf("SetFields(零值): %v", err)
	}

	got := &cmddb.DBUserBaseInfo{}
	if err := got.GetFields(conn, testREDBKey, 9, 0); err != nil {
		t.Fatalf("GetFields(零值): %v", err)
	}
	want := &cmddb.DBUserBaseInfo{
		Token: []byte{}, // nil bytes 直存后回读为空 []byte
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("零值往返不一致:\n got = %#v\nwant = %#v", got, want)
	}
}

// TestEmptyCollectionRoundTrip 空集合（非 nil 的切片/map）的往返行为。
// 契约：包裹 message 内的集合无元素即回读为 nil；bytes 字段空 []byte{} 往返不变。
func TestEmptyCollectionRoundTrip(t *testing.T) {
	conn := dialRedis(t)
	t.Cleanup(func() { conn.Do("DEL", fmt.Sprintf("REDB#%d:10:0", testREDBKey)) })

	u := &cmddb.DBUserBaseInfo{
		Friends:  cmddb.DBUserBaseInfo_DBFriends{Items: []string{}},
		Settings: cmddb.DBUserBaseInfo_DBSettings{Kv: map[string]string{}},
		Weapons:  cmddb.DBUserBaseInfo_DBWeapons{Items: []cmddb.DBWeapon{}},
		Token:    []byte{},
	}
	if err := u.SetFields(conn, testREDBKey, 10, 0); err != nil {
		t.Fatalf("SetFields(空集合): %v", err)
	}

	got := &cmddb.DBUserBaseInfo{}
	if err := got.GetFields(conn, testREDBKey, 10, 0); err != nil {
		t.Fatalf("GetFields(空集合): %v", err)
	}
	want := &cmddb.DBUserBaseInfo{
		Token: []byte{}, // 空 bytes 往返不变
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("空集合往返不一致:\n got = %#v\nwant = %#v", got, want)
	}
}

// ---------- 集合字段（message 包裹）整体读写 ----------

// TestWrappedCollectionFieldMarshal 包裹 message 内裸集合字段的字段级序列化方法往返
// （MarshalRedisProto<Field> -> UnmarshalRedisProto<Field>，不依赖 Redis），
// 覆盖 string/数值/message 元素、map、空元素（""）。
func TestWrappedCollectionFieldMarshal(t *testing.T) {
	// repeated string
	f := &cmddb.DBUserBaseInfo_DBFriends{Items: []string{"alice", "bob", ""}}
	b, err := f.MarshalRedisProtoItems()
	if err != nil {
		t.Fatalf("DBFriends.MarshalRedisProto: %v", err)
	}
	gf := &cmddb.DBUserBaseInfo_DBFriends{}
	if err := gf.UnmarshalRedisProtoItems(b); err != nil {
		t.Fatalf("DBFriends.UnmarshalRedisProto: %v", err)
	}
	if !reflect.DeepEqual(gf.Items, f.Items) {
		t.Errorf("DBFriends 往返不一致:\n got = %#v\nwant = %#v", gf.Items, f.Items)
	}

	// repeated int32（含负值）
	l := &cmddb.DBUserBaseInfo_DBInt32List{Items: []int32{1, -2, 3}}
	b, err = l.MarshalRedisProtoItems()
	if err != nil {
		t.Fatalf("DBInt32List.MarshalRedisProto: %v", err)
	}
	gl := &cmddb.DBUserBaseInfo_DBInt32List{}
	if err := gl.UnmarshalRedisProtoItems(b); err != nil {
		t.Fatalf("DBInt32List.UnmarshalRedisProto: %v", err)
	}
	if !reflect.DeepEqual(gl.Items, l.Items) {
		t.Errorf("DBInt32List 往返不一致: %#v", gl.Items)
	}

	// map<string,string>
	s := &cmddb.DBUserBaseInfo_DBSettings{Kv: map[string]string{"sound": "80", "lang": "zh-CN"}}
	b, err = s.MarshalRedisProtoKv()
	if err != nil {
		t.Fatalf("DBSettings.MarshalRedisProto: %v", err)
	}
	gs := &cmddb.DBUserBaseInfo_DBSettings{}
	if err := gs.UnmarshalRedisProtoKv(b); err != nil {
		t.Fatalf("DBSettings.UnmarshalRedisProto: %v", err)
	}
	if !reflect.DeepEqual(gs.Kv, s.Kv) {
		t.Errorf("DBSettings 往返不一致: %#v", gs.Kv)
	}

	// repeated message 元素
	w := &cmddb.DBUserBaseInfo_DBWeapons{Items: []cmddb.DBWeapon{
		{Name: "sword", Damage: 10, Element: "fire"},
		{Name: "bow", Damage: 8, Element: "ice"},
	}}
	b, err = w.MarshalRedisProtoItems()
	if err != nil {
		t.Fatalf("DBWeapons.MarshalRedisProto: %v", err)
	}
	gw := &cmddb.DBUserBaseInfo_DBWeapons{}
	if err := gw.UnmarshalRedisProtoItems(b); err != nil {
		t.Fatalf("DBWeapons.UnmarshalRedisProto: %v", err)
	}
	if !reflect.DeepEqual(gw.Items, w.Items) {
		t.Errorf("DBWeapons 往返不一致: %#v", gw.Items)
	}

	// map<int32, message>
	wm := &cmddb.DBUserBaseInfo_DBWeaponMap{Items: map[int32]cmddb.DBWeapon{
		1: {Name: "w1", Damage: 1, Element: "e1"},
		2: {Name: "w2", Damage: 2, Element: "e2"},
	}}
	b, err = wm.MarshalRedisProtoItems()
	if err != nil {
		t.Fatalf("DBWeaponMap.MarshalRedisProto: %v", err)
	}
	gwm := &cmddb.DBUserBaseInfo_DBWeaponMap{}
	if err := gwm.UnmarshalRedisProtoItems(b); err != nil {
		t.Fatalf("DBWeaponMap.UnmarshalRedisProto: %v", err)
	}
	if !reflect.DeepEqual(gwm.Items, wm.Items) {
		t.Errorf("DBWeaponMap 往返不一致: %#v", gwm.Items)
	}

	// 空集合（无元素）编码为空字节，反序列化后为 nil
	b, err = (&cmddb.DBUserBaseInfo_DBFriends{}).MarshalRedisProtoItems()
	if err != nil {
		t.Fatalf("空 DBFriends.MarshalRedisProto: %v", err)
	}
	if len(b) != 0 {
		t.Errorf("空集合应编码为空字节, got % X", b)
	}
	gf = &cmddb.DBUserBaseInfo_DBFriends{}
	if err := gf.UnmarshalRedisProtoItems(b); err != nil {
		t.Fatalf("空 DBFriends.UnmarshalRedisProto: %v", err)
	}
	if gf.Items != nil {
		t.Errorf("空集合反序列化应为 nil, got %#v", gf.Items)
	}
}

// TestWrappedCollectionRoundTrip 包裹 message 的集合字段整体读写（Redis）：
// SetFields/GetFields 单字段读写，覆盖写入会清掉旧元素（整体替换语义）。
func TestWrappedCollectionRoundTrip(t *testing.T) {
	conn := dialRedis(t)
	t.Cleanup(func() { conn.Do("DEL", fmt.Sprintf("REDB#%d:11:0", testREDBKey)) })

	u := &cmddb.DBUserBaseInfo{
		Friends:  cmddb.DBUserBaseInfo_DBFriends{Items: []string{"x", "y"}},
		Settings: cmddb.DBUserBaseInfo_DBSettings{Kv: map[string]string{"a": "1"}},
	}
	if err := u.SetFields(conn, testREDBKey, 11, 0, cmddb.FieldDBUserBaseInfo_Friends, cmddb.FieldDBUserBaseInfo_Settings); err != nil {
		t.Fatalf("SetFields(集合字段): %v", err)
	}

	got := &cmddb.DBUserBaseInfo{}
	if err := got.GetFields(conn, testREDBKey, 11, 0, cmddb.FieldDBUserBaseInfo_Friends, cmddb.FieldDBUserBaseInfo_Settings); err != nil {
		t.Fatalf("GetFields(集合字段): %v", err)
	}
	if !reflect.DeepEqual(got.Friends, u.Friends) || !reflect.DeepEqual(got.Settings, u.Settings) {
		t.Errorf("集合字段回读不一致:\n got = %#v", got)
	}

	// 整体覆盖写入：旧元素必须消失
	u2 := &cmddb.DBUserBaseInfo{Settings: cmddb.DBUserBaseInfo_DBSettings{Kv: map[string]string{"b": "2"}}}
	if err := u2.SetFields(conn, testREDBKey, 11, 0, cmddb.FieldDBUserBaseInfo_Settings); err != nil {
		t.Fatalf("SetFields(覆盖): %v", err)
	}
	got = &cmddb.DBUserBaseInfo{}
	if err := got.GetFields(conn, testREDBKey, 11, 0, cmddb.FieldDBUserBaseInfo_Settings); err != nil {
		t.Fatalf("GetFields(覆盖后): %v", err)
	}
	if want := map[string]string{"b": "2"}; !reflect.DeepEqual(got.Settings.Kv, want) {
		t.Errorf("覆盖后 Settings = %#v, want %#v（旧键 a 应消失）", got.Settings.Kv, want)
	}
}

// ---------- protobuf wire format 一致性测试（不依赖 Redis） ----------
//
// 期望字节全部按 protobuf 编码规范手算（https://protobuf.dev/programming-guides/encoding/），
// 用于证明生成代码输出的是真正的 protobuf wire format，可被任何语言的实现解析。

// TestMarshalRedisProtoConformance 用规范手算的字节验证 MarshalRedisProto 输出：
// varint（int32/enum/uint64/bool）、fixed32（float）、fixed64（double）、
// length-delimited（string/bytes/message）、map 条目、repeated、零值跳过、负 int32 符号扩展。
func TestMarshalRedisProtoConformance(t *testing.T) {
	// DBWeapon{name:"fire", damage:1, element:"e"}
	// 0A 04 66 69 72 65 | 10 01 | 1A 01 65
	w := cmddb.DBWeapon{Name: "fire", Damage: 1, Element: "e"}
	got, err := w.MarshalRedisProto()
	if err != nil {
		t.Fatalf("DBWeapon.MarshalRedisProto: %v", err)
	}
	want := []byte{0x0A, 0x04, 'f', 'i', 'r', 'e', 0x10, 0x01, 0x1A, 0x01, 'e'}
	if !bytes.Equal(got, want) {
		t.Errorf("DBWeapon 编码 = % X, want % X", got, want)
	}

	// 零值 message 应编码为空字节流（proto3 跳过零值标量/空字符串）
	empty, err := (&cmddb.DBWeapon{}).MarshalRedisProto()
	if err != nil {
		t.Fatalf("空 DBWeapon.MarshalRedisProto: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("零值 DBWeapon 编码 = % X, want 空", empty)
	}

	u := &cmddb.DBUserBaseInfo{
		UserId:   7,                                                                // tag 1  varint
		Username: "ab",                                                             // tag 2  string
		Gender:   cmddb.Gender_GENDER_MALE,                                         // tag 4  枚举 varint
		Balance:  3.5,                                                              // tag 7  float -> fixed32
		Friends:  cmddb.DBUserBaseInfo_DBFriends{Items: []string{"x"}},             // tag 8  包裹 repeated
		Settings: cmddb.DBUserBaseInfo_DBSettings{Kv: map[string]string{"k": "v"}}, // tag 9  包裹 map
		Weapon:   cmddb.DBWeapon{Name: "w", Damage: 2},                             // tag 13 message（element 空串不编码）
		Coin:     1,                                                                // tag 15 uint32
		Vip:      true,                                                             // tag 17 bool
		Token:    []byte{0xFF},                                                     // tag 19 bytes
		// Profile 为零值但 message 字段恒编码 -> 空子消息
	}
	got, err = u.MarshalRedisProto()
	if err != nil {
		t.Fatalf("DBUserBaseInfo.MarshalRedisProto: %v", err)
	}
	want = []byte{
		0x08, 0x07, // UserId = 7
		0x12, 0x02, 'a', 'b', // Username = "ab"
		0x20, 0x01, // Gender = GENDER_MALE
		0x3D, 0x00, 0x00, 0x60, 0x40, // Balance = 3.5f（小端 fixed32）
		0x42, 0x03, 0x0A, 0x01, 'x', // Friends = DBFriends{Items:["x"]}
		0x4A, 0x08, 0x0A, 0x06, 0x0A, 0x01, 'k', 0x12, 0x01, 'v', // Settings 包裹 {kv:{"k":"v"}}
		0x5A, 0x00, // Int32List = 空消息（未设置，恒编码）
		0x62, 0x00, // Weapons = 空消息（未设置，恒编码）
		0x6A, 0x05, 0x0A, 0x01, 'w', 0x10, 0x02, // Weapon{name:"w", damage:2}
		0x72, 0x00, // WeaponMap = 空消息（未设置，恒编码）
		0x78, 0x01, // Coin = 1
		0x88, 0x01, 0x01, // Vip = true
		0x9A, 0x01, 0x01, 0xFF, // Token = [0xFF]
		0xA2, 0x01, 0x00, // Profile = 空消息（恒编码）
	}
	if !bytes.Equal(got, want) {
		t.Errorf("DBUserBaseInfo 编码 = % X\nwant           = % X", got, want)
	}

	// 负 int32/int64 必须符号扩展为 10 字节 varint（与 protoc 一致）
	neg := &cmddb.DBUserBaseInfo{UserId: -1, Exp: -2}
	got, err = neg.MarshalRedisProto()
	if err != nil {
		t.Fatalf("负值 MarshalRedisProto: %v", err)
	}
	want = []byte{
		0x08, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x01, // int32(-1)
		0x30, 0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x01, // int64(-2)
		0x42, 0x00, // Friends = 空消息（message 字段恒编码）
		0x4A, 0x00, // Settings = 空消息（message 字段恒编码）
		0x5A, 0x00, // Int32List = 空消息（message 字段恒编码）
		0x62, 0x00, // Weapons = 空消息（message 字段恒编码）
		0x6A, 0x00, // Weapon = 空消息（message 字段恒编码）
		0x72, 0x00, // WeaponMap = 空消息（message 字段恒编码）
		0xA2, 0x01, 0x00, // Profile = 空消息（message 字段恒编码）
	}
	if !bytes.Equal(got, want) {
		t.Errorf("负值编码 = % X, want % X", got, want)
	}
}

// TestUnmarshalRedisProtoConformance 用规范手算的字节验证 UnmarshalRedisProto：
// 未知字段（varint/fixed32/fixed64/length-delimited）跳过、packed repeated 解码、
// 负值回读、wire type 校验、截断数据报错。
func TestUnmarshalRedisProtoConformance(t *testing.T) {
	b := []byte{
		0x82, 0x06, 0x02, 'a', 'b', // 未知字段 96（wire 2）-> 跳过
		0x89, 0x06, 0x01, 0, 0, 0, 0, 0, 0, 0, // 未知字段 97（wire 1 fixed64）-> 跳过
		0x95, 0x06, 0xDE, 0xAD, 0xBE, 0xEF, // 未知字段 98（wire 5 fixed32）-> 跳过
		0x98, 0x06, 0x05, // 未知字段 99（wire 0 varint）-> 跳过
		0x08, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x01, // UserId = -1
		0x30, 0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x01, // Exp = -2
		0x5A, 0x05, 0x0A, 0x03, 0x01, 0x02, 0x03, // Int32List = DBInt32List{Items:[1,2,3]}（packed）
		0x42, 0x04, 0x0A, 0x02, 'x', 'y', // Friends = DBFriends{Items:["xy"]}
	}
	u := &cmddb.DBUserBaseInfo{Friends: cmddb.DBUserBaseInfo_DBFriends{Items: []string{"stale"}}} // 反序列化前会被重置
	if err := u.UnmarshalRedisProto(b); err != nil {
		t.Fatalf("UnmarshalRedisProto: %v", err)
	}
	if u.UserId != -1 || u.Exp != -2 {
		t.Errorf("负值回读 = %d, %d, want -1, -2", u.UserId, u.Exp)
	}
	if want := []int32{1, 2, 3}; !reflect.DeepEqual(u.Int32List.Items, want) {
		t.Errorf("Int32List.Items = %v, want %v", u.Int32List.Items, want)
	}
	if want := []string{"xy"}; !reflect.DeepEqual(u.Friends.Items, want) {
		t.Errorf("Friends.Items = %v, want %v", u.Friends.Items, want)
	}
	if u.Username != "" {
		t.Errorf("重置失败: Username = %q, want 空", u.Username)
	}

	// wire type 不匹配应报错
	if err := (&cmddb.DBUserBaseInfo{}).UnmarshalRedisProto([]byte{0x0A, 0x01, 0x00}); err == nil {
		t.Error("字段 1 用 wire 2 编码应报错（期望 varint）")
	}
	// 截断的 varint 应报错
	if err := (&cmddb.DBUserBaseInfo{}).UnmarshalRedisProto([]byte{0x08, 0x80}); err == nil {
		t.Error("截断的 varint 应报错")
	}
	// 截断的 length-delimited 应报错
	if err := (&cmddb.DBUserBaseInfo{}).UnmarshalRedisProto([]byte{0x12, 0x05, 'a'}); err == nil {
		t.Error("截断的 length-delimited 应报错")
	}
}

// TestMarshalRedisProtoRoundTrip 全字段数据的 编码 -> 解码 往返一致性。
func TestMarshalRedisProtoRoundTrip(t *testing.T) {
	want := newTestUser()
	got, err := want.MarshalRedisProto()
	if err != nil {
		t.Fatalf("MarshalRedisProto: %v", err)
	}
	u := &cmddb.DBUserBaseInfo{}
	if err := u.UnmarshalRedisProto(got); err != nil {
		t.Fatalf("UnmarshalRedisProto: %v", err)
	}
	if !reflect.DeepEqual(u, want) {
		t.Errorf("protobuf 往返不一致:\n got = %#v\nwant = %#v", u, want)
	}
}
