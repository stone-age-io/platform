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
})

async function loadData() {
  if (!id) return
  loading.value = true
  try {
    const record = await pb.collection('location_types').getOne(id)
    form.value = {
      name: record.name,
      description: record.description,
      code: record.code,
    }
  } catch (err: any) {
    toast.error('Failed to load type')
    router.push('/locations/types')
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
      await pb.collection('location_types').update(id!, data)
      toast.success('Updated')
    } else {
      await pb.collection('location_types').create(data)
      toast.success('Created')
    }
    router.push('/locations/types')
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
          <li><router-link to="/locations/types">Location Types</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">{{ isEdit ? 'Edit' : 'Create' }} Location Type</h1>
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
        <BaseCard title="Details">
          <div class="space-y-4">
            <div class="form-control">
              <label class="label">Description</label>
              <textarea v-model="form.description" class="textarea textarea-bordered" rows="5"></textarea>
            </div>
          </div>
        </BaseCard>
      </div>
      
      <!-- Actions -->
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
