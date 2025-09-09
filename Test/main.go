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

	{
		u := cmddb.UserBaseInfo{
			User_id:      133333333,
			Username:     "3",
			Avatar_url:   "sdw",
			Gender:       cmddb.Gender_GENDER_FEMALE,
			Level:        4,
			Exp:          3,
			Balance:      4,
			Friends:      []string{"1", "2"},
			Settings:     map[string]string{"key": "value"},
			Login_source: cmddb.LoginSource_SOURCE_H5,
			Listint32:    []int32{1, 2},
			Weapons: []cmddb.Weapon{
				{
					Name:    "mmm1",
					Damage:  222,
					Element: "ele",
				},
			},
			Weapon: cmddb.Weapon{Name: "1",
				Damage:  222,
				Element: "mmmele",
			},
			WeaponMap: map[int32]cmddb.Weapon{
				1: {Name: "mmmm1",
					Damage:  222,
					Element: "mmmele"},
				2: {Name: "2", Damage: 233, Element: "mmmmele"},
			},
		}
		u.SetFields(conn, uint32(2), 1, 0,
			cmddb.FieldUserBaseInfo_WeaponMap, cmddb.FieldUserBaseInfo_Weapons,
			cmddb.FieldUserBaseInfo_Weapon, cmddb.FieldUserBaseInfo_Gender, cmddb.FieldUserBaseInfo_User_id)
	}
	{
		u := cmddb.UserBaseInfo{}
		u.GetFields(conn, 2, 1, 0, cmddb.FieldUserBaseInfo_WeaponMap, cmddb.FieldUserBaseInfo_Weapons,
			cmddb.FieldUserBaseInfo_Weapon, cmddb.FieldUserBaseInfo_Gender, cmddb.FieldUserBaseInfo_User_id)
		fmt.Printf("%#v\n", u)
	}
}
