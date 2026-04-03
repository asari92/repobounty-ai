Ниже финальное ТЗ на смарт-контракт v1 для MVP RepoBounty AI под Solana.

Техническое задание
Смарт-контракт наград за вклад в open source репозитории GitHub

1. Цель

Разработать один Solana program, который позволяет:

* спонсору создать кампанию награждения для публичного GitHub-репозитория
* сразу при создании перевести средства кампании в escrow аккаунт программы
* после наступления дедлайна зафиксировать итоговое распределение награды между контрибьюторами по GitHub user id
* позволить контрибьюторам клеймить награду позже, даже если на момент создания и окончания кампании у них не было Solana-кошелька
* поддержать два режима оплаты комиссии за claim:

  * пользователь сам платит комиссию, если у него есть SOL
  * backend оплачивает комиссию, если у пользователя SOL нет
* хранить средства кампании изолированно от других кампаний
* исключить повторный claim
* позволить спонсору вернуть невостребованный остаток через 365 дней после финализации

2. Общая архитектура

On-chain:

* Campaign account
* ClaimRecord account
* Config account
* Escrow authority PDA
* Escrow token account для каждой кампании

Off-chain:

* GitHub OAuth
* проверка существования и публичности репозитория
* ежедневная синхронизация данных по активным репозиториям
* хранение mirror репозитория и/или снапшотов в БД
* AI-анализ вклада контрибьюторов
* подготовка итоговой аллокации
* сборка и отправка finalize и claim транзакций

3. Основные бизнес-правила

3.1. Кампания создается только для существующего публичного GitHub-репозитория.

3.2. Кампания создается только вместе с депозитом. Пустые кампании запрещены.

3.3. Для каждой кампании создается отдельный escrow token account.

3.4. Дедлайн кампании:

* минимум: текущее время + 24 часа
* максимум: текущее время + 365 дней

3.5. После дедлайна backend финализирует кампанию и передает в контракт итоговый список получателей:

* github_user_id
* github_username
* amount в минимальных единицах токена

3.6. Все суммы в контракте и в backend считаются только в integer units:

* для SOL это lamports

3.7. При финализации вся сумма кампании должна быть распределена полностью до последней минимальной единицы.
Если после округления есть остаток, он добавляется top contributor с максимальным impact score.
В итоге:
sum(allocation amounts) == total_amount кампании

3.8. Контрибьютор может не иметь Solana-кошелька ни при создании, ни при дедлайне кампании.
Кошелек нужен только в момент claim.

3.9. Claim возможен в течение 365 дней после финализации.

3.10. После истечения 365 дней спонсор может вернуть себе невостребованный остаток.

3.11. Если репозиторий после создания кампании был скрыт, удален или переименован:

* это не отменяет кампанию
* быстрый refund не разрешается
* финализация должна выполняться по ранее собранным данным backend
* возврат остатка спонсору возможен только по общему правилу после наступления дедлайна + 365 дней 

4. Работа с GitHub-репозиторием

4.1. При создании кампании backend обязан проверить:

* репозиторий существует
* репозиторий публичный
* у репозитория есть стабильный github_repo_id

4.2. В on-chain Campaign необходимо хранить:

* github_repo_id как основной идентификатор
* repo_owner
* repo_name
* repo_url вычисляется off-chain из owner+name, на on-chain не хранится

4.3. Пока кампания активна, backend должен не реже одного раза в сутки синхронизировать данные по репозиторию:

* коммиты
* авторов
* временные метки
* diff/statistics
* при необходимости PR metadata

4.4. Для надежности backend должен иметь зеркало репозитория под своим контролем.
Зеркало может быть реализовано:

* как mirror clone на своем сервере
* либо как отдельная подконтрольная репозитория/копия

4.5. Финализация должна выполняться по данным из:

* текущего состояния репозитория, если он доступен
* либо по ранее накопленному зеркалу/снапшотам, если исходный репозиторий скрыт, удален или переименован

5. Роли

5.1. Sponsor

* создает кампанию
* вносит депозит
* может получить невостребованный остаток после claim window

5.2. Finalize Authority

* backend-кошелек
* единственная роль, которая может вызывать finalize_campaign

5.3. Claim Authority

* backend-кошелек или тот же кошелек
* используется для режима claim через backend

5.4. Contributor

* логинится через GitHub на сайте
* подключает или создает Solana-кошелек
* инициирует claim

5.5. Admin

* инициализирует Config
* может менять authority и паузить программу, если это предусмотрено

6. Сущности on-chain

6.1. Config
Поля:

* admin: Pubkey
* finalize_authority: Pubkey
* claim_authority: Pubkey
* paused: bool
* bump: u8

6.2. Campaign
Поля:

