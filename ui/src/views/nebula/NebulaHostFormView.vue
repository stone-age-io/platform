<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import type { NebulaHost, NebulaNetwork } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'
import RecordPicker from '@/components/common/RecordPicker.vue'
import type { PickerOption } from '@/types/picker'

const props = defineProps<{
  embedded?: boolean
}>()

const emit = defineEmits<{
  (e: 'success', record: NebulaHost): void
  (e: 'cancel'): void
}>()

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const hostId = route.params.id as string | undefined
const isEdit = computed(() => !!hostId && !props.embedded)

// Form data
const formData = ref({
  // Identity
  hostname: '',
  email: '',
  password: '',
  passwordConfirm: '',
  
  // Network
  network_id: '',
  groups: '', // Managed as string for input
  overlay_ip: '',
  
  // Config
  is_lighthouse: false,
  is_relay: false,
  public_host_port: '',
  active: true,

  // Certificate (optional; empty = network default)
  validity_years: '',

  // Routing and transport. The list fields are edited as newline-separated
  // text for the same reason groups is edited as a comma-separated string:
  // one textarea beats a bespoke chip editor for something an operator pastes
  // in from their network documentation.
  unsafe_networks: '',
  preferred_ranges: '',
  unsafe_routes: [] as Array<{ route: string; via: string }>,
  mtu: '',
  tun_device: '',
})

/** Split a textarea into trimmed, non-empty lines. */
function lines(value: string): string[] {
  return value.split('\n').map(v => v.trim()).filter(v => v.length > 0)
}

/**
 * A lighthouse and a relay both need a reachable address, for different
 * reasons. Peers read a lighthouse's endpoint out of their own static_host_map;
 * a relay's address is learned at runtime, but without a public_host_port
 * pb-nebula gives it listen.port 0 (ephemeral) while handing every peer its
 * overlay IP as a usable path. The server rejects either case -- this only
 * stops the operator finding out by round trip.
 */
const needsPublicEndpoint = computed(
  () => formData.value.is_lighthouse || formData.value.is_relay,
)

function addUnsafeRoute() {
  formData.value.unsafe_routes.push({ route: '', via: '' })
}

function removeUnsafeRoute(index: number) {
  formData.value.unsafe_routes.splice(index, 1)
}

// Relation options
const networks = ref<NebulaNetwork[]>([])

const networkOptions = computed<PickerOption[]>(() =>
  networks.value.map(n => ({ id: n.id, label: n.name, sublabel: n.cidr_range }))
)

// State
const loading = ref(false)
const loadingOptions = ref(true)

/**
 * Load options (networks)
 */
async function loadOptions() {
  loadingOptions.value = true
  
  try {
    networks.value = await pb.collection('nebula_networks').getFullList<NebulaNetwork>({ 
      sort: 'name',
      filter: 'active = true'
    })
    
    // Auto-select first network if creating and only one exists
    if (!isEdit.value && networks.value.length === 1) {
      formData.value.network_id = networks.value[0].id
    }
  } catch (err: any) {
    toast.error('Failed to load networks')
  } finally {
    loadingOptions.value = false
  }
}

/**
 * Load existing host
 */
async function loadHost() {
  if (!hostId || props.embedded) return
  
  loading.value = true
  
  try {
    const host = await pb.collection('nebula_hosts').getOne<NebulaHost>(hostId)
    
    // Convert groups array to comma-separated string
    const groupsStr = Array.isArray(host.groups) ? host.groups.join(', ') : ''
    
    formData.value = {
      hostname: host.hostname,
      email: host.email,
      password: '',
      passwordConfirm: '',
      network_id: host.network_id,
      groups: groupsStr,
      overlay_ip: host.overlay_ip || '',
      is_lighthouse: host.is_lighthouse || false,
      is_relay: host.is_relay || false,
      public_host_port: host.public_host_port || '',
      active: host.active ?? true,
      validity_years: host.validity_years ? String(host.validity_years) : '',
      unsafe_networks: (host.unsafe_networks || []).join('\n'),
      preferred_ranges: (host.preferred_ranges || []).join('\n'),
      // Copied, not aliased: editing the rows must not mutate the loaded
      // record, or a cancelled edit would leave the form's idea of "unchanged"
      // wrong.
      unsafe_routes: (host.unsafe_routes || []).map(r => ({ ...r })),
      mtu: host.mtu ? String(host.mtu) : '',
      tun_device: host.tun_device || '',
    }
  } catch (err: any) {
    toast.error('Failed to load Nebula host')
    router.push('/nebula/hosts')
  } finally {
    loading.value = false
  }
}

/**
 * Handle form submission
 */
