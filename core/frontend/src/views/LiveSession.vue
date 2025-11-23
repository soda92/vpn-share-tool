<template>
  <div class="container">
    <!-- Header removed, title handled by toolbar/view context -->
    <DebugView :sessionId="'live_session'" :isLive="true" @save-session="saveSession" />

    <!-- Floating Action Button for Saving -->
    <button class="fab" @click="saveSession" title="Save Session">
      💾
    </button>
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
.container {
  height: 100%;
  display: flex;
  flex-direction: column;
  position: relative;
  /* For FAB positioning */
}

/* Removed session-header styles */

.fab {
  position: absolute;
  bottom: 20px;
  right: 20px;
  width: 56px;
  height: 56px;
  border-radius: 50%;
  background-color: #28a745;
  color: white;
  border: none;
  font-size: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 10px rgba(0, 0, 0, 0.3);
  cursor: pointer;
  z-index: 1000;
  transition: transform 0.2s, background-color 0.2s;
}

.fab:hover {
  background-color: #218838;
  transform: scale(1.05);
}

.fab:active {
  transform: scale(0.95);
}
</style>
