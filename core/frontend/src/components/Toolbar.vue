<template>
  <nav class="toolbar">
    <router-link to="/live" class="toolbar-link">Live Session</router-link>
    <router-link to="/" class="toolbar-link">Saved Sessions</router-link>
    <router-link to="/decoder" class="toolbar-link">Form Decoder</router-link>
  </nav>
</template>

<script setup lang="ts">
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  background-color: #ffffff;
  padding: 0 1.5rem;
  height: 60px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  border-bottom: 1px solid #e0e0e0;
}

.toolbar-link {
  margin-right: 1.5rem;
  text-decoration: none;
  color: #555;
  font-weight: 600;
  padding: 0.5rem 1rem;
  border-radius: 5px;
  transition: background-color 0.2s, color 0.2s;
}

.toolbar-link:hover {
  background-color: #f0f0f0;
  color: #333;
}

.toolbar-link.router-link-exact-active {
  background-color: #007bff;
  color: white;
}
</style>