async function handleSubmit() {
  // Validation: Create mode requires password
  if (!isEdit.value && !formData.value.password) {
    toast.error('Password is required for new hosts')
    return
  }
  
  if (formData.value.password && formData.value.password !== formData.value.passwordConfirm) {
    toast.error('Passwords do not match')
    return
  }

  loading.value = true
  
  try {
    // Process groups string into array
    const groupsArray = formData.value.groups
      .split(',')
      .map(g => g.trim())
      .filter(g => g.length > 0)

    const data: any = {
      hostname: formData.value.hostname,
      email: formData.value.email,
      emailVisibility: true,
      network_id: formData.value.network_id,
      groups: groupsArray,
      is_lighthouse: formData.value.is_lighthouse,
      is_relay: formData.value.is_relay,
      public_host_port: formData.value.public_host_port || null,
      active: formData.value.active,
      overlay_ip: formData.value.overlay_ip,
      unsafe_networks: lines(formData.value.unsafe_networks),
      preferred_ranges: lines(formData.value.preferred_ranges),
      // A half-filled row is dropped rather than sent. pb-nebula would reject
      // it, but an operator who added a row and changed their mind has not
      // asked for an error.
      unsafe_routes: formData.value.unsafe_routes
        .map(r => ({ route: r.route.trim(), via: r.via.trim() }))
        .filter(r => r.route && r.via),
      tun_device: formData.value.tun_device.trim(),
      // 0 is pb-nebula's "inherit the default", and the schema's Min bound is
      // short-circuited on it -- so an empty box has to send 0, not null.
      mtu: formData.value.mtu ? parseInt(formData.value.mtu, 10) : 0,
    }

    // Only send validity_years when set; empty lets pb-nebula use its default
    // (and avoids the schema's min:1 check rejecting a null/0).
    if (formData.value.validity_years) {
      data.validity_years = parseInt(formData.value.validity_years, 10)
    }

    // Only send password if entered
    if (formData.value.password) {
      data.password = formData.value.password
      data.passwordConfirm = formData.value.passwordConfirm
    }
    
    let record: NebulaHost

    if (isEdit.value) {
      record = await pb.collection('nebula_hosts').update<NebulaHost>(hostId!, data)
      toast.success('Nebula host updated')
    } else {
      data.organization = authStore.currentOrgId
      record = await pb.collection('nebula_hosts').create<NebulaHost>(data)
      toast.success('Nebula host created')
    }
    
    if (props.embedded) {
      emit('success', record)
    } else {
      router.push('/nebula/hosts')
    }
  } catch (err: any) {
    toast.error(err.message || 'Failed to save Nebula host')
  } finally {
    loading.value = false
  }
}

function handleCancel() {
  if (props.embedded) {
    emit('cancel')
  } else {
    router.back()
  }
}

