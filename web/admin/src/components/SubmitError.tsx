import { responseOf } from '../api/queries'
import { strings } from '../i18n/strings'
import { ErrorState, ForbiddenState } from './states'

// SubmitError renders a failed mutation under its form: 403 as the
// forbidden state, 400 as a generic validation message (field-level checks
// already ran client-side), anything else with a copyable request id.
// Forms that map a status to a specific field (e.g. 409) handle it first.
export function SubmitError({ error }: { error: unknown }) {
  const response = responseOf(error)
  if (response?.status === 403) return <ForbiddenState />
  if (response?.status === 400) {
    return (
      <p role="alert" className="field-error">
        {strings.form.invalidInput}
      </p>
    )
  }
  return <ErrorState response={response} />
}
