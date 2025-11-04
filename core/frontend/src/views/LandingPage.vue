<template>
  <div class="container">
    <h1>Welcome to the Debugger</h1>
    <router-link to="/live" class="start-session-link">Start New Live Session</router-link>
    <h2>Saved Sessions</h2>
    <ul>
      <li v-for="session in sessions" :key="session.id">
        <router-link :to="`/session/${session.id}`">{{ session.name }}</router-link>
        <div>
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

onMounted(fetchSessions);
</script>

<style scoped>
.container {
  max-width: 800px;
  margin: 2rem auto;
  padding: 2rem;
  background-color: #fff;
  border-radius: 8px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
}

h1 {
  color: #0056b3;
  border-bottom: 2px solid #007bff;
  padding-bottom: 0.5rem;
  margin-bottom: 1.5rem;
}

h2 {
  color: #333;
  margin-top: 2rem;
  margin-bottom: 1rem;
}

.start-session-link {
  display: inline-block;
  background-color: #007bff;
  color: white;
  padding: 0.75rem 1.5rem;
  border-radius: 5px;
  text-decoration: none;
  font-weight: bold;
  transition: background-color 0.2s;
}

.start-session-link:hover {
  background-color: #0056b3;
}

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

.delete-btn {
  background-color: #dc3545 !important;
}

.delete-btn:hover {
  background-color: #c82333 !important;
}
</style>
