<template>
  <!-- 
    lg:hidden: Hides on desktop (sidebar handles nav).
    sticky top-0 z-30: Keeps header fixed at top of viewport when scrolling on mobile.
  -->
  <header class="navbar bg-base-100 border-b border-base-300 min-h-[4rem] lg:hidden sticky top-0 z-30">
    <div class="flex-none">
      <label for="sidebar-drawer" class="btn btn-square btn-ghost">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="inline-block w-6 h-6 stroke-current">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"></path>
        </svg>
      </label>
    </div>
    
    <!-- Optional: Add Mobile Title/Logo here if desired in the future -->
    <div class="flex-1">
      <span class="btn btn-ghost normal-case text-xl truncate">Stone-Age.io</span>
    </div>
  </header>
</template>