* campaign_id: u64
* sponsor: Pubkey
* token_mint: Pubkey
* escrow_token_account: Pubkey
* github_repo_id: u64
* repo_owner: String (max 39 chars)
* repo_name: String (max 100 chars)
* created_at: i64
* deadline_at: i64
* claim_deadline_at: i64
* total_amount: u64
* allocated_amount: u64
* claimed_amount: u64
* allocations_count: u32
* claimed_count: u32
* status: u8
* bump: u8

Статусы:

* 0 Active
* 1 Finalizing (батчевая финализация в процессе)
* 2 Finalized
* 3 Closed

6.3. ClaimRecord
Поля:

* campaign: Pubkey
* github_user_id: u64
* github_username: String
* amount: u64
* claimed: bool
* claimed_to_wallet: Option<Pubkey>
* claimed_at: Option<i64>
* bump: u8

7. PDA

7.1. Campaign PDA
seed:
["campaign", sponsor_pubkey, campaign_id]

7.2. Escrow Authority PDA
seed:
["escrow_authority", campaign_pubkey]

7.3. ClaimRecord PDA
seed:
["claim", campaign_pubkey, github_user_id]

8. Инструкции контракта

8.1. initialize_config

Назначение:
первичная инициализация программы

Кто вызывает:
admin

Параметры:

* finalize_authority
* claim_authority

Что делает:

* создает Config
* записывает admin
* записывает finalize_authority
* записывает claim_authority
* paused = false

Проверки:

* Config еще не существует

8.1.1. update_config

Назначение:
обновить authorities или паузу программы

Кто вызывает:
admin

Параметры (все опциональные):

* new_admin: Option<Pubkey>
* new_finalize_authority: Option<Pubkey>
* new_claim_authority: Option<Pubkey>
* paused: Option<bool>

Проверки:

* signer == config.admin

Что делает:

* обновляет только переданные (Some) поля
* позволяет ротацию ключей при компрометации
* позволяет паузить/возобновлять программу

8.2. create_campaign_with_deposit

Назначение:
создать кампанию и сразу перевести средства в escrow

Кто вызывает:
sponsor

Параметры:

* campaign_id: u64
* github_repo_id: u64
* repo_owner: String (max 39 chars)
* repo_name: String (max 100 chars)
* deadline_at: i64
* total_amount: u64

Accounts:

* sponsor signer
* config
* campaign PDA
* escrow authority PDA
* escrow token account
* sponsor token account
* token mint
* token program
* associated token program
* system program

Проверки:

* программа не paused
* deadline_at >= now + 86400 (24h)
* deadline_at <= now + 31536000 (365d)
* total_amount > 0
* repo_owner.len() <= 39
* repo_name.len() <= 100
* sponsor token account соответствует token_mint
* на sponsor token account хватает средств
* campaign PDA еще не существует

Что делает:

* создает Campaign
* создает escrow token account кампании
* переводит total_amount из sponsor token account в escrow token account
* устанавливает:

  * created_at = now
  * claim_deadline_at = deadline_at + 365 дней
  * allocated_amount = 0
  * claimed_amount = 0
  * allocations_count = 0
  * claimed_count = 0
  * status = Active

Важно:
если перевод средств не удался, кампания не считается созданной

8.3. finalize_campaign (батчевая)

Назначение:
зафиксировать on-chain итоговое распределение награды.
Поддерживает батчевый вызов — до MAX_ALLOCATIONS_PER_BATCH (5) allocations за транзакцию.
Это необходимо из-за лимита размера транзакции Solana (~1232 байт).

Кто вызывает:
только finalize_authority

Параметры:

* allocations: список, где каждый элемент содержит:
  * github_user_id: u64
  * github_username: String
  * amount: u64
* is_final_batch: bool — true, если это последний батч

Accounts:

* finalize_authority signer
* config
* campaign
* system program
* remaining accounts для ClaimRecord PDA

Проверки:

* программа не paused
* signer == config.finalize_authority
* campaign.status == Active или Finalizing
* now >= campaign.deadline_at
* список allocations не пуст
* len(allocations) <= MAX_ALLOCATIONS_PER_BATCH (5)
* нет дубликатов github_user_id в текущем батче
* amount > 0 для каждой записи
* ClaimRecord для этих github_user_id еще не существует
* если is_final_batch: allocated_amount + sum(batch amounts) == campaign.total_amount

Что делает:

* для каждого allocation создает ClaimRecord PDA
* записывает github_user_id, github_username, amount
* claimed = false
* campaign.allocated_amount += sum(batch amounts)
* campaign.allocations_count += len(allocations)
* если is_final_batch == false:
  * campaign.status = Finalizing
