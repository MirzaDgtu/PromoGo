---
name: PromoGo Admin
description: Operating tool for a retail loyalty platform — the register's body around a working sheet.
colors:
  frame: "#edf1f3"
  sheet: "#ffffff"
  raised: "#f4f6f7"
  active: "#dde4e7"
  border: "#e1e6e9"
  rule: "#eceff1"
  control: "#8a959b"
  control-soft: "#cfd6da"
  text: "#131416"
  muted: "#565e63"
  faint: "#636c71"
  accent: "#1b6c80"
  accent-hover: "#155a6b"
  accent-soft: "#e2eff2"
  accent-ink: "#0e4452"
  on-accent: "#ffffff"
  positive: "#1e7b4f"
  positive-soft: "#ddf5e7"
  positive-ink: "#0c4027"
  warning: "#92400e"
  warning-soft: "#fdf3e1"
  warning-ink: "#6b2f09"
  negative: "#c4242a"
  negative-soft: "#fdeceb"
  negative-ink: "#7f1a1d"
  neutral-soft: "#e9edef"
  neutral-ink: "#353a3d"
  dark-frame: "#0d1214"
  dark-sheet: "#151b1e"
  dark-raised: "#1b2327"
  dark-active: "#253035"
  dark-border: "#273236"
  dark-text: "#e6eaec"
  dark-muted: "#a0abb1"
  dark-accent: "#5fb3c7"
  dark-on-accent: "#06191f"
typography:
  title:
    fontFamily: "IBM Plex Sans, system-ui, sans-serif"
    fontSize: "22px"
    fontWeight: 600
    lineHeight: 1.25
    letterSpacing: "-0.01em"
  section:
    fontFamily: "IBM Plex Sans, system-ui, sans-serif"
    fontSize: "16px"
    fontWeight: 600
    lineHeight: 1.25
  body:
    fontFamily: "IBM Plex Sans, system-ui, sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.5
  label:
    fontFamily: "IBM Plex Sans, system-ui, sans-serif"
    fontSize: "14px"
    fontWeight: 500
    lineHeight: 1.5
  small:
    fontFamily: "IBM Plex Sans, system-ui, sans-serif"
    fontSize: "12px"
    fontWeight: 500
    lineHeight: 1.2
  figure:
    fontFamily: "IBM Plex Sans, system-ui, sans-serif"
    fontSize: "28px"
    fontWeight: 400
    lineHeight: 1.1
  code:
    fontFamily: "IBM Plex Mono, ui-monospace, monospace"
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1.5
rounded:
  control: "6px"
  card: "8px"
  dialog: "12px"
  sheet: "16px"
  pill: "999px"
spacing:
  "1": "4px"
  "2": "8px"
  "3": "12px"
  "4": "16px"
  "5": "20px"
  "6": "24px"
  "8": "32px"
  "10": "40px"
components:
  button-primary:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.on-accent}"
    rounded: "{rounded.control}"
    padding: "0 12px"
    height: "32px"
  button-primary-hover:
    backgroundColor: "{colors.accent-hover}"
  button-secondary:
    backgroundColor: "{colors.sheet}"
    textColor: "{colors.text}"
    rounded: "{rounded.control}"
    padding: "0 12px"
    height: "32px"
  button-secondary-hover:
    backgroundColor: "{colors.raised}"
  button-danger:
    backgroundColor: "{colors.negative}"
    textColor: "{colors.sheet}"
    rounded: "{rounded.control}"
    height: "32px"
  input:
    backgroundColor: "{colors.sheet}"
    textColor: "{colors.text}"
    rounded: "{rounded.control}"
    padding: "0 12px"
    height: "32px"
  nav-item-active:
    backgroundColor: "{colors.active}"
    textColor: "{colors.text}"
    rounded: "{rounded.control}"
    height: "34px"
  status-active:
    backgroundColor: "{colors.positive-soft}"
    textColor: "{colors.positive-ink}"
    rounded: "{rounded.pill}"
    typography: "{typography.small}"
  status-warn:
    backgroundColor: "{colors.warning-soft}"
    textColor: "{colors.warning-ink}"
    rounded: "{rounded.pill}"
    typography: "{typography.small}"
  badge:
    backgroundColor: "{colors.neutral-soft}"
    textColor: "{colors.neutral-ink}"
    rounded: "{rounded.pill}"
    typography: "{typography.small}"
  sheet:
    backgroundColor: "{colors.sheet}"
    rounded: "{rounded.sheet}"
