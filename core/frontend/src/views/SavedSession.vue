<template>
  <div>
    <h1>Saved Session: {{ sessionName }}</h1>
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
