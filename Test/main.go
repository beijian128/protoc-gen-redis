package main

import (
	"encoding/json"
	"flag"
	"fmt"
	cmddb "github.com/beijian128/protoc-gen-redis/generated"
	"github.com/gomodule/redigo/redis"
	"log"
	"os"
)

var configPath = flag.String("config", "bin/config.json", "Path to config file")

type RedisCfg struct {
	Address  string `json:"address"`
	Password string `json:"password"`
}
type Config struct {
	RedisCfg RedisCfg `json:"redis"`
}

func main() {
	flag.Parse()

	configFile, err := os.Open(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	defer configFile.Close()
	decoder := json.NewDecoder(configFile)
	var config Config
	err = decoder.Decode(&config)
	if err != nil {
		log.Fatal(err)
	}

	conn, err := redis.Dial("tcp", config.RedisCfg.Address,
		redis.DialPassword(config.RedisCfg.Password))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// 演示：全字段写入 + 全字段读取（覆盖所有类型：标量/枚举/bytes/包裹集合/嵌套）
	u := &cmddb.DBUserBaseInfo{
		UserId:      133333333,
		Username:    "demo",
		AvatarUrl:   "https://example.com/avatar.png",
		Gender:      cmddb.Gender_GENDER_FEMALE,
		Level:       4,
		Exp:         3,
		Balance:     4.5,
		Friends:     cmddb.DBUserBaseInfo_DBFriends{Items: []string{"1", "2"}},
		Settings:    cmddb.DBUserBaseInfo_DBSettings{Kv: map[string]string{"key": "value"}},
		LoginSource: cmddb.LoginSource_SOURCE_H5,
		Int32List:   cmddb.DBUserBaseInfo_DBInt32List{Items: []int32{1, 2}},
		Weapons: cmddb.DBUserBaseInfo_DBWeapons{Items: []cmddb.DBWeapon{
			{Name: "mmm1", Damage: 222, Element: "ele"},
		}},
		Weapon: cmddb.DBWeapon{Name: "1", Damage: 222, Element: "mmmele"},
		WeaponMap: cmddb.DBUserBaseInfo_DBWeaponMap{Items: map[int32]cmddb.DBWeapon{
			1: {Name: "mmmm1", Damage: 222, Element: "mmmele"},
			2: {Name: "2", Damage: 233, Element: "mmmmele"},
		}},
		Coin:     100,
		Gem:      99999,
		Vip:      true,
		Score:    9.75,
		Token:    []byte{0x01, 0x02, 0x03},
		Profile:  cmddb.DBUserBaseInfo_DBProfile{Nickname: "demo-nick", Age: 18},
		VipLevel: cmddb.DBUserBaseInfo_VIP_1,
	}
	if err := u.SetFields(conn, 2, 1, 0); err != nil {
		log.Fatalf("SetFields 失败: %v", err)
	}

	got := &cmddb.DBUserBaseInfo{}
	if err := got.GetFields(conn, 2, 1, 0); err != nil {
		log.Fatalf("GetFields 失败: %v", err)
	}
	fmt.Printf("回读结果: %#v\n", got)

	// 演示：按需读写（只操作指定字段）
	u.Level = 10
	if err := u.SetFields(conn, 2, 1, 0, cmddb.FieldDBUserBaseInfo_Level); err != nil {
		log.Fatalf("SetFields(局部) 失败: %v", err)
	}
	got2 := &cmddb.DBUserBaseInfo{}
	if err := got2.GetFields(conn, 2, 1, 0, cmddb.FieldDBUserBaseInfo_Level); err != nil {
		log.Fatalf("GetFields(局部) 失败: %v", err)
	}
	fmt.Printf("局部读取 Level: %#v\n", got2.Level)

	// 演示：集合字段整体读写（message 包裹，一个 hash field 存整个集合的 protobuf 字节）
	u.Friends = cmddb.DBUserBaseInfo_DBFriends{Items: []string{"alice", "bob"}}
	u.WeaponMap = cmddb.DBUserBaseInfo_DBWeaponMap{Items: map[int32]cmddb.DBWeapon{
		3: {Name: "w3", Damage: 33, Element: "ice"},
	}}
	if err := u.SetFields(conn, 2, 1, 0, cmddb.FieldDBUserBaseInfo_Friends, cmddb.FieldDBUserBaseInfo_WeaponMap); err != nil {
		log.Fatalf("SetFields(集合字段) 失败: %v", err)
	}
	got3 := &cmddb.DBUserBaseInfo{}
	if err := got3.GetFields(conn, 2, 1, 0, cmddb.FieldDBUserBaseInfo_Friends, cmddb.FieldDBUserBaseInfo_WeaponMap); err != nil {
		log.Fatalf("GetFields(集合字段) 失败: %v", err)
	}
	fmt.Printf("集合字段回读: Friends=%#v WeaponMap=%#v\n", got3.Friends, got3.WeaponMap)
}
