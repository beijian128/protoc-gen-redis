
 go build -o protoc-gen-redis.exe .
 protoc --plugin=./protoc-gen-redis.exe --redis_out=./generated proto/user.proto