---

# Design System: PromoGo Admin

Source of truth for values: `web/admin/src/index.css` (`:root` and its dark-scheme override). This file explains how to use them.

## Overview

**«Касса» — the register.** Admin Web is an operating tool (Operate mode) for two equal audiences: retailer staff who come in to onboard a store, issue a register key, add staff and tune the points rules, and the PromoGo platform team who work across every organization. The interface borrows its structure from the cash register that feeds the whole product: a tinted body (the L-shaped frame of sidebar and top bar) around a white working screen (the sheet). The frame holds where you are; the sheet holds what you are doing.

The one element only this product has is the **exchange light** in the top bar: for the selected store, a signal lamp and one line on whether the register or 1C is talking to the loyalty core ("Касса или 1С · подключена 5 минут назад"). It is the first thing both audiences check, it links to the API keys where a broken exchange is fixed, and when the exchange needs a look its ring flares twice on arrival, then holds still.

The system is restrained by design: colour goes on data and state, not chrome. Familiar patterns (side nav, breadcrumb scope, tables, inline forms) are kept because the job is to be trusted at a glance, not admired.

## Colors

One ground, one sheet, one accent, plus status families. Light and dark follow `prefers-color-scheme`; there is no toggle.

- **Frame** (`--frame`, a low-chroma petrol grey) is the register's body: sidebar, top bar and the ground of standalone screens.
- **Sheet** (`--sheet`) is the working surface. Content never sits on the frame directly inside the shell.
- **Raised / active** are the two tone steps used for separation (a panel on the sheet, the active nav row, the inline-editor row). Steps, not shadows.
- **Accent — petrol** (`--accent`) marks the one primary action per screen, the current selection (active nav icon, current organization/store name), links and focus. Never a large area.
- **Status families** (positive / warning / negative / neutral) each have a solid, a soft fill and an ink. Pills and messages use soft + ink; icons and lamps use the solid.

### Named Rules

- **Colour budget under 3%.** If a screen looks colourful, something in the chrome took colour that belongs to data.
- **State never rests on colour alone.** Every status carries its text, and its mark changes shape: a filled dot (ok), a ring (needs attention), a dash (revoked/expired).
- **AA in both themes.** All text pairs ≥ 4.5:1, form-control borders and status marks ≥ 3:1 (verified 2026-10-29).

## Typography

IBM Plex Sans for everything, IBM Plex Mono only for machine strings (API key ids, secrets, request ids, IPs, receipt numbers, access codes). Both self-hosted via `@fontsource` with Cyrillic subsets; no runtime CDN.

### Hierarchy

- **Title** 22px / 600 — one per screen, the screen's name with its scope ("API-ключи — Океан").
- **Section** 16px / 600 — headings inside the sheet.
- **Body** 14px / 400, line-height 1.5; **label** 14px / 500 for field labels and buttons.
- **Small** 12px / 500 — table headers, pills.
- **Figure** 28px / 400 — a single headline number (client balance), tabular.

### Named Rules

- **Fixed scale, no fluid type.** Product UI is read at a steady DPI.
- **Tabular figures everywhere a number can line up** (tables, balances, preview amounts).
- **Sentence case.** No tracked capitals, no eyebrows above headings.

## Layout

