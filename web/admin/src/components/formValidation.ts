import { strings } from '../i18n/strings'

// Limits mirror the backend (internal/httpserver/admin_organizations.go).
// Lengths count code points like Go's utf8.RuneCountInString — not UTF-16
// units, not bytes — and apply to the trimmed value, as the server trims too.
export const MAX_NAME_LENGTH = 255
export const MAX_ADDRESS_LENGTH = 500
export const MAX_EXTERNAL_CODE_LENGTH = 64

function charCount(value: string): number {
  return [...value.trim()].length
}

export function validateName(value: string): true | string {
  if (value.trim() === '') return strings.form.nameRequired
  if (charCount(value) > MAX_NAME_LENGTH) return strings.form.tooLong(MAX_NAME_LENGTH)
  return true
}

export function maxLength(max: number) {
  return (value: string): true | string => charCount(value) <= max || strings.form.tooLong(max)
}
