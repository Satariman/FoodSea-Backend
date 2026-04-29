#!/bin/bash
set -e

BOOTSTRAP="${KAFKA_BOOTSTRAP_SERVERS:-kafka:29092}"

echo "Waiting for Kafka to be ready..."
sleep 5

create_topic() {
    local topic=$1
    local partitions=${2:-3}
    local replication=${3:-1}

    kafka-topics --bootstrap-server "$BOOTSTRAP" \
        --create \
        --if-not-exists \
        --topic "$topic" \
        --partitions "$partitions" \
        --replication-factor "$replication"

    echo "Topic created: $topic"
}

create_topic "cart.events" 3 1
create_topic "optimization.events" 3 1
create_topic "order.events" 3 1
create_topic "saga.commands" 3 1
create_topic "saga.replies" 3 1

echo "All topics created successfully"
kafka-topics --bootstrap-server "$BOOTSTRAP" --list
