# RepoBounty AI — Полная инструкция по запуску и деплою

## Содержание

1. [Требования](#1-требования)
2. [Локальный запуск (разработка)](#2-локальный-запуск-разработка)
3. [Запуск через Docker](#3-запуск-через-docker)
4. [Настройка внешних сервисов](#4-настройка-внешних-сервисов)
5. [Solana: сборка и деплой смарт-контракта](#5-solana-сборка-и-деплой-смарт-контракта)
6. [Подключение к devnet (тестовая сеть)](#6-подключение-к-devnet-тестовая-сеть)
7. [Подключение к mainnet-beta (основная сеть)](#7-подключение-к-mainnet-beta-основная-сеть)
8. [Полный e2e flow](#8-полный-e2e-flow)
9. [Устранение проблем](#9-устранение-проблем)

---

## 1. Требования

### Системные

| Компонент | Версия |
|-----------|--------|
| Go | 1.25+ |
| Node.js | 20+ |
| Rust | 1.79+ (stable) |
| Solana CLI | 1.18+ (agave) |
| Anchor CLI | 0.30.1 |
| Docker & Docker Compose | latest |

### Установка Solana CLI

```bash
# Установка (macOS/Linux)
sh -c "$(curl -sSfL https://release.anza.xyz/stable/install)"

# Добавить в PATH (~/.zshrc или ~/.bashrc)
export PATH="$HOME/.local/share/solana/install/active_release/bin:$PATH"

# Проверка
solana --version
```

### Установка Anchor CLI

```bash
# Через cargo
cargo install --git https://github.com/coral-xyz/anchor --tag v0.30.1 anchor-cli

# Проверка
anchor --version
```

---

## 2. Локальный запуск (разработка)

### 2.1. Клонирование и подготовка

```bash
git clone <repo-url> repobounty-ai
cd repobounty-ai
```

### 2.2. Настройка переменных окружения

```bash
cd backend
cp .env.example .env
```

Отредактируй `.env`:

```env
# === Обязательные ===
PORT=8080
JWT_SECRET=your-secret-minimum-32-characters-long-string
FRONTEND_URL=http://localhost:3000

# === GitHub ===
GITHUB_TOKEN=ghp_xxxxxxxxxxxx          # Personal Access Token (classic), scopes: repo, read:user
GITHUB_CLIENT_ID=Ov23li...             # OAuth App → Client ID
GITHUB_CLIENT_SECRET=...               # OAuth App → Client Secret

# === Solana ===
SOLANA_RPC_URL=https://api.devnet.solana.com
SOLANA_PRIVATE_KEY=<base58-или-json>   # Keypair backend-authority (см. раздел 5)
PROGRAM_ID=97t3t188wnRoogkD8SoZKWaWbP9qDdN9gUwS4Bdw7Qdo

# === AI (опционально) ===
OPENROUTER_API_KEY=sk-or-...           # Без ключа работает deterministic fallback
MODEL=nvidia/nemotron-3-super-120b-a12b:free

# === Storage ===
DATABASE_PATH=repobounty.db            # Пустое = in-memory
```

> **Без внешних ключей** backend запустится в mock-режиме: mock GitHub data, deterministic AI, mock Solana transactions.

### 2.3. Запуск backend

```bash
cd backend
go mod tidy
go run ./cmd/api
# → Listening on :8080
```

### 2.4. Запуск frontend

```bash
cd frontend
npm install
npm run dev
# → http://localhost:3000
# Vite проксирует /api → http://localhost:8080
```

### 2.5. Проверка

```bash
curl http://localhost:8080/api/health
# → {"status":"ok"}
```

---

## 3. Запуск через Docker

```bash
# Из корня проекта
docker compose up --build

# Frontend: http://localhost:5173
# Backend:  http://localhost:8080
```

Docker Compose поднимает 3 сервиса:
- `solana-program` — собирает Anchor-программу (build-only контейнер)
- `backend` — Go API сервер
- `frontend` — nginx + собранный React SPA, проксирует `/api` на backend

Переменные окружения берутся из `.env` в корне или из `backend/.env`.

---

## 4. Настройка внешних сервисов

### 4.1. GitHub Personal Access Token

1. https://github.com/settings/tokens → **Generate new token (classic)**
2. Scopes: `repo`, `read:user`, `user:email`
3. Скопируй токен → `GITHUB_TOKEN` в `.env`

### 4.2. GitHub OAuth App (для логина пользователей)

1. https://github.com/settings/developers → **New OAuth App**
2. Application name: `RepoBounty AI`
3. Homepage URL: `http://localhost:3000`
4. Authorization callback URL: `http://localhost:3000/auth/callback`
5. Скопируй Client ID → `GITHUB_CLIENT_ID`
6. Generate client secret → `GITHUB_CLIENT_SECRET`

> Для продакшена замени URLs на реальный домен.

### 4.3. OpenRouter (AI, опционально)

1. https://openrouter.ai → Sign up → API Keys
2. Скопируй ключ → `OPENROUTER_API_KEY`
3. По умолчанию используется бесплатная модель `nvidia/nemotron-3-super-120b-a12b:free`

---

## 5. Solana: сборка и деплой смарт-контракта

### 5.1. Генерация keypair (если нет)

```bash
# Keypair для backend authority (финализирует кампании)
solana-keygen new -o ~/.config/solana/authority.json

# Посмотреть публичный ключ
solana-keygen pubkey ~/.config/solana/authority.json

# Keypair для деплоя программы (если нужен новый program ID)
solana-keygen new -o program/target/deploy/repobounty-keypair.json
```

### 5.2. Настройка Solana CLI

```bash
# Для devnet
solana config set --url https://api.devnet.solana.com

# Указать кошелек по умолчанию
solana config set --keypair ~/.config/solana/authority.json

# Проверка
solana config get
```

### 5.3. Получение SOL для деплоя (devnet)

```bash
solana airdrop 5
solana balance
```

### 5.4. Сборка программы

```bash
cd program
yarn install
anchor build
# Артефакты: target/deploy/repobounty.so + repobounty-keypair.json
```

> В Docker используется `anchor build --no-idl` (IDL генерация отключена для совместимости).

### 5.5. Деплой на devnet

```bash
anchor deploy --provider.cluster devnet
```

Anchor выведет Program ID. Если используешь существующий keypair — ID останется прежним:
```
Program Id: 97t3t188wnRoogkD8SoZKWaWbP9qDdN9gUwS4Bdw7Qdo
```

### 5.6. Деплой на mainnet-beta

```bash
anchor deploy --provider.cluster mainnet
```

### 5.7. Обновление Program ID

Если ты задеплоил с новым keypair и получил новый Program ID:

1. **Anchor.toml** — обнови `[programs.devnet]` и `[programs.localnet]`:
   ```toml
   [programs.devnet]
   repobounty = "<NEW_PROGRAM_ID>"
   ```

2. **Программа (lib.rs)** — обнови `declare_id!`:
   ```rust
   declare_id!("<NEW_PROGRAM_ID>");
   ```

3. **Backend (.env)** — обнови:
   ```env
   PROGRAM_ID=<NEW_PROGRAM_ID>
   ```

4. Пересобери и передеплой:
   ```bash
   anchor build
   anchor deploy --provider.cluster devnet
   ```

### 5.8. Запуск тестов программы

```bash
cd program
anchor test
# Запускает localnet validator, деплоит программу, запускает ts-mocha тесты
```

### 5.9. Использование своего keypair для backend authority

Приватный ключ в `.env` принимает два формата:

```env
# Формат 1: Base58
SOLANA_PRIVATE_KEY=4wBqpZM9...длинная_строка_base58

# Формат 2: JSON массив байтов
SOLANA_PRIVATE_KEY=[174,23,45,...]
```

Чтобы получить Base58 из JSON keypair:
```bash
cat ~/.config/solana/authority.json
# → [174,23,45,...] — это можно вставить напрямую
```

---

## 6. Подключение к devnet (тестовая сеть)

Devnet — основная среда для разработки и демонстрации. SOL бесплатный.

### Backend

```env
SOLANA_RPC_URL=https://api.devnet.solana.com
```

### Frontend

В `frontend/src/main.tsx` (уже настроено по умолчанию):
```typescript
const endpoint = clusterApiUrl("devnet");
```

### Solana CLI

```bash
solana config set --url https://api.devnet.solana.com
```

### Получение тестовых SOL

```bash
# Через CLI
solana airdrop 5

# Через faucet (если CLI лимитирует)
# https://faucet.solana.com
```

### Phantom Wallet

1. Открой Phantom → Settings → Developer Settings
2. Включи **Testnet Mode**
3. Выбери **Solana Devnet**
4. Запроси SOL через faucet в Phantom или через CLI

### Просмотр транзакций

Все транзакции видны на [Solana Explorer (devnet)](https://explorer.solana.com/?cluster=devnet).

---

## 7. Подключение к mainnet-beta (основная сеть)

> **Внимание:** на mainnet используются реальные SOL. Убедись, что контракт протестирован на devnet.

### 7.1. Backend

```env
SOLANA_RPC_URL=https://api.mainnet-beta.solana.com
```

Для продакшена рекомендуется платный RPC (Helius, QuickNode, Alchemy):
```env
SOLANA_RPC_URL=https://mainnet.helius-rpc.com/?api-key=YOUR_KEY
```

### 7.2. Frontend

Измени `frontend/src/main.tsx`:
```typescript
// Было:
const endpoint = clusterApiUrl("devnet");

// Стало (публичный RPC):
const endpoint = clusterApiUrl("mainnet-beta");

// Или кастомный RPC:
const endpoint = "https://mainnet.helius-rpc.com/?api-key=YOUR_KEY";
```

### 7.3. Деплой программы на mainnet

```bash
# Переключи CLI
solana config set --url https://api.mainnet-beta.solana.com

# Убедись, что на кошельке деплойера достаточно SOL (~3-5 SOL)
solana balance

# Деплой
cd program
anchor deploy --provider.cluster mainnet
```

### 7.4. Обнови Anchor.toml

```toml
[programs.mainnet]
repobounty = "<PROGRAM_ID>"

[provider]
cluster = "mainnet"
```

### 7.5. Phantom Wallet

1. Отключи Testnet Mode в настройках Phantom
2. Сеть автоматически переключится на mainnet

### 7.6. Чеклист перед mainnet

- [ ] Все тесты проходят на devnet
- [ ] Полный e2e flow протестирован (create → fund → finalize → claim)
- [ ] Программа проверена на уязвимости (overflow, authority checks, PDA seed collisions)
- [ ] Backend authority keypair в безопасном хранилище (не в git!)
- [ ] Rate limiting включен
- [ ] CORS ограничен вашим доменом
- [ ] HTTPS настроен на фронтенде и бэкенде

---

## 8. Полный e2e flow

Пошаговая проверка всего сценария:

### 1. Запуск

```bash
# Терминал 1: backend
cd backend && go run ./cmd/api

# Терминал 2: frontend
cd frontend && npm run dev
```

### 2. Настройка Phantom

- Откройте Phantom в браузере
- Переключитесь на Devnet
- Запросите SOL: `solana airdrop 5 <ваш_phantom_адрес>`

### 3. Создание кампании

1. Откройте http://localhost:3000
2. Подключите Phantom wallet (кнопка Connect Wallet)
3. Нажмите Create Campaign
4. Заполните: репозиторий (`owner/repo`), сумма (SOL), дедлайн (мин. 15 мин от текущего времени)
5. Подтвердите создание → на шаге 2 подпишите транзакцию funding через Phantom

### 4. Ожидание дедлайна

- Auto-finalize worker проверяет кампании каждые 5 минут
- Или дождитесь дедлайна и нажмите Finalize вручную на странице кампании

### 5. Finalize

- Backend собирает GitHub данные → AI распределяет → транзакция finalize на Solana
- Кампания переходит в статус Finalized
- Аллокации видны на странице кампании

### 6. Claim

1. Залогиньтесь через GitHub (Login with GitHub)
2. Привяжите wallet на странице Profile
3. Перейдите на страницу кампании или Profile
4. Нажмите Claim → SOL переведутся из vault на ваш кошелек

### 7. Проверка

```bash
# Баланс vault (должен уменьшиться)
solana balance <vault_pda_address>

# Баланс получателя (должен увеличиться)
solana balance <contributor_wallet>
```

---

## 9. Устранение проблем

### Backend не стартует

```
Error: failed to parse SOLANA_PRIVATE_KEY
```
→ Проверь формат ключа. Должен быть Base58 строка или JSON массив `[u8,u8,...]`.

### "Mock mode" в логах

```
Using mock Solana client
```
→ `SOLANA_PRIVATE_KEY` или `PROGRAM_ID` не заданы. Задай для реальных транзакций.

### anchor build fails

```
error: package `solana-program v...` not found
```
→ Проверь `rust-toolchain.toml` в `program/`. Нужен Rust 1.87+ и `solana-cli` в PATH.

### Phantom не подключается

→ Убедись, что Phantom на правильной сети (Devnet vs Mainnet). В testnet mode Phantom не видит mainnet.

### Transaction simulation failed

```
Error: Blockhash not found
```
→ Devnet иногда нестабильна. Повтори через 5-10 секунд или используй кастомный RPC.

### "Insufficient funds"

```
Transfer: insufficient lamports
```
→ Пополни кошелек: `solana airdrop 5` (devnet) или переведи SOL (mainnet).

### CORS ошибки

→ Проверь `ALLOWED_ORIGINS` в `.env`. Для локальной разработки должно быть `http://localhost:3000,http://localhost:5173`.

### GitHub OAuth redirect mismatch

→ Authorization callback URL в настройках GitHub OAuth App должен совпадать с `{FRONTEND_URL}/auth/callback`.
