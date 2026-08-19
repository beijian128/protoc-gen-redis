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

	// 演示：全字段写入 + 全字段读取（覆盖所有类型：标量/枚举/bytes/repeated/map/嵌套）
	u := &cmddb.UserBaseInfo{
		UserId:      133333333,
		Username:    "demo",
		AvatarUrl:   "https://example.com/avatar.png",
		Gender:      cmddb.Gender_GENDER_FEMALE,
		Level:       4,
		Exp:         3,
		Balance:     4.5,
		Friends:     []string{"1", "2"},
		Settings:    map[string]string{"key": "value"},
		LoginSource: cmddb.LoginSource_SOURCE_H5,
		Listint32:   []int32{1, 2},
		Weapons: []cmddb.Weapon{
			{Name: "mmm1", Damage: 222, Element: "ele"},
		},
		Weapon: cmddb.Weapon{Name: "1", Damage: 222, Element: "mmmele"},
		WeaponMap: map[int32]cmddb.Weapon{
			1: {Name: "mmmm1", Damage: 222, Element: "mmmele"},
			2: {Name: "2", Damage: 233, Element: "mmmmele"},
		},
		Coin:     100,
		Gem:      99999,
		Vip:      true,
		Score:    9.75,
		Token:    []byte{0x01, 0x02, 0x03},
		Profile:  cmddb.UserBaseInfo_Profile{Nickname: "demo-nick", Age: 18},
		VipLevel: cmddb.UserBaseInfo_VIP_1,
	}
	if err := u.SetFields(conn, 2, 1, 0); err != nil {
		log.Fatalf("SetFields 失败: %v", err)
	}

	got := &cmddb.UserBaseInfo{}
	if err := got.GetFields(conn, 2, 1, 0); err != nil {
		log.Fatalf("GetFields 失败: %v", err)
	}
	fmt.Printf("回读结果: %#v\n", got)

	// 演示：按需读写（只操作指定字段）
	u.Level = 10
	if err := u.SetFields(conn, 2, 1, 0, cmddb.FieldUserBaseInfo_Level); err != nil {
		log.Fatalf("SetFields(局部) 失败: %v", err)
	}
	got2 := &cmddb.UserBaseInfo{}
	if err := got2.GetFields(conn, 2, 1, 0, cmddb.FieldUserBaseInfo_Level); err != nil {
		log.Fatalf("GetFields(局部) 失败: %v", err)
	}
	fmt.Printf("局部读取 Level: %#v\n", got2.Level)

	// 演示：集合字段元素级操作（只读写单个元素，不触碰其他元素）
	if err := u.SetSettings(conn, 2, 1, 0, "sound", "80"); err != nil {
		log.Fatalf("SetSettings 失败: %v", err)
	}
	if v, ok, err := u.GetSettings(conn, 2, 1, 0, "sound"); err != nil {
		log.Fatalf("GetSettings 失败: %v", err)
	} else {
		fmt.Printf("元素级读取 settings[sound]: %q (存在=%v)\n", v, ok)
	}
	if idx, err := u.AppendFriends(conn, 2, 1, 0, "carol"); err != nil {
		log.Fatalf("AppendFriends 失败: %v", err)
	} else {
		fmt.Printf("AppendFriends 新下标: %d\n", idx)
	}
	if err := u.SetWeaponMap(conn, 2, 1, 0, 3, cmddb.Weapon{Name: "w3", Damage: 33, Element: "ice"}); err != nil {
		log.Fatalf("SetWeaponMap 失败: %v", err)
	}
	if w, ok, err := u.GetWeaponMap(conn, 2, 1, 0, 3); err != nil {
		log.Fatalf("GetWeaponMap 失败: %v", err)
	} else {
		fmt.Printf("元素级读取 weaponMap[3]: %#v (存在=%v)\n", w, ok)
	}
}
