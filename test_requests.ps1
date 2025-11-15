
$baseUrl = "http://localhost:8080"

Write-Host "`n=== ТЕСТИРОВАНИЕ PR REVIEWER SERVICE ===" -ForegroundColor Cyan


Write-Host "`n1. Создание команды 'backend' с 3 участниками..." -ForegroundColor Yellow
$body = @{
    team_name = "backend"
    members = @(
        @{user_id="u1"; username="Alice"; is_active=$true},
        @{user_id="u2"; username="Bob"; is_active=$true},
        @{user_id="u3"; username="Charlie"; is_active=$true}
    )
} | ConvertTo-Json -Depth 3

$response = Invoke-RestMethod -Uri "$baseUrl/team/add" -Method Post -Body $body -ContentType "application/json"
$response | ConvertTo-Json -Depth 5
Write-Host "✓ Команда создана" -ForegroundColor Green

Write-Host "`n2. Создание команды 'frontend' с 2 участниками..." -ForegroundColor Yellow
$body = @{
    team_name = "frontend"
    members = @(
        @{user_id="u4"; username="David"; is_active=$true},
        @{user_id="u5"; username="Eve"; is_active=$true}
    )
} | ConvertTo-Json -Depth 3

$response = Invoke-RestMethod -Uri "$baseUrl/team/add" -Method Post -Body $body -ContentType "application/json"
$response | ConvertTo-Json -Depth 5
Write-Host "✓ Команда создана" -ForegroundColor Green

# 3. Получение команды backend
Write-Host "`n3. Получение информации о команде 'backend'..." -ForegroundColor Yellow
$response = Invoke-RestMethod -Uri "$baseUrl/team/get?team_name=backend" -Method Get
$response | ConvertTo-Json -Depth 5
Write-Host "✓ Команда получена" -ForegroundColor Green

Write-Host "`n4. Создание PR-1001 от Alice (u1)..." -ForegroundColor Yellow
$body = @{
    pull_request_id = "pr-1001"
    pull_request_name = "Add search feature"
    author_id = "u1"
} | ConvertTo-Json

$response = Invoke-RestMethod -Uri "$baseUrl/pullRequest/create" -Method Post -Body $body -ContentType "application/json"
$response | ConvertTo-Json -Depth 5
Write-Host "✓ PR создан, назначены ревьюверы: $($response.pr.assigned_reviewers -join ', ')" -ForegroundColor Green

Write-Host "`n5. Получение списка PR для ревьювера Bob (u2)..." -ForegroundColor Yellow
$response = Invoke-RestMethod -Uri "$baseUrl/users/getReview?user_id=u2" -Method Get
$response | ConvertTo-Json -Depth 5
Write-Host "✓ Найдено PR: $($response.pull_requests.Count)" -ForegroundColor Green

Write-Host "`n6. Деактивация пользователя Charlie (u3)..." -ForegroundColor Yellow
$body = @{
    user_id = "u3"
    is_active = $false
} | ConvertTo-Json

$response = Invoke-RestMethod -Uri "$baseUrl/users/setIsActive" -Method Post -Body $body -ContentType "application/json"
$response | ConvertTo-Json -Depth 5
Write-Host "✓ Пользователь деактивирован" -ForegroundColor Green

Write-Host "`n7. Создание PR-1002 от Bob (u2), u3 неактивен..." -ForegroundColor Yellow
$body = @{
    pull_request_id = "pr-1002"
    pull_request_name = "Add authentication"
    author_id = "u2"
} | ConvertTo-Json

$response = Invoke-RestMethod -Uri "$baseUrl/pullRequest/create" -Method Post -Body $body -ContentType "application/json"
$response | ConvertTo-Json -Depth 5
Write-Host "✓ PR создан, назначены ревьюверы: $($response.pr.assigned_reviewers -join ', ')" -ForegroundColor Green

Write-Host "`n8. Активация пользователя Charlie (u3) обратно..." -ForegroundColor Yellow
$body = @{
    user_id = "u3"
    is_active = $true
} | ConvertTo-Json

$response = Invoke-RestMethod -Uri "$baseUrl/users/setIsActive" -Method Post -Body $body -ContentType "application/json"
$response | ConvertTo-Json -Depth 5
Write-Host "✓ Пользователь активирован" -ForegroundColor Green

Write-Host "`n9. Переназначение ревьювера u1 в PR-1002..." -ForegroundColor Yellow
$body = @{
    pull_request_id = "pr-1002"
    old_user_id = "u1"
} | ConvertTo-Json

$response = Invoke-RestMethod -Uri "$baseUrl/pullRequest/reassign" -Method Post -Body $body -ContentType "application/json"
$response | ConvertTo-Json -Depth 5
Write-Host "✓ Ревьювер переназначен на: $($response.replaced_by)" -ForegroundColor Green

