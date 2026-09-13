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
      // Visualizer is the home page
      {
        path: '',
        name: 'Visualizer',
        meta: { title: 'Dashboard' },
        component: () => import('@/views/dashboard/VisualizerView.vue'),
      },
      // Organizations (Operators and Super Users)
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

      // Leaf Nodes (edge nodes)
      { path: 'leaf-nodes', name: 'LeafNodes', component: () => import('@/views/leaf_nodes/LeafNodeListView.vue'), meta: { title: 'Leaf Nodes', requiresCapability: 'manageLeafNodes' } },
      { path: 'leaf-nodes/new', name: 'LeafNodeNew', component: () => import('@/views/leaf_nodes/LeafNodeFormView.vue'), meta: { title: 'New Leaf Node', requiresCapability: 'manageLeafNodes' } },
      { path: 'leaf-nodes/:id', name: 'LeafNodeDetail', component: () => import('@/views/leaf_nodes/LeafNodeDetailView.vue'), meta: { title: 'Leaf Node', requiresCapability: 'manageLeafNodes' } },
      { path: 'leaf-nodes/:id/edit', name: 'LeafNodeEdit', component: () => import('@/views/leaf_nodes/LeafNodeFormView.vue'), meta: { title: 'Edit Leaf Node', requiresCapability: 'manageLeafNodes' } },

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
      // audit_logs has no organization field, so its read rule is operators only.
      // Without this gate the page loads and renders an empty list for everyone else.
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
router.beforeEach(async (to) => {
  const authStore = useAuthStore()

  // Wait for auth hydration before reading anything off the store.
  //
  // Installing the router starts the initial navigation immediately, so without
  // this the FIRST guard run of a cold load raced main.ts's hydration: the token
  // is in localStorage and `user` is set synchronously, so isAuthenticated
  // passed -- but memberships arrive over the network, so every capability below
  // read false and every gated route bounced to '/'. That made deep links and
  // browser-refresh-on-any-gated-page silently land on the dashboard, while the
  // sidebar (reading the same capabilities, a tick later) showed the very link
  // that had just been refused.
  //
  // Memoized in the store, so this is free on every navigation after the first.
  await authStore.initializeFromAuth()

  if (to.meta.requiresAuth !== false && !authStore.isAuthenticated) {
    // Send them where they were going once they are signed in, rather than
    // dropping them on the dashboard -- the whole point of a working deep link.
    return to.fullPath === '/' ? '/login' : { path: '/login', query: { redirect: to.fullPath } }
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


/**
 * Keep the browser tab in step with the route.
 *
 * main.ts used to set document.title once, to the branding name, and nothing
 * ever changed it again -- so every tab of the console read the same word. In
 * a tool people keep four or five tabs of (a stream, a KV bucket, the host
 * they are debugging) that makes the tab strip useless.
 *
 * The title lives in each route meta rather than in a name -> title table off
 * to the side, so a route and its title are added, renamed and deleted
 * together. A route without one falls back to the bare app name.
 *
 * Exported because main.ts has to call it once more after branding resolves:
 * the first navigation is already settled by then, and appName until that
 * point is the compiled-in default.
 */
export function applyDocumentTitle(route = router.currentRoute.value) {
  const appName = useBrandingStore().appName
  const page = route.meta.title as string | undefined
  document.title = page ? page + ' · ' + appName : appName
}

router.afterEach((to) => applyDocumentTitle(to))

export default router
