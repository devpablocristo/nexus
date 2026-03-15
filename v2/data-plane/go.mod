module nexus/v2/data-plane

go 1.26.0

require (
	github.com/alicebob/miniredis/v2 v2.37.0
	github.com/google/cel-go v0.27.0
	github.com/google/uuid v1.6.0
	github.com/redis/go-redis/v9 v9.18.0
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1
	github.com/devpablocristo/nexus/v2/pkgs/go-pkg v0.0.0
)

replace github.com/devpablocristo/nexus/v2/pkgs/go-pkg => ../pkgs/go-pkg

require (
	cel.dev/expr v0.25.1 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20240823005443-9b4947da3948 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240826202546-f6391c0de4c7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240826202546-f6391c0de4c7 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)
