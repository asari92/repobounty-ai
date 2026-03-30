# План: Escrow, Claim Flow, GitHub Auth, AI Impact Engine

## Контекст

Текущий MVP — демо без реальных денег. `pool_amount` — просто число на чейне, аллокации — GitHub юзернеймы без кошельков, транзакции подписывает бэкенд. Нужно превратить это в рабочий продукт: спонсор замораживает SOL, контрибьюторы клеймят награды, AI оценивает реальный импакт кода.

---

## Фаза 1: Solana Program — Escrow + Claim

**Файл:** `program/programs/repobounty/src/lib.rs`

### Изменения в стейт-машине
```
Created → Funded → Finalized → (частично claimed) → Completed
```

### Новая архитектура аккаунтов

**Campaign PDA** — seeds меняются на `["campaign", campaign_id]` (убираем authority из seed, т.к. теперь два authority: sponsor и backend).

**Vault PDA** — новый аккаунт `["vault", campaign_pda]`, system-owned, хранит SOL. Отдельный от Campaign аккаунта, чтобы не мешать ресайзингу данных.

**Campaign struct — новые поля:**
- `sponsor: Pubkey` — кошелёк спонсора (отдельно от `authority`)
- `vault_bump: u8` — bump для vault PDA
- `total_claimed: u64` — сколько уже заклеймлено

**Allocation struct — новые поля:**
- `claimed: bool`
- `claimant: Option<Pubkey>` — кошелёк получателя

### Новые инструкции

**`fund_campaign`** — спонсор переводит `pool_amount` SOL из своего кошелька в vault PDA через `system_program::transfer`. Constraint: `campaign.state == Created`, `campaign.sponsor == signer`. После трансфера: `state = Funded`.

**`claim`** — аргумент `contributor_github: String`. Логика:
1. Найти аллокацию по github username
2. Проверить `!claimed`
3. `invoke_signed` — перевод из vault в `contributor_wallet` (передаётся как аккаунт)
4. `claimed = true`, `claimant = Some(wallet)`
5. `total_claimed += amount`
6. Если всё заклеймлено → `state = Completed`

**`finalize_campaign`** — constraint меняется на `state == Funded`.

### Тесты
**Файл:** `program/tests/repobounty.ts` — добавить тесты на fund, claim, double-claim rejection, partial claims.

---

## Фаза 2: Backend Auth — GitHub OAuth + JWT

### Новый пакет `backend/internal/auth/`

**`github_oauth.go`** — обмен code на access_token (`POST github.com/login/oauth/access_token`), получение профиля (`GET api.github.com/user`).

**`jwt.go`** — генерация/валидация HS256 JWT. Claims: `{sub: github_username, github_id: N, exp: ...}`. Зависимость: `golang-jwt/jwt/v5`.

**`middleware.go`** — Chi middleware, читает `Authorization: Bearer <jwt>`, кладёт claims в context. Плюс `OptionalAuth` вариант.

### Новые эндпоинты
```
GET  /api/auth/github/url         → {url: "https://github.com/login/oauth/authorize?..."}
GET  /api/auth/github?code=XXX    → {token: "jwt...", user: {...}}
GET  /api/auth/me                 → текущий юзер (protected)
POST /api/profile/link-wallet     → {wallet_address: "..."} (protected)
GET  /api/claims                  → список доступных клеймов (protected)
POST /api/claims                  → выполнить клейм (protected)
```

### User Store
**Файл:** `backend/internal/store/memory.go` — добавить `users map[string]*models.User` с методами `CreateUser`, `GetUser`, `UpdateUser`, `GetWalletForGitHub`.

### Новые модели
**Файл:** `backend/internal/models/models.go`
- `User{GitHubUsername, GitHubID, AvatarURL, WalletAddress, CreatedAt}`
- `StateFunded = "funded"`, `StateCompleted = "completed"`
- `Allocation` += `Claimed bool`, `ClaimantWallet string`

