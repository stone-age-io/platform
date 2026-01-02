<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const id = route.params.id as string | undefined
const isEdit = computed(() => !!id)
const loading = ref(false)

const form = ref({
  name: '',
  description: '',
  code: '',
  capabilities: [] as string[],
})

const availableCapabilities = ['read', 'command', 'action']

async function loadData() {
  if (!id) return
  loading.value = true
  try {
    const record = await pb.collection('thing_types').getOne(id)
    form.value = {
      name: record.name,
      description: record.description,
      code: record.code,
      capabilities: record.capabilities || [],
    }
  } catch (err: any) {
    toast.error('Failed to load type')
    router.push('/things/types')
  } finally {
    loading.value = false
  }
}

async function submit() {
  loading.value = true
  try {
    const data = {
      ...form.value,
      organization: isEdit.value ? undefined : authStore.currentOrgId
    }
    
    if (isEdit.value) {
      await pb.collection('thing_types').update(id!, data)
      toast.success('Updated')
    } else {
      await pb.collection('thing_types').create(data)
      toast.success('Created')
    }
    router.push('/things/types')
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  if (isEdit.value) loadData()
})
</script>

<template>
  <div class="mx-auto space-y-6">
    <div class="breadcrumbs text-sm">
      <ul>
        <li><router-link to="/things/types">Thing Types</router-link></li>
        <li>{{ isEdit ? 'Edit' : 'New' }}</li>
      </ul>
    </div>
    
    <h1 class="text-3xl font-bold">{{ isEdit ? 'Edit' : 'Create' }} Thing Type</h1>

    <form @submit.prevent="submit" class="max-w-4xl mx-auto">
      <BaseCard>
        <div class="space-y-4">
          <div class="form-control">
            <label class="label">Name *</label>
            <input v-model="form.name" type="text" class="input input-bordered" required />
          </div>
          
          <div class="form-control">
            <label class="label">Code</label>
            <input v-model="form.code" type="text" class="input input-bordered font-mono" placeholder="Optional identifier" />
          </div>
          
          <div class="form-control">
            <label class="label">Description</label>
            <textarea v-model="form.description" class="textarea textarea-bordered" rows="2"></textarea>
          </div>

          <div class="form-control">
            <label class="label">Capabilities</label>
            <select v-model="form.capabilities" multiple class="select select-bordered h-32">
              <option v-for="cap in availableCapabilities" :key="cap" :value="cap">
                {{ cap.toUpperCase() }}
              </option>
            </select>
            <label class="label">
              <span class="label-text-alt">Hold Ctrl/Cmd to select multiple</span>
            </label>
          </div>
        </div>
        
        <div class="flex justify-end gap-2 mt-6">
          <button type="button" class="btn btn-ghost" @click="router.back()">Cancel</button>
          <button type="submit" class="btn btn-primary" :disabled="loading">
            <span v-if="loading" class="loading loading-spinner"></span>
            Save
          </button>
        </div>
      </BaseCard>
    </form>
  </div>
</template>
