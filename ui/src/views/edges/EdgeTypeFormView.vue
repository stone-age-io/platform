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
  features: '', // JSON String
})

async function loadData() {
  if (!id) return
  loading.value = true
  try {
    const record = await pb.collection('edge_types').getOne(id)
    form.value = {
      name: record.name,
      description: record.description,
      code: record.code,
      features: record.features ? JSON.stringify(record.features, null, 2) : '',
    }
  } catch (err: any) {
    toast.error('Failed to load type')
    router.push('/edges/types')
  } finally {
    loading.value = false
  }
}

async function submit() {
  let featuresJson = null
  if (form.value.features.trim()) {
    try {
      featuresJson = JSON.parse(form.value.features)
    } catch (e) {
      toast.error('Invalid JSON in Features field')
      return
    }
  }

  loading.value = true
  try {
    const data = {
      name: form.value.name,
      description: form.value.description,
      code: form.value.code,
      features: featuresJson,
      organization: isEdit.value ? undefined : authStore.currentOrgId
    }
    
    if (isEdit.value) {
      await pb.collection('edge_types').update(id!, data)
      toast.success('Updated')
    } else {
      await pb.collection('edge_types').create(data)
      toast.success('Created')
    }
    router.push('/edges/types')
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
  <div class="space-y-6">
    <div>
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/edges/types">Edge Types</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">{{ isEdit ? 'Edit' : 'Create' }} Edge Type</h1>
    </div>

    <form @submit.prevent="submit" class="space-y-6">
      
      <div class="grid grid-cols-1 md:grid-cols-2 gap-6 items-start">
        <!-- Left Column -->
        <BaseCard title="Identity">
          <div class="space-y-4">
            <div class="form-control">
              <label class="label">Name *</label>
              <input v-model="form.name" type="text" class="input input-bordered" required />
            </div>
            
            <div class="form-control">
              <label class="label">Code</label>
              <input v-model="form.code" type="text" class="input input-bordered font-mono" placeholder="Optional identifier" />
            </div>
          </div>
        </BaseCard>

        <!-- Right Column -->
        <BaseCard title="Configuration">
          <div class="space-y-4">
            <div class="form-control">
              <label class="label">Description</label>
              <textarea v-model="form.description" class="textarea textarea-bordered" rows="2"></textarea>
            </div>

            <div class="form-control">
              <label class="label">Features (JSON)</label>
              <textarea v-model="form.features" class="textarea textarea-bordered font-mono" rows="5" placeholder='{"hw_version": "1.0"}'></textarea>
            </div>
          </div>
        </BaseCard>
      </div>
      
      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-ghost" @click="router.back()">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="loading">
          <span v-if="loading" class="loading loading-spinner"></span>
          Save
        </button>
      </div>
    </form>
  </div>
</template>
