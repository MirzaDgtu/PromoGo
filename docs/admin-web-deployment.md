# Выкладка и откат Admin Web

Как собрать, выложить и откатить админку для пилота. Про выкладку самого
бэкенда и миграций — [deployment.md](deployment.md), общий чек-лист пилота —
[pilot-rollout-checklist.md](pilot-rollout-checklist.md).

## Схема

Админка и API живут на одном домене (`docs/admin-web-plan.md`, 5.2), поэтому
CORS не нужен:

```
браузер ── HTTPS ──> балансировщик / ingress (TLS)
                         │
                         ▼
               admin-web (nginx, :8080)
                 /           -> index.html (маршруты SPA)
                 /assets/*   -> файлы сборки
                 /api/*      -> app (Go, :8080)
```

- Образ `admin-web` собирается из [web/admin/Dockerfile](../web/admin/Dockerfile).
  Внутри nginx с собранным SPA, конфиг лежит в
  [web/admin/nginx/](../web/admin/nginx/).
- `/metrics`, `/healthz` и `/readyz` бэкенда через admin-web не публикуются,
  на этих путях открывается SPA. Проверки здоровья и сбор метрик ходят
  напрямую в `app:8080`.
- nginx работает от непривилегированного пользователя на порту 8080. TLS
  снимает балансировщик перед ним.

## 1. Один раз: регистрация в OIDC-провайдере

Админка — публичный OIDC-клиент: Authorization Code с PKCE, без
`client_secret`. Для домена `https://admin.example.ru` в провайдере
регистрируются:

| Параметр | Значение |
|---|---|
| Redirect URI | `https://admin.example.ru/auth/callback` |
| Post-logout redirect URI | `https://admin.example.ru/login` |
| Web origin (CORS для token endpoint) | `https://admin.example.ru` |
| Scopes | `openid profile email` |

У бэкенда должны совпадать:
- `PROMOGO_OIDC_ISSUER_URL` с issuer провайдера;
- `PROMOGO_OIDC_AUDIENCE` с client id админки;
- `PROMOGO_OIDC_JWKS_URL` с JWKS провайдера.

## 2. Сборка образа

Vite вшивает параметры OIDC в бандл при сборке, поэтому образ собирается
отдельно под каждое окружение (stage, production). Это не секреты: их видит
любой браузер.

```sh
cd web/admin
docker build \
  --build-arg VITE_OIDC_AUTHORITY=https://login.example.ru/promogo \
  --build-arg VITE_OIDC_CLIENT_ID=promogo-admin-web \
  -t registry.example.ru/promogo/admin-web:prod-$(git rev-parse --short HEAD) .
```

- Без обоих аргументов сборка падает сразу, а не выпускает админку с экраном
  «OIDC не настроен».
- Тег — окружение плюс коммит. Тег `latest` не используйте: откат делается
  выбором предыдущего тега, и он должен однозначно указывать на сборку.
- `.env.local` и другие `.env*` в образ не попадают, их исключает
  `.dockerignore`.

## 3. Настройки при запуске

| Где | Переменная | Значение |
|---|---|---|
| admin-web | `PROMOGO_API_UPSTREAM` | адрес бэкенда, по умолчанию `http://app:8080` |
| app | `PROMOGO_HTTP_TRUSTED_PROXIES` | CIDR-подсети admin-web **и** балансировщика через запятую, например `10.0.1.0/24,10.0.2.0/24` |

`PROMOGO_HTTP_TRUSTED_PROXIES` обязателен. Без него бэкенд видит
все запросы с адреса nginx. Тогда все администраторы попадают в один общий
лимит запросов (`ratelimit.admin_*`), а в аудите у всех одинаковый IP. Слишком
широкий диапазон тоже плох: любой клиент сможет подставить себе чужой IP через
`X-Forwarded-For`.

## 4. Порядок выкладки

Админка сгенерирована из OpenAPI того же коммита и рассчитывает на его API.
Поэтому:

1. **Сначала бэкенд**, вместе с миграциями, по [deployment.md](deployment.md).
   Изменения API должны быть обратно совместимыми (expand → migrate →
   contract), чтобы старая админка, открытая в браузерах, продолжала работать.
2. **Потом admin-web** с тегом того же коммита.
3. **Проверка после выкладки:**
   - `https://admin.example.ru/` открывает страницу входа, вход через SSO
     возвращает в приложение;
   - прямой заход на `https://admin.example.ru/api-keys` открывает SPA, а не
     404 бэкенда;
   - `curl -sI https://admin.example.ru/` показывает `Cache-Control: no-cache`
     и `X-Frame-Options: DENY`;
   - `curl -s https://admin.example.ru/metrics` возвращает HTML админки, а не
     метрики;
   - в аудите вход отображается с реальным IP администратора, а не с адресом
     nginx.

Уже открытые вкладки продолжают работать на старом бандле до перезагрузки:
SPA собирается одним файлом без подгружаемых частей. `index.html` не
кешируется, поэтому при следующей загрузке подтягивается новая версия.

## 5. Откат

- **Только админка** (сломался экран, бэкенд в порядке): перезапустить
  admin-web с предыдущим тегом. Данных админка не хранит, откат мгновенный и
  безопасный.
- **Бэкенд**: откатывать в обратном порядке — сначала admin-web на тег,
  соответствующий целевой версии бэкенда, потом сам бэкенд по
  [pilot-rollout-checklist.md](pilot-rollout-checklist.md) («Rollback
  triggers and procedure»). Иначе новая админка может обратиться к полям и
  эндпоинтам, которых в старом бэкенде нет.
- После отката повторить проверки из п. 4.

Пример для docker compose: в `image:` сервиса admin-web указать предыдущий
тег и выполнить `docker compose up -d admin-web`.

## Локальная проверка образа

На локальном стеке с mock OIDC (`web/admin/README.md`, «Local OIDC for
dev»):

```sh
make admin-web-up        # собирает образ, админка на http://localhost:8092
cd web/admin && E2E_BASE_URL=http://localhost:8092 npm run e2e
```

Compose берёт параметры OIDC для сборки из `deployments/.env`
(`PROMOGO_OIDC_ISSUER_URL`, `PROMOGO_OIDC_AUDIENCE`). Проверено
2026-10-29: все 7 браузерных сценариев проходят против собранного образа.

## Ещё не сделано

- Content-Security-Policy — задача M3. Политика должна разрешать
  выбранный OIDC-провайдер, а он ещё не выбран (DEC-004).
- Сборка и публикация образа в CI: CI в проекте пока не настроен.
