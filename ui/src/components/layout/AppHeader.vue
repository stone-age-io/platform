<template>
  <!-- 
    "lg:hidden": Hides the entire header on large screens (Desktop).
    On desktop, the sidebar is always open, so we reclaim this vertical space.
  -->
  <header class="navbar bg-base-100 border-b border-base-300 min-h-[4rem] lg:hidden">
    <div class="flex-none">
      <label for="sidebar-drawer" class="btn btn-square btn-ghost">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="inline-block w-6 h-6 stroke-current">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"></path>
        </svg>
      </label>
    </div>
  </header>
</template>
