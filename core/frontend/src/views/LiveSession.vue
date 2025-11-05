<template>
  <div class="session-view-container">
    <div class="session-header">
      <h1>Live Session</h1>
      <button @click="saveSession">Save as New Session</button>
    </div>
    <DebugView :sessionId="'live_session'" :isLive="true" />
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router';
import DebugView from '../components/DebugView.vue';
import axios from 'axios';

const router = useRouter();

const saveSession = async () => {
  const name = prompt('Enter a name for this session:');
  if (name) {
    try {
      const response = await axios.post('/debug/sessions', { name });
      alert(`Session saved as '${name}'.`);
      router.push(`/session/${response.data.id}`);
    } catch (error) {
      console.error('Error saving session:', error);
      alert('Failed to save session.');
    }
  }
};
</script>

<style scoped>
.session-view-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
}

.session-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem;
  background-color: #f8f9fa;
  border-bottom: 1px solid #ddd;
}

h1 {
  margin: 0;
  font-size: 1.5rem;
}

button {
  background-color: #28a745;
  color: white;
  border: none;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  cursor: pointer;
  transition: background-color 0.2s;
}

button:hover {
  background-color: #218838;
}
</style>
