import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { ComingSoonPage } from './ComingSoonPage'

describe('ComingSoonPage', () => {
  it('renders the given title as a heading', () => {
    render(<ComingSoonPage title="Клиенты" />)
    expect(screen.getByRole('heading', { name: 'Клиенты' })).toBeInTheDocument()
  })
})
