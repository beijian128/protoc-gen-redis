package main

import (
	"fmt"
	"path"
	"strings"

	"github.com/beijian128/protoc-gen-redis/generator"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

func main() {
	keyFormat := generator.DefaultKeyFormat
	protogen.Options{
		ParamFunc: func(name, value string) error {
			switch name {
			case "key_format":
				keyFormat = value
				return nil
			default:
				return fmt.Errorf("unknown parameter %q", name)
			}
		},
	}.Run(func(gen *protogen.Plugin) error {
		return run(gen, keyFormat)
	})
}

// run 是插件主逻辑，独立出来便于测试。
func run(gen *protogen.Plugin, keyFormat string) error {
	gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)

	for _, f := range gen.Files {
		if !f.Generate {
			continue
		}

		// 每个 proto 文件生成一个总的 Redis 代码文件，如 user.redis.go
		g := gen.NewGeneratedFile(outputFilename(f, gen), f.GoImportPath)

		head, err := generator.GenerateRedisCodeHeadWithEnums(f)
		if err != nil {
			return fmt.Errorf("生成 %s 的包头/枚举代码失败: %v", f.Desc.Name(), err)
		}
		if _, err := g.Write(head); err != nil {
			return err
		}

		// 遍历该 proto 文件中的所有 message（含嵌套 message）
		for _, msg := range generator.CollectMessages(f) {
			code, err := generator.GenerateRedisCode(gen, f, msg, g, keyFormat)
			if err != nil {
				return fmt.Errorf("生成 message %s 的 Redis 代码失败: %v", msg.Desc.Name(), err)
			}
			if _, err := g.Write([]byte("\n// --- Message: " + string(msg.GoIdent.GoName) + " ---\n")); err != nil {
				return err
			}
			if _, err := g.Write(code); err != nil {
				return err
			}
			if _, err := g.Write([]byte("\n\n")); err != nil {
				return err
			}
		}
	}
	return nil
}

// outputFilename 计算生成文件的路径。
// 默认（paths=import 或未指定）输出到 --redis_out 根目录，文件名为 proto 文件基名；
// paths=source_relative 时按 proto 文件的源路径镜像输出（如 proto/user.proto -> proto/user.redis.go）。
func outputFilename(f *protogen.File, gen *protogen.Plugin) string {
	prefix := string(f.GeneratedFilenamePrefix)
	for _, param := range strings.Split(gen.Request.GetParameter(), ",") {
		if param == "paths=source_relative" {
			return prefix + ".redis.go"
		}
	}
	return path.Base(prefix) + ".redis.go"
}
