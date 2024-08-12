protoc --go_out=internal/ .\protoschemas\mymessage.proto
go build -o server.exe ./cmd/server
go build -o client.exe ./cmd/client
go build -o master.exe ./cmd/master
