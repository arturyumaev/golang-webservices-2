rm ../api_handlers.go
rm codegen
go build codegen.go
./codegen ../api.go ../api_handlers.go

# cd ..
# echo '\n=== Testing... ===\n'
# go test -v