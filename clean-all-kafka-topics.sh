#!/bin/bash
# clean-all-kafka-topics.sh

echo "🧹 Начинаем очистку Kafka топиков..."

# Получаем список всех топиков
TOPICS=$(kubectl exec -n market kafka-service-0 -- \
  kafka-topics.sh --bootstrap-server localhost:9092 --list 2>/dev/null)

echo "Найдены топики:"
echo "$TOPICS"
echo ""

# Очищаем каждый топик
for TOPIC in $TOPICS; do
    # Пропускаем системные топики (начинающиеся с _)
    if [[ $TOPIC == _* ]]; then
        echo "⏭️  Пропускаем системный топик: $TOPIC"
        continue
    fi
    
    echo "🧹 Очищаем топик: $TOPIC"
    
    # 1. Устанавливаем минимальный retention
    kubectl exec -n market kafka-service-0 -- \
      kafka-configs.sh --bootstrap-server localhost:9092 \
      --entity-type topics \
      --entity-name $TOPIC \
      --alter \
      --add-config retention.ms=1000 2>/dev/null
    
    # 2. Ждем очистки
    sleep 1
    
    # 3. Удаляем настройку retention (возвращаем по умолчанию)
    kubectl exec -n market kafka-service-0 -- \
      kafka-configs.sh --bootstrap-server localhost:9092 \
      --entity-type topics \
      --entity-name $TOPIC \
      --alter \
      --delete-config retention.ms 2>/dev/null
    
    echo "✅ Топик $TOPIC очищен"
done

echo "🎉 Все топики очищены!"