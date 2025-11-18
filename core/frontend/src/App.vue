<template>
  <div>
    <Toolbar />
    <main>
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import Toolbar from './components/Toolbar.vue';
</script>

<style>
body {
  margin: 0;
  font-family: sans-serif;
}
</style>
