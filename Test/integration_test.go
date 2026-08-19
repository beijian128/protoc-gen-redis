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
func newTestUser() *cmddb.UserBaseInfo {
	return &cmddb.UserBaseInfo{
		UserId:      -2147483648, // int32 最小值
		Username:    "测试用户-中文",
		AvatarUrl:   "https://example.com/a.png?x=1&y=2",
		Gender:      cmddb.Gender_GENDER_FEMALE,
		Level:       99,
		Exp:         9223372036854775807, // int64 最大值
		Balance:     3.5,
		Friends:     []string{"alice", "bob", ""},
		Settings:    map[string]string{"sound": "80", "lang": "zh-CN"},
		LoginSource: cmddb.LoginSource_SOURCE_MINI_PROGRAM,
		Listint32:   []int32{1, -2, 3},
		Weapons: []cmddb.Weapon{
			{Name: "sword", Damage: 10, Element: "fire"},
			{Name: "bow", Damage: 8, Element: "ice"},
		},
		Weapon: cmddb.Weapon{Name: "knife", Damage: 5, Element: "poison"},
		WeaponMap: map[int32]cmddb.Weapon{
			1: {Name: "w1", Damage: 1, Element: "e1"},
			2: {Name: "w2", Damage: 2, Element: "e2"},
		},
		Coin:     4294967295,           // uint32 最大值
		Gem:      18446744073709551615, // uint64 最大值
		Vip:      true,
		Score:    3.25,
		Token:    []byte{0x00, 0x01, 0xFF}, // 含不可打印字节
		Profile:  cmddb.UserBaseInfo_Profile{Nickname: "nick", Age: 30},
		VipLevel: cmddb.UserBaseInfo_VIP_2,
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

	got := &cmddb.UserBaseInfo{}
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

	u := &cmddb.UserBaseInfo{UserId: 7, Username: "partial", Vip: true}
	if err := u.SetFields(conn, testREDBKey, 2, 0,
		cmddb.FieldUserBaseInfo_UserId, cmddb.FieldUserBaseInfo_Username, cmddb.FieldUserBaseInfo_Vip); err != nil {
		t.Fatalf("SetFields(局部): %v", err)
	}

	got := &cmddb.UserBaseInfo{}
	if err := got.GetFields(conn, testREDBKey, 2, 0,
		cmddb.FieldUserBaseInfo_UserId, cmddb.FieldUserBaseInfo_Username, cmddb.FieldUserBaseInfo_Vip,
		cmddb.FieldUserBaseInfo_Level, // 顺带读一个从未写入的字段
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

	u := &cmddb.UserBaseInfo{Exp: 1234567890123}
	if err := u.SetFields(conn, testREDBKey, 3, 0, cmddb.FieldUserBaseInfo_Exp); err != nil {
		t.Fatalf("SetFields(单字段): %v", err)
	}

	got := &cmddb.UserBaseInfo{}
	if err := got.GetFields(conn, testREDBKey, 3, 0, cmddb.FieldUserBaseInfo_Exp); err != nil {
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

	got := &cmddb.UserBaseInfo{}
	if err := got.GetFields(conn, testREDBKey, 4, 0); err != nil {
		t.Fatalf("GetFields(空键): %v", err)
	}
	want := &cmddb.UserBaseInfo{}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("空键读取应全零: %#v", got)
	}
}

// TestKeyIsolation 不同 ida/idb 分片之间互不影响。
func TestKeyIsolation(t *testing.T) {
	conn := dialRedis(t)
	t.Cleanup(func() { conn.Do("DEL", fmt.Sprintf("REDB#%d:5:1", testREDBKey)) })

	u := &cmddb.UserBaseInfo{UserId: 100}
	if err := u.SetFields(conn, testREDBKey, 5, 1); err != nil {
		t.Fatalf("SetFields: %v", err)
	}

	got := &cmddb.UserBaseInfo{}
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
	if err := u.SetFields(conn, testREDBKey, 6, 0, cmddb.FieldUserBaseInfo_Username); err != nil {
		t.Fatalf("SetFields(单字段覆盖): %v", err)
	}

	got := &cmddb.UserBaseInfo{}
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

	u := &cmddb.UserBaseInfo{}
	if err := u.SetFields(conn, testREDBKey, 7, 0, cmddb.FieldUserBaseInfo(999)); err == nil {
		t.Error("SetFields 未知字段应返回错误")
	}
	if err := u.GetFields(conn, testREDBKey, 7, 0, cmddb.FieldUserBaseInfo(999)); err == nil {
		t.Error("GetFields 未知字段应返回错误")
	}
}

// TestInvalidValueReturnsError 数值/枚举字段被写入脏数据时，GetFields 应返回错误而非静默零值。
func TestInvalidValueReturnsError(t *testing.T) {
	conn := dialRedis(t)
	t.Cleanup(func() { conn.Do("DEL", fmt.Sprintf("REDB#%d:8:0", testREDBKey)) })

	key := fmt.Sprintf("REDB#%d:8:0", testREDBKey)
	if _, err := conn.Do("HSET", key, cmddb.FieldUserBaseInfo_UserId, "not-a-number"); err != nil {
		t.Fatalf("HSET 脏数据: %v", err)
	}
	if _, err := conn.Do("HSET", key, cmddb.FieldUserBaseInfo_Gender, "abc"); err != nil {
		t.Fatalf("HSET 脏数据: %v", err)
	}
	// 集合字段按元素级存储；message 元素字段（protobuf 序列化）名形如 "<tag>:<index>"
	if _, err := conn.Do("HSET", key, fmt.Sprintf("%d:0", cmddb.FieldUserBaseInfo_Weapons), []byte{0xDE, 0xAD}); err != nil {
		t.Fatalf("HSET 脏数据: %v", err)
	}

	u := &cmddb.UserBaseInfo{}
	if err := u.GetFields(conn, testREDBKey, 8, 0, cmddb.FieldUserBaseInfo_UserId); err == nil {
		t.Error("int32 字段脏数据应返回错误")
	}
	if err := u.GetFields(conn, testREDBKey, 8, 0, cmddb.FieldUserBaseInfo_Gender); err == nil {
		t.Error("枚举字段脏数据应返回错误")
	}
	if err := u.GetFields(conn, testREDBKey, 8, 0, cmddb.FieldUserBaseInfo_Weapons); err == nil {
		t.Error("protobuf 元素字段脏数据应返回错误")
	}
}

// TestZeroValueRoundTrip 零值全字段的写入与读取。
// 契约：集合字段（map/repeated）无元素即回读为 nil；
// bytes 字段经 redigo 直存，nil 写回后是空 []byte{}。
func TestZeroValueRoundTrip(t *testing.T) {
	conn := dialRedis(t)
	t.Cleanup(func() { conn.Do("DEL", fmt.Sprintf("REDB#%d:9:0", testREDBKey)) })

	u := &cmddb.UserBaseInfo{} // 全零值
	if err := u.SetFields(conn, testREDBKey, 9, 0); err != nil {
		t.Fatalf("SetFields(零值): %v", err)
	}

	got := &cmddb.UserBaseInfo{}
	if err := got.GetFields(conn, testREDBKey, 9, 0); err != nil {
		t.Fatalf("GetFields(零值): %v", err)
	}
	want := &cmddb.UserBaseInfo{
		Token: []byte{}, // nil bytes 直存后回读为空 []byte
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("零值往返不一致:\n got = %#v\nwant = %#v", got, want)
	}
}

// TestEmptyCollectionRoundTrip 空集合（非 nil 的切片/map）的往返行为。
// 契约：集合字段无元素即回读为 nil；bytes 字段空 []byte{} 往返不变。
func TestEmptyCollectionRoundTrip(t *testing.T) {
	conn := dialRedis(t)
	t.Cleanup(func() { conn.Do("DEL", fmt.Sprintf("REDB#%d:10:0", testREDBKey)) })

	u := &cmddb.UserBaseInfo{
		Friends:  []string{},
		Settings: map[string]string{},
		Weapons:  []cmddb.Weapon{},
		Token:    []byte{},
	}
	if err := u.SetFields(conn, testREDBKey, 10, 0); err != nil {
		t.Fatalf("SetFields(空集合): %v", err)
	}

	got := &cmddb.UserBaseInfo{}
	if err := got.GetFields(conn, testREDBKey, 10, 0); err != nil {
		t.Fatalf("GetFields(空集合): %v", err)
	}
	want := &cmddb.UserBaseInfo{
		Token: []byte{}, // 空 bytes 往返不变
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("空集合往返不一致:\n got = %#v\nwant = %#v", got, want)
	}
}

// ---------- 元素级操作（方案 A：map/repeated 按元素存储） ----------

// TestMapElementOps map 元素的 Set/Get/Del 单键操作。
func TestMapElementOps(t *testing.T) {
	conn := dialRedis(t)
	t.Cleanup(func() { conn.Do("DEL", fmt.Sprintf("REDB#%d:11:0", testREDBKey)) })

	u := &cmddb.UserBaseInfo{}
	if err := u.SetSettings(conn, testREDBKey, 11, 0, "sound", "80"); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	if err := u.SetSettings(conn, testREDBKey, 11, 0, "lang", "zh-CN"); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}

	// 单键读取
	v, ok, err := u.GetSettings(conn, testREDBKey, 11, 0, "sound")
	if err != nil || !ok || v != "80" {
		t.Errorf("GetSettings(sound) = %q, %v, %v; want \"80\", true, nil", v, ok, err)
	}
	// 不存在的键
	if v, ok, err := u.GetSettings(conn, testREDBKey, 11, 0, "missing"); err != nil || ok || v != "" {
		t.Errorf("GetSettings(missing) = %q, %v, %v; want 零值, false, nil", v, ok, err)
	}
	// 单键删除
	deleted, err := u.DelSettings(conn, testREDBKey, 11, 0, "lang")
	if err != nil || !deleted {
		t.Errorf("DelSettings = %v, %v; want true, nil", deleted, err)
	}
	// 删除不存在的键返回 false
	if deleted, err := u.DelSettings(conn, testREDBKey, 11, 0, "lang"); err != nil || deleted {
		t.Errorf("重复 DelSettings = %v, %v; want false, nil", deleted, err)
	}

	// 整体读回只剩 sound
	got := &cmddb.UserBaseInfo{}
	if err := got.GetSettingsAll(conn, testREDBKey, 11, 0); err != nil {
		t.Fatalf("GetSettingsAll: %v", err)
	}
	want := map[string]string{"sound": "80"}
	if !reflect.DeepEqual(got.Settings, want) {
		t.Errorf("Settings = %#v, want %#v", got.Settings, want)
	}
}

// TestRepeatedElementOps repeated 元素的 Set/Get/Del/Append 操作。
func TestRepeatedElementOps(t *testing.T) {
	conn := dialRedis(t)
	t.Cleanup(func() { conn.Do("DEL", fmt.Sprintf("REDB#%d:12:0", testREDBKey)) })

	u := &cmddb.UserBaseInfo{}
	if err := u.SetFriends(conn, testREDBKey, 12, 0, 0, "alice"); err != nil {
		t.Fatalf("SetFriends(0): %v", err)
	}
	idx, err := u.AppendFriends(conn, testREDBKey, 12, 0, "bob")
	if err != nil || idx != 1 {
		t.Fatalf("AppendFriends = %d, %v; want 1, nil", idx, err)
	}

	// 单下标读取
	if v, ok, err := u.GetFriends(conn, testREDBKey, 12, 0, 0); err != nil || !ok || v != "alice" {
		t.Errorf("GetFriends(0) = %q, %v, %v; want alice, true, nil", v, ok, err)
	}
	// 不存在的下标
	if v, ok, err := u.GetFriends(conn, testREDBKey, 12, 0, 99); err != nil || ok || v != "" {
		t.Errorf("GetFriends(99) = %q, %v, %v; want 零值, false, nil", v, ok, err)
	}

	// 删除下标 0（留下空洞），GetAll 应只剩 bob 且下标自动按存储扫描
	deleted, err := u.DelFriends(conn, testREDBKey, 12, 0, 0)
	if err != nil || !deleted {
		t.Fatalf("DelFriends(0) = %v, %v; want true, nil", deleted, err)
	}
	got := &cmddb.UserBaseInfo{}
	if err := got.GetFriendsAll(conn, testREDBKey, 12, 0); err != nil {
		t.Fatalf("GetFriendsAll: %v", err)
	}
	if want := []string{"bob"}; !reflect.DeepEqual(got.Friends, want) {
		t.Errorf("Friends = %#v, want %#v", got.Friends, want)
	}

	// 删除后再 Append：从最大下标 + 1 继续（空洞保留）
	if idx, err := u.AppendFriends(conn, testREDBKey, 12, 0, "carol"); err != nil || idx != 2 {
		t.Errorf("AppendFriends(after del) = %d, %v; want 2, nil", idx, err)
	}
	if err := got.GetFriendsAll(conn, testREDBKey, 12, 0); err != nil {
		t.Fatalf("GetFriendsAll: %v", err)
	}
	if want := []string{"bob", "carol"}; !reflect.DeepEqual(got.Friends, want) {
		t.Errorf("Friends = %#v, want %#v", got.Friends, want)
	}
}

// TestCollectionReplaceAll 整体替换语义：旧元素必须被清除（map 与 repeated）。
func TestCollectionReplaceAll(t *testing.T) {
	conn := dialRedis(t)
	t.Cleanup(func() { conn.Do("DEL", fmt.Sprintf("REDB#%d:13:0", testREDBKey)) })

	u := &cmddb.UserBaseInfo{}
	if err := u.SetSettingsAll(conn, testREDBKey, 13, 0, map[string]string{"a": "1", "b": "2"}); err != nil {
		t.Fatalf("SetSettingsAll(1): %v", err)
	}
	// 第二次整体替换，旧键 a 必须消失
	if err := u.SetSettingsAll(conn, testREDBKey, 13, 0, map[string]string{"b": "22", "c": "3"}); err != nil {
		t.Fatalf("SetSettingsAll(2): %v", err)
	}
	got := &cmddb.UserBaseInfo{}
	if err := got.GetSettingsAll(conn, testREDBKey, 13, 0); err != nil {
		t.Fatalf("GetSettingsAll: %v", err)
	}
	if want := map[string]string{"b": "22", "c": "3"}; !reflect.DeepEqual(got.Settings, want) {
		t.Errorf("Settings = %#v, want %#v（旧键应被清除）", got.Settings, want)
	}

	// repeated：替换会重建下标 0..n-1，可修复删除留下的空洞
	if err := u.SetFriends(conn, testREDBKey, 13, 0, 5, "x"); err != nil {
		t.Fatalf("SetFriends(5): %v", err)
	}
	if err := u.SetFriendsAll(conn, testREDBKey, 13, 0, []string{"x", "y", "z"}); err != nil {
		t.Fatalf("SetFriendsAll: %v", err)
	}
	if err := got.GetFriendsAll(conn, testREDBKey, 13, 0); err != nil {
		t.Fatalf("GetFriendsAll: %v", err)
	}
	if want := []string{"x", "y", "z"}; !reflect.DeepEqual(got.Friends, want) {
		t.Errorf("Friends = %#v, want %#v（下标应重建为 0..n-1）", got.Friends, want)
	}

	// DelAll 清空
	if err := u.DelSettingsAll(conn, testREDBKey, 13, 0); err != nil {
		t.Fatalf("DelSettingsAll: %v", err)
	}
	if err := got.GetSettingsAll(conn, testREDBKey, 13, 0); err != nil {
		t.Fatalf("GetSettingsAll: %v", err)
	}
	if got.Settings != nil {
		t.Errorf("DelAll 后 Settings = %#v, want nil", got.Settings)
	}
}

// TestMessageElementOps message 元素的单元素 protobuf 序列化往返。
func TestMessageElementOps(t *testing.T) {
	conn := dialRedis(t)
	t.Cleanup(func() { conn.Do("DEL", fmt.Sprintf("REDB#%d:14:0", testREDBKey)) })

	u := &cmddb.UserBaseInfo{}
	w := cmddb.Weapon{Name: "sword", Damage: 10, Element: "fire"}
	if err := u.SetWeaponMap(conn, testREDBKey, 14, 0, 7, w); err != nil {
		t.Fatalf("SetWeaponMap: %v", err)
	}
	gotW, ok, err := u.GetWeaponMap(conn, testREDBKey, 14, 0, 7)
	if err != nil || !ok || !reflect.DeepEqual(gotW, w) {
		t.Errorf("GetWeaponMap(7) = %#v, %v, %v; want %#v, true, nil", gotW, ok, err, w)
	}

	idx, err := u.AppendWeapons(conn, testREDBKey, 14, 0, w)
	if err != nil || idx != 0 {
		t.Fatalf("AppendWeapons = %d, %v; want 0, nil", idx, err)
	}
	got := &cmddb.UserBaseInfo{}
	if err := got.GetWeaponsAll(conn, testREDBKey, 14, 0); err != nil {
		t.Fatalf("GetWeaponsAll: %v", err)
	}
	if want := []cmddb.Weapon{w}; !reflect.DeepEqual(got.Weapons, want) {
		t.Errorf("Weapons = %#v, want %#v", got.Weapons, want)
	}
}

// TestElementAndBulkCoexist 元素级写入与 GetFields/SetFields 整体读写互通。
func TestElementAndBulkCoexist(t *testing.T) {
	conn := dialRedis(t)
	t.Cleanup(func() { conn.Do("DEL", fmt.Sprintf("REDB#%d:15:0", testREDBKey)) })

	// 元素级写入后，GetFields 整体读回可见
	u := &cmddb.UserBaseInfo{}
	if err := u.SetSettings(conn, testREDBKey, 15, 0, "sound", "80"); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	got := &cmddb.UserBaseInfo{}
	if err := got.GetFields(conn, testREDBKey, 15, 0, cmddb.FieldUserBaseInfo_Settings); err != nil {
		t.Fatalf("GetFields(Settings): %v", err)
	}
	if want := map[string]string{"sound": "80"}; !reflect.DeepEqual(got.Settings, want) {
		t.Errorf("Settings = %#v, want %#v", got.Settings, want)
	}

	// SetFields 整体写入后，元素级读取可见
	u2 := &cmddb.UserBaseInfo{Friends: []string{"f1", "f2"}}
	if err := u2.SetFields(conn, testREDBKey, 15, 0, cmddb.FieldUserBaseInfo_Friends); err != nil {
		t.Fatalf("SetFields(Friends): %v", err)
	}
	if v, ok, err := u2.GetFriends(conn, testREDBKey, 15, 0, 1); err != nil || !ok || v != "f2" {
		t.Errorf("GetFriends(1) = %q, %v, %v; want f2, true, nil", v, ok, err)
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
	// Weapon{name:"fire", damage:1, element:"e"}
	// 0A 04 66 69 72 65 | 10 01 | 1A 01 65
	w := cmddb.Weapon{Name: "fire", Damage: 1, Element: "e"}
	got, err := w.MarshalRedisProto()
	if err != nil {
		t.Fatalf("Weapon.MarshalRedisProto: %v", err)
	}
	want := []byte{0x0A, 0x04, 'f', 'i', 'r', 'e', 0x10, 0x01, 0x1A, 0x01, 'e'}
	if !bytes.Equal(got, want) {
		t.Errorf("Weapon 编码 = % X, want % X", got, want)
	}

	// 零值 message 应编码为空字节流（proto3 跳过零值标量/空字符串）
	empty, err := (&cmddb.Weapon{}).MarshalRedisProto()
	if err != nil {
		t.Fatalf("空 Weapon.MarshalRedisProto: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("零值 Weapon 编码 = % X, want 空", empty)
	}

	u := &cmddb.UserBaseInfo{
		UserId:   7,                       // tag 1  varint
		Username: "ab",                    // tag 2  string
		Gender:   cmddb.Gender_GENDER_MALE, // tag 4  枚举 varint
		Balance:  3.5,                     // tag 7  float -> fixed32
		Friends:  []string{"x"},           // tag 8  repeated string
		Settings: map[string]string{"k": "v"}, // tag 9  map 条目
		Weapon:   cmddb.Weapon{Name: "w", Damage: 2}, // tag 13 message（element 空串不编码）
		Coin:     1,                       // tag 15 uint32
		Vip:      true,                    // tag 17 bool
		Token:    []byte{0xFF},            // tag 19 bytes
		// Profile 为零值但 message 字段恒编码 -> 空子消息
	}
	got, err = u.MarshalRedisProto()
	if err != nil {
		t.Fatalf("UserBaseInfo.MarshalRedisProto: %v", err)
	}
	want = []byte{
		0x08, 0x07,                      // UserId = 7
		0x12, 0x02, 'a', 'b',            // Username = "ab"
		0x20, 0x01,                      // Gender = GENDER_MALE
		0x3D, 0x00, 0x00, 0x60, 0x40,    // Balance = 3.5f（小端 fixed32）
		0x42, 0x01, 'x',                 // Friends = ["x"]
		0x4A, 0x06, 0x0A, 0x01, 'k', 0x12, 0x01, 'v', // Settings 条目 {1:"k", 2:"v"}
		0x6A, 0x05, 0x0A, 0x01, 'w', 0x10, 0x02, // Weapon{name:"w", damage:2}
		0x78, 0x01,                      // Coin = 1
		0x88, 0x01, 0x01,                // Vip = true
		0x9A, 0x01, 0x01, 0xFF,          // Token = [0xFF]
		0xA2, 0x01, 0x00,                // Profile = 空消息（恒编码）
	}
	if !bytes.Equal(got, want) {
		t.Errorf("UserBaseInfo 编码 = % X\nwant           = % X", got, want)
	}

	// 负 int32/int64 必须符号扩展为 10 字节 varint（与 protoc 一致）
	neg := &cmddb.UserBaseInfo{UserId: -1, Exp: -2}
	got, err = neg.MarshalRedisProto()
	if err != nil {
		t.Fatalf("负值 MarshalRedisProto: %v", err)
	}
	want = []byte{
		0x08, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x01, // int32(-1)
		0x30, 0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x01, // int64(-2)
		0x6A, 0x00,                      // Weapon = 空消息（message 字段恒编码）
		0xA2, 0x01, 0x00,                // Profile = 空消息（message 字段恒编码）
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
		0x82, 0x06, 0x02, 'a', 'b',       // 未知字段 96（wire 2）-> 跳过
		0x89, 0x06, 0x01, 0, 0, 0, 0, 0, 0, 0, // 未知字段 97（wire 1 fixed64）-> 跳过
		0x95, 0x06, 0xDE, 0xAD, 0xBE, 0xEF, // 未知字段 98（wire 5 fixed32）-> 跳过
		0x98, 0x06, 0x05,                // 未知字段 99（wire 0 varint）-> 跳过
		0x08, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x01, // UserId = -1
		0x30, 0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x01, // Exp = -2
		0x5A, 0x03, 0x01, 0x02, 0x03,    // Listint32 packed [1,2,3]（其他语言默认打包）
		0x42, 0x02, 'x', 'y',            // Friends 追加 "xy"
	}
	u := &cmddb.UserBaseInfo{Friends: []string{"stale"}} // 反序列化前会被重置
	if err := u.UnmarshalRedisProto(b); err != nil {
		t.Fatalf("UnmarshalRedisProto: %v", err)
	}
	if u.UserId != -1 || u.Exp != -2 {
		t.Errorf("负值回读 = %d, %d, want -1, -2", u.UserId, u.Exp)
	}
	if want := []int32{1, 2, 3}; !reflect.DeepEqual(u.Listint32, want) {
		t.Errorf("Listint32 = %v, want %v", u.Listint32, want)
	}
	if want := []string{"xy"}; !reflect.DeepEqual(u.Friends, want) {
		t.Errorf("Friends = %v, want %v", u.Friends, want)
	}
	if u.Username != "" {
		t.Errorf("重置失败: Username = %q, want 空", u.Username)
	}

	// wire type 不匹配应报错
	if err := (&cmddb.UserBaseInfo{}).UnmarshalRedisProto([]byte{0x0A, 0x01, 0x00}); err == nil {
		t.Error("字段 1 用 wire 2 编码应报错（期望 varint）")
	}
	// 截断的 varint 应报错
	if err := (&cmddb.UserBaseInfo{}).UnmarshalRedisProto([]byte{0x08, 0x80}); err == nil {
		t.Error("截断的 varint 应报错")
	}
	// 截断的 length-delimited 应报错
	if err := (&cmddb.UserBaseInfo{}).UnmarshalRedisProto([]byte{0x12, 0x05, 'a'}); err == nil {
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
	u := &cmddb.UserBaseInfo{}
	if err := u.UnmarshalRedisProto(got); err != nil {
		t.Fatalf("UnmarshalRedisProto: %v", err)
	}
	if !reflect.DeepEqual(u, want) {
		t.Errorf("protobuf 往返不一致:\n got = %#v\nwant = %#v", u, want)
	}
}
