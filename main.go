package main

import (
	"bytes"
	"fmt"
	"github.com/beijian128/protoc-gen-redis/generator"

	"google.golang.org/protobuf/compiler/protogen"
)

func main() {
	protogen.Options{}.Run(func(gen *protogen.Plugin) error {
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}

			// ✅ 只为每个 proto 文件生成一个总的 Redis 代码文件
			var allCode bytes.Buffer
			{
				code, err := generator.GenerateRedisCodeHead(gen, f)
				if err != nil {
					return fmt.Errorf("生成的Pkg head代码失败: %v", err)
				}

				allCode.Write(code)
				allCode.WriteString("\n\n")
			}
			// 遍历该 proto 文件中的所有 message
			for _, msg := range f.Messages {
				// 为每个 message 生成代码
				code, err := generator.GenerateRedisCode(gen, f, msg)
				if err != nil {
					return fmt.Errorf("生成 message %s 的 Redis 代码失败: %v", msg.Desc.Name(), err)
				}

				// 将每个 message 的代码追加到总代码中
				allCode.WriteString(string("// --- Message: " + msg.Desc.Name() + " ---\n"))
				allCode.Write(code)
				allCode.WriteString("\n\n")
			}

			// ✅ 最终输出成一个文件，如：example_redis_gen.go
			filename := fmt.Sprintf("%s.redis.go", f.Desc.Name()) // 如 example_redis_gen.go
			g := gen.NewGeneratedFile(filename, f.GoImportPath)
			if _, err := g.Write(allCode.Bytes()); err != nil {
				return fmt.Errorf("写入生成文件失败: %v", err)
			}
		}
		return nil
	})
}
