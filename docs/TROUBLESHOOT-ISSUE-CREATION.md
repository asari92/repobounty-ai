# Troubleshooting: Issue Creation

Если issue не создается при создании campaign, используйте эту инструкцию для отладки.

## Шаги Отладки

### 1. Проверьте логи backend

```bash
# Запустите backend и смотрите логи
cd backend
go run ./cmd/api
```

Ищите строки типа:
```
createCampaignIssue: started for campaign=abc123 repo=owner/repo user=username
createCampaignIssue: attempting GitHub App method (app_id=123)
createCampaignIssue: attempting user token method for user=username (token_length=40)
```

### 2. Проверьте, сохранился ли GitHub токен

При OAuth callback должны видеть:
```
github oauth: exchanged code for user=username with token length=40
github oauth: created new user=username with GitHub token
```

Или для существующего пользователя:
```
github oauth: updated existing user=username with GitHub token
```

### 3. Проверьте, что у пользователя есть токен в БД

#### Через SQLite CLI:
```bash
sqlite3 repobounty.db "SELECT github_username, LENGTH(github_token), github_token FROM users WHERE github_username='your_github_username';"
```

Должно показать:
- `github_username`: ваше имя
- `LENGTH(github_token)`: > 0 (обычно ~40 символов)
- `github_token`: не пусто

#### Если токен пуст (`""`):
- Пользователь не залогинился через GitHub OAuth
- **Решение**: Откройте фронтенд, нажмите "Login with GitHub", авторизуйтесь

### 4. Проверьте GitHub token scope

```bash
# Используйте свой GitHub token для проверки что он может создавать issue
curl -H "Authorization: Bearer YOUR_TOKEN" \
  https://api.github.com/user/repos \
  -H "Accept: application/vnd.github+json" | head -20
```

Если ошибка 401 - токен невалидный или истек

### 5. Проверьте GitHub App установки

Если пытаетесь использовать GitHub App:

```bash
# Проверьте конфигурацию
printenv | grep GITHUB_APP
```

Должны видеть:
```
GITHUB_APP_ID=123456
GITHUB_APP_PRIVATE_KEY=-----BEGIN RSA PRIVATE KEY-----...
```

Если пусто - GitHub App не настроена, система использует токен пользователя

### 6. Проверьте наличие доступа к репозиторию

```bash
# Проверьте, может ли токен создавать issue в целевом репо
TOKEN="your_github_token"
REPO="owner/repo"

curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Accept: application/vnd.github+json" \
  https://api.github.com/repos/$REPO/issues \
  -d '{"title":"Test","body":"Test issue"}'
```

Если ошибка 422 или 403 - нет доступа

### 7. Полный тестовый сценарий

#### Шаг 1: Login через GitHub
```bash
# Получите GitHub OAuth URL
curl http://localhost:8080/api/auth/github/url

# Откройте ссылку в браузере, авторизуйтесь
# После redirect на фронтенд, JWT токен сохранится
```

#### Шаг 2: Проверьте, что пользователь создан с токеном
```bash
sqlite3 repobounty.db "SELECT github_username, LENGTH(github_token) FROM users LIMIT 1;"
```

#### Шаг 3: Создайте campaign
```bash
# Используйте JWT токен из шага 1
JWT_TOKEN="your_jwt_token"

curl -X POST http://localhost:8080/api/campaigns \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "repo": "owner/repo",
    "pool_amount": 1000000000,
    "deadline": "2026-04-05T00:00:00Z",
    "sponsor_wallet": "...",
    "challenge_id": "...",
    "signature": "..."
  }'
```

#### Шаг 4: Проверьте логи backend
- Должна появиться строка `createCampaignIssue: ✓ successfully created campaign issue`
- Или детальное сообщение об ошибке

#### Шаг 5: Проверьте GitHub репозиторий
- Откройте репо на GitHub
- Перейдите в Issues вкладку
- Должен быть новый issue с labels `repobounty` и `reward`

## Возможные ошибки и решения

### Ошибка: "User token is empty"
```
createCampaignIssue: user username has no GitHub token
```
**Причина**: Пользователь не были залогинен через GitHub\
**Решение**: Залогиньтесь через GitHub OAuth

### Ошибка: "App not installed on repo"
```
createCampaignIssue: app not installed on repo owner/repo
```
**Причина**: GitHub App не установлена на целевом репозитории\
**Решение**: 
- Установите GitHub App на репозиторий, где хотите создавать issue
- Или используйте пользовательский токен

### Ошибка: "401 Unauthorized"
```
createCampaignIssue: failed to create issue via user token: create issue: 401
```
**Причина**: GitHub токен неправильный или истек\
**Решение**: 
- Переавторизуйтесь через GitHub (снова сделайте OAuth login)
- Проверьте, что токен имеет правильные scope

### Ошибка: "422 Validation Failed"
```
createCampaignIssue: failed to create issue: 422 {"errors":[...]}
```
**Причина**: Обычно - репозиторий не существует или пользователь не имеет доступа\
**Решение**:
- Проверьте название репозитория
- Убедитесь, что пользователь имеет права на создание issue в этом репо
- Используйте публичный репо для тестирования

### Ошибка: "No GitHub App or user token available"
```
campaign issue creation skipped for abc123: no GitHub App or user token available
```
**Причина**: 
- GitHub App не установлена И
- У пользователя нет GitHub токена
\
**Решение**: Залогиньтесь через GitHub OAuth

## Дополнительные проверки

### Проверить, что функция выполняется
Добавьте в логи вывод всех шагов:

```bash
# Запустите с verbose logging (если добавить флаг)
LOG_LEVEL=debug go run ./cmd/api
```

### Проверить GitHub API rate limiting
```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  https://api.github.com/rate_limit
```

Если `remaining: 0` - исчерпана квота API GitHub

## Если ничего не помогает

1. Очистите БД и начните заново:
   ```bash
   rm repobounty.db
   go run ./cmd/api
   ```

2. Используйте test repo:
   ```bash
   # Создайте тестовый репо для экспериментов
   # Убедитесь, что он публичный
   ```

3. Проверьте GitHub App конфигурацию:
   - Перейдите на https://github.com/settings/apps
   - Убедитесь, что app установлена
   - Проверьте permissions (должны быть `issues:write`)

4. Проверьте network логи:
   ```bash
   # Используйте tcpdump или Wireshark для отладки HTTP запросов
   tcpdump -i lo port 443
   ```
