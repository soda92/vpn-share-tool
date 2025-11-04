<template>
  <div v-if="request">
    <h1>Request Details: {{ request.id }}</h1>
    <pre>{{ request }}</pre>
  </div>
  <div v-else>
    <p>Loading request...</p>
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
