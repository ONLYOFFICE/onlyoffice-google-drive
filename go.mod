module github.com/ONLYOFFICE/onlyoffice-gdrive

go 1.19

require (
	github.com/go-chi/chi/v5 v5.0.8
	github.com/go-chi/cors v1.2.1
	github.com/go-micro/plugins/v4/broker/nats v1.2.0
	github.com/go-micro/plugins/v4/broker/rabbitmq v1.2.1
	github.com/go-micro/plugins/v4/registry/consul v1.2.0
	github.com/go-micro/plugins/v4/registry/etcd v1.2.0
	github.com/go-micro/plugins/v4/registry/kubernetes v1.1.1
	github.com/go-micro/plugins/v4/registry/mdns v1.2.0
	github.com/go-micro/plugins/v4/server/http v1.2.1
	github.com/go-micro/plugins/v4/wrapper/breaker/hystrix v1.2.0
	github.com/go-micro/plugins/v4/wrapper/ratelimiter/uber v1.2.0
	github.com/go-micro/plugins/v4/wrapper/select/roundrobin v1.2.0
	github.com/go-micro/plugins/v4/wrapper/trace/opentelemetry v1.2.0
	github.com/google/uuid v1.3.0
	github.com/gorilla/csrf v1.7.1
	github.com/gorilla/sessions v1.2.1
	github.com/hellofresh/health-go/v5 v5.0.0
	github.com/justinas/alice v1.2.0
	github.com/mitchellh/mapstructure v1.3.3
	github.com/olivere/elastic/v7 v7.0.32
	github.com/prometheus/client_golang v1.14.0
	github.com/sethvargo/go-envconfig v0.8.3
	github.com/sethvargo/go-limiter v0.7.2
	github.com/stretchr/testify v1.8.2
	github.com/urfave/cli/v2 v2.17.1
	go-micro.dev/v4 v4.9.0
	go.mongodb.org/mongo-driver v1.9.1
	go.opentelemetry.io/otel v1.15.1
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.12.0
	go.opentelemetry.io/otel/exporters/zipkin v1.12.0
	go.opentelemetry.io/otel/sdk v1.12.0
	go.uber.org/fx v1.19.2
	go.uber.org/ratelimit v0.2.0
	golang.org/x/sync v0.1.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	cloud.google.com/go/compute v1.19.0 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	github.com/Microsoft/go-winio v0.6.0 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20220930113650-c6815a8c17ad // indirect
	github.com/acomagu/bufpipe v1.0.3 // indirect
	github.com/afex/hystrix-go v0.0.0-20180502004556-fa1af6a1f4f5 // indirect
	github.com/andres-erbsen/clock v0.0.0-20160526145045-9e14626cd129 // indirect
	github.com/armon/go-metrics v0.0.0-20180917152333-f0300d1749da // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bitly/go-simplejson v0.5.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cloudflare/circl v1.2.0 // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/go-systemd/v22 v22.3.2 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/fatih/color v1.9.0 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/fsnotify/fsnotify v1.5.4 // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/go-git/go-billy/v5 v5.3.1 // indirect
	github.com/go-git/go-git/v5 v5.4.2 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.2.3 // indirect
	github.com/googleapis/gax-go/v2 v2.8.0 // indirect
	github.com/gopherjs/gopherjs v1.17.2 // indirect
	github.com/gorilla/securecookie v1.1.1 // indirect
	github.com/hashicorp/consul/api v1.9.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.1 // indirect
	github.com/hashicorp/go-hclog v0.12.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.0.0 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/golang-lru v0.5.1 // indirect
	github.com/hashicorp/serf v0.9.5 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jtolds/gls v4.20.0+incompatible // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/klauspost/compress v1.15.11 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/miekg/dns v1.1.50 // indirect
	github.com/minio/highwayhash v1.0.2 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/hashstructure v1.1.0 // indirect
	github.com/nats-io/jwt/v2 v2.3.0 // indirect
	github.com/nats-io/nats.go v1.16.0 // indirect
	github.com/nats-io/nkeys v0.3.0 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/onsi/gomega v1.20.0 // indirect
	github.com/openzipkin/zipkin-go v0.4.1 // indirect
	github.com/oxtoacart/bpool v0.0.0-20190530202638-03653db5a59c // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sergi/go-diff v1.2.0 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/streadway/amqp v1.0.0 // indirect
	github.com/vmihailenco/msgpack v4.0.4+incompatible // indirect
	github.com/xanzy/ssh-agent v0.3.2 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.0.2 // indirect
	github.com/xdg-go/stringprep v1.0.2 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	github.com/youmark/pkcs8 v0.0.0-20181117223130-1be2e3e5546d // indirect
	go.etcd.io/etcd/api/v3 v3.5.2 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.2 // indirect
	go.etcd.io/etcd/client/v3 v3.5.2 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/otel/metric v0.38.1 // indirect
	go.opentelemetry.io/otel/trace v1.15.1 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/dig v1.16.1 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.23.0 // indirect
	golang.org/x/crypto v0.6.0 // indirect
	golang.org/x/exp v0.0.0-20221126150942-6ab00d035af9 // indirect
	golang.org/x/mod v0.8.0 // indirect
	golang.org/x/net v0.9.0 // indirect
	golang.org/x/sys v0.7.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	golang.org/x/tools v0.6.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230331144136-dcfb400f0633 // indirect
	google.golang.org/grpc v1.54.0 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

require (
	github.com/coocood/freecache v1.2.3
	github.com/eko/gocache/lib/v4 v4.1.3
	github.com/eko/gocache/store/freecache/v4 v4.1.2
	github.com/eko/gocache/store/redis/v4 v4.1.2
	github.com/go-micro/plugins/v4/broker/memory v1.2.0
	github.com/go-redis/redis/v8 v8.11.5
	github.com/golang-jwt/jwt/v5 v5.0.0
	github.com/gookit/event v1.0.6
	github.com/hibiken/asynq v0.24.0
	github.com/josharian/intern v1.0.0 // indirect
	github.com/kamva/mgm/v3 v3.5.0
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mileusna/useragent v1.2.1
	github.com/natefinch/lumberjack v2.0.0+incompatible
	github.com/nicksnyder/go-i18n/v2 v2.2.1
	github.com/pkg/errors v0.9.1 // indirect
	github.com/sirupsen/logrus v1.9.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.41.1
	golang.org/x/oauth2 v0.7.0
	google.golang.org/api v0.116.0
)
