// @vitest-environment jsdom
import { effectScope, ref } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { useEscapeKey } from './useEscapeKey'

// jsdom, because the whole subject is a real `keydown` travelling up to
// `window` and the deference rule reads `event.defaultPrevented` off it. A
// stubbed listener map would assert the stub, not the behaviour: the bug this
// guards against is two modals closing on one keypress, which only shows up
// when the events are actually dispatched.

const scopes: Array<ReturnType<typeof effectScope>> = []

/** Register inside a disposable scope, the way a component would. */
function register(isOpen: Parameters<typeof useEscapeKey>[0], onEscape: () => void) {
  const scope = effectScope()
  scopes.push(scope)
  scope.run(() => useEscapeKey(isOpen, onEscape))
  return scope
}

function pressEscape() {
  return document.body.dispatchEvent(
    new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }),
  )
}

afterEach(() => {
  while (scopes.length) scopes.pop()!.stop()
})

describe('useEscapeKey', () => {
  it('calls the handler while open', () => {
    const onEscape = vi.fn()
    register(ref(true), onEscape)
    pressEscape()
    expect(onEscape).toHaveBeenCalledOnce()
  })

  it('ignores Escape while closed', () => {
    const onEscape = vi.fn()
    register(ref(false), onEscape)
    pressEscape()
    expect(onEscape).not.toHaveBeenCalled()
  })

  it('follows the open state rather than the component lifetime', () => {
    const open = ref(false)
    const onEscape = vi.fn()
    register(open, onEscape)

    pressEscape()
    expect(onEscape).not.toHaveBeenCalled()

    open.value = true
    pressEscape()
    expect(onEscape).toHaveBeenCalledOnce()

    open.value = false
    pressEscape()
    expect(onEscape).toHaveBeenCalledOnce()
  })

  it('ignores keys other than Escape', () => {
    const onEscape = vi.fn()
    register(ref(true), onEscape)
    document.body.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }),
    )
    expect(onEscape).not.toHaveBeenCalled()
  })

  // The reason the stack exists: a quick-add modal opened from inside a form
  // view. One press must unwind one layer, not collapse both.
  it('closes only the innermost of two open modals', () => {
    const outer = vi.fn()
    const inner = vi.fn()
    register(ref(true), outer)
    const innerScope = register(ref(true), inner)

    pressEscape()
    expect(inner).toHaveBeenCalledOnce()
    expect(outer).not.toHaveBeenCalled()

    innerScope.stop()
    scopes.splice(scopes.indexOf(innerScope), 1)

    pressEscape()
    expect(outer).toHaveBeenCalledOnce()
  })

  // ConfirmDialog and RecordPicker both handle their own Escape and
  // preventDefault it. Without this, confirming a delete from inside a modal
  // would cancel the confirm AND close the modal underneath it.
  it('defers to a handler closer to the event that already consumed the key', () => {
    const onEscape = vi.fn()
    register(ref(true), onEscape)

    const consume = (e: Event) => e.preventDefault()
    document.body.addEventListener('keydown', consume)
    pressEscape()
    document.body.removeEventListener('keydown', consume)

    expect(onEscape).not.toHaveBeenCalled()
  })

  it('marks the event handled so an outer listener can defer in turn', () => {
    register(ref(true), () => {})
    // dispatchEvent returns false once preventDefault has been called.
    expect(pressEscape()).toBe(false)
  })

  it('stops listening once the owning scope is disposed', () => {
    const onEscape = vi.fn()
    const scope = register(ref(true), onEscape)
    scope.stop()
    scopes.splice(scopes.indexOf(scope), 1)

    pressEscape()
    expect(onEscape).not.toHaveBeenCalled()
  })
})
