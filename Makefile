.PHONY: compile-proto

compile-proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go_opt=Mcable/grpc/proto/service.proto=github.com/foundation-go/foundation/cable/grpc \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		--go-grpc_opt=Mcable/grpc/proto/service.proto=github.com/foundation-go/foundation/cable/grpc \
		./cable/grpc/proto/service.proto

	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		./errors/proto/errors.proto

	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		./examples/clubchat/protos/**/*.proto

	protoc -I . --grpc-gateway_out=. \
		--grpc-gateway_opt logtostderr=true \
		--grpc-gateway_opt paths=source_relative \
		--grpc-gateway_opt generate_unbound_methods=true \
		--openapiv2_opt omit_enum_default_value=true \
		--openapiv2_opt generate_unbound_methods=true \
		--openapiv2_opt allow_merge=true \
		--openapiv2_opt merge_file_name=./examples/clubchat/api \
		--openapiv2_out=. \
		./examples/clubchat/protos/**/service.proto