onMounted(() => {
  loadOptions()
  if (isEdit.value) {
    loadHost()
  }
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header: Hidden if Embedded -->
    <div v-if="!embedded">
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/nebula/hosts">Nebula Hosts</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">
        {{ isEdit ? 'Edit Nebula Host' : 'Provision Nebula Host' }}
      </h1>
    </div>
    
    <!-- Loading State -->
    <div v-if="loadingOptions" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Form -->
    <form v-else @submit.prevent="handleSubmit" class="space-y-6">
      
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        
        <!-- Left Column: Identity & Auth -->
        <div class="space-y-6">
          <BaseCard title="Identity">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Hostname *</span>
                </label>
                <input 
                  v-model="formData.hostname"
                  type="text" 
                  placeholder="e.g. laptop-01, server-prod"
                  class="input input-bordered font-mono"
                  required
                />
                <label class="label">
                  <span class="label-text-alt">
                    Unique identifier for this host in the Nebula network
                  </span>
                </label>
              </div>
            </div>
          </BaseCard>

          <BaseCard title="Authentication">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Email (Identity) *</span>
                </label>
                <input 
                  v-model="formData.email"
                  type="email" 
                  placeholder="host-uuid@nebula.local"
                  class="input input-bordered"
                  required
                />
                <label class="label">
                  <span class="label-text-alt">Unique email used for PocketBase authentication record.</span>
                </label>
              </div>
              
              <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div class="form-control">
                  <label class="label">
                    <span class="label-text">Password {{ isEdit ? '(Optional)' : '*' }}</span>
                  </label>
                  <input 
                    v-model="formData.password"
                    type="password" 
                    class="input input-bordered"
                    :required="!isEdit"
                    minlength="8"
                  />
                </div>
                
                <div class="form-control">
                  <label class="label">
                    <span class="label-text">Confirm Password {{ isEdit ? '(Optional)' : '*' }}</span>
                  </label>
                  <input 
                    v-model="formData.passwordConfirm"
                    type="password" 
                    class="input input-bordered"
                    :required="!!formData.password"
                  />
                </div>
              </div>
            </div>
          </BaseCard>
        </div>

        <!-- Right Column: Network, Role, Status -->
        <div class="space-y-6">
          <BaseCard title="Network Configuration">
            <div class="space-y-4">
              <!-- Network -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Network *</span>
                </label>
                <RecordPicker
                  v-model="formData.network_id"
                  :options="networkOptions"
                  title="Network"
                  placeholder="Select a network"
                  empty-text="No Nebula networks defined yet."
                />
              </div>

              <!-- Overlay IP (Required) -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Overlay IP *</span>
                </label>
                <input 
                  v-model="formData.overlay_ip"
                  type="text" 
                  placeholder="e.g. 10.100.0.5"
                  class="input input-bordered font-mono"
                  required
                />
                <label class="label">
                  <span class="label-text-alt text-base-content/60">
                    Ensure this address is unique within the network.
                  </span>
                </label>
              </div>

              <!-- Groups -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Groups</span>
                </label>
                <input 
                  v-model="formData.groups"
                  type="text" 
                  placeholder="server, ssh, http"
                  class="input input-bordered"
                />
                <label class="label">
                  <span class="label-text-alt">
                    Comma-separated list of groups for firewall rules
                  </span>
                </label>
              </div>
            </div>
          </BaseCard>

          <BaseCard title="Role & Status">
            <div class="space-y-4">
              <!-- Is Lighthouse -->
              <div class="form-control">
                <label class="label cursor-pointer justify-start gap-4">
                  <input 
                    v-model="formData.is_lighthouse"
                    type="checkbox" 
                    class="checkbox checkbox-primary"
                  />
                  <span class="label-text">
                    <span class="font-medium">Is Lighthouse</span>
                    <span class="block text-sm text-base-content/70 mt-1">
                      This host will serve as a lighthouse for other nodes. Requires a static public IP.
                    </span>
                  </span>
                </label>
              </div>

              <!-- Is Relay -->
              <div class="form-control">
                <label class="label cursor-pointer justify-start gap-4">
                  <input 
                    v-model="formData.is_relay"
                    type="checkbox" 
                    class="checkbox checkbox-primary"
                  />
                  <span class="label-text">
                    <span class="font-medium">Is Relay</span>
                    <span class="block text-sm text-base-content/70 mt-1">
                      Forwards traffic for peers that cannot reach each other directly.
                      Config only — a relay's certificate is no different.
                    </span>
                  </span>
                </label>
              </div>

              <!-- Public IP/Port (Conditional) -->
              <div v-if="needsPublicEndpoint" class="form-control pl-8 border-l-2 border-base-300">
                <label class="label">
                  <span class="label-text">Public Host:Port *</span>
                </label>
                <input 
                  v-model="formData.public_host_port"
                  type="text" 
                  placeholder="1.2.3.4:4242"
                  class="input input-bordered font-mono"
                  :required="needsPublicEndpoint"
                />
                <label class="label">
                  <span class="label-text-alt">
                    <template v-if="formData.is_lighthouse">
                      The publicly accessible address of this lighthouse. Peers read it
                      from their own static host map.
                    </template>
                    <template v-else>
                      Required for a relay too: without it the host listens on an
                      ephemeral port while every peer is handed its overlay IP as a
                      usable path.
                    </template>
                  </span>
                </label>
              </div>

              <!-- Active -->
              <div class="form-control">
                <label class="label cursor-pointer justify-start gap-4">
                  <input 
                    v-model="formData.active"
                    type="checkbox" 
                    class="toggle toggle-success"
                  />
                  <span class="label-text">
                    <span class="font-medium">Active Status</span>
                    <span class="block text-sm text-base-content/70">
                      Allow this host to connect to the network
                    </span>
                  </span>
                </label>
              </div>

              <!-- Certificate Validity -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Certificate Validity (years)</span>
                </label>
                <input
                  v-model="formData.validity_years"
                  type="number"
                  min="1"
                  max="10"
                  placeholder="Network default"
                  class="input input-bordered font-mono"
                />
                <label class="label">
                  <span class="label-text-alt">
                    Optional (1–10). Leave blank to use the default. Changing it on an existing host re-issues the certificate.
                  </span>
                </label>
              </div>
            </div>
          </BaseCard>
        </div>
      </div>
      
      <!--
        Routing and transport. Below the two columns rather than inside them:
        every field here is optional, most hosts need none of it, and putting it
        alongside the hostname would suggest otherwise.
      -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">

        <BaseCard title="Gateway Routing">
          <div class="space-y-4">
            <p class="text-sm text-base-content/70">
              The two halves of reaching a subnet that is not on the overlay. They live
              on <em>different</em> hosts and neither implies the other.
            </p>

            <div class="form-control">
              <label class="label">
                <span class="label-text">Unsafe Networks</span>
              </label>
              <textarea
                v-model="formData.unsafe_networks"
                rows="3"
                placeholder="192.168.1.0/24&#10;10.200.0.0/16"
                class="textarea textarea-bordered font-mono text-sm"
              ></textarea>
              <label class="label">
                <span class="label-text-alt">
                  Subnets <strong>this</strong> host routes to, one per line. Signed into
                  the certificate — Nebula authorizes routing on the certificate, not on
                  config, so saving this re-issues it and the change is inert until the
                  host picks up the new one.
                </span>
              </label>
            </div>

            <div class="form-control">
              <label class="label">
                <span class="label-text">Unsafe Routes</span>
              </label>

              <div v-if="formData.unsafe_routes.length === 0" class="text-sm text-base-content/50 pb-2">
                None. Add one to reach a subnet behind another host.
              </div>

              <div
                v-for="(row, index) in formData.unsafe_routes"
                :key="index"
                class="flex flex-col sm:flex-row gap-2 mb-2"
              >
                <input
                  v-model="row.route"
                  type="text"
                  placeholder="192.168.5.0/24"
                  class="input input-bordered input-sm font-mono flex-1"
                  aria-label="Route"
                />
                <input
                  v-model="row.via"
                  type="text"
                  placeholder="via 10.100.0.7"
                  class="input input-bordered input-sm font-mono flex-1"
                  aria-label="Gateway overlay IP"
                />
                <button
                  type="button"
                  @click="removeUnsafeRoute(index)"
                  class="btn btn-sm btn-ghost text-error"
                >
                  Remove
                </button>
              </div>

              <button type="button" @click="addUnsafeRoute" class="btn btn-sm btn-outline self-start">
                + Add Route
              </button>

              <label class="label">
                <span class="label-text-alt">
                  Subnets this host reaches <strong>through</strong> another, where
                  <code>via</code> is that gateway's overlay IP. Routes are never derived
                  from anyone's unsafe networks: two sites can legitimately advertise the
                  same prefix, and Nebula would load-balance across both — sending half of
                  every flow to the wrong LAN.
                </span>
              </label>
            </div>
          </div>
        </BaseCard>

        <BaseCard title="Transport">
          <div class="space-y-4">
            <div class="form-control">
              <label class="label">
                <span class="label-text">Preferred Ranges</span>
              </label>
              <textarea
                v-model="formData.preferred_ranges"
                rows="3"
                placeholder="192.168.1.0/24&#10;fd00::/8"
                class="textarea textarea-bordered font-mono text-sm"
              ></textarea>
              <label class="label">
                <span class="label-text-alt">
                  <strong>Underlay</strong> prefixes to favour when a peer advertises
                  several addresses — usually the LAN this host sits on, so two machines in
                  one rack talk over private addresses instead of public ones. IPv6 is
                  allowed here and nowhere else, because this is not the overlay.
                </span>
              </label>
            </div>

            <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div class="form-control">
                <label class="label">
                  <span class="label-text">MTU</span>
                </label>
                <input
                  v-model="formData.mtu"
                  type="number"
                  min="576"
                  max="9000"
                  placeholder="Default (1300)"
                  class="input input-bordered font-mono"
                />
                <label class="label">
                  <span class="label-text-alt">Blank inherits the default.</span>
                </label>
              </div>

              <div class="form-control">
                <label class="label">
                  <span class="label-text">TUN Device</span>
                </label>
                <input
                  v-model="formData.tun_device"
                  type="text"
                  maxlength="15"
                  placeholder="Default (nebula1)"
                  class="input input-bordered font-mono"
                />
                <label class="label">
                  <span class="label-text-alt">Interface name on the host.</span>
                </label>
              </div>
            </div>
          </div>
        </BaseCard>
      </div>

      <!-- Actions -->
      <div class="flex flex-col sm:flex-row justify-end gap-2 sm:gap-4">
        <button 
          type="button" 
          @click="handleCancel" 
          class="btn btn-ghost order-2 sm:order-1"
          :disabled="loading"
        >
          Cancel
        </button>
        <button 
          type="submit" 
          class="btn btn-primary order-1 sm:order-2"
          :disabled="loading"
        >
          <span v-if="loading" class="loading loading-spinner"></span>
          <span v-else>{{ isEdit ? 'Update' : 'Provision' }} Host</span>
        </button>
      </div>
    </form>
  </div>
</template>
