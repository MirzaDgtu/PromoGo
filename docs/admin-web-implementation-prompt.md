# Промт на реализацию PromoGo Admin Web

Ниже расположен готовый промт для отдельной сессии разработки. Перед запуском заменить значения в угловых скобках и убедиться, что владелец проекта разрешил новый цикл функциональной разработки.

---

Ты работаешь в репозитории PromoGo. Реализуй административный web-сайт раньше пользовательского мобильного приложения.

## Разрешение и границы

- Явное разрешение на разработку Admin Web: `<ДА/НЕТ, ссылка на решение или сообщение>`.
- Статус release/feature freeze: `<снят / Admin Web явно разрешён как отдельный цикл / не снят>`.
- Если разрешение не подтверждено, не меняй product-код: только сообщи о блокере.
- Mobile/Flutter не реализовывай и не смешивай с административной авторизацией.
- Не принимай за владельца проекта юридические, продуктовые или rollout-решения. Неопределённые решения оставляй предложенными и поднимай только в момент, когда они действительно блокируют следующий шаг.

## Сначала ознакомься

1. Прочитай `AGENTS.md` и следуй его правилам.
2. Для понимания проекта сначала используй существующий `graphify-out/graph.json`, затем проверяй вывод по исходникам.
3. Прочитай полностью:
   - `docs/admin-web-plan.md`;
   - `MVP-scope.md`;
   - `docs/openapi/openapi.yaml`;
   - `internal/httpserver/routes.go`;
   - `internal/domain/rbac.go`;
   - `knowledge/Decisions.md`, прежде всего DEC-004, DEC-005, DEC-008, DEC-009, DEC-023;
   - relevant ADR и открытые вопросы Group C/D/G в `knowledge/Project Questions.md`.
4. Проверь рабочее дерево и не перезаписывай чужие изменения. Не считай ожидаемые изменения `graphify-out/` причиной отменять работу.
5. Проверь фактические routes, domain types и tests. Не считай endpoint существующим только потому, что он упомянут в документации.

## Цель

Создай production-minded, но минимальный Admin Web, с помощью которого уполномоченный сотрудник может:

- войти через OIDC;
- выбрать доступную организацию и магазин;
- управлять организациями/магазинами в пределах своих прав;
- управлять memberships сотрудников;
- просматривать клиентов, баланс и историю транзакций с серверным PII masking;
- изменять, просматривать историю и откатывать loyalty config;
- создавать и отзывать scoped store API keys, видя secret только один раз;
- просматривать audit log.

Управление кассами и соединениями с 1С реализуй отдельным milestone только после согласования реальной domain-модели и протокола. Не выдавай список API-ключей за реестр оборудования.

## Архитектурные ограничения

- Сохрани три независимых security contours:
  1. staff: OIDC + PromoGo staff token;
  2. customer: OTP + customer sessions;
  3. machine principals: store/integration API keys.
- Все authorization, tenant scope, IDOR-защита и PII masking обязательны на backend. Frontend guards нужны только для UX.
- Размести frontend в `web/admin` этого репозитория.
- Используй React, TypeScript strict и Vite; маршрутизацию — React Router, server state — TanStack Query.
- Формы должны иметь typed validation. Допустимо использовать React Hook Form + Zod.
- Сгенерируй API client/types из `docs/openapi/openapi.yaml`; не поддерживай параллельные handwritten DTO.
- Выбери актуальные стабильные совместимые версии после проверки официальной документации, зафиксируй exact dependency graph lock-файлом. Не обновляй несвязанные зависимости.
- Для пилота используй same-origin deployment: SPA на `/`, backend на `/api/*`. Не добавляй wildcard CORS.
- Staff access token храни только в памяти. Не используй `localStorage`/`sessionStorage` для access token, ID token, refresh token или API-key secret.
- Не добавляй `dangerouslySetInnerHTML`, не логируй токены/ПДн/secrets.
- Ошибки должны сохранять и показывать копируемый `X-Request-Id`, не раскрывая внутренние details.
- Accessibility обязательна: semantic HTML, labels, focus management, keyboard navigation, контраст, reduced motion; действия не различаются только цветом.
- Интерфейс desktop-first, рабочий на планшете, русский первым; пользовательские строки централизуй для будущей локализации.

## Порядок работы

### Milestone 0 — contract readiness

Сначала добавь недостающие backend contracts:

1. `GET /api/v1/staff/me`: staff profile, только активные memberships, roles/scopes и вычисленные permissions.
2. `GET /api/v1/admin/organizations`: platform admin видит все; остальные — только организации активных memberships.
3. `GET /api/v1/admin/organizations/{orgID}/stores`: только доступные магазины; org-wide membership видит все в org, store-scoped — только свой store.
4. Пагинированный `GET /api/v1/admin/organizations/{orgID}/stores/{storeID}/clients` с устойчивой сортировкой/cursor, балансом и действующим masking для `support_viewer`.
5. Staff list DTO, который вместе с membership возвращает безопасные профильные поля `display_name` и `email`; UI не должен состоять из одних `staff_user_id`.

Для каждого endpoint:

- сначала напиши/обнови contract и negative tests;
- реализуй domain/repository/service/handler минимально в существующей архитектуре;
- проверь cross-org и cross-store IDOR;
- обнови OpenAPI schemas/examples/security;
- сохрани route/OpenAPI parity;
- не возвращай внутренние модели напрямую, если это раскрывает лишние поля.

Если продуктовый выбор фильтров списка клиентов блокирует реализацию, начни с минимального cursor pagination и безопасного точного поиска, явно оставив расширенные фильтры вне scope.

