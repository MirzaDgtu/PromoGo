import type { SVGProps } from 'react'

// One small authored icon set: 16px grid, 1.5px stroke, round joins. Icons
// are always decorative here — every one sits next to its text label.
type IconProps = SVGProps<SVGSVGElement>

function Icon({ children, ...props }: IconProps) {
  return (
    <svg
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.5}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
      {...props}
    >
      {children}
    </svg>
  )
}

// The PromoGo mark: a register's receipt slip with its torn edge, the one
// printed thing every sale in this product leaves behind.
export function BrandMark(props: IconProps) {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false" {...props}>
      <rect width="24" height="24" rx="6" fill="currentColor" />
      <path d="M7 5.5h10V18l-1.67-1.25L13.67 18 12 16.75 10.33 18 8.67 16.75 7 18z" style={{ fill: 'var(--sheet)' }} />
      <path d="M9.5 9h5M9.5 12h3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  )
}

export const OverviewIcon = (p: IconProps) => (
  <Icon {...p}>
    <rect x="2" y="2" width="5" height="6" rx="1" />
    <rect x="9" y="2" width="5" height="4" rx="1" />
    <rect x="2" y="10" width="5" height="4" rx="1" />
    <rect x="9" y="8" width="5" height="6" rx="1" />
  </Icon>
)

export const StoresIcon = (p: IconProps) => (
  <Icon {...p}>
    <path d="M2.5 6.5 3.5 2.5h9l1 4" />
    <path d="M2.5 6.5c0 1.1.9 2 2 2s1.75-.9 1.75-2c0 1.1.9 2 1.75 2s1.75-.9 1.75-2c0 1.1.9 2 1.75 2s2-.9 2-2" />
    <path d="M3.5 8.5v5h9v-5M6.5 13.5v-3h3v3" />
  </Icon>
)

export const StaffIcon = (p: IconProps) => (
  <Icon {...p}>
    <circle cx="6" cy="5.5" r="2.5" />
    <path d="M1.5 13.5c.5-2.3 2.3-3.5 4.5-3.5s4 1.2 4.5 3.5" />
    <path d="M11 3.2a2.5 2.5 0 0 1 0 4.6M12.5 10.3c1.1.5 1.8 1.6 2 3.2" />
  </Icon>
)

export const LoyaltyIcon = (p: IconProps) => (
  <Icon {...p}>
    <rect x="1.5" y="3.5" width="13" height="9" rx="1.5" />
    <path d="M1.5 6.5h13M4 10h3" />
  </Icon>
)

export const ClientsIcon = (p: IconProps) => (
  <Icon {...p}>
    <rect x="4" y="1.5" width="8" height="13" rx="1.5" />
    <path d="M7 12h2" />
  </Icon>
)

export const KeyIcon = (p: IconProps) => (
  <Icon {...p}>
    <circle cx="5" cy="11" r="2.75" />
    <path d="m7 9 6.5-6.5M11 5l1.75 1.75M9.5 6.5l1.25 1.25" />
  </Icon>
)

export const AuditIcon = (p: IconProps) => (
  <Icon {...p}>
    <path d="M3.5 1.5h6l3 3v10h-9z" />
    <path d="M9.5 1.5v3h3M5.5 8h5M5.5 10.5h5M5.5 13h3" />
  </Icon>
)

export const LogoutIcon = (p: IconProps) => (
  <Icon {...p}>
    <path d="M6 2.5H3.5a1 1 0 0 0-1 1v9a1 1 0 0 0 1 1H6M10.5 11l3-3-3-3M13.5 8H6" />
  </Icon>
)

export const MenuIcon = (p: IconProps) => (
  <Icon {...p}>
    <path d="M2.5 4h11M2.5 8h11M2.5 12h11" />
  </Icon>
)

export const ChevronIcon = (p: IconProps) => (
  <Icon {...p}>
    <path d="m6 3.5 4.5 4.5L6 12.5" />
  </Icon>
)

export const CheckIcon = (p: IconProps) => (
  <Icon strokeWidth={2} {...p}>
    <path d="m3.5 8.5 3 3 6-7" />
  </Icon>
)

export const AttentionIcon = (p: IconProps) => (
  <Icon strokeWidth={2} {...p}>
    <path d="M8 3.5v5.5M8 12.25v.25" />
  </Icon>
)

export const InfoIcon = (p: IconProps) => (
  <Icon strokeWidth={1.75} {...p}>
    <path d="M8 7.5v5M8 3.75v.25" />
  </Icon>
)
