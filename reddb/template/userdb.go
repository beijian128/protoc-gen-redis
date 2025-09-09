package template

import (
	"fmt"
	cmddb "github.com/beijian128/protoc-gen-redis/proto"
	"github.com/gomodule/redigo/redis"
	"strconv"
)

type FieldUserDB uint32

const (
	FieldUserDB_Id   FieldUserDB = 1
	FieldUserDB_Name FieldUserDB = 2
	FieldUserDB_Age  FieldUserDB = 3
)

type UserDB struct {
	cmddb.User
}

func NewUserDB() *UserDB {
	return &UserDB{}
}

func (p *UserDB) SetFields(conn redis.Conn, REDBKey cmddb.REDBKey, ida, idb uint64, fields ...FieldUserDB) error {
	key := fmt.Sprintf("REDB#%d:%d:%d", REDBKey, ida, idb)

	// 构造 HSET 的参数：key, field1, value1, field2, value2, ...
	args := []interface{}{key}
	for _, field := range fields {
		switch field {
		case FieldUserDB_Id:
			args = append(args, field, p.Id)
		case FieldUserDB_Name:
			args = append(args, field, p.Name)
		case FieldUserDB_Age:
			args = append(args, field, p.Age)
		}
	}

	// 一次性执行 HSET 多字段
	_, err := conn.Do("HSET", args...)
	return err
}

func (p *UserDB) GetFields(conn redis.Conn, REDBKey cmddb.REDBKey, ida, idb uint64, fields ...FieldUserDB) error {
	key := fmt.Sprintf("REDB#%d:%d:%d", REDBKey, ida, idb)

	// 1. 构造 HMGET 参数：key, field1, field2, ...
	args := []interface{}{key}
	for _, field := range fields {
		args = append(args, field)
	}

	// 2. 执行 HMGET
	reply, err := redis.Values(conn.Do("HMGET", args...))
	if err != nil {
		return err
	}

	// 3. 按顺序解析每个字段的返回值
	for i, field := range fields {
		val, err := redis.String(reply[i], nil)
		if err != nil && err != redis.ErrNil {
			return fmt.Errorf("解析字段 %v 失败: %v", field, err)
		}

		if val == "" && err == redis.ErrNil {
			// 字段不存在，可以跳过或设置零值
			continue
		}

		switch field {
		case FieldUserDB_Id:
			id, err := strconv.ParseUint(val, 10, 64)
			if err != nil {
				return fmt.Errorf("解析 Id 失败: %v", err)
			}
			p.Id = id

		case FieldUserDB_Name:
			p.Name = val

		case FieldUserDB_Age:
			age, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return fmt.Errorf("解析 Age 失败: %v", err)
			}
			p.Age = uint32(age)

		default:
			return fmt.Errorf("未知字段: %v", field)
		}
	}

	return nil
}