Write-Host "`n10. Merge PR-1001..." -ForegroundColor Yellow
$body = @{
    pull_request_id = "pr-1001"
} | ConvertTo-Json

$response = Invoke-RestMethod -Uri "$baseUrl/pullRequest/merge" -Method Post -Body $body -ContentType "application/json"
$response | ConvertTo-Json -Depth 5
Write-Host "✓ PR merged, статус: $($response.pr.status)" -ForegroundColor Green

Write-Host "`n11. Повторный merge PR-1001 (проверка идемпотентности)..." -ForegroundColor Yellow
$body = @{
    pull_request_id = "pr-1001"
} | ConvertTo-Json

$response = Invoke-RestMethod -Uri "$baseUrl/pullRequest/merge" -Method Post -Body $body -ContentType "application/json"
$response | ConvertTo-Json -Depth 5
Write-Host "✓ Идемпотентность работает, статус: $($response.pr.status)" -ForegroundColor Green

Write-Host "`n12. Попытка переназначения ревьювера в merged PR-1001 (должна провалиться)..." -ForegroundColor Yellow
$body = @{
    pull_request_id = "pr-1001"
    old_user_id = "u2"
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/pullRequest/reassign" -Method Post -Body $body -ContentType "application/json"
} catch {
    $errorResponse = $_.ErrorDetails.Message | ConvertFrom-Json
    $errorResponse | ConvertTo-Json -Depth 5
    Write-Host "✓ Ожидаемая ошибка: $($errorResponse.error.code) - $($errorResponse.error.message)" -ForegroundColor Green
}

Write-Host "`n13. Создание PR-2001 от David (u4) из команды frontend..." -ForegroundColor Yellow
$body = @{
    pull_request_id = "pr-2001"
    pull_request_name = "Redesign UI"
    author_id = "u4"
} | ConvertTo-Json

$response = Invoke-RestMethod -Uri "$baseUrl/pullRequest/create" -Method Post -Body $body -ContentType "application/json"
$response | ConvertTo-Json -Depth 5
Write-Host "✓ PR создан, назначены ревьюверы: $($response.pr.assigned_reviewers -join ', ')" -ForegroundColor Green

Write-Host "`n14. Попытка создать дубликат команды 'backend' (должна провалиться)..." -ForegroundColor Yellow
$body = @{
    team_name = "backend"
    members = @(
        @{user_id="u6"; username="Frank"; is_active=$true}
    )
} | ConvertTo-Json -Depth 3

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/team/add" -Method Post -Body $body -ContentType "application/json"
} catch {
    $errorResponse = $_.ErrorDetails.Message | ConvertFrom-Json
    $errorResponse | ConvertTo-Json -Depth 5
    Write-Host "✓ Ожидаемая ошибка: $($errorResponse.error.code) - $($errorResponse.error.message)" -ForegroundColor Green
}

Write-Host "`n15. Попытка создать дубликат PR-1001 (должна провалиться)..." -ForegroundColor Yellow
$body = @{
    pull_request_id = "pr-1001"
    pull_request_name = "Duplicate PR"
    author_id = "u1"
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/pullRequest/create" -Method Post -Body $body -ContentType "application/json"
} catch {
    $errorResponse = $_.ErrorDetails.Message | ConvertFrom-Json
    $errorResponse | ConvertTo-Json -Depth 5
    Write-Host "✓ Ожидаемая ошибка: $($errorResponse.error.code) - $($errorResponse.error.message)" -ForegroundColor Green
}

Write-Host "`n16. Попытка получить несуществующую команду (должна провалиться)..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$baseUrl/team/get?team_name=nonexistent" -Method Get
} catch {
    $errorResponse = $_.ErrorDetails.Message | ConvertFrom-Json
    $errorResponse | ConvertTo-Json -Depth 5
    Write-Host "✓ Ожидаемая ошибка: $($errorResponse.error.code) - $($errorResponse.error.message)" -ForegroundColor Green
}

Write-Host "`n17. Получение PR для всех активных ревьюверов..." -ForegroundColor Yellow
@("u1", "u2", "u3", "u5") | ForEach-Object {
    Write-Host "  - Ревьювер $_:" -ForegroundColor Gray
    $response = Invoke-RestMethod -Uri "$baseUrl/users/getReview?user_id=$_" -Method Get
    Write-Host "    PR: $($response.pull_requests.pull_request_id -join ', ')" -ForegroundColor Gray
}
Write-Host "✓ Все PR получены" -ForegroundColor Green

Write-Host "`n=== ВСЕ ТЕСТЫ ЗАВЕРШЕНЫ ===" -ForegroundColor Cyan
Write-Host "Проверьте результаты выше. Все функции должны работать корректно!" -ForegroundColor Green
