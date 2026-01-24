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
      // Admin
      {
        path: 'organizations',
        name: 'AdminOrgList',
        component: () => import('@/views/admin/OrganizationListView.vue'),
        meta: { requiresSuperUser: true },
      },
      {
        path: 'organizations/new',
        name: 'AdminOrgNew',
        component: () => import('@/views/admin/OrganizationFormView.vue'),
        meta: { requiresSuperUser: true },
      },
      {
        path: 'organizations/:id',
        name: 'AdminOrgDetail',
        component: () => import('@/views/admin/OrganizationDetailView.vue'),
        meta: { requiresSuperUser: true },
      },
      {
        path: 'organizations/:id/edit',
        name: 'AdminOrgEdit',
        component: () => import('@/views/admin/OrganizationFormView.vue'),
        meta: { requiresSuperUser: true },
      },

      // Things
      { path: 'things/types', name: 'ThingTypes', component: () => import('@/views/things/ThingTypeListView.vue'), meta: { requiresRole: ['owner', 'admin'] } },
      { path: 'things/types/new', name: 'ThingTypeNew', component: () => import('@/views/things/ThingTypeFormView.vue'), meta: { requiresRole: ['owner', 'admin'] } },
      { path: 'things/types/:id/edit', name: 'ThingTypeEdit', component: () => import('@/views/things/ThingTypeFormView.vue'), meta: { requiresRole: ['owner', 'admin'] } },
      { path: 'things', name: 'Things', component: () => import('@/views/things/ThingListView.vue') },
      { path: 'things/new', name: 'ThingNew', component: () => import('@/views/things/ThingFormView.vue') },
      { path: 'things/:id', name: 'ThingDetail', component: () => import('@/views/things/ThingDetailView.vue') },
      { path: 'things/:id/edit', name: 'ThingEdit', component: () => import('@/views/things/ThingFormView.vue') },
      
      // Locations
      { path: 'locations/types', name: 'LocationTypes', component: () => import('@/views/locations/LocationTypeListView.vue'), meta: { requiresRole: ['owner', 'admin'] } },
      { path: 'locations/types/new', name: 'LocationTypeNew', component: () => import('@/views/locations/LocationTypeFormView.vue'), meta: { requiresRole: ['owner', 'admin'] } },
      { path: 'locations/types/:id/edit', name: 'LocationTypeEdit', component: () => import('@/views/locations/LocationTypeFormView.vue'), meta: { requiresRole: ['owner', 'admin'] } },
      { path: 'locations', name: 'Locations', component: () => import('@/views/locations/LocationListView.vue') },
      { path: 'locations/new', name: 'LocationNew', component: () => import('@/views/locations/LocationFormView.vue') },
      { path: 'locations/:id', name: 'LocationDetail', component: () => import('@/views/locations/LocationDetailView.vue') },
      { path: 'locations/:id/edit', name: 'LocationEdit', component: () => import('@/views/locations/LocationFormView.vue') },
      
      // NATS
      { path: 'nats', redirect: '/nats/account' },
      { path: 'nats/account', name: 'NatsAccountDetail', component: () => import('@/views/nats/NatsAccountDetailView.vue') },
      { path: 'nats/users', name: 'NatsUsers', component: () => import('@/views/nats/NatsUserListView.vue') },
      { path: 'nats/users/new', name: 'NatsUserNew', component: () => import('@/views/nats/NatsUserFormView.vue') },
      { path: 'nats/users/:id', name: 'NatsUserDetail', component: () => import('@/views/nats/NatsUserDetailView.vue') },
      { path: 'nats/users/:id/edit', name: 'NatsUserEdit', component: () => import('@/views/nats/NatsUserFormView.vue') },
      { path: 'nats/roles', name: 'NatsRoles', component: () => import('@/views/nats/NatsRoleListView.vue') },
      { path: 'nats/roles/new', name: 'NatsRoleNew', component: () => import('@/views/nats/NatsRoleFormView.vue') },
      { path: 'nats/roles/:id', name: 'NatsRoleDetail', component: () => import('@/views/nats/NatsRoleDetailView.vue') },
      { path: 'nats/roles/:id/edit', name: 'NatsRoleEdit', component: () => import('@/views/nats/NatsRoleFormView.vue') },
      
      // Nebula
      { path: 'nebula', redirect: '/nebula/ca' },
      { path: 'nebula/ca', name: 'NebulaCADetail', component: () => import('@/views/nebula/NebulaCADetailView.vue') },
      { path: 'nebula/networks', name: 'NebulaNetworks', component: () => import('@/views/nebula/NebulaNetworkListView.vue') },
      { path: 'nebula/networks/new', name: 'NebulaNetworkNew', component: () => import('@/views/nebula/NebulaNetworkFormView.vue') },
      { path: 'nebula/networks/:id', name: 'NebulaNetworkDetail', component: () => import('@/views/nebula/NebulaNetworkDetailView.vue') },
      { path: 'nebula/networks/:id/edit', name: 'NebulaNetworkEdit', component: () => import('@/views/nebula/NebulaNetworkFormView.vue') },
      { path: 'nebula/hosts', name: 'NebulaHosts', component: () => import('@/views/nebula/NebulaHostListView.vue') },
      { path: 'nebula/hosts/new', name: 'NebulaHostNew', component: () => import('@/views/nebula/NebulaHostFormView.vue') },
      { path: 'nebula/hosts/:id', name: 'NebulaHostDetail', component: () => import('@/views/nebula/NebulaHostDetailView.vue') },
      { path: 'nebula/hosts/:id/edit', name: 'NebulaHostEdit', component: () => import('@/views/nebula/NebulaHostFormView.vue') },
      
      // Audit
      { path: 'audit', name: 'AuditLogs', component: () => import('@/views/audit/AuditLogView.vue') },
      
      // Organization
      {
        path: 'organization',
        name: 'Organization',
        redirect: '/organization/members',
        meta: { requiresRole: ['owner', 'admin'] },
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
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to, _from, next) => {
  const authStore = useAuthStore()
  
  if (to.meta.requiresAuth !== false && !authStore.isAuthenticated) {
    next('/login')
    return
  }
  
  if (to.meta.requiresSuperUser && !authStore.isSuperAdmin) {
    next('/')
    return
  }

  if (to.meta.requiresRole && !authStore.isSuperAdmin) {
    const allowedRoles = to.meta.requiresRole as string[]
    if (!allowedRoles.includes(authStore.userRole)) {
      next('/')
      return
    }
  }
  
  next()
})

export default router
