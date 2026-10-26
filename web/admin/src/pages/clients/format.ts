// formatPhone renders a stored E.164 Russian number as "+7 926 123-45-67".
// Anything else — foreign numbers, support_viewer's masked "********4567" —
// is shown exactly as the server sent it.
export function formatPhone(phone: string): string {
  const m = /^\+7(\d{3})(\d{3})(\d{2})(\d{2})$/.exec(phone)
  return m ? `+7 ${m[1]} ${m[2]}-${m[3]}-${m[4]}` : phone
}

export function isMaskedPhone(phone: string): boolean {
  return phone.startsWith('*')
}

// formatMoney formats a decimal string amount; an unknown currency code
// falls back to the bare number plus the code.
export function formatMoney(amount: string, currency: string): string {
  const value = Number(amount)
  try {
    return new Intl.NumberFormat('ru-RU', { style: 'currency', currency }).format(value)
  } catch {
    return `${value.toLocaleString('ru-RU')} ${currency}`
  }
}

// formatPointsDelta always carries an explicit sign, so accrual and
// redemption read apart without relying on color.
export function formatPointsDelta(delta: number): string {
  const abs = Math.abs(delta).toLocaleString('ru-RU')
  return delta > 0 ? `+${abs}` : delta < 0 ? `−${abs}` : '0'
}
