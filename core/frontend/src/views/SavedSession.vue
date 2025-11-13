<template>
  <div class="session-view-container">
    <div class="session-header">
      <h1>Saved Session: {{ sessionName }}</h1>
    </div>
    <DebugView :isLive="false" :sessionId="sessionId" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import DebugView from '../components/DebugView.vue';

const route = useRoute();
const sessionId = route.params.id;
const sessionName = ref(''); // In a real app, you'd fetch the session name
</script>

<style scoped>
.session-view-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
}

.session-header {
  padding: 1rem;
  background-color: #f8f9fa;
  border-bottom: 1px solid #ddd;
}

h1 {
  margin: 0;
  font-size: 1.5rem;
}
</style>
