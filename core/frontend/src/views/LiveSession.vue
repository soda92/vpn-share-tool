<template>
  <div>
    <h1>Live Session</h1>
    <button @click="saveSession">Save Session</button>
    <DebugView :isLive="true" />
  </div>
</template>

<script setup>
import DebugView from '../components/DebugView.vue';
import axios from 'axios';

const saveSession = async () => {
  const name = prompt('Enter a name for this session:');
  if (name) {
    await axios.post('/debug/sessions', { name });
    alert('Session saved!');
  }
};
</script>
