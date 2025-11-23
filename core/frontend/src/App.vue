<template>
  <div class="app-container">
    <header class="app-header">
      <Toolbar />
    </header>
    <main class="app-content">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import Toolbar from './components/Toolbar.vue';
</script>

<style>
*, *::before, *::after {
  box-sizing: border-box;
}

body {
  margin: 0;
  font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
  background-color: #f4f7f6;
  height: 100vh;
  width: 100%; /* Safer than 100vw */
  overflow: hidden; /* Default desktop */
  overflow-x: hidden; /* Never horizontal scroll body */
}

.app-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
  width: 100%;
  max-width: 100%; /* Prevent growth beyond parent */
  overflow-x: hidden;
}

.app-header {
  flex-shrink: 0;
  z-index: 100;
}

.app-content {
  flex-grow: 1;
  overflow: hidden; /* Views handle their own scrolling */
  display: flex;
  flex-direction: column;
  min-width: 0; /* Allow flex children to shrink */
}

/* Responsive tweaks */
@media (max-width: 768px) {
  body {
    height: auto !important;
    overflow-y: auto !important;
    overflow-x: hidden !important;
  }
  .app-container {
    height: auto !important;
    overflow-y: visible !important;
    overflow-x: hidden !important;
  }
  .app-content {
    overflow: visible !important;
    height: auto !important;
  }
}
</style>
