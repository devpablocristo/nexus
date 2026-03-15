module nexus/v2/control-plane

go 1.26.0

require (
	github.com/google/cel-go v0.27.0
	github.com/google/uuid v1.6.0
	github.com/devpablocristo/nexus/v2/pkgs/go-pkg v0.0.0
)

replace github.com/devpablocristo/nexus/v2/pkgs/go-pkg => ../pkgs/go-pkg

require (
	cel.dev/expr v0.25.1 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	golang.org/x/exp v0.0.0-20240823005443-9b4947da3948 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240826202546-f6391c0de4c7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240826202546-f6391c0de4c7 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)
