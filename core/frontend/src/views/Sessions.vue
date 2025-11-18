<template>
  <div class="container">
    <h2>Saved Sessions</h2>
    <ul>
      <li v-for="session in sessions" :key="session.id">
        <router-link :to="`/session/${session.id}`">{{ session.name }}</router-link>
        <div>
          <button @click="exportHar(session.id, session.name)">Export</button>
          <button @click="renameSession(session.id, session.name)">Rename</button>
          <button @click="deleteSession(session.id)" class="delete-btn">Delete</button>
        </div>
      </li>
    </ul>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import axios from 'axios';

const sessions = ref([]);

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

onMounted(fetchSessions);
</script>

<style scoped>
ul {
  list-style: none;
  padding: 0;
}

li {
  background-color: #f8f9fa;
  padding: 0.75rem 1rem;
  border-radius: 4px;
  margin-bottom: 0.5rem;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

li button {
  background-color: #6c757d;
  color: white;
  border: none;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  cursor: pointer;
  margin-left: 0.5rem;
  transition: background-color 0.2s;
}

li button:hover {
  background-color: #5a6268;
}
</style>
