<template>
  <nav class="toolbar">
    <div class="toolbar-nav">
      <router-link to="/live" class="toolbar-link">Live Session</router-link>
      <router-link to="/sessions" class="toolbar-link">Saved Sessions</router-link>
      <router-link to="/decoder" class="toolbar-link">Decoder</router-link>
    </div>
  </nav>
</template>

<script setup lang="ts">
// No script logic needed for links
</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: center; /* Center links since actions are gone */
  align-items: center;
  background-color: #ffffff;
  padding: 0 1rem;
  height: 60px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  border-bottom: 1px solid #e0e0e0;
}

.toolbar-nav {
  display: flex;
  align-items: center;
  overflow-x: auto;
  width: 100%;
  justify-content: center;
}

.toolbar-link {
  margin: 0 0.5rem;
  text-decoration: none;
  color: #555;
  font-weight: 600;
  padding: 0.5rem 0.8rem;
  border-radius: 5px;
  white-space: nowrap;
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

@media (max-width: 600px) {
  .toolbar {
    height: auto;
    padding: 0.5rem;
  }
  
  .toolbar-link {
    font-size: 0.9rem;
    padding: 0.4rem 0.6rem;
  }
}
</style>
