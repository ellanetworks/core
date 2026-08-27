module github.com/ellanetworks/core

go 1.26.5

require (
	github.com/canonical/sqlair v0.0.0-20250120155751-a83645b9a121
	github.com/cilium/ebpf v0.22.0
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/gopacket v1.1.19
	github.com/google/nftables v0.3.0
	github.com/hashicorp/go-hclog v1.6.3
	github.com/hashicorp/raft v1.7.3
	github.com/hashicorp/raft-autopilot v0.3.0
	github.com/hashicorp/raft-boltdb/v2 v2.3.1
	github.com/mattn/go-sqlite3 v1.14.42
	github.com/moby/moby/client v0.5.1
	github.com/osrg/gobgp/v4 v4.8.0
	github.com/prometheus/client_golang v1.24.1
	github.com/songgao/water v0.0.0-20200317203138-2b4b6d7c09d8
	github.com/spf13/cobra v1.10.2
	github.com/vishvananda/netlink v1.3.1
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.70.0
	go.opentelemetry.io/otel v1.45.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.45.0
	go.opentelemetry.io/otel/sdk v1.45.0
	go.uber.org/zap v1.28.0
	golang.org/x/crypto v0.55.0
	golang.org/x/net v0.58.0
	golang.org/x/sys v0.47.0
	golang.org/x/tools v0.49.0
	gopkg.in/yaml.v2 v2.4.0
)

require go.yaml.in/yaml/v3 v3.0.5 // indirect

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/armon/go-metrics v0.4.1
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/boltdb/bolt v1.3.1 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/dgryski/go-farm v0.0.0-20240924180020-3414d57e47da // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/go-connections v0.7.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/eapache/channels v1.1.0 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/ellanetworks/core/lppa v0.0.0-00010101000000-000000000000
	github.com/ellanetworks/core/nas v0.0.0-00010101000000-000000000000
	github.com/ellanetworks/core/ngap v0.0.0-00010101000000-000000000000
	github.com/ellanetworks/core/nrppa v0.0.0-00010101000000-000000000000
	github.com/ellanetworks/core/per v0.0.0-00010101000000-000000000000
	github.com/ellanetworks/core/s1ap v0.0.0-00010101000000-000000000000
	github.com/fatih/color v1.13.0 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/gaissmai/bart v0.26.1 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/uuid v1.6.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.29.0 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.0.0 // indirect
	github.com/hashicorp/go-metrics v0.5.4 // indirect
	github.com/hashicorp/go-msgpack/v2 v2.1.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/golang-lru v0.5.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/k-sone/critbitgo v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mdlayher/netlink v1.7.3-0.20250113171957-fbb4dce95f42 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/moby/api v1.55.0
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/orcaman/concurrent-map/v2 v2.0.1 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/prometheus/client_model v0.6.2
	github.com/prometheus/common v0.70.1 // indirect
	github.com/prometheus/procfs v0.21.1 // indirect
	github.com/sagikazarmark/locafero v0.7.0 // indirect
	github.com/segmentio/fasthash v1.0.3 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.12.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/spf13/pflag v1.0.10
	github.com/spf13/viper v1.20.1 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/stretchr/testify v1.12.1
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/vishvananda/netns v0.0.5 // indirect
	go.etcd.io/bbolt v1.3.5 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.45.0 // indirect
	go.opentelemetry.io/otel/metric v1.45.0 // indirect
	go.opentelemetry.io/otel/trace v1.45.0
	go.opentelemetry.io/proto/otlp v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/mod v0.39.0 // indirect
	golang.org/x/sync v0.22.0
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260803160001-6ac0973c030d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260803160001-6ac0973c030d // indirect
	google.golang.org/grpc v1.83.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1
)

// Fork of mattn/go-sqlite3 that exposes the SQLite session extension
// (sqlite3session_* / sqlite3changeset_apply) behind the sqlite_session
// build tag. Used by internal/raft to replicate write-set changesets
// rather than typed commands.
replace github.com/mattn/go-sqlite3 => github.com/ellanetworks/go-sqlite3 v0.0.0-20260827163036-c5b342dd0d66

replace github.com/ellanetworks/core/lppa => ./lppa

replace github.com/ellanetworks/core/s1ap => ./s1ap

replace github.com/ellanetworks/core/nas => ./nas

replace github.com/ellanetworks/core/ngap => ./ngap

replace github.com/ellanetworks/core/nrppa => ./nrppa

replace github.com/ellanetworks/core/per => ./per