Отдельно проверь staff enrollment. Текущий create membership принимает OIDC `external_subject`; не подменяй это фиктивным приглашением по email. До self-service сценария согласуй один из безопасных вариантов: управляемая выдача subject IdP-администратором либо отдельный invite/request-access flow с проверенной OIDC identity. Если решения нет, сохрани subject flow как явно подписанное ограничение пилота и не объявляй onboarding сотрудников полностью self-service.

### Milestone 1 — frontend foundation

- Создай `web/admin` с понятным feature-based layout без преждевременной абстракции.
- Настрой strict typecheck, lint, unit/component tests и production build.
- Добавь generation/check команды для OpenAPI client. CI должен падать при drift generated artifacts.
- Реализуй OIDC Authorization Code + PKCE через корректную библиотеку выбранного IdP.
- Выполни ID-token exchange через `/api/v1/staff/auth/oidc`, затем загрузи `/api/v1/staff/me`.
- Реализуй controlled re-auth при истечении PromoGo token; бесконечных retry loops быть не должно.
- Создай org/store context, permission-aware navigation, route guards, error boundary и стандартные loading/empty/error/forbidden states.
- Обработай deep links и refresh любого frontend route.

Не строй собственный криптографический OIDC flow и не доверяй permissions из неподписанных browser данных.

### Milestone 2 — Pilot Admin modules

Реализуй вертикальными срезами, каждый с тестами и рабочей навигацией:

1. Dashboard/readiness выбранного магазина из безопасного application endpoint; browser не должен читать Prometheus `/metrics` напрямую.
2. Organizations and stores.
3. Staff memberships and RBAC.
4. Loyalty config, version history and rollback. Пока `Q-P0-068` не решён, явно показывай, что изменение применяется немедленно; не изобретай drafts/effective dates.
5. Clients, balance and transactions с server-defined masking.
6. API keys: list/create/revoke, scopes, expiry, last used; secret — one-time modal без возможности восстановить после закрытия.
7. Audit log; не заявляй полноту событий сверх реально реализованной.

Для destructive/irreversible действий требуй явное подтверждение с названием target. После mutation корректно invalidates/reloads server state. Double submit предотвращай на UI, но не полагайся на это вместо backend invariants.

### Milestone 3 — equipment and 1C integrations

Перед кодом подготовь короткое proposed domain decision и запроси подтверждение владельца для:

- `StoreDevice` (касса/POS, внешний ID, kind/status/version/last seen);
- `StoreIntegration` (1C/POS, направление, status, safe metadata, last success/error);
- связи device/integration с credential;
- secret storage и redaction;
- реального протокола health/connection check.

После подтверждения добавь миграции, permissions, repository/service/API/OpenAPI/tests и UI. Обязательны cross-store IDOR tests, audit, secret redaction и отсутствие raw PII payloads. Если интеграция только входящая, показывай последнюю фактическую активность; не делай фиктивную кнопку исходящего «теста соединения».

### Milestone 4 — CI, deployment and hardening

- Добавь frontend CI: frozen install, generated-client drift, typecheck, lint, unit/component tests, build.
- Добавь Playwright критических путей: login, org/store switch, loyalty update/rollback, API-key create/revoke, forbidden role.
- Используй real Postgres/Redis и контролируемый test OIDC issuer/verifier для интеграционного E2E; не используй production secrets.
- Добавь automated accessibility check.
- Добавь pinned container/static-server build и vulnerability/SBOM/secret gates по существующим правилам проекта.
- Настрой CSP и security headers, корректный SPA fallback и запрет fallback для `/api/*`.
- Обнови deployment, rollback, runbooks и regression/security review для новой поверхности.
- После изменения кода выполни `graphify update .`.

## Обязательные проверки

После каждого логического среза запускай релевантные тесты. Перед итогом должны быть зелёными:

- gofmt/vet/staticcheck/govulncheck и полный Go test suite, включая race там, где он принят CI;
- OpenAPI parity/contract tests;
- frontend typecheck, lint, unit/component tests и production build;
- Playwright critical paths;
- secret scan, dependency/container scan и SBOM job;
- миграции up/down/up и migration deploy model, если добавлялись миграции;
- pilot E2E/regression текущего backend.

Не исправляй несвязанные проблемы молча. Если baseline уже красный, зафиксируй точную команду и отличие до/после.

## Definition of Done

Работа считается завершённой только если:

- уполномоченный сотрудник выполняет pilot onboarding и настройку без curl/SQL;
- роли и store/org scopes проверены как через UI, так и прямыми запрещёнными API-вызовами;
- токены, API-key secrets и ПДн не попадают в browser storage, логи или error telemetry;
- все страницы имеют loading, empty, validation, conflict, forbidden, not found и server error состояния;
- one-time secret действительно нельзя получить повторно;
- UI доступен с клавиатуры и проходит согласованные accessibility checks;
- OpenAPI — единственный источник frontend API types;
- CI и регрессия зелёные;
- документация rollout/rollback обновлена;
- human security/regression review новой поверхности выполнен до production rollout.

## Формат выполнения и отчёта

- Делай небольшие логические коммиты; не смешивай backend contract, frontend foundation, feature screens и deployment в один commit.
- После каждого milestone сообщай: что готово, какие команды проверки прошли, какие решения/риски остались.
- Не объявляй milestone завершённым по одним mock tests: покажи воспроизводимый сценарий с реальным backend там, где это требуется.
- В финале дай таблицу `milestone -> commits -> tests -> remaining decisions`, перечисли изменённые контракты и отдельно укажи всё, что отложено или требует human sign-off.

---

Исходный подробный план: `docs/admin-web-plan.md`.
