#!/bin/bash

echo "🚀 Развертывание ZooKeeper и Kafka в namespace market..."

echo "🗑️  Удаление старой Kafka (если есть)..."
kubectl delete -n market service kafka 2>/dev/null || true
kubectl delete -n market statefulset kafka 2>/dev/null || true
kubectl delete -n market service zookeeper 2>/dev/null || true
kubectl delete -n market statefulset zookeeper 2>/dev/null || true

echo "📦 Развертывание ZooKeeper..."
kubectl apply -f zookeeper.yaml

echo "⏳ Ожидание запуска ZooKeeper (30 секунд)..."
sleep 30
kubectl wait --namespace market --for=condition=ready pod -l app=zookeeper --timeout=120s

echo "📦 Развертывание Kafka..."
kubectl apply -f kafka-with-zookeeper.yaml

echo "⏳ Ожидание запуска Kafka (40 секунд)..."
sleep 40
kubectl wait --namespace market --for=condition=ready pod -l app=kafka --timeout=120s

echo ""
echo "✅ ZooKeeper и Kafka успешно развернуты!"
echo ""
echo "📊 Статус:"
kubectl get pods -n market
echo ""
kubectl get services -n market
echo ""
echo "🔌 Доступ к Kafka:"
echo "   Внутри кластера: kafka.market:9092"
echo "   Снаружи (через minikube):"
echo "     Хост: $(minikube ip)"
echo "     Порт: 31090"
echo ""
echo "🔍 Тестирование работы:"
echo "   1. Создать топик:"
echo "      kubectl exec -n market kafka-0 -- kafka-topics --bootstrap-server localhost:9092 --create --topic test-topic --partitions 1 --replication-factor 1"
echo ""
echo "   2. Отправить сообщение:"
echo "      kubectl exec -n market kafka-0 -- bash -c \"echo 'Hello Kafka!' | kafka-console-producer --bootstrap-server localhost:9092 --topic test-topic\""
echo ""
echo "   3. Получить сообщение:"
echo "      kubectl exec -n market kafka-0 -- kafka-console-consumer --bootstrap-server localhost:9092 --topic test-topic --from-beginning --max-messages 1 --timeout-ms 10000"
