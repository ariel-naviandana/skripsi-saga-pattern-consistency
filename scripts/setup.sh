#!/usr/bin/env bash
set -e

TOPIC_PREFIX="saga."
TOPICS=(
  order.created
  payment.processed
  payment.failed
  inventory.reserved
  inventory.failed
  shipping.scheduled
  shipping.failed
  closed
)

echo "Creating Kafka topics..."
for topic in "${TOPICS[@]}"; do
  docker exec saga-kafka /opt/kafka/bin/kafka-topics.sh \
    --bootstrap-server localhost:9092 \
    --create --if-not-exists \
    --topic "${TOPIC_PREFIX}${topic}" \
    --partitions 1 \
    --replication-factor 1
  echo "  Created: ${TOPIC_PREFIX}${topic}"
done

echo ""
echo "Initializing database schemas..."
# (opsional — akan diisi setelah service stubs dibuat)
echo "Schemas will be initialized by services on their first run."

echo "Setup complete."
