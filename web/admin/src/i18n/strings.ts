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
  comingSoon: {
    title: 'Раздел в разработке',
    body: 'Этот экран появится в следующем этапе (Milestone 2).',
  },
} as const
