<template>
  <div>
    <h1>Welcome to the Debugger</h1>
    <router-link to="/live">Start New Live Session</router-link>
    <h2>Saved Sessions</h2>
    <ul>
      <li v-for="session in sessions" :key="session.id">
        <router-link :to="`/session/${session.id}`">{{ session.name }}</router-link>
        <button @click="renameSession(session.id, session.name)">Rename</button>
        <button @click="deleteSession(session.id)">Delete</button>
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
