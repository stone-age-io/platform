<script setup lang="ts">
// The code prefix on a Thing Type or Location Type (ADR 0003 in platform-docs),
// shared so the help text cannot drift between the two forms.
//
// The server is the authority (schema.json pattern ^[A-Z]{1,4}$; hooks/codes.go
// refuses a prefix the other type collection holds). The input only uppercases
// and trims, so a lowercase prefix does not fail at save with no explanation.
const props = defineProps<{ kind: 'Thing' | 'Location'; example: string }>()
const model = defineModel<string>({ default: '' })

const otherKind = props.kind === 'Thing' ? 'Location' : 'Thing'

function onInput(e: Event) {
  const el = e.target as HTMLInputElement
  const cleaned = el.value.toUpperCase().replace(/[^A-Z]/g, '').slice(0, 4)
  model.value = cleaned
  el.value = cleaned
}
</script>

<template>
  <div class="form-control">
    <label class="label">Code prefix</label>
    <input
      :value="model"
      type="text"
      class="input input-bordered font-mono"
      :placeholder="`e.g. ${example}`"
      maxlength="4"
      autocomplete="off"
      @input="onInput"
    />
    <label class="label">
      <span class="label-text-alt">
        1–4 capital letters. A {{ kind }} of this type saved without a code gets a
        generated one like <code>{{ model || example }}-9KD-4PX</code>. Must differ from
        every {{ otherKind }} Type prefix. Changing it affects future codes only.
      </span>
    </label>
  </div>
</template>
