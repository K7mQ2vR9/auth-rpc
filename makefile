
code:
	sh ./dsl/main.sh
	# goctl根据go.mod中的module authRpc生成项目内导入路径。
	goctl rpc protoc dsl/auth_rpc.proto --go_out=./  --go-grpc_out=./  --zrpc_out=./ --client=true --multiple=true --style=go_zero

	cp -R authRpc ../rpc-client/auth-rpc/ && \
	cp -R client ../rpc-client/auth-rpc/ && \
	find ../rpc-client/auth-rpc/client/ -type f -exec sed -i '' 's/"authRpc\/authRpc"/"gitlab.888bbm.com\/go-package\/rpc-client\/auth-rpc\/authRpc"/g' {} +

bb:
	GOOS=linux GOARCH=amd64 go build -o authRpcxxx
