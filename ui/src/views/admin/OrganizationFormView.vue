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
  active: true,
})

async function loadData() {
  if (!id) return
  loading.value = true
  try {
    const record = await pb.collection('organizations').getOne(id)
    form.value = {
      name: record.name,
      description: record.description || '',
      active: record.active,
    }
  } catch (err: any) {
    toast.error('Failed to load organization')
    router.push('/organizations')
  } finally {
    loading.value = false
  }
}

async function submit() {
  loading.value = true
  try {
    const data: any = { ...form.value }
    
    // For new orgs, assign current superuser as owner
    if (!isEdit.value && authStore.user) {
      data.owner = authStore.user.id
    }

    if (isEdit.value) {
      await pb.collection('organizations').update(id!, data)
      toast.success('Organization updated')
    } else {
      await pb.collection('organizations').create(data)
      toast.success('Organization created')
    }
    router.push('/organizations')
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
  <div class="max-w-2xl mx-auto space-y-6">
    <div class="breadcrumbs text-sm">
      <ul>
        <li><router-link to="/organizations">Organizations</router-link></li>
        <li>{{ isEdit ? 'Edit' : 'New' }}</li>
      </ul>
    </div>
    
    <h1 class="text-3xl font-bold">{{ isEdit ? 'Edit' : 'Create' }} Organization</h1>

    <form @submit.prevent="submit">
      <BaseCard>
        <div class="space-y-4">
          <div class="form-control">
            <label class="label">Name *</label>
            <input v-model="form.name" type="text" class="input input-bordered" required />
          </div>
          
          <div class="form-control">
            <label class="label">Description</label>
            <textarea v-model="form.description" class="textarea textarea-bordered" rows="3"></textarea>
          </div>

          <div class="form-control">
            <label class="label cursor-pointer justify-start gap-4">
              <input v-model="form.active" type="checkbox" class="toggle toggle-success" />
              <span class="label-text">Active</span>
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
