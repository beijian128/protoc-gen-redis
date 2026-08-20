package generator

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ValidateConventions 校验一个 proto 文件中的 message 定义是否符合项目约定：
//
//  1. 所有 message（顶层与嵌套，map entry 除外）名称必须以 "DB" 前缀开头；
//  2. 顶层 message 的字段不能直接定义 repeated / map，集合字段必须用嵌套 message 包起来。
//
// 返回的 error 会作为插件错误上报给 protoc，使编译失败并提示违规位置。
func ValidateConventions(file *protogen.File) error {
	for _, m := range CollectMessages(file) {
		name := string(m.Desc.Name())
		if !strings.HasPrefix(name, "DB") {
			return fmt.Errorf("message %q 必须以 DB 前缀开头（约定），建议改为 %q", name, "DB"+name)
		}
	}
	for _, m := range file.Messages {
		if m.Desc.IsMapEntry() {
			continue
		}
		for _, f := range m.Fields {
			if f.Desc.Cardinality() == protoreflect.Repeated {
				return fmt.Errorf(
					"顶层 message %q 的字段 %q 不能直接定义 repeated/map（约定），集合字段必须用嵌套 message 包起来，如 message DB%s { repeated ... items = 1; }",
					m.Desc.Name(), f.Desc.Name(), goCamelCase(string(f.Desc.Name())))
			}
		}
	}
	return nil
}
