<template>
  <div class="container">
    <h2 class="view-title">Saved Sessions</h2>
    <div class="session-list-container">
      <ul v-if="sessions.length" class="session-list">
        <li v-for="session in sessions" :key="session.id" class="session-item">
          <router-link :to="`/session/${session.id}`" class="session-link">{{ session.name }}</router-link>
          <div class="session-actions">
            <button @click="exportHar(session.id, session.name)" class="action-btn">Export</button>
            <button @click="renameSession(session.id, session.name)" class="action-btn">Rename</button>
            <button @click="deleteSession(session.id)" class="action-btn delete-btn">Delete</button>
          </div>
        </li>
      </ul>
      <div v-else class="empty-state">No saved sessions.</div>
    </div>

    <!-- Floating Action Button for Import -->
    <button class="fab" @click="triggerImport" title="Import HAR">
      📂
    </button>
    <input type="file" ref="fileInput" @change="importHar" accept=".har" style="display: none" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { useRouter } from 'vue-router';
import { useToast } from 'vue-toastification';

const sessions = ref([]);
const fileInput = ref(null);
const router = useRouter();
const toast = useToast();

const fetchSessions = async () => {
  try {
    const response = await axios.get('/debug/sessions');
    sessions.value = response.data || [];
  } catch (e) {
    console.error('Failed to fetch sessions', e);
  }
};

const renameSession = async (id, oldName) => {
  const newName = prompt('Enter new session name:', oldName);
  if (newName && newName !== oldName) {
    await axios.put(`/debug/sessions/${id}`, { name: newName });
    fetchSessions();
  }
};

const deleteSession = async (id) => {
  if (confirm('Are you sure you want to delete this session?')) {
    await axios.delete(`/debug/sessions/${id}`);
    fetchSessions();
  }
};

const exportHar = (id, name) => {
  window.open(`/debug/sessions/${id}/har`, '_blank');
};

const triggerImport = () => {
  fileInput.value?.click();
};

const importHar = async (event) => {
  const target = event.target;
  const file = target.files?.[0];
  if (!file) return;

  const reader = new FileReader();
  reader.onload = async (e) => {
    try {
      const har = JSON.parse(e.target?.result);
      await axios.post(`/debug/har/import?name=${file.name}`, har);
      toast.success('HAR file imported successfully!');
      fetchSessions();
    } catch (error) {
      console.error('Failed to import HAR file', error);
      toast.error('Failed to import HAR file. See console for details.');
    }
  };
  reader.readAsText(file);
};

onMounted(fetchSessions);
</script>

<style scoped>
.container {
  height: 100%;
  display: flex;
  flex-direction: column;
  position: relative;
  padding: 1rem;
  overflow: hidden;
}

.view-title {
  margin: 0 0 1rem 0;
  color: #333;
}

.session-list-container {
  flex-grow: 1;
  overflow-y: auto;
}

.session-list {
  list-style: none;
  padding: 0;
  margin: 0;
}

.session-item {
  background-color: #ffffff;
  padding: 1rem;
  border-radius: 6px;
  margin-bottom: 0.8rem;
  display: flex;
  justify-content: space-between;
  align-items: center;
  box-shadow: 0 1px 3px rgba(0,0,0,0.1);
  border: 1px solid #e0e0e0;
}

.session-link {
  font-weight: 600;
  color: #007bff;
  text-decoration: none;
  font-size: 1.1rem;
}

.session-actions {
  display: flex;
  gap: 0.5rem;
}

.action-btn {
  background-color: #6c757d;
  color: white;
  border: none;
  padding: 0.4rem 0.8rem;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.9rem;
  transition: background-color 0.2s;
}

.action-btn:hover {
  background-color: #5a6268;
}

.delete-btn {
  background-color: #dc3545;
}

.delete-btn:hover {
  background-color: #c82333;
}

.empty-state {
  text-align: center;
  color: #777;
  margin-top: 2rem;
}

.fab {
  position: absolute;
  bottom: 20px;
  right: 20px;
  width: 56px;
  height: 56px;
  border-radius: 50%;
  background-color: #007bff;
  color: white;
  border: none;
  font-size: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 10px rgba(0,0,0,0.3);
  cursor: pointer;
  z-index: 1000;
  transition: transform 0.2s, background-color 0.2s;
}

.fab:hover {
  background-color: #0056b3;
  transform: scale(1.05);
}

.fab:active {
  transform: scale(0.95);
}

@media (max-width: 768px) {
  .session-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.8rem;
  }
  
  .session-actions {
    width: 100%;
    justify-content: flex-end;
  }
}
</style>
