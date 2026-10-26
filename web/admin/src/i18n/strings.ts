// Every user-facing string lives here rather than inline in components, so
// a future locale switch (see docs/admin-web-implementation-prompt.md:
// "русский первым, строки готовы к будущей локализации") only touches this
// file. There is exactly one locale today (ru) — this is not an i18n
// framework, just the single seam a real one would slot into later.
export const strings = {
  common: {
    loading: 'Загрузка…',
    retry: 'Повторить',
    cancel: 'Отмена',
    confirm: 'Подтвердить',
    copyRequestId: 'Скопировать ID запроса',
    copied: 'Скопировано',
  },
  auth: {
    loginTitle: 'Вход в PromoGo Admin',
    loginButton: 'Войти через SSO',
    loggingIn: 'Выполняется вход…',
    sessionExpiredTitle: 'Сессия истекла',
    sessionExpiredBody: 'Пожалуйста, войдите снова, чтобы продолжить работу.',
    loginAgain: 'Войти снова',
    unconfiguredTitle: 'OIDC-провайдер не настроен',
    unconfiguredBody:
      'Скопируйте web/admin/.env.example в .env.local и укажите VITE_OIDC_AUTHORITY и VITE_OIDC_CLIENT_ID вашего провайдера.',
  },
  noMembership: {
    title: 'Нет доступа к организациям',
    body: 'Ваша учётная запись подтверждена, но не привязана ни к одной организации. Обратитесь к администратору, чтобы получить доступ.',
  },
  disabledAccount: {
    title: 'Учётная запись отключена',
    body: 'Обратитесь к администратору платформы.',
  },
  forbidden: {
    title: 'Недостаточно прав',
    body: 'У вас нет прав для выполнения этого действия в выбранной организации или магазине.',
  },
  notFound: {
    title: 'Страница не найдена',
    body: 'Проверьте адрес или вернитесь на главную.',
    backHome: 'На главную',
  },
  serverError: {
    title: 'Ошибка сервера',
    body: 'Что-то пошло не так на нашей стороне. Если ошибка повторяется, передайте ID запроса в поддержку.',
  },
  orgStore: {
    organization: 'Организация',
    store: 'Магазин',
    allStores: 'Все магазины организации',
    selectOrganization: 'Выберите организацию',
    selectStore: 'Выберите магазин',
    noOrganizations: 'Нет доступных организаций',
  },
  nav: {
    dashboard: 'Обзор',
    organizations: 'Организации и магазины',
    staff: 'Сотрудники и доступ',
    loyaltyConfig: 'Программа лояльности',
    clients: 'Клиенты',
    apiKeys: 'API-ключи',
    audit: 'Аудит',
    logout: 'Выйти',
  },
  form: {
    nameRequired: 'Укажите название',
    tooLong: (max: number) => `Не длиннее ${max} символов`,
    invalidInput: 'Сервер отклонил данные. Проверьте поля формы.',
    saving: 'Сохранение…',
  },
  organizations: {
    title: 'Организации и магазины',
    addOrganization: '+ Организация',
    createLabel: 'Название новой организации',
    createSubmit: 'Создать',
    created: (name: string) => `Организация «${name}» создана и выбрана`,
    empty: 'Организаций пока нет',
    storeCount: (n: number) => {
      const mod10 = n % 10
      const mod100 = n % 100
      if (mod10 === 1 && mod100 !== 11) return `${n} магазин`
      if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return `${n} магазина`
      return `${n} магазинов`
    },
    collapse: (name: string) => `Свернуть «${name}»`,
    expand: (name: string) => `Развернуть «${name}»`,
    current: 'Текущая',
  },
  stores: {
    name: 'Название',
    address: 'Адрес',
    externalCode: 'Код в 1С',
    externalCodeHint: 'Необязательно. Уникален в пределах организации.',
    externalCodeTaken: 'Этот код уже используется другим магазином организации',
    id: 'ID',
    actions: 'Действия',
    notSet: '—',
    current: 'Текущий',
    select: (name: string) => `Сделать «${name}» текущим магазином`,
    add: '+ Магазин',
    addLabel: (orgName: string) => `Добавить магазин в «${orgName}»`,
    createTitle: (orgName: string) => `Новый магазин в «${orgName}»`,
    createSubmit: 'Создать',
    created: (name: string) => `Магазин «${name}» создан`,
    edit: 'Изменить',
    editLabel: (name: string) => `Изменить магазин «${name}»`,
    editTitle: (name: string) => `Изменение магазина «${name}»`,
    editSubmit: 'Сохранить',
    updated: (name: string) => `Магазин «${name}» сохранён`,
    empty: 'Магазинов пока нет. Создайте первый, затем настройте для него программу лояльности и API-ключ кассы.',
  },
  comingSoon: {
    title: 'Раздел в разработке',
    body: 'Этот экран появится в следующем этапе (Milestone 2).',
  },
} as const
