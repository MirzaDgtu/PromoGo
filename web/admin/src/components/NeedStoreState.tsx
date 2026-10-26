import { Link } from 'react-router-dom'

import { strings } from '../i18n/strings'
import { EmptyState } from './states'

// NeedStoreState is the page body for store-scoped screens (API keys,
// loyalty config, clients) while no store is selected.
export function NeedStoreState({ body }: { body: string }) {
  return (
    <EmptyState
      title={strings.storeRequired.title}
      body={
        <p>
          {body} <Link to="/organizations">{strings.storeRequired.toStores}</Link>
        </p>
      }
    />
  )
}