### Config
**Файл:** `backend/internal/config/config.go` — добавить `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `JWT_SECRET`, `FRONTEND_URL`.

---

## Фаза 3: Backend Solana Client — Escrow + Claim

**Файл:** `backend/internal/solana/client.go`

- `CreateCampaign` — теперь принимает `sponsorPubkey`, передаёт как аккаунт в инструкцию. Seeds меняются на `["campaign", campaign_id]`.
- `GetVaultPDA(campaignPDA)` — деривация vault адреса для возврата фронтенду.
- `ClaimAllocation(ctx, campaignID, contributorGitHub, walletAddress)` — строит и отправляет `claim` инструкцию.
- `decodeCampaignAccount` — обновить парсинг для новых полей (sponsor, vault_bump, total_claimed, claimed/claimant в аллокациях).
- `CreateCampaignResponse` += `vault_address`, `campaign_pda`.

---

## Фаза 4: AI Impact Engine

### Расширенный сбор данных с GitHub
**Файл:** `backend/internal/github/client.go`

Новые методы:
- `FetchContributorsDetailed(ctx, repo)` — основной entry point
- `FetchPRsWithDiffs(ctx, repo)` — merged PRs + file changes + patches
- `FetchPRDiff(ctx, owner, repo, prNumber)` — `/pulls/{n}/files` (с полем `patch`)
- `FetchReviews(ctx, owner, repo, prNumber)` — `/pulls/{n}/reviews`

Ограничения на размер:
- Max 10 контрибьюторов
- Max 5 самых значимых PR на контрибьютора
- Max 3 самых значимых файла на PR
- Max 50 строк diff на файл
- Параллельные запросы с ограничением (5 concurrent)

### Новый промпт для AI
**Файл:** `backend/internal/ai/allocator.go`

Мультимерная оценка с весами:

| Dimension | Weight | Что оценивает |
|-----------|--------|---------------|
| **Impact & Significance** | 35% | Решает ли критичную проблему? Новый алгоритм vs CRUD? |
| **Code Complexity & Novelty** | 25% | Сложная логика, уникальный подход, не бойлерплейт |
| **Scope & Consistency** | 20% | Объём осмысленных изменений, стабильность контрибуций |
| **Quality Signals** | 10% | Ревью фидбек, тесты, документация |
| **Community** | 10% | Ревью другим, помощь контрибьюторам |

Промпт отправляет AI **реальные диффы** кода, не только метрики. AI видит что именно написал каждый контрибьютор и оценивает значимость.

Формат ответа:
```json
[{
  "contributor": "username",
  "percentage": 5000,
  "scores": {"impact": 85, "complexity": 90, "scope": 60, "quality": 70, "community": 50},
  "reasoning": "Implemented novel caching algorithm that reduced latency 10x..."
}]
```

Детерминистический фолбек тоже обновляется: `weight = commits*3 + PRs*5 + reviews*2 + filesChanged*2`.

---

## Фаза 5: Frontend — Auth + Funding + Claims

### Auth Context
**Новый файл:** `frontend/src/contexts/AuthContext.tsx` — React context с JWT в localStorage, auto-validate на mount.

### Новые страницы
- `frontend/src/pages/AuthCallback.tsx` — обработка OAuth redirect, сохранение JWT
- `frontend/src/pages/Profile.tsx` — профиль, привязка кошелька, список клеймов

### Новые компоненты
- `frontend/src/components/GitHubLoginButton.tsx`
- `frontend/src/components/ClaimCard.tsx` — карточка клейма с кнопкой

### Обновления существующих файлов

**`CreateCampaign.tsx`** — после создания кампании добавляется шаг фандинга:
1. Backend возвращает `vault_address`
2. Frontend строит `SystemProgram.transfer` от кошелька спонсора к vault
3. `useWallet().sendTransaction()` подписывает через Phantom
4. Двухшаговый визард: Create → Fund

**`CampaignDetails.tsx`** — новые стейты (Funded), статус клеймов по аллокациям, кнопка Claim для залогиненного контрибьютора.

**`Layout.tsx`** — GitHub login кнопка + ссылка на профиль в навигации.

**`App.tsx`** — новые роуты `/auth/callback`, `/profile`, обёртка `AuthProvider`.

**`api/client.ts`** — auth header injection, новые методы: `getGitHubAuthURL`, `githubCallback`, `getMe`, `linkWallet`, `listClaims`, `claim`.

**`types/index.ts`** — `User`, `ClaimableAllocation`, обновлённые `Campaign` и `Allocation`.

---

## Порядок реализации

```
Фаза 1 (Solana Program)  ──────────────────┐
                                             ├──→ Фаза 3 (Backend Solana Client) ──┐
