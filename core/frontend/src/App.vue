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
body {
  margin: 0;
  font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
  background-color: #f4f7f6;
  height: 100vh;
  overflow: hidden;
}

.app-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
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
}

/* Responsive tweaks if needed globally */
@media (max-width: 768px) {
  .app-content {
    overflow-y: auto; /* On mobile, maybe full scroll is better if views aren't complex flexboxes */
  }
}
</style>
