<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { formatDate } from '@/utils/format'
import type { Organization, User } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()

interface OrganizationWithExpand extends Organization {
  expand?: {
    owner?: User
  }
}

const org = ref<OrganizationWithExpand | null>(null)
const stats = ref({ members: 0, things: 0 })
const loading = ref(true)

const id = route.params.id as string

async function loadData() {
  loading.value = true
  try {
    org.value = await pb.collection('organizations').getOne<OrganizationWithExpand>(id, {
      expand: 'owner',
    })

    // Load Stats (Operator bypasses rules, so we must manually filter by org id)
    const [m, t] = await Promise.all([
      pb.collection('memberships').getList(1, 1, { filter: `organization = "${id}"` }),
      pb.collection('things').getList(1, 1, { filter: `organization = "${id}"` }),
    ])

    stats.value = { members: m.totalItems, things: t.totalItems }
  } catch (err: any) {
    toast.error('Failed to load organization')
    router.push('/organizations')
  } finally {
    loading.value = false
  }
}

onMounted(() => loadData())
</script>

<template>
  <div class="space-y-6">
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <template v-else-if="org">
      <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
        <div>
          <div class="breadcrumbs text-sm">
            <ul>
              <li><router-link to="/organizations">Organizations</router-link></li>
              <li>{{ org.name }}</li>
            </ul>
          </div>
          <div class="flex items-center gap-3">
            <h1 class="text-3xl font-bold">{{ org.name }}</h1>
            <span class="badge" :class="org.active ? 'badge-success' : 'badge-error'">
              {{ org.active ? 'Active' : 'Inactive' }}
            </span>
          </div>
        </div>
        <div class="flex gap-2">
          <router-link :to="`/organizations/${org.id}/edit`" class="btn btn-primary">
            Edit
          </router-link>
        </div>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
        <!-- Stats -->
        <div class="stats shadow bg-base-100 w-full md:col-span-3">
          <div class="stat">
            <div class="stat-title">Members</div>
            <div class="stat-value">{{ stats.members }}</div>
          </div>
          <div class="stat">
            <div class="stat-title">Things</div>
            <div class="stat-value text-primary">{{ stats.things }}</div>
          </div>
        </div>

        <!-- Info -->
        <div class="md:col-span-2">
          <BaseCard title="Details">
            <dl class="space-y-4">
              <div>
                <dt class="text-sm font-medium opacity-70">ID</dt>
                <dd class="font-mono text-sm">{{ org.id }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium opacity-70">Owner</dt>
                <dd>
                  {{ org.expand?.owner?.email || 'Unknown' }}
                  <span v-if="org.expand?.owner?.name" class="opacity-70">
                    ({{ org.expand.owner.name }})
                  </span>
                </dd>
              </div>
              <div>
                <dt class="text-sm font-medium opacity-70">Description</dt>
                <dd>{{ org.description || '—' }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium opacity-70">Created</dt>
                <dd>{{ formatDate(org.created) }}</dd>
              </div>
            </dl>
          </BaseCard>
        </div>
      </div>
    </template>
  </div>
</template>
