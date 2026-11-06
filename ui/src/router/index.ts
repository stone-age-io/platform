import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useBrandingStore } from '@/stores/branding'
import MainLayout from '@/components/layout/MainLayout.vue'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/auth/LoginView.vue'),
    meta: { title: 'Sign In', requiresAuth: false },
  },
  {
    path: '/accept-invite',
    name: 'AcceptInvite',
    component: () => import('@/views/auth/AcceptInviteView.vue'),
    meta: { title: 'Accept Invitation', requiresAuth: false },
  },
  {
    path: '/',
    component: MainLayout,
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        name: 'Visualizer',
        meta: { title: 'Dashboard' },
        component: () => import('@/views/dashboard/VisualizerView.vue'),
      },
      // Organizations
      {
        path: 'organizations',
        name: 'AdminOrgList',
        component: () => import('@/views/admin/OrganizationListView.vue'),
        meta: { title: 'Organizations', requiresOperator: true },
      },
      {
        path: 'organizations/new',
        name: 'AdminOrgNew',
        component: () => import('@/views/admin/OrganizationFormView.vue'),
        meta: { title: 'New Organization', requiresOperator: true },
      },
      {
        path: 'organizations/:id',
        name: 'AdminOrgDetail',
        component: () => import('@/views/admin/OrganizationDetailView.vue'),
        meta: { title: 'Organization', requiresOperator: true },
      },
      {
        path: 'organizations/:id/edit',
        name: 'AdminOrgEdit',
        component: () => import('@/views/admin/OrganizationFormView.vue'),
        meta: { title: 'Edit Organization', requiresOperator: true },
      },

      // Things
      { path: 'things/types', name: 'ThingTypes', component: () => import('@/views/things/ThingTypeListView.vue'), meta: { title: 'Thing Types', requiresCapability: 'manageDefinitions' } },
      { path: 'things/types/new', name: 'ThingTypeNew', component: () => import('@/views/things/ThingTypeFormView.vue'), meta: { title: 'New Thing Type', requiresCapability: 'manageDefinitions' } },
      { path: 'things/types/:id/edit', name: 'ThingTypeEdit', component: () => import('@/views/things/ThingTypeFormView.vue'), meta: { title: 'Edit Thing Type', requiresCapability: 'manageDefinitions' } },
      { path: 'things/operations', name: 'ThingTypeOperations', component: () => import('@/views/things/ThingTypeOperationListView.vue'), meta: { title: 'Operations', requiresCapability: 'manageDefinitions' } },
      { path: 'things/operations/new', name: 'ThingTypeOperationNew', component: () => import('@/views/things/ThingTypeOperationFormView.vue'), meta: { title: 'New Operation', requiresCapability: 'manageDefinitions' } },
      { path: 'things/operations/:id/edit', name: 'ThingTypeOperationEdit', component: () => import('@/views/things/ThingTypeOperationFormView.vue'), meta: { title: 'Edit Operation', requiresCapability: 'manageDefinitions' } },
      // List and detail are viewInventory (member + viewer); the forms are
      // manageInventory. A viewer that types /things/x/edit lands back on '/'.
      { path: 'things', name: 'Things', component: () => import('@/views/things/ThingListView.vue'), meta: { title: 'Things', requiresCapability: 'viewInventory' } },
      { path: 'things/new', name: 'ThingNew', component: () => import('@/views/things/ThingFormView.vue'), meta: { title: 'New Thing', requiresCapability: 'manageInventory' } },
      { path: 'things/:id', name: 'ThingDetail', component: () => import('@/views/things/ThingDetailView.vue'), meta: { title: 'Thing', requiresCapability: 'viewInventory' } },
      { path: 'things/:id/edit', name: 'ThingEdit', component: () => import('@/views/things/ThingFormView.vue'), meta: { title: 'Edit Thing', requiresCapability: 'manageInventory' } },


      // Locations
      { path: 'locations/types', name: 'LocationTypes', component: () => import('@/views/locations/LocationTypeListView.vue'), meta: { title: 'Location Types', requiresCapability: 'manageDefinitions' } },
      { path: 'locations/types/new', name: 'LocationTypeNew', component: () => import('@/views/locations/LocationTypeFormView.vue'), meta: { title: 'New Location Type', requiresCapability: 'manageDefinitions' } },
      { path: 'locations/types/:id/edit', name: 'LocationTypeEdit', component: () => import('@/views/locations/LocationTypeFormView.vue'), meta: { title: 'Edit Location Type', requiresCapability: 'manageDefinitions' } },
      { path: 'locations', name: 'Locations', component: () => import('@/views/locations/LocationListView.vue'), meta: { title: 'Locations', requiresCapability: 'viewInventory' } },
      { path: 'locations/new', name: 'LocationNew', component: () => import('@/views/locations/LocationFormView.vue'), meta: { title: 'New Location', requiresCapability: 'manageInventory' } },
      { path: 'locations/:id', name: 'LocationDetail', component: () => import('@/views/locations/LocationDetailView.vue'), meta: { title: 'Location', requiresCapability: 'viewInventory' } },
      { path: 'locations/:id/edit', name: 'LocationEdit', component: () => import('@/views/locations/LocationFormView.vue'), meta: { title: 'Edit Location', requiresCapability: 'manageInventory' } },
      
      // NATS
      { path: 'nats', redirect: '/nats/account' },
      { path: 'nats/account', name: 'NatsAccountDetail', component: () => import('@/views/nats/NatsAccountDetailView.vue'), meta: { title: 'NATS Account', requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/users', name: 'NatsUsers', component: () => import('@/views/nats/NatsUserListView.vue'), meta: { title: 'NATS Users', requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/users/new', name: 'NatsUserNew', component: () => import('@/views/nats/NatsUserFormView.vue'), meta: { title: 'New NATS User', requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/users/:id', name: 'NatsUserDetail', component: () => import('@/views/nats/NatsUserDetailView.vue'), meta: { title: 'NATS User', requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/users/:id/edit', name: 'NatsUserEdit', component: () => import('@/views/nats/NatsUserFormView.vue'), meta: { title: 'Edit NATS User', requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/roles', name: 'NatsRoles', component: () => import('@/views/nats/NatsRoleListView.vue'), meta: { title: 'NATS Roles', requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/roles/new', name: 'NatsRoleNew', component: () => import('@/views/nats/NatsRoleFormView.vue'), meta: { title: 'New NATS Role', requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/roles/:id', name: 'NatsRoleDetail', component: () => import('@/views/nats/NatsRoleDetailView.vue'), meta: { title: 'NATS Role', requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/roles/:id/edit', name: 'NatsRoleEdit', component: () => import('@/views/nats/NatsRoleFormView.vue'), meta: { title: 'Edit NATS Role', requiresCapability: 'manageInfrastructure' } },

      // NATS Account Exports
      { path: 'nats/exports', name: 'NatsExports', component: () => import('@/views/nats/NatsExportListView.vue'), meta: { title: 'Account Exports', requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/exports/new', name: 'NatsExportNew', component: () => import('@/views/nats/NatsExportFormView.vue'), meta: { title: 'New Export', requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/exports/:id', name: 'NatsExportDetail', component: () => import('@/views/nats/NatsExportDetailView.vue'), meta: { title: 'Export', requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/exports/:id/edit', name: 'NatsExportEdit', component: () => import('@/views/nats/NatsExportFormView.vue'), meta: { title: 'Edit Export', requiresCapability: 'manageInfrastructure' } },

      // NATS Account Imports
      { path: 'nats/imports', name: 'NatsImports', component: () => import('@/views/nats/NatsImportListView.vue'), meta: { title: 'Account Imports', requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/imports/new', name: 'NatsImportNew', component: () => import('@/views/nats/NatsImportFormView.vue'), meta: { title: 'New Import', requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/imports/:id', name: 'NatsImportDetail', component: () => import('@/views/nats/NatsImportDetailView.vue'), meta: { title: 'Import', requiresCapability: 'manageInfrastructure' } },
      { path: 'nats/imports/:id/edit', name: 'NatsImportEdit', component: () => import('@/views/nats/NatsImportFormView.vue'), meta: { title: 'Edit Import', requiresCapability: 'manageInfrastructure' } },

      // JetStream Streams
      { path: 'nats/streams', name: 'JetStreamStreams', component: () => import('@/views/nats/StreamListView.vue'), meta: { title: 'Streams', requiresCapability: 'manageMessaging' } },
      { path: 'nats/streams/new', name: 'JetStreamStreamNew', component: () => import('@/views/nats/StreamFormView.vue'), meta: { title: 'New Stream', requiresCapability: 'manageMessaging' } },
      { path: 'nats/streams/:name', name: 'JetStreamStreamDetail', component: () => import('@/views/nats/StreamDetailView.vue'), meta: { title: 'Stream', requiresCapability: 'manageMessaging' } },
      { path: 'nats/streams/:name/edit', name: 'JetStreamStreamEdit', component: () => import('@/views/nats/StreamFormView.vue'), meta: { title: 'Edit Stream', requiresCapability: 'manageMessaging' } },

      // KV Buckets
      { path: 'nats/kv', name: 'KvBuckets', component: () => import('@/views/nats/KvBucketListView.vue'), meta: { title: 'KV Buckets', requiresCapability: 'manageMessaging' } },
      { path: 'nats/kv/new', name: 'KvBucketNew', component: () => import('@/views/nats/KvBucketFormView.vue'), meta: { title: 'New KV Bucket', requiresCapability: 'manageMessaging' } },
      { path: 'nats/kv/:name', name: 'KvBucketDetail', component: () => import('@/views/nats/KvBucketDetailView.vue'), meta: { title: 'KV Bucket', requiresCapability: 'manageMessaging' } },

      // Micro service discovery ($SRV), read-only
      { path: 'nats/services', name: 'NatsServices', component: () => import('@/views/nats/ServiceListView.vue'), meta: { title: 'Services', requiresCapability: 'manageMessaging' } },
      { path: 'nats/services/:name', name: 'NatsServiceDetail', component: () => import('@/views/nats/ServiceDetailView.vue'), meta: { title: 'Service', requiresCapability: 'manageMessaging' } },

      // Nebula
      { path: 'nebula', redirect: '/nebula/ca' },
      { path: 'nebula/ca', name: 'NebulaCADetail', component: () => import('@/views/nebula/NebulaCADetailView.vue'), meta: { title: 'Nebula CA', requiresCapability: 'manageInfrastructure' } },
      { path: 'nebula/networks', name: 'NebulaNetworks', component: () => import('@/views/nebula/NebulaNetworkListView.vue'), meta: { title: 'Nebula Networks', requiresCapability: 'manageInfrastructure' } },
      { path: 'nebula/networks/new', name: 'NebulaNetworkNew', component: () => import('@/views/nebula/NebulaNetworkFormView.vue'), meta: { title: 'New Network', requiresCapability: 'manageInfrastructure' } },
      { path: 'nebula/networks/:id', name: 'NebulaNetworkDetail', component: () => import('@/views/nebula/NebulaNetworkDetailView.vue'), meta: { title: 'Network', requiresCapability: 'manageInfrastructure' } },
      { path: 'nebula/networks/:id/edit', name: 'NebulaNetworkEdit', component: () => import('@/views/nebula/NebulaNetworkFormView.vue'), meta: { title: 'Edit Network', requiresCapability: 'manageInfrastructure' } },
      { path: 'nebula/hosts', name: 'NebulaHosts', component: () => import('@/views/nebula/NebulaHostListView.vue'), meta: { title: 'Nebula Hosts', requiresCapability: 'manageInfrastructure' } },
      { path: 'nebula/hosts/new', name: 'NebulaHostNew', component: () => import('@/views/nebula/NebulaHostFormView.vue'), meta: { title: 'New Host', requiresCapability: 'manageInfrastructure' } },
      { path: 'nebula/hosts/:id', name: 'NebulaHostDetail', component: () => import('@/views/nebula/NebulaHostDetailView.vue'), meta: { title: 'Host', requiresCapability: 'manageInfrastructure' } },
      { path: 'nebula/hosts/:id/edit', name: 'NebulaHostEdit', component: () => import('@/views/nebula/NebulaHostFormView.vue'), meta: { title: 'Edit Host', requiresCapability: 'manageInfrastructure' } },
      
      // Audit
      // A capability rather than requiresOperator: `activity` is org-scoped,
      // unlike audit_logs. Navigation only; the schema.json read rule is the boundary.
      { path: 'activity', name: 'Activity', component: () => import('@/views/activity/ActivityView.vue'), meta: { title: 'Activity', requiresCapability: 'viewActivity' } },

      // audit_logs has no organization field, so only operators can read it.
      { path: 'audit', name: 'AuditLogs', component: () => import('@/views/audit/AuditLogView.vue'), meta: { title: 'Audit Log', requiresOperator: true } },
      
      // Organization
      {
        path: 'organization',
        name: 'Organization',
        redirect: '/organization/members',
        meta: { requiresCapability: 'manageMembers' },
        children: [
          { path: 'members', name: 'OrganizationMembers', component: () => import('@/views/organization/MembersView.vue') , meta: { title: 'Members' } },
          { path: 'members/:id', name: 'MemberDetail', component: () => import('@/views/organization/MemberDetailView.vue'), meta: { title: 'Member' } },
          { path: 'invitations', name: 'OrganizationInvitations', component: () => import('@/views/organization/InvitationsView.vue') , meta: { title: 'Invitations' } },
        ]
      },
      
      // Settings
      { path: 'settings', name: 'Settings', component: () => import('@/views/settings/UserSettingsView.vue') , meta: { title: 'Settings' } },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

// The widget gallery (views/dev/WidgetGalleryView.vue). It has no auth guard, so
// it must stay inside `import.meta.env.DEV`: a production build drops the block
// and the view from the bundle. Spliced in before the catch-all, which would
// otherwise swallow the path.
if (import.meta.env.DEV) {
  routes.splice(routes.length - 1, 0, {
    path: '/dev/widgets',
    name: 'DevWidgetGallery',
    component: () => import('@/views/dev/WidgetGalleryView.vue'),
    meta: { title: 'Widget Gallery', requiresAuth: false },
  })
}

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// Vue Router 5 deprecates next() (VUE_ROUTER_R0025): return a path to redirect,
// false to abort, nothing to allow.
router.beforeEach(async (to) => {
  const authStore = useAuthStore()

  // Must await hydration before reading a capability. The initial navigation
  // starts as soon as the router is installed; `user` is set synchronously but
  // memberships arrive over the network, so without this every capability reads
  // false on a cold load and every gated deep link or refresh bounces to '/'.
  // Memoized in the store, so later navigations cost nothing.
  await authStore.initializeFromAuth()

  if (to.meta.requiresAuth !== false && !authStore.isAuthenticated) {
    // Carry the deep link through login. LoginView accepts ?redirect= only as a
    // relative in-app path, or it would be an open redirect.
    return to.fullPath === '/' ? '/login' : { path: '/login', query: { redirect: to.fullPath } }
  }

  // Dashboard users reach only '/' and /settings. VisualizerView strips its
  // chrome by role, not by path, so they need no separate route tree.
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

  if (to.meta.requiresOperator && !authStore.canManageOrganizations) {
    return '/'
  }

  // The capability table is in stores/auth.ts, shared with the sidebar. This is
  // navigation, not enforcement: the server's API rules are the boundary.
  if (to.meta.requiresCapability && !authStore.isSuperAdmin) {
    const capability = to.meta.requiresCapability as keyof typeof authStore.can
    if (!authStore.can[capability]) {
      return '/'
    }
  }
})


/**
 * Sets document.title from the route's `meta.title`, falling back to the app name.
 * Exported because main.ts calls it again once branding resolves, after the
 * first navigation has already used the compiled-in default name.
 */
export function applyDocumentTitle(route = router.currentRoute.value) {
  const appName = useBrandingStore().appName
  const page = route.meta.title as string | undefined
  document.title = page ? page + ' · ' + appName : appName
}

router.afterEach((to) => applyDocumentTitle(to))

export default router
