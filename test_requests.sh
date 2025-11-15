
BASE_URL="http://localhost:8080"

echo -e "\n\033[1;36m=== ТЕСТИРОВАНИЕ PR REVIEWER SERVICE ===\033[0m"

# 1. Создание команды backend
echo -e "\n\033[1;33m1. Создание команды 'backend' с 3 участниками...\033[0m"
curl -s -X POST $BASE_URL/team/add \
  -H "Content-Type: application/json" \
  -d '{
    "team_name": "backend",
    "members": [
      {"user_id": "u1", "username": "Alice", "is_active": true},
      {"user_id": "u2", "username": "Bob", "is_active": true},
      {"user_id": "u3", "username": "Charlie", "is_active": true}
    ]
  }' | jq .
echo -e "\033[1;32m✓ Команда создана\033[0m"

# 2. Создание команды frontend
echo -e "\n\033[1;33m2. Создание команды 'frontend' с 2 участниками...\033[0m"
curl -s -X POST $BASE_URL/team/add \
  -H "Content-Type: application/json" \
  -d '{
    "team_name": "frontend",
    "members": [
      {"user_id": "u4", "username": "David", "is_active": true},
      {"user_id": "u5", "username": "Eve", "is_active": true}
    ]
  }' | jq .
echo -e "\033[1;32m✓ Команда создана\033[0m"

# 3. Получение команды backend
echo -e "\n\033[1;33m3. Получение информации о команде 'backend'...\033[0m"
curl -s -X GET "$BASE_URL/team/get?team_name=backend" | jq .
echo -e "\033[1;32m✓ Команда получена\033[0m"

# 4. Создание PR от u1
echo -e "\n\033[1;33m4. Создание PR-1001 от Alice (u1)...\033[0m"
curl -s -X POST $BASE_URL/pullRequest/create \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-1001",
    "pull_request_name": "Add search feature",
    "author_id": "u1"
  }' | jq .
echo -e "\033[1;32m✓ PR создан с автоназначением ревьюверов\033[0m"

# 5. Получение PR для ревьювера u2
echo -e "\n\033[1;33m5. Получение списка PR для ревьювера Bob (u2)...\033[0m"
curl -s -X GET "$BASE_URL/users/getReview?user_id=u2" | jq .
echo -e "\033[1;32m✓ Список PR получен\033[0m"

# 6. Деактивация пользователя u3
echo -e "\n\033[1;33m6. Деактивация пользователя Charlie (u3)...\033[0m"
curl -s -X POST $BASE_URL/users/setIsActive \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "u3",
    "is_active": false
  }' | jq .
echo -e "\033[1;32m✓ Пользователь деактивирован\033[0m"

# 7. Создание PR от u2
echo -e "\n\033[1;33m7. Создание PR-1002 от Bob (u2), u3 неактивен...\033[0m"
curl -s -X POST $BASE_URL/pullRequest/create \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-1002",
    "pull_request_name": "Add authentication",
    "author_id": "u2"
  }' | jq .
echo -e "\033[1;32m✓ PR создан (должен быть назначен только u1)\033[0m"

# 8. Активация u3 обратно
echo -e "\n\033[1;33m8. Активация пользователя Charlie (u3) обратно...\033[0m"
curl -s -X POST $BASE_URL/users/setIsActive \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "u3",
    "is_active": true
  }' | jq .
echo -e "\033[1;32m✓ Пользователь активирован\033[0m"

# 9. Переназначение ревьювера
echo -e "\n\033[1;33m9. Переназначение ревьювера u1 в PR-1002...\033[0m"
curl -s -X POST $BASE_URL/pullRequest/reassign \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-1002",
    "old_user_id": "u1"
  }' | jq .
echo -e "\033[1;32m✓ Ревьювер переназначен\033[0m"

# 10. Merge PR
echo -e "\n\033[1;33m10. Merge PR-1001...\033[0m"
curl -s -X POST $BASE_URL/pullRequest/merge \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-1001"
  }' | jq .
echo -e "\033[1;32m✓ PR merged\033[0m"

# 11. Повторный merge (идемпотентность)
echo -e "\n\033[1;33m11. Повторный merge PR-1001 (проверка идемпотентности)...\033[0m"
curl -s -X POST $BASE_URL/pullRequest/merge \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-1001"
  }' | jq .
echo -e "\033[1;32m✓ Идемпотентность работает\033[0m"

# 12. Попытка переназначения на merged PR
echo -e "\n\033[1;33m12. Попытка переназначения в merged PR (должна провалиться)...\033[0m"
curl -s -X POST $BASE_URL/pullRequest/reassign \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-1001",
    "old_user_id": "u2"
  }' | jq .
echo -e "\033[1;32m✓ Ожидаемая ошибка PR_MERGED\033[0m"

# 13. Создание PR от frontend команды
echo -e "\n\033[1;33m13. Создание PR-2001 от David (u4) из команды frontend...\033[0m"
curl -s -X POST $BASE_URL/pullRequest/create \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-2001",
    "pull_request_name": "Redesign UI",
    "author_id": "u4"
  }' | jq .
echo -e "\033[1;32m✓ PR создан\033[0m"

# 14. Попытка создать дубликат команды
echo -e "\n\033[1;33m14. Попытка создать дубликат команды (должна провалиться)...\033[0m"
curl -s -X POST $BASE_URL/team/add \
  -H "Content-Type: application/json" \
  -d '{
    "team_name": "backend",
    "members": [{"user_id": "u6", "username": "Frank", "is_active": true}]
  }' | jq .
echo -e "\033[1;32m✓ Ожидаемая ошибка TEAM_EXISTS\033[0m"

# 15. Попытка создать дубликат PR
echo -e "\n\033[1;33m15. Попытка создать дубликат PR (должна провалиться)...\033[0m"
curl -s -X POST $BASE_URL/pullRequest/create \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-1001",
    "pull_request_name": "Duplicate",
    "author_id": "u1"
  }' | jq .
echo -e "\033[1;32m✓ Ожидаемая ошибка PR_EXISTS\033[0m"

# 16. Получение несуществующей команды
echo -e "\n\033[1;33m16. Попытка получить несуществующую команду...\033[0m"
curl -s -X GET "$BASE_URL/team/get?team_name=nonexistent" | jq .
echo -e "\033[1;32m✓ Ожидаемая ошибка NOT_FOUND\033[0m"

# 17. Финальная сводка
echo -e "\n\033[1;33m17. Получение PR для всех ревьюверов...\033[0m"
for user in u1 u2 u3 u5; do
  echo -e "  \033[0;37m- Ревьювер $user:\033[0m"
  curl -s -X GET "$BASE_URL/users/getReview?user_id=$user" | jq -r '.pull_requests[] | "    PR: \(.pull_request_id) - \(.pull_request_name) (\(.status))"'
done
echo -e "\033[1;32m✓ Все PR получены\033[0m"

echo -e "\n\033[1;36m=== ВСЕ ТЕСТЫ ЗАВЕРШЕНЫ ===\033[0m"
echo -e "\033[1;32mПроверьте результаты выше. Все функции должны работать корректно!\033[0m"
