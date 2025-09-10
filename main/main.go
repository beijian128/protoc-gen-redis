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

	//{
	//	u := cmddb.NewUser2()
	//	u.Id2 = 1234
	//	u.Name2 = "Mikexx23xx"
	//	u.List = []int32{1, 2, 3, 4, 5}
	//	u.Mp = make(map[uint32]int32)
	//	u.Mp[1] = 1
	//	u.Mp[2] = 2
	//	err = u.SetFields(conn, 2, 1, 0)
	//	if err != nil {
	//		panic(err)
	//	}
	//}

	{
		u := cmddb.NewUser2()
		err = u.GetFields(conn, 2, 1, 0)
		if err != nil {
			panic(err)
		}
		data, _ := json.Marshal(u)
		fmt.Println(string(data))
	}

}
