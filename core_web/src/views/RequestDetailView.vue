<template>
  <div class="container">
    <div v-if="request">
      <h1>Request Details: {{ request.id }}</h1>
      <pre>{{ request }}</pre>
    </div>
    <div v-else>
      <p>Loading request...</p>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import axios from 'axios';

const route = useRoute();
const requestId = route.params.id;
const request = ref(null);

onMounted(async () => {
  try {
    const response = await axios.get(`/debug/requests/${requestId}`);
    request.value = response.data;
  } catch (e) {
    console.error('Failed to fetch request', e);
  }
});
</script>

<style scoped>
.container {
  max-width: 1200px;
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

pre {
  background-color: #f8f9fa;
  padding: 1rem;
  border: 1px solid #ddd;
  border-radius: 6px;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: 'Fira Code', 'Courier New', Courier, monospace;
}
</style>
