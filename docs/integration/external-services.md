# 外部服务集成

默认测试不依赖外部服务。下面的 fixture 只用于显式的 Docker 集成测试；没有对应环境变量时，测试会以 `t.Skip` 结束，不能把 skip 解释为通过。

## MySQL

启动并等待 ready：

```sh
docker run --name esper-mysql \
  --env MYSQL_ROOT_PASSWORD=password \
  --env MYSQL_DATABASE=test \
  --publish 3306:3306 \
  --detach mysql:8.0

until docker exec esper-mysql mysqladmin ping -h 127.0.0.1 -uroot -ppassword --silent; do
  sleep 2
done
```

加载 Esper 固定回归夹具。`ESPER_JAVA_ROOT` 应指向固定 Java checkout，例如 `/root/app/esper`；SQL 文件中的说明性 `//` 行需要过滤：

```sh
ESPER_JAVA_ROOT=/root/app/esper
sed '/^[[:space:]]*\/\//d' "$ESPER_JAVA_ROOT/common/etc/regression/create_testdb.sql" \
  | docker exec -i esper-mysql mysql -uroot -ppassword test
```

设置 Go DSN 并运行 MySQL 门控：

```sh
export ESPER_MYSQL_DSN='root:password@tcp(127.0.0.1:3306)/test?parseTime=true&charset=utf8mb4'
go test ./connectors/db -run '^TestDBConnectorMySQLDocker$' -count=1
go test ./internal/esper \
  -run '^(TestSQLHistoricalProviderMySQLDocker|TestSQLHistoricalFireAndForgetMySQLDocker|TestSQLSinkMySQLDocker)$' \
  -count=1
```

## Kafka

启动单节点 KRaft broker：

```sh
docker run --name esper-kafka \
  --publish 9092:9092 \
  --env KAFKA_NODE_ID=1 \
  --env KAFKA_PROCESS_ROLES=broker,controller \
  --env KAFKA_LISTENERS=PLAINTEXT://:9092,CONTROLLER://:9093 \
  --env KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://127.0.0.1:9092 \
  --env KAFKA_CONTROLLER_LISTENER_NAMES=CONTROLLER \
  --env KAFKA_CONTROLLER_QUORUM_VOTERS=1@127.0.0.1:9093 \
  --env KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1 \
  --env KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR=1 \
  --env KAFKA_TRANSACTION_STATE_LOG_MIN_ISR=1 \
  --env KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS=0 \
  --detach apache/kafka:3.8.1

until docker exec esper-kafka /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server 127.0.0.1:9092 --list >/dev/null 2>&1; do
  sleep 2
done
```

运行 connector round-trip：

```sh
export ESPER_KAFKA_BROKERS='127.0.0.1:9092'
go test ./connectors/kafka -run '^TestKafkaDockerRoundTrip$' -count=1
```

## RabbitMQ

启动并等待 ready：

```sh
docker run --name esper-rabbitmq \
  --publish 5672:5672 \
  --publish 15672:15672 \
  --detach rabbitmq:3.13-management

until docker exec esper-rabbitmq rabbitmq-diagnostics -q ping >/dev/null 2>&1; do
  sleep 2
done
```

运行 source 和 sink round-trip：

```sh
export ESPER_AMQP_URL='amqp://guest:guest@127.0.0.1:5672/'
go test ./connectors/amqp \
  -run '^(TestAMQPDockerRoundTrip|TestAMQPDockerSinkRoundTrip)$' \
  -count=1
```

测试完成后可移除 fixture 容器：

```sh
docker rm -f esper-mysql esper-kafka esper-rabbitmq
```
