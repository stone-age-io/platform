import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import MainLayout from '@/components/layout/MainLayout.vue'

/**
 * Route definitions
 */
const routes: RouteRecordRaw[] = [
  // ============================================================================
  // PUBLIC ROUTES (No auth required)
  // ============================================================================
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/auth/LoginView.vue'),
    meta: { requiresAuth: false },
  },
  
  // ============================================================================
  // AUTHENTICATED ROUTES
  // ============================================================================
  {
    path: '/accept-invite',
    name: 'AcceptInvite',
    component: () => import('@/views/auth/AcceptInviteView.vue'),
    meta: { requiresAuth: true },
  },
  
  // ============================================================================
  // MAIN APPLICATION (with layout)
  // ============================================================================
  {
    path: '/',
    component: MainLayout,
    meta: { requiresAuth: true },
    children: [
      // Dashboard
      {
        path: '',
        name: 'Dashboard',
        component: () => import('@/views/dashboard/DashboardView.vue'),
      },
      
      // --- THINGS ---
      {
        path: 'things',
        name: 'Things',
        component: () => import('@/views/things/ThingListView.vue'),
      },
      {
        path: 'things/new',
        name: 'ThingNew',
        component: () => import('@/views/things/ThingFormView.vue'),
      },
      // Thing Types (Admin/Owner only) - Defined BEFORE :id to prevent conflict
      {
        path: 'things/types',
        name: 'ThingTypes',
        component: () => import('@/views/things/ThingTypeListView.vue'),
        meta: { requiresRole: ['owner', 'admin'] },
      },
      {
        path: 'things/types/new',
        name: 'ThingTypeNew',
        component: () => import('@/views/things/ThingTypeFormView.vue'),
        meta: { requiresRole: ['owner', 'admin'] },
      },
      {
        path: 'things/types/:id/edit',
        name: 'ThingTypeEdit',
        component: () => import('@/views/things/ThingTypeFormView.vue'),
        meta: { requiresRole: ['owner', 'admin'] },
      },
      // Specific Thing Routes
      {
        path: 'things/:id',
        name: 'ThingDetail',
        component: () => import('@/views/things/ThingDetailView.vue'),
      },
      {
        path: 'things/:id/edit',
        name: 'ThingEdit',
        component: () => import('@/views/things/ThingFormView.vue'),
      },
      
      // --- EDGES ---
      {
        path: 'edges',
        name: 'Edges',
        component: () => import('@/views/edges/EdgeListView.vue'),
      },
      {
        path: 'edges/new',
        name: 'EdgeNew',
        component: () => import('@/views/edges/EdgeFormView.vue'),
      },
      // Edge Types (Admin/Owner only)
      {
        path: 'edges/types',
        name: 'EdgeTypes',
        component: () => import('@/views/edges/EdgeTypeListView.vue'),
        meta: { requiresRole: ['owner', 'admin'] },
      },
      {
        path: 'edges/types/new',
        name: 'EdgeTypeNew',
        component: () => import('@/views/edges/EdgeTypeFormView.vue'),
        meta: { requiresRole: ['owner', 'admin'] },
      },
      {
        path: 'edges/types/:id/edit',
        name: 'EdgeTypeEdit',
        component: () => import('@/views/edges/EdgeTypeFormView.vue'),
        meta: { requiresRole: ['owner', 'admin'] },
      },
      // Specific Edge Routes
      {
        path: 'edges/:id',
        name: 'EdgeDetail',
        component: () => import('@/views/edges/EdgeDetailView.vue'),
      },
      {
        path: 'edges/:id/edit',
        name: 'EdgeEdit',
        component: () => import('@/views/edges/EdgeFormView.vue'),
      },
      
      // --- LOCATIONS ---
      {
        path: 'locations',
        name: 'Locations',
        component: () => import('@/views/locations/LocationListView.vue'),
      },
      {
        path: 'locations/new',
        name: 'LocationNew',
        component: () => import('@/views/locations/LocationFormView.vue'),
      },
      // Location Types (Admin/Owner only)
      {
        path: 'locations/types',
        name: 'LocationTypes',
        component: () => import('@/views/locations/LocationTypeListView.vue'),
        meta: { requiresRole: ['owner', 'admin'] },
      },
      {
        path: 'locations/types/new',
        name: 'LocationTypeNew',
        component: () => import('@/views/locations/LocationTypeFormView.vue'),
        meta: { requiresRole: ['owner', 'admin'] },
      },
      {
        path: 'locations/types/:id/edit',
        name: 'LocationTypeEdit',
        component: () => import('@/views/locations/LocationTypeFormView.vue'),
        meta: { requiresRole: ['owner', 'admin'] },
      },
      // Specific Location Routes
      {
        path: 'locations/:id',
        name: 'LocationDetail',
        component: () => import('@/views/locations/LocationDetailView.vue'),
      },
      {
        path: 'locations/:id/edit',
        name: 'LocationEdit',
        component: () => import('@/views/locations/LocationFormView.vue'),
      },
      
      // --- MAP ---
      {
        path: 'map',
        name: 'Map',
        component: () => import('@/views/map/MapView.vue'),
      },
      
      // --- NATS ---
      {
        path: 'nats',
        redirect: '/nats/accounts',
      },
      {
        path: 'nats/accounts',
        name: 'NatsAccounts',
        component: () => import('@/views/nats/NatsAccountListView.vue'),
      },
      {
        path: 'nats/accounts/:id',
        name: 'NatsAccountDetail',
        component: () => import('@/views/nats/NatsAccountDetailView.vue'),
      },
      {
        path: 'nats/users',
        name: 'NatsUsers',
        component: () => import('@/views/nats/NatsUserListView.vue'),
      },
      {
        path: 'nats/users/new',
        name: 'NatsUserNew',
        component: () => import('@/views/nats/NatsUserFormView.vue'),
      },
      {
        path: 'nats/users/:id',
        name: 'NatsUserDetail',
        component: () => import('@/views/nats/NatsUserDetailView.vue'),
      },
      {
        path: 'nats/users/:id/edit',
        name: 'NatsUserEdit',
        component: () => import('@/views/nats/NatsUserFormView.vue'),
      },
      {
        path: 'nats/roles',
        name: 'NatsRoles',
        component: () => import('@/views/nats/NatsRoleListView.vue'),
      },
      {
        path: 'nats/roles/new',
        name: 'NatsRoleNew',
        component: () => import('@/views/nats/NatsRoleFormView.vue'),
      },
      {
        path: 'nats/roles/:id',
        name: 'NatsRoleDetail',
        component: () => import('@/views/nats/NatsRoleDetailView.vue'),
      },
      {
        path: 'nats/roles/:id/edit',
        name: 'NatsRoleEdit',
        component: () => import('@/views/nats/NatsRoleFormView.vue'),
      },
      
      // --- NEBULA ---
      {
        path: 'nebula',
        redirect: '/nebula/cas',
      },
      {
        path: 'nebula/cas',
        name: 'NebulaCAs',
        component: () => import('@/views/nebula/NebulaCAListView.vue'),
      },
      {
        path: 'nebula/cas/:id',
        name: 'NebulaCADetail',
        component: () => import('@/views/nebula/NebulaCADetailView.vue'),
      },
      {
        path: 'nebula/networks',
        name: 'NebulaNetworks',
        component: () => import('@/views/nebula/NebulaNetworkListView.vue'),
      },
      {
        path: 'nebula/networks/new',
        name: 'NebulaNetworkNew',
        component: () => import('@/views/nebula/NebulaNetworkFormView.vue'),
      },
      {
        path: 'nebula/networks/:id',
        name: 'NebulaNetworkDetail',
        component: () => import('@/views/nebula/NebulaNetworkDetailView.vue'),
      },
      {
        path: 'nebula/networks/:id/edit',
        name: 'NebulaNetworkEdit',
        component: () => import('@/views/nebula/NebulaNetworkFormView.vue'),
      },
      {
        path: 'nebula/hosts',
        name: 'NebulaHosts',
        component: () => import('@/views/nebula/NebulaHostListView.vue'),
      },
      {
        path: 'nebula/hosts/new',
        name: 'NebulaHostNew',
        component: () => import('@/views/nebula/NebulaHostFormView.vue'),
      },
      {
        path: 'nebula/hosts/:id',
        name: 'NebulaHostDetail',
        component: () => import('@/views/nebula/NebulaHostDetailView.vue'),
      },
      {
        path: 'nebula/hosts/:id/edit',
        name: 'NebulaHostEdit',
        component: () => import('@/views/nebula/NebulaHostFormView.vue'),
      },
      
      // --- AUDIT LOGS ---
      {
        path: 'audit',
        name: 'AuditLogs',
        component: () => import('@/views/audit/AuditLogView.vue'),
      },
      
      // --- ORGANIZATION MANAGEMENT (Admin Only) ---
      {
        path: 'organization',
        name: 'Organization',
        redirect: '/organization/members',
        meta: { requiresRole: ['owner', 'admin'] },
      },
      {
        path: 'organization/members',
        name: 'OrganizationMembers',
        component: () => import('@/views/organization/MembersView.vue'),
        meta: { requiresRole: ['owner', 'admin'] },
      },
      {
        path: 'organization/invitations',
        name: 'OrganizationInvitations',
        component: () => import('@/views/organization/InvitationsView.vue'),
        meta: { requiresRole: ['owner', 'admin'] },
      },
      
      // --- USER SETTINGS ---
      {
        path: 'settings',
        name: 'Settings',
        component: () => import('@/views/settings/UserSettingsView.vue'),
      },
    ],
  },
]

/**
 * Create router instance
 */
const router = createRouter({
  history: createWebHistory(),
  routes,
})

/**
 * Navigation guard
 * Checks authentication and role requirements before each route
 */
router.beforeEach((to, _from, next) => {
  const authStore = useAuthStore()
  
  // Check if route requires authentication
  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    next('/login')
    return
  }
  
  // Check if route requires specific role
  if (to.meta.requiresRole) {
    const allowedRoles = to.meta.requiresRole as string[]
    if (!allowedRoles.includes(authStore.userRole)) {
      // Redirect to dashboard if user doesn't have required role
      next('/')
      return
    }
  }
  
  next()
})

export default router
