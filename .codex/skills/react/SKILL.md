---
name: react
description: Senior React development guidance for building, reviewing, refactoring, and debugging React applications and components. Use when working with React components, hooks, state management, forms, accessibility, performance, UI behavior, tests, or files such as .jsx, .tsx, React component modules, custom hooks, and frontend feature code.
---

# React

## Workflow

Start by reading the surrounding component patterns, design system conventions, routing setup, state management approach, and existing test style before changing code.

Prefer the smallest component shape that makes behavior clear. Keep rendering, state transitions, and side effects easy to follow. Extract components or hooks only when they remove real repetition or isolate a meaningful behavior.

## Components and State

Use controlled state intentionally. Keep local UI state close to the component that owns it, lift state only when multiple consumers truly need it, and avoid adding global state for transient view concerns.

Model async and form states explicitly: idle, loading, success, empty, error, disabled, and validation states should be visible in the UI when users can encounter them.

Keep props stable and specific. Prefer domain names over vague names like data, item, or value when a clearer name exists in the codebase.

## Hooks and Effects

Use hooks for reusable behavior, not as a dumping ground for unrelated logic. Keep hook return values ergonomic and typed when TypeScript is present.

Treat effects as synchronization with something outside render: network, DOM, timers, subscriptions, storage, or imperative APIs. Avoid effects for derived data that can be computed during render.

Clean up subscriptions, timers, and in-flight work when the existing codebase pattern supports it.

## Forms, Accessibility, and UX

Make forms keyboard-friendly and screen-reader coherent. Use semantic elements, associated labels, useful error text, and focus behavior that matches user intent.

Do not hide required interaction behind hover-only affordances. Keep loading and disabled states understandable without explanatory copy that clutters the UI.

## Performance and Tests

Optimize after identifying real re-render or data-flow pressure. Prefer clear data boundaries, stable keys, memoization only where it pays for itself, and avoid premature useMemo/useCallback noise.

Add or update tests for user-visible behavior, edge states, and regressions. Prefer React Testing Library-style interaction tests when available.