* если is_final_batch == true:
  * проверяет allocated_amount == total_amount
  * campaign.status = Finalized

Важно:
финализация не переводит токены контрибьюторам, а только фиксирует их права на claim.
Между батчами кампания находится в статусе Finalizing — claim невозможен до полной финализации.

8.4. claim_user_paid

Назначение:
claim, при котором комиссию оплачивает пользователь.
Backend обязательно выступает co-signer для on-chain авторизации.

Кто вызывает:
пользователь (fee payer) + claim_authority (co-signer)

Параметры:

* github_user_id
* recipient_wallet

Accounts:

* user signer (fee payer)
* claim_authority signer (co-signer для авторизации)
* config
* campaign
* claim_record
* escrow authority PDA
* escrow token account
* recipient token account
* token mint
* token program
* associated token program
* system program

Проверки:

* программа не paused
* signer == config.claim_authority (co-signer авторизация)
* campaign.status == Finalized
* now <= campaign.claim_deadline_at
* claim_record относится к campaign
* claim_record.github_user_id == github_user_id
* claim_record.claimed == false
* claim_record.amount > 0
* в escrow достаточно средств

Что делает:

* при необходимости создает recipient token account
* переводит claim_record.amount из escrow в recipient token account
* claim_record.claimed = true
* claim_record.claimed_to_wallet = recipient_wallet
* claim_record.claimed_at = now
* campaign.claimed_amount += amount
* campaign.claimed_count += 1
* если campaign.claimed_count == campaign.allocations_count:

  * campaign.status = Closed

Важно:
claim_authority (backend) выступает обязательным co-signer — это предотвращает
несанкционированный claim чужой аллокации. Без co-signer злоумышленник мог бы
вызвать claim с публично известным github_user_id и перенаправить токены на свой кошелёк.
Разница между claim_user_paid и claim_backend_paid — только в том, кто платит fee.

8.5. claim_backend_paid

Назначение:
claim, при котором комиссию оплачивает backend

Кто вызывает:
claim_authority

Параметры:

* github_user_id
* recipient_wallet

Accounts:

* claim_authority signer
* config
* campaign
* claim_record
* escrow authority PDA
* escrow token account
* recipient token account
* token mint
* token program
* associated token program
* system program

Проверки:

* программа не paused
* signer == config.claim_authority
* campaign.status == Finalized
* now <= campaign.claim_deadline_at
* claim_record относится к campaign
* claim_record.github_user_id == github_user_id
* claim_record.claimed == false
* claim_record.amount > 0
* в escrow достаточно средств

Что делает:

* при необходимости создает recipient token account
* переводит claim_record.amount из escrow в recipient token account
* claim_record.claimed = true
* claim_record.claimed_to_wallet = recipient_wallet
* claim_record.claimed_at = now
* campaign.claimed_amount += amount
* campaign.claimed_count += 1
* если campaign.claimed_count == campaign.allocations_count:

  * campaign.status = Closed

8.6. refund_unclaimed

Назначение:
вернуть спонсору невостребованный остаток после истечения claim window

Кто вызывает:
sponsor

Accounts:

* sponsor signer
* campaign
* escrow authority PDA
* escrow token account
* sponsor refund token account
* token program

Проверки:

* campaign.sponsor == signer
* campaign.status == Finalized или Closed (при наличии остатка)
* now > campaign.claim_deadline_at
* на escrow есть остаток > 0
* НЕ проверяется paused — refund всегда доступен

Что делает:

* переводит весь остаток из escrow на sponsor refund token account
* campaign.status = Closed

9. Подтверждение успешности claim

9.1. Источником истины считается подтвержденная транзакция в Solana.

9.2. Claim считается успешным только если:

* транзакция подтверждена
* claim_record.claimed == true
* claimed_to_wallet заполнен
* amount списан из escrow и зачислен на recipient token account

9.3. Если claim-транзакция упала:

* состояние не меняется
* claim_record.claimed остается false
* средства не считаются выплаченными
* claim можно повторить

10. Что обязан делать backend

10.1. До create_campaign_with_deposit:

* проверить, что репозиторий существует
* проверить, что репозиторий публичный
* получить github_repo_id
* сохранить метаданные кампании в БД

10.2. Пока кампания Active:

* не реже раза в сутки синхронизировать данные репозитория
* обновлять зеркало и/или snapshots
* хранить mapping contributor activity

10.3. После дедлайна:

* провести финальный анализ по зеркалу и накопленным данным
* посчитать impact score
* перевести impact score в точные integer allocations
* распределить остаток округления top contributor
* сформировать список allocations
* вызвать finalize_campaign

10.4. При claim:

