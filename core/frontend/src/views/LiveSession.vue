<template>
  <div class="container">
    <div class="session-header">
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
.session-header {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  padding: 1rem 0;
}
</style>
