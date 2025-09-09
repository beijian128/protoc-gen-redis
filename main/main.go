package main

import (
	"encoding/json"
	"flag"
	"fmt"
	cmddb "github.com/beijian128/protoc-gen-redis/proto"
	"github.com/beijian128/protoc-gen-redis/reddb/template"
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
		u := template.NewUserDB()
		u.Id = 123
		u.Name = "Mike"
		err = u.SetFields(conn, cmddb.REDBKey_User, 1, 0, template.FieldUserDB_Id, template.FieldUserDB_Name)
		if err != nil {
			panic(err)
		}
	}
	{
		u := template.NewUserDB()
		err = u.GetFields(conn, cmddb.REDBKey_User, 1, 0, template.FieldUserDB_Id, template.FieldUserDB_Name)
		if err != nil {
			panic(err)
		}
		fmt.Println(u)
	}

}
