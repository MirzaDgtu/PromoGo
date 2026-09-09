import { strings } from '../i18n/strings'

// Placeholder for every nav destination until its real screen lands in
// Milestone 2 (docs/admin-web-implementation-prompt.md explicitly scopes
// Milestone 1 to the shell/foundation, not feature screens).
export function ComingSoonPage({ title }: { title: string }) {
  return (
    <div>
      <h1>{title}</h1>
      <p>{strings.comingSoon.body}</p>
    </div>
  )
}
