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
  justify-content: center;
  align-items: center;
  background-color: #ffffff;
  padding: 0 1rem;
  height: 60px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
  border-bottom: 1px solid #f0f0f0;
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
  color: #666;
  font-weight: 600;
  font-size: 0.95rem;
  padding: 0.5rem 1rem;
  border-radius: 20px;
  white-space: nowrap;
  transition: all 0.2s ease;
}

.toolbar-link:hover {
  background-color: #f5f5f5;
  color: #333;
}

.toolbar-link.router-link-exact-active {
  background-color: #673ab7;
  color: white;
  box-shadow: 0 2px 4px rgba(103, 58, 183, 0.25);
}

@media (max-width: 600px) {
  .toolbar {
    height: auto;
    padding: 0.5rem;
  }
  
  .toolbar-link {
    font-size: 0.85rem;
    padding: 0.4rem 0.8rem;
  }
}
</style>
