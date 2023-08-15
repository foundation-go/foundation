generate:
	PATH="$$PATH:$$(go env GOPATH)/bin" && \
		protoc --go_out=. --go_opt=paths=source_relative \
			--go_opt=Mcable/grpc/proto/service.proto=github.com/ri-nat/foundation/cable/grpc \
			--go-grpc_out=. --go-grpc_opt=paths=source_relative \
			--go-grpc_opt=Mcable/grpc/proto/service.proto=github.com/ri-nat/foundation/cable/grpc \
			./cable/grpc/proto/service.proto

.PHONY: generate
