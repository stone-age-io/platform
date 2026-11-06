// @vitest-environment jsdom
import { DOMWrapper, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import { nextTick } from 'vue'

import ConfirmDialog from './ConfirmDialog.vue'

// Mounted, an exception to the no-mounting rule: the DOM contract (dialog
// semantics, focus, keys) is the subject, and this dialog gates every
// destructive action in the console.

function mountDialog(props: Record<string, unknown> = {}) {
  return mount(ConfirmDialog, {
    attachTo: document.body,
    props: {
      modelValue: true,
      title: 'Delete thing?',
      message: 'This cannot be undone.',
      ...props,
    },
  })
}

// The component teleports to <body>, so wrapper.find() cannot see it. Query the
// document and wrap, so .trigger() still flushes Vue's queue.
function q(selector: string) {
  const el = document.querySelector(selector)
  if (!el) throw new Error('not found in document: ' + selector)
  return new DOMWrapper(el as Element)
}

function qAll(selector: string) {
  return Array.from(document.querySelectorAll(selector)) as HTMLElement[]
}

// Teleported markup lives in <body> and jsdom does not reset it between tests,
// so a leaked dialog from one test is visible to the next.
afterEach(() => {
  document.body.innerHTML = ''
})

function dialog(_wrapper?: unknown) {
  return q('[role="dialog"]')
}

describe('ConfirmDialog accessibility', () => {
  it('announces itself as a modal dialog', () => {
    const wrapper = mountDialog()
    const el = dialog(wrapper)

    expect(el.exists()).toBe(true)
    expect(el.attributes('aria-modal')).toBe('true')
    wrapper.unmount()
  })

  it('labels and describes itself from the title and message it renders', () => {
    const wrapper = mountDialog()
    const el = dialog(wrapper)

    const labelId = el.attributes('aria-labelledby')!
    const describedById = el.attributes('aria-describedby')!
    expect(document.getElementById(labelId)?.textContent).toContain('Delete thing?')
    expect(document.getElementById(describedById)?.textContent).toContain('This cannot be undone.')
    wrapper.unmount()
  })

  // Both in one app: useId is unique per app instance, so two separate apps
  // would prove nothing.
  it('gives two dialogs in the same app distinct label ids', () => {
    const host = mount(
      {
        components: { ConfirmDialog },
        template:
          '<div><ConfirmDialog :model-value="true" title="A" message="m" /><ConfirmDialog :model-value="true" title="B" message="m" /></div>',
      },
      { attachTo: document.body },
    )

    const dialogs = qAll('[role="dialog"]')
    expect(dialogs).toHaveLength(2)
    expect(dialogs[0].getAttribute('aria-labelledby')).not.toBe(
      dialogs[1].getAttribute('aria-labelledby'),
    )
    host.unmount()
  })

  it('hides the decorative icon from assistive technology', () => {
    const wrapper = mountDialog()
    expect(q('.confirm-icon').attributes('aria-hidden')).toBe('true')
    wrapper.unmount()
  })
})

describe('ConfirmDialog focus behaviour', () => {
  // The container takes focus, not a button, so Enter cannot confirm unread.
  it('focuses the dialog itself, so Enter cannot confirm by accident', async () => {
    const wrapper = mountDialog({ modelValue: false })
    await wrapper.setProps({ modelValue: true })
    await nextTick()

    expect(document.activeElement).toBe(dialog(wrapper).element)
    expect((document.activeElement as HTMLElement).tagName).not.toBe('BUTTON')
    wrapper.unmount()
  })

  it('puts no autofocus on the destructive button', () => {
    const wrapper = mountDialog()
    expect(q('.btn-confirm').attributes('autofocus')).toBeUndefined()
    wrapper.unmount()
  })

  it('returns focus to whatever opened it', async () => {
    const opener = document.createElement('button')
    document.body.appendChild(opener)
    opener.focus()
    expect(document.activeElement).toBe(opener)

    const wrapper = mountDialog({ modelValue: false })
    await wrapper.setProps({ modelValue: true })
    await nextTick()
    expect(document.activeElement).not.toBe(opener)

    await wrapper.setProps({ modelValue: false })
    await nextTick()
    expect(document.activeElement).toBe(opener)

    wrapper.unmount()
    opener.remove()
  })

  // The confirmed action often removes the row whose button opened the dialog,
  // so restoring focus has to tolerate the target being gone.
  it('does not throw when the element that opened it is gone', async () => {
    const opener = document.createElement('button')
    document.body.appendChild(opener)
    opener.focus()

    const wrapper = mountDialog({ modelValue: false })
    await wrapper.setProps({ modelValue: true })
    await nextTick()
    opener.remove()

    await expect(wrapper.setProps({ modelValue: false })).resolves.not.toThrow()
    wrapper.unmount()
  })

  // aria-modal claims the page is inert, so Tab must not walk out of the dialog.
  it('wraps Tab from the last control back to the first', async () => {
    const wrapper = mountDialog()
    const el = dialog(wrapper)
    const buttons = qAll('[role="dialog"] button')
    const first = buttons[0]
    const last = buttons[buttons.length - 1]

    last.focus()
    await el.trigger('keydown', { key: 'Tab' })
    expect(document.activeElement).toBe(first)

    wrapper.unmount()
  })

  it('wraps Shift+Tab from the first control back to the last', async () => {
    const wrapper = mountDialog()
    const el = dialog(wrapper)
    const buttons = qAll('[role="dialog"] button')
    const first = buttons[0]
    const last = buttons[buttons.length - 1]

    first.focus()
    await el.trigger('keydown', { key: 'Tab', shiftKey: true })
    expect(document.activeElement).toBe(last)

    wrapper.unmount()
  })
})

describe('ConfirmDialog dismissal', () => {
  it('cancels on Escape', async () => {
    const wrapper = mountDialog()
    await dialog(wrapper).trigger('keydown', { key: 'Escape' })

    expect(wrapper.emitted('cancel')).toHaveLength(1)
    expect(wrapper.emitted('confirm')).toBeUndefined()
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([false])
    wrapper.unmount()
  })

  it('does not confirm on Escape', async () => {
    const wrapper = mountDialog()
    await dialog(wrapper).trigger('keydown', { key: 'Escape' })
    expect(wrapper.emitted('confirm')).toBeUndefined()
    wrapper.unmount()
  })

  it('still confirms and cancels by click', async () => {
    const wrapper = mountDialog()

    await q('.btn-confirm').trigger('click')
    expect(wrapper.emitted('confirm')).toHaveLength(1)

    await q('.btn-secondary').trigger('click')
    expect(wrapper.emitted('cancel')).toHaveLength(1)
    wrapper.unmount()
  })

  it('renders nothing when closed', () => {
    const wrapper = mountDialog({ modelValue: false })
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    wrapper.unmount()
  })
})

describe('ConfirmDialog typed gate', () => {
  // Every failure here is silent. The dialog is one shared instance in App.vue,
  // so a gate that survives a cancel arms the next, different record.

  it('is absent unless a record asks for it', () => {
    const wrapper = mountDialog()
    expect(document.querySelector('.confirm-gate')).toBeNull()
    expect(q('.btn-confirm').attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })

  it('holds the confirm button shut until the text matches', async () => {
    const wrapper = mountDialog({ requireText: 'gw-01' })
    expect(q('.btn-confirm').attributes('disabled')).toBeDefined()

    await q('.confirm-gate-input').setValue('gw-0')
    expect(q('.btn-confirm').attributes('disabled')).toBeDefined()

    await q('.confirm-gate-input').setValue('gw-01')
    expect(q('.btn-confirm').attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })

  it('does not confirm on a click while the gate is shut', async () => {
    const wrapper = mountDialog({ requireText: 'gw-01' })
    await q('.btn-confirm').trigger('click')
    expect(wrapper.emitted('confirm')).toBeUndefined()
    wrapper.unmount()
  })

  it('treats a different case as a different string', async () => {
    const wrapper = mountDialog({ requireText: 'gw-01' })
    await q('.confirm-gate-input').setValue('GW-01')
    expect(q('.btn-confirm').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('forgives whitespace from a paste', async () => {
    const wrapper = mountDialog({ requireText: 'gw-01' })
    await q('.confirm-gate-input').setValue('  gw-01 ')
    expect(q('.btn-confirm').attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })

  it('renders the required string so the reader can see which record this is', () => {
    const wrapper = mountDialog({ requireText: 'gw-01' })
    expect(q('.confirm-gate-token').text()).toBe('gw-01')
    wrapper.unmount()
  })

  it('starts empty on every open, so a cancelled delete cannot arm the next one', async () => {
    const wrapper = mountDialog({ requireText: 'gw-01' })
    await q('.confirm-gate-input').setValue('gw-01')
    expect(q('.btn-confirm').attributes('disabled')).toBeUndefined()

    await wrapper.setProps({ modelValue: false })
    await wrapper.setProps({ modelValue: true })
    await nextTick()

    expect((q('.confirm-gate-input').element as HTMLInputElement).value).toBe('')
    expect(q('.btn-confirm').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  // Mounted closed, then opened: the focus move is a watch on modelValue.
  it('focuses the gate rather than a button', async () => {
    const wrapper = mountDialog({ modelValue: false, requireText: 'gw-01' })
    await wrapper.setProps({ modelValue: true })
    await nextTick()
    expect(document.activeElement).toBe(q('.confirm-gate-input').element)
    wrapper.unmount()
  })

  it('still cancels on Escape with the gate shut', async () => {
    const wrapper = mountDialog({ requireText: 'gw-01' })
    await dialog(wrapper).trigger('keydown', { key: 'Escape' })
    expect(wrapper.emitted('cancel')).toHaveLength(1)
    wrapper.unmount()
  })
})

