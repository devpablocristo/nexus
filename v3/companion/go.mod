module github.com/devpablocristo/nexus/v3/companion

go 1.26.1

require (
	github.com/devpablocristo/core/backend/go v0.0.0
	github.com/devpablocristo/core/databases/postgres/go v0.0.0
	github.com/devpablocristo/core/ai/go v0.0.0
	github.com/devpablocristo/core/governance/go v0.0.0
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.8.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	google.golang.org/protobuf v1.36.8 // indirect
)

replace github.com/devpablocristo/core/backend/go => ../../../core/backend/go

replace github.com/devpablocristo/core/databases/postgres/go => ../../../core/databases/postgres/go

replace github.com/devpablocristo/core/ai/go => ../../../core/ai/go

replace github.com/devpablocristo/core/governance/go => ../../../core/governance/go
