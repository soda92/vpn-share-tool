<template>
  <nav class="toolbar">
    <div class="toolbar-nav">
      <router-link to="/live" class="toolbar-link">Live Session</router-link>
      <router-link to="/sessions" class="toolbar-link">Saved Sessions</router-link>
      <router-link to="/decoder" class="toolbar-link">Form Decoder</router-link>
    </div>
    <div class="toolbar-actions">
      <button @click="triggerImport" class="action-btn">Import HAR</button>
      <input type="file" ref="fileInput" @change="importHar" accept=".har" style="display: none" />
    </div>
  </nav>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import axios from 'axios';
import { useRouter } from 'vue-router';

const fileInput = ref<HTMLInputElement | null>(null);
const router = useRouter();

const triggerImport = () => {
  fileInput.value?.click();
};

const importHar = async (event: Event) => {
  const target = event.target as HTMLInputElement;
  const file = target.files?.[0];
  if (!file) return;

  const reader = new FileReader();
  reader.onload = async (e) => {
    try {
      const har = JSON.parse(e.target?.result as string);
      await axios.post(`/debug/har/import?name=${file.name}`, har);
      alert('HAR file imported successfully!');
      // After import, navigate to the sessions page to see the new session
      router.push('/sessions');
    } catch (error) {
      console.error('Failed to import HAR file', error);
      alert('Failed to import HAR file. See console for details.');
    }
  };
  reader.readAsText(file);
};
</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
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
  overflow-x: auto; /* Allow horizontal scroll on very small screens */
}

.toolbar-link {
  margin-right: 1rem;
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

.toolbar-actions .action-btn {
  font-size: 0.9rem;
  padding: 0.5rem 1rem;
}

@media (max-width: 600px) {
  .toolbar {
    height: auto;
    flex-direction: column;
    padding: 0.5rem;
    gap: 0.5rem;
  }
  
  .toolbar-nav {
    width: 100%;
    justify-content: center;
  }
  
  .toolbar-link {
    margin: 0 0.2rem;
    font-size: 0.9rem;
    padding: 0.4rem 0.6rem;
  }
  
  .toolbar-actions {
    width: 100%;
    display: flex;
    justify-content: center;
  }
}
</style>
