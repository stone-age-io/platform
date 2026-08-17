import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import MainLayout from '@/components/layout/MainLayout.vue'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/auth/LoginView.vue'),
    meta: { requiresAuth: false },
  },
  {
    path: '/accept-invite',
    name: 'AcceptInvite',
    component: () => import('@/views/auth/AcceptInviteView.vue'),
    meta: { requiresAuth: false },
  },
  {
    path: '/',
    component: MainLayout,
    meta: { requiresAuth: true },
    children: [
      // Visualizer is the home page
      {
        path: '',
        name: 'Visualizer',
        component: () => import('@/views/dashboard/VisualizerView.vue'),
      },
      // Organizations (Operators and Super Users)
      {
        path: 'organizations',
        name: 'AdminOrgList',
        component: () => import('@/views/admin/OrganizationListView.vue'),
        meta: { requiresOperator: true },
      },
      {
        path: 'organizations/new',
        name: 'AdminOrgNew',
        component: () => import('@/views/admin/OrganizationFormView.vue'),
        meta: { requiresOperator: true },
      },
      {
        path: 'organizations/:id',
        name: 'AdminOrgDetail',
        component: () => import('@/views/admin/OrganizationDetailView.vue'),
        meta: { requiresOperator: true },
      },
      {
        path: 'organizations/:id/edit',
        name: 'AdminOrgEdit',
        component: () => import('@/views/admin/OrganizationFormView.vue'),
        meta: { requiresOperator: true },
      },

      // Things
      { path: 'things/types', name: 'ThingTypes', component: () => import('@/views/things/ThingTypeListView.vue'), meta: { requiresCapability: 'manageDefinitions' } },
      { path: 'things/types/new', name: 'ThingTypeNew', component: () => import('@/views/things/ThingTypeFormView.vue'), meta: { requiresCapability: 'manageDefinitions' } },
      { path: 'things/types/:id/edit', name: 'ThingTypeEdit', component: () => import('@/views/things/ThingTypeFormView.vue'), meta: { requiresCapability: 'manageDefinitions' } },
      { path: 'things/operations', name: 'ThingTypeOperations', component: () => import('@/views/things/ThingTypeOperationListView.vue'), meta: { requiresCapability: 'manageDefinitions' } },
      { path: 'things/operations/new', name: 'ThingTypeOperationNew', component: () => import('@/views/things/ThingTypeOperationFormView.vue'), meta: { requiresCapability: 'manageDefinitions' } },
      { path: 'things/operations/:id/edit', name: 'ThingTypeOperationEdit', component: () => import('@/views/things/ThingTypeOperationFormView.vue'), meta: { requiresCapability: 'manageDefinitions' } },
      { path: 'things/schemas', name: 'MessageSchemas', component: () => import('@/views/things/MessageSchemaListView.vue'), meta: { requiresCapability: 'manageDefinitions' } },
      { path: 'things/schemas/new', name: 'MessageSchemaNew', component: () => import('@/views/things/MessageSchemaFormView.vue'), meta: { requiresCapability: 'manageDefinitions' } },
      { path: 'things/schemas/:id/edit', name: 'MessageSchemaEdit', component: () => import('@/views/things/MessageSchemaFormView.vue'), meta: { requiresCapability: 'manageDefinitions' } },
      // List and detail are viewInventory (member + viewer); the forms are
      // manageInventory. A viewer that types /things/x/edit lands back on '/'.
      { path: 'things', name: 'Things', component: () => import('@/views/things/ThingListView.vue'), meta: { requiresCapability: 'viewInventory' } },
      { path: 'things/new', name: 'ThingNew', component: () => import('@/views/things/ThingFormView.vue'), meta: { requiresCapability: 'manageInventory' } },
      { path: 'things/:id', name: 'ThingDetail', component: () => import('@/views/things/ThingDetailView.vue'), meta: { requiresCapability: 'viewInventory' } },
      { path: 'things/:id/edit', name: 'ThingEdit', component: () => import('@/views/things/ThingFormView.vue'), meta: { requiresCapability: 'manageInventory' } },

      // Leaf Nodes (edge nodes)
      { path: 'leaf-nodes', name: 'LeafNodes', component: () => import('@/views/leaf_nodes/LeafNodeListView.vue'), meta: { requiresCapability: 'manageLeafNodes' } },
      { path: 'leaf-nodes/new', name: 'LeafNodeNew', component: () => import('@/views/leaf_nodes/LeafNodeFormView.vue'), meta: { requiresCapability: 'manageLeafNodes' } },
      { path: 'leaf-nodes/:id', name: 'LeafNodeDetail', component: () => import('@/views/leaf_nodes/LeafNodeDetailView.vue'), meta: { requiresCapability: 'manageLeafNodes' } },
      { path: 'leaf-nodes/:id/edit', name: 'LeafNodeEdit', component: () => import('@/views/leaf_nodes/LeafNodeFormView.vue'), meta: { requiresCapability: 'manageLeafNodes' } },

      // Locations
      { path: 'locations/types', name: 'LocationTypes', component: () => import('@/views/locations/LocationTypeListView.vue'), meta: { requiresCapability: 'manageDefinitions' } },
      { path: 'locations/types/new', name: 'LocationTypeNew', component: () => import('@/views/locations/LocationTypeFormView.vue'), meta: { requiresCapability: 'manageDefinitions' } },
      { path: 'locations/types/:id/edit', name: 'LocationTypeEdit', component: () => import('@/views/locations/LocationTypeFormView.vue'), meta: { requiresCapability: 'manageDefinitions' } },
      { path: 'locations', name: 'Locations', component: () => import('@/views/locations/LocationListView.vue'), meta: { requiresCapability: 'viewInventory' } },
      { path: 'locations/new', name: 'LocationNew', component: () => import('@/views/locations/LocationFormView.vue'), meta: { requiresCapability: 'manageInventory' } },
      { path: 'locations/:id', name: 'LocationDetail', component: () => import('@/views/locations/LocationDetailView.vue'), meta: { requiresCapability: 'viewInventory' } },
      { path: 'locations/:id/edit', name: 'LocationEdit', component: () => import('@/views/locations/LocationFormView.vue'), meta: { requiresCapability: 'manageInventory' } },
      
      // NATS
      { path: 'nats', redirect: '/nats/account' },
      { path: 'nats/account', name: 'NatsAccountDetail', component: () => import('@/views/nats/NatsAccountDetailView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/users', name: 'NatsUsers', component: () => import('@/views/nats/NatsUserListView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/users/new', name: 'NatsUserNew', component: () => import('@/views/nats/NatsUserFormView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/users/:id', name: 'NatsUserDetail', component: () => import('@/views/nats/NatsUserDetailView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/users/:id/edit', name: 'NatsUserEdit', component: () => import('@/views/nats/NatsUserFormView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/roles', name: 'NatsRoles', component: () => import('@/views/nats/NatsRoleListView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/roles/new', name: 'NatsRoleNew', component: () => import('@/views/nats/NatsRoleFormView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/roles/:id', name: 'NatsRoleDetail', component: () => import('@/views/nats/NatsRoleDetailView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/roles/:id/edit', name: 'NatsRoleEdit', component: () => import('@/views/nats/NatsRoleFormView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },

      // NATS Account Exports
      { path: 'nats/exports', name: 'NatsExports', component: () => import('@/views/nats/NatsExportListView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/exports/new', name: 'NatsExportNew', component: () => import('@/views/nats/NatsExportFormView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/exports/:id', name: 'NatsExportDetail', component: () => import('@/views/nats/NatsExportDetailView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/exports/:id/edit', name: 'NatsExportEdit', component: () => import('@/views/nats/NatsExportFormView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },

      // NATS Account Imports
      { path: 'nats/imports', name: 'NatsImports', component: () => import('@/views/nats/NatsImportListView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/imports/new', name: 'NatsImportNew', component: () => import('@/views/nats/NatsImportFormView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/imports/:id', name: 'NatsImportDetail', component: () => import('@/views/nats/NatsImportDetailView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/imports/:id/edit', name: 'NatsImportEdit', component: () => import('@/views/nats/NatsImportFormView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },

      // JetStream Streams
      { path: 'nats/streams', name: 'JetStreamStreams', component: () => import('@/views/nats/StreamListView.vue'), meta: { requiresCapability: 'manageMessaging' } },
      { path: 'nats/streams/new', name: 'JetStreamStreamNew', component: () => import('@/views/nats/StreamFormView.vue'), meta: { requiresCapability: 'manageMessaging' } },
      { path: 'nats/streams/:name', name: 'JetStreamStreamDetail', component: () => import('@/views/nats/StreamDetailView.vue'), meta: { requiresCapability: 'manageMessaging' } },
      { path: 'nats/streams/:name/edit', name: 'JetStreamStreamEdit', component: () => import('@/views/nats/StreamFormView.vue'), meta: { requiresCapability: 'manageMessaging' } },

      // KV Buckets
      { path: 'nats/kv', name: 'KvBuckets', component: () => import('@/views/nats/KvBucketListView.vue'), meta: { requiresCapability: 'manageMessaging' } },
      { path: 'nats/kv/new', name: 'KvBucketNew', component: () => import('@/views/nats/KvBucketFormView.vue'), meta: { requiresCapability: 'manageMessaging' } },
      { path: 'nats/kv/:name', name: 'KvBucketDetail', component: () => import('@/views/nats/KvBucketDetailView.vue'), meta: { requiresCapability: 'manageMessaging' } },

      // Nebula
      { path: 'nebula', redirect: '/nebula/ca' },
      { path: 'nebula/ca', name: 'NebulaCADetail', component: () => import('@/views/nebula/NebulaCADetailView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nebula/networks', name: 'NebulaNetworks', component: () => import('@/views/nebula/NebulaNetworkListView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nebula/networks/new', name: 'NebulaNetworkNew', component: () => import('@/views/nebula/NebulaNetworkFormView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nebula/networks/:id', name: 'NebulaNetworkDetail', component: () => import('@/views/nebula/NebulaNetworkDetailView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nebula/networks/:id/edit', name: 'NebulaNetworkEdit', component: () => import('@/views/nebula/NebulaNetworkFormView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nebula/hosts', name: 'NebulaHosts', component: () => import('@/views/nebula/NebulaHostListView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nebula/hosts/new', name: 'NebulaHostNew', component: () => import('@/views/nebula/NebulaHostFormView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nebula/hosts/:id', name: 'NebulaHostDetail', component: () => import('@/views/nebula/NebulaHostDetailView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      { path: 'nebula/hosts/:id/edit', name: 'NebulaHostEdit', component: () => import('@/views/nebula/NebulaHostFormView.vue'), meta: { requiresCapability: 'manageInfrastructure' } },
      
      // Audit
      // audit_logs has no organization field, so its read rule is operators only.
      // Without this gate the page loads and renders an empty list for everyone else.
      { path: 'audit', name: 'AuditLogs', component: () => import('@/views/audit/AuditLogView.vue'), meta: { requiresOperator: true } },
      
      // Organization
      {
        path: 'organization',
        name: 'Organization',
        redirect: '/organization/members',
        meta: { requiresCapability: 'manageMembers' },
        children: [
          { path: 'members', name: 'OrganizationMembers', component: () => import('@/views/organization/MembersView.vue') },
          { path: 'members/:id', name: 'MemberDetail', component: () => import('@/views/organization/MemberDetailView.vue')},
          { path: 'invitations', name: 'OrganizationInvitations', component: () => import('@/views/organization/InvitationsView.vue') },
        ]
      },
      
      // Settings
      { path: 'settings', name: 'Settings', component: () => import('@/views/settings/UserSettingsView.vue') },
    ],
  },
  // Catch-all: redirect unknown paths to home
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// Vue Router 5 deprecates the next() callback (VUE_ROUTER_R0025) in favour of
// returning the decision: a path string redirects, false aborts, and returning
// nothing lets the navigation through.
router.beforeEach((to) => {
  const authStore = useAuthStore()

  if (to.meta.requiresAuth !== false && !authStore.isAuthenticated) {
    return '/login'
  }

  // Dashboard users reach the Visualizer at '/' and their own settings, nothing
  // else. There is no separate route tree for them: '/' is already the Visualizer,
  // and VisualizerView strips its chrome off the ROLE rather than off the path.
  if (authStore.isDashboardUser && to.meta.requiresAuth !== false) {
    const allowed = ['/', '/settings']
    const permitted = allowed.some(p => to.path === p || (p !== '/' && to.path.startsWith(p + '/')))
    if (!permitted) {
      return '/'
    }
  }

  if (to.meta.requiresSuperUser && !authStore.isSuperAdmin) {
    return '/'
  }

  // Operator routes: accessible to operators and super users
  if (to.meta.requiresOperator && !authStore.canManageOrganizations) {
    return '/'
  }

  // Capability routes. The capability -> role table lives in stores/auth.ts so
  // the sidebar and individual views resolve access the same way this does.
  // This is navigation convenience, not enforcement: the server's API rules are
  // the boundary, and scripts/test-authz.sh is what proves them.
  if (to.meta.requiresCapability && !authStore.isSuperAdmin) {
    const capability = to.meta.requiresCapability as keyof typeof authStore.can
    if (!authStore.can[capability]) {
      return '/'
    }
  }
})

export default router
