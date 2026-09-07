<!-- ui/src/components/common/ManagedRecordNotice.vue -->
<script setup lang="ts">
/**
 * Explains why a platform-provisioned NATS export/import is not editable here.
 *
 * One component rather than the same paragraph in six places (two list views,
 * two detail views, two form views). See utils/managedExports.ts for how these
 * records are recognised and which fields actually get rewritten.
 */
defineProps<{
  kind: 'export' | 'import'
  /** 'banner' explains; 'inline' is the one-word badge for a list row. */
  variant?: 'banner' | 'inline'
}>()
</script>

<template>
  <span v-if="variant === 'inline'" class="badge badge-sm badge-ghost gap-1" title="Provisioned and reconciled by the platform">
    Managed
  </span>

  <div v-else role="alert" class="alert">
    <span class="text-xl">&#128274;</span>
    <div>
      <h3 class="font-bold">Platform-managed {{ kind }}</h3>
      <div class="text-sm mt-1 space-y-1">
        <p>
          Provisioned by the <strong>Managed</strong> flag on the organization and
          reconciled every time that organization is saved, which rewrites
          <strong>subject</strong>, <strong>type</strong>, <strong>description</strong>
          <template v-if="kind === 'import'">, <strong>account</strong> and <strong>local subject</strong></template>.
          An edit here would be undone without warning, so the console does not offer one.
        </p>
        <p>
          Deleting it does not retire it either — the next save recreates it. To remove
          the pair, clear <strong>Managed</strong> on the organization.
        </p>
      </div>
    </div>
  </div>
</template>