- **Shell:** grid of a 248px sidebar and the content column; a 48px top bar on the frame; the sheet inset 8px from the window's right and bottom edges with a 16px radius and a 1px border.
- **Top bar:** organization › store scope selectors (content-sized), the exchange light pushed right.
- **Page padding:** 24px top, 32px sides inside the sheet (20px at tablet).
- **Overview:** checklist column + 384px activity rail; stacks under 1180px.
- **Loyalty config:** form column (≤ 560px) + sticky 360px preview; stacks under 1100px.
- **Tablet (≤ 1100px):** the sidebar becomes a 280px drawer behind a "Меню" button in the top bar, with a scrim; Escape or any navigation closes it. Selector labels become visually hidden, never removed.
- **Tables** sit in `.table-wrap` and scroll horizontally rather than crush columns; dates never wrap.
- Body keeps `min-width: 768px`: phones are out of scope for Admin Web.

## Elevation & Depth

Flat by default. Separation is a 1px border/rule or one tone step. Only what truly floats gets `--shadow-float`: the one-time-secret dialog and the tablet drawer.

## Shapes

Controls 6px, cards/panels 8px, dialog 12px, sheet 16px, pills fully round. Nothing else.

## Components

### Buttons
32px tall, 12px side padding, label weight 500. **Primary** (accent fill) is the one main action of a screen; a page header's action button is primary automatically. **Secondary** is the sheet with a soft border. **Danger** only inside a confirmation. Press moves the button 1px down; disabled is 50% opacity with a not-allowed cursor.

### Inputs / Fields
32px, 1px `--control` border (≥ 3:1), focus = accent border + 3px accent-soft ring. Label above (500), hint or error below at 13px; errors in `--negative` and announced. Selects use an authored chevron. Read-only values sit on `--raised`.

### Navigation
Sidebar items 34px with a 16px authored stroke icon. Active item: `--active` pill, ink text, accent icon. The brand row (receipt-slip mark + "PromoGo Admin") tops the sidebar; the account and "Выйти" sit at its bottom.

### Pills
`.status` (with its shape-coded dot), `.badge` and scope chips: 22px, fully round, 12px / 500.

### Tables
No bands, no zebra. 36px header row in small muted type, 48px body rows on hairline rules, numbers right-aligned, hover tints the row. Inline editors and confirmations replace their row on a `--raised` band and open with a short rise.

### States
Loading is a four-line skeleton (pulsing in `--active`); the label stays for screen readers. Empty and forbidden states are a raised panel; errors are a negative-soft panel with a copyable request id and a retry.

### Exchange light (signature)
Top-bar link: lamp (8px) + "Касса или 1С · <state>". Ok = filled positive dot; needs attention = warning ring that flares twice; no data = faint ring.

### Overview checklist
One bordered panel of rows; each row has a 24px state icon (filled check / warning ring with a bar / faint ring with i), a title, a muted line and the action link.

## Motion

Short and functional: 90ms press, 140ms hovers and colour, 200ms routine state, 260ms overlays. Arrivals use `cubic-bezier(0.16, 1, 0.3, 1)`. Route change: the sheet's content fades up 4px. Dialog: fade + 8px rise + 0.98 scale. Inline rows and forms rise 4–6px. A completed action shows a positive notice whose check ticks in. Field errors nudge once. Under `prefers-reduced-motion` every arrival becomes a plain fade, the drawer cross-fades, and nothing loops (skeleton and lamp hold still).

## Do's and Don'ts

- **Do** keep one primary action per screen, in the page header when it exists.
- **Do** put colour on status and data; keep the frame and nav grey.
- **Do** use Plex Mono only for machine strings.
- **Don't** add coloured left borders, gradients, glass, or shadows on things at rest.
- **Don't** use glyphs (✓ ! ▸) as icons; use the authored set in `components/icons.tsx`.
- **Don't** hide a form label with `display: none`; use the visually-hidden pattern.
- **Don't** add a UI library; extend the tokens and classes in `index.css`.
