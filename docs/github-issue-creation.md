# GitHub Issue Creation на создание компании

## Версия
- **Последнее обновление**: 2026-04-01
- **Статус**: ✅ Исправлено (исправлены баги с сохранением токена и HTTP client)

## Описание

При создании новой компании (campaign) в репозитории GitHub автоматически создается issue, который информирует об открытой награде. Issue содержит:
- Размер фонда награды (в SOL)
- ID компании
- Дедлайн кампании
- Ссылку на сайт для просмотра деталей кампании
- Специальные метки: `repobounty` и `reward`

## Механизм работы

### 1. Двухуровневая стратегия создания Issue

#### Уровень 1: GitHub App (рекомендуется)
Если GitHub App установлена на репозитории:
- Используется GitHub App токен для создания issue от имени приложения
- Это выглядит профессионально и показывает, что компания официально поддерживает GitHub
- Requires: `GITHUB_APP_ID` и `GITHUB_APP_PRIVATE_KEY` env variables

#### Уровень 2: Пользовательский GitHub токен (fallback)
Если GitHub App не установлена или произойдет ошибка:
- Используется GitHub токен пользователя, который создал компанию
- Issue создается от имени пользователя
- Требует, чтобы пользователь был залогинен через GitHub OAuth

### 2. Сохранение GitHub токена пользователя

При OAuth callback (когда пользователь логинится через GitHub):
- Сохраняется `access_token` в поле `github_token` таблицы `users`
- Этот токен используется для создания issue, если GitHub App недоступен
- Токен обновляется при каждом логине для поддержания валидности

### 3. Flow создания Issue

```
CreateCampaign Request
    ↓
Создание компании в Solana
    ↓
Успешное обновление в БД
    ↓
[Async] Попытка создать Issue
    ├─→ GitHub App способен? (есть installation)
    │   └─→ ДА: Создаем issue через App ✓
    │   └─→ НЕТ: Проверяем пользовательский токен
    │
    └─→ У пользователя есть токен?
        └─→ ДА: Создаем issue от имени пользователя ✓
        └─→ НЕТ: Логируем и продолжаем (issue creation optional)
    
Возврат ответа клиенту (не дожидаемся issue creation)
```

## Изменения в коде

### 1. Backend изменения

#### [store/memory.go] и [store/sqlite.go]
- Добавлено поле `GitHubToken string` к структуре `User`
- Обновлена таблица `users` с новым столбцом `github_token`
- Обновлены методы `GetUser`, `CreateUser`, `UpdateUser`

#### [auth/github_oauth.go]
- `ExchangeCode` уже возвращает `accessToken` (второй return value)
- Токен теперь используется в `GitHubCallback` handler

#### [http/handlers.go]
- `GitHubCallback`: Сохраняет GitHub токен при OAuth
  - ✨ **Исправлено**: Добавлено детальное логирование при сохранении токена
  - ✨ **Исправлено**: Токен обновляется и для existing users
  
- `CreateCampaign`: 
  - ✨ **Исправлено**: Теперь загружает свежий user из БД перед созданием issue (чтобы получить сохраненный токен)
  - Вызывает `createCampaignIssue` async после создания
  
- `createCampaignIssue`: 
  - ✨ **Усовершенствовано**: Добавлено детальное логирование на каждом шаге
  - Реализует двухуровневую стратегию с fallback механизмом

#### [github/client.go]
- `CreateCampaignIssue`: Метод для создания issue через обычный GitHub API
  - ✨ **ИСПРАВЛЕНО**: Использует `c.httpClient` вместо `http.DefaultClient`
  - Требует: `repo` (owner/repo), `CreateIssueBody` с данными
  - Использует: пользовательский GitHub токен

#### [githubapp/client.go]
- `CreateCampaignIssue`: Метод для создания issue через GitHub App
- Требует: `installToken`, `repo`, `CreateIssueBody`
- Создает issue от имени GitHub App

### 2. Issue шаблон

```markdown
# 🚀 RepoBounty AI Campaign

A sponsor is rewarding **{poolSOL} SOL** for contributions to this repository!

**Campaign:** {campaignID}
**Total Reward Pool:** {poolSOL} SOL
**Deadline:** {deadline}

## About RepoBounty AI
RepoBounty AI uses artificial intelligence to analyze code contributions and 
automatically allocate rewards based on impact.

→ [View Campaign Details]({campaignURL})

*This issue was created automatically by RepoBounty AI.*
```

### 3. Замечание о Labels

Labels (`repobounty`, `reward`) были убраны из создания issue, так как это требует прав на создание labels на репозитории, которых может не быть у пользователя. Issue создается успешно и содержит всю необходимую информацию — это визуальное оформление не влияет на функциональность.

## Environment переменные

Для работы GitHub App (опционально):
```
GITHUB_APP_ID=<app-id>
GITHUB_APP_PRIVATE_KEY=<private-key-pem>
```

Для GitHub OAuth (требуется):
```
GITHUB_CLIENT_ID=<client-id>
GITHUB_CLIENT_SECRET=<client-secret>
GITHUB_TOKEN=<personal-access-token>  # для базовых операций
```

## Обработка ошибок

- Если GitHub App установлен, но возникает ошибка → автоматически переходит на пользовательский токен
- Если пользовательский токен недоступен → логируется ошибка, создание issue пропускается (не критично)
- Если оба способа недоступны → issue просто не создается, но кампания работает нормально
- HTTP ответ возвращается без ожидания создания issue (async операция)

## Требования для работы

1. **GitHub App установлена на репозитории** (предпочтительно):
   - App должна быть установлена на целевом репозитории
   - Требуются permissions: `issues:write`

2. **ИЛИ пользователь залогинен через GitHub OAuth**:
   - Пользователь должен иметь доступ для создания issue в репозитории
   - GitHub токен должен содержать scope для создания issue

## Тестирование

### Local тестирование
```bash
# 1. Убедитесь, что backend запущен
cd backend
go run ./cmd/api

# 2. Создайте campaign через API
curl -X POST http://localhost:8080/api/campaigns \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer {jwt_token}" \
  -d '{
    "repo": "owner/repo",
    "pool_amount": 1000000000,
    "deadline": "2026-04-05T00:00:00Z",
    "sponsor_wallet": "...",
    "challenge_id": "...",
    "signature": "..."
  }'

# 3. Проверьте репозиторий на наличие нового issue
```

## Future улучшения

1. Добавить retry logic если GitHub API недоступен
2. Добавить webhook для отслеживания статуса issue
3. Отправлять уведомления контрибьютерам через PR comments (уже реализовано)
4. Сохранять номер issue в campaign для последующего riferimento