* проверить GitHub OAuth сессию пользователя
* определить github_user_id
* проверить, что claim_record существует и еще не забран
* определить, кто платит комиссию:

  * если у пользователя есть SOL, можно использовать user-paid flow
  * если SOL нет, использовать backend-paid flow
* собрать и отправить нужную транзакцию

11. Обязательные edge cases

11.1. Репозиторий был переименован

* кампания не ломается
* используется github_repo_id
* отображаемое имя обновляется off-chain

11.2. Репозиторий стал private

* кампания не отменяется
* быстрый refund не разрешен
* финализация проводится по ранее накопленным данным

11.3. Репозиторий удален

* кампания не отменяется
* быстрый refund не разрешен
* финализация проводится по ранее накопленным данным, если они есть

11.4. У пользователя не было кошелька до окончания кампании

* это допустимо
* кошелек создается/подключается в момент claim

11.5. У пользователя нет SOL на комиссию

* используется claim_backend_paid

11.6. У пользователя есть SOL на комиссию

* может использоваться claim_user_paid

11.7. Остатка из-за округления быть не должно

* вся сумма должна быть распределена при finalize полностью
* sum(allocations) строго равна total_amount

12. Ошибки контракта

Минимальный набор:

* ProgramPaused
* DeadlineTooSoon (< 24h)
* DeadlineTooFar (> 365d)
* InvalidAmount
* CampaignNotActive
* CampaignNotActiveOrFinalizing
* CampaignNotFinalized
* CampaignClosed
* DeadlineNotReached
* ClaimWindowExpired
* ClaimWindowNotExpired (для refund — нельзя раньше)
* Unauthorized
* DuplicateAllocation
* AllocationTotalMismatch
* EmptyAllocations
* TooManyAllocations (> MAX_ALLOCATIONS_PER_BATCH)
* ClaimAlreadyClaimed
* EscrowInsufficientFunds
* EscrowEmpty
* InvalidSponsor
* InvalidClaimRecord
* RepoOwnerTooLong
* RepoNameTooLong
* InvalidGithubUserId
* GithubUsernameTooLong
* ZeroAllocationAmount
* ArithmeticOverflow

13. Минимальный UX flow

13.1. Sponsor

* подключает кошелек
* выбирает публичный GitHub repo
* указывает сумму и дедлайн
* подписывает create_campaign_with_deposit

13.2. Backend

* зеркалит репозиторий и собирает данные ежедневно

13.3. После дедлайна

* backend считает impact
* backend вызывает finalize_campaign

13.4. Contributor

* заходит на сайт
* логинится через GitHub
* подключает или создает Solana-кошелек
* нажимает Claim
* если есть SOL, может заплатить комиссию сам
* если SOL нет, комиссию платит backend

13.5. Через 365 дней после финализации

* sponsor видит кнопку "Вернуть невостребованные средства"
* вызывает refund_unclaimed

14. Что не входит в MVP on-chain

Не реализовывать в первой версии:

* on-chain проверку GitHub OAuth
* on-chain проверку существования репозитория
* on-chain AI анализ
* автоматические действия по времени без внешней транзакции
* несколько программ вместо одной
* общую кассу для всех кампаний

15. Итоговая формула MVP

# 1 campaign

1 Campaign account
+
1 отдельный escrow token account
+
N ClaimRecord

Create:
sponsor создает кампанию и сразу вносит депозит

Finalize (батчевая):
backend после дедлайна передает список github_user_id и точных amount
до 5 allocations за транзакцию, несколько батчей при необходимости

Claim:
каждый contributor забирает свою долю отдельно
claim_authority обязательно co-sign для авторизации

Refund:
через 365 дней sponsor может вернуть невостребованный остаток

16. Изменения относительно исходной версии

16.1. [CRITICAL] claim_user_paid: добавлен обязательный co-signer (claim_authority).
Без этого злоумышленник мог бы украсть чужую аллокацию, зная публичный github_user_id.

16.2. [CRITICAL] finalize_campaign: реализована батчевая финализация (MAX_ALLOCATIONS_PER_BATCH = 5).
Одна транзакция Solana (~1232 байт) не вмещает более 8 allocations с init ClaimRecord.

16.3. [HIGH] Добавлен статус Finalizing для батчевой финализации.

16.4. [HIGH] claim_user_paid: добавлена проверка paused (была пропущена в исходной версии).

16.5. [HIGH] repo_url убран из on-chain state (вычисляется из owner+name).

16.6. [HIGH] Добавлена инструкция update_config для ротации ключей и управления паузой.

16.7. [HIGH] sponsor_token_account убран из Campaign state (нужен только при создании).

16.8. [MEDIUM] campaign_id зафиксирован как u64.

16.9. [MEDIUM] Расширен список ошибок с 18 до 27 для полного покрытия edge cases.