Фаза 2 (Backend Auth)     ──────────────────┤                                      ├──→ Фаза 5 (Frontend)
                                             ├──→ Claim handlers                    │
Фаза 4 (AI Engine)        ═══ параллельно ══┘                                      │
                                                                                    │
Фаза 6 (GitHub App)       ═══ опционально, после Фазы 2+5 ════════════════════════╛
```

Фазы 1, 2, 4 (AI) можно делать **параллельно**. Фаза 3 зависит от 1. Фаза 5 зависит от 2+3. Фаза 6 опциональна.

---

## Фаза 6 (опционально): GitHub App — уведомления в PR

### Концепция
Если владелец репозитория установил наш GitHub App, после финализации кампании бот автоматически комментирует в закрытые/merged PR каждого контрибьютора: сколько ему начислено, ссылка на клейм.

### GitHub App Setup
- Permissions: `pull_requests: write` (для комментирования), `metadata: read`
- Events: не нужны (мы пишем, не слушаем)
- Регистрация: `https://github.com/settings/apps/new`

### Backend
**Новый файл:** `backend/internal/githubapp/client.go`
- Аутентификация через JWT (App ID + private key → installation access token)
- `PostAllocationComment(ctx, repo, prNumber, contributor, amount, claimURL)` — создаёт комментарий в PR
- Зависимость: `github.com/bradleyfalzon/ghinstallation/v2`

**Обновление:** `backend/internal/http/handlers.go` — в `Finalize()` после успешной финализации:
1. Проверить, установлен ли GitHub App на этом репо (GET `/repos/{owner}/{repo}/installation`)
2. Если да — для каждого контрибьютора найти его последний merged PR и оставить комментарий:
   ```
   🎉 @username, you earned X.XX SOL (YY.Y%) for your contributions to this campaign!
   → Claim your reward: https://repobounty.ai/claims
   ```
3. Если App не установлен — пропустить молча

### Config
- `GITHUB_APP_ID`, `GITHUB_APP_PRIVATE_KEY` (PEM) — опциональные, фича отключена без них

### Frontend
- На странице кампании (для owner'а репо): баннер "Install RepoBounty GitHub App to notify contributors in PRs" со ссылкой на установку

### Зависимости
- Зависит от Фазы 2 (auth) и финализации
- Полностью опциональна — система работает без неё

---

## Верификация

1. **Solana Program:** `anchor test` — полный lifecycle: create → fund → finalize → claim
2. **Backend Auth:** `curl` тесты: OAuth flow, JWT validation, wallet linking
3. **Backend Claim:** create campaign → fund → finalize → claim via API
4. **AI Engine:** `POST /campaigns/{id}/finalize-preview` на реальном репо — проверить что диффы в промпте, scores в ответе
5. **E2E:** browser — создать кампанию, зафандить через Phantom, финализировать, залогиниться как контрибьютор, привязать кошелёк, забрать награду
6. **GitHub App (опционально):** финализировать кампанию на репо с установленным App — проверить комментарий в PR
