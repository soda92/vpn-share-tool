<template>
  <div class="server-info-bar">
    <span class="server-label" v-t="'active_servers_title'"></span>:
    <span v-for="(server, index) in servers" :key="server.address" class="server-item">
      {{ server.address }}
      <span v-if="server.version" class="server-version">({{ server.version }})</span>
      <button 
        v-if="latestVersion && server.version && server.version !== latestVersion" 
        class="update-btn"
        @click="$emit('update-server', server.address)"
        :title="'Update to ' + latestVersion"
      >
        ↑
      </button>
      <button class="log-btn" @click="$emit('open-logs', server.address)" title="View Logs">≡</button>
      <span v-if="index < servers.length - 1">, </span>
    </span>
    <span v-if="!servers.length">{{ $t('no_active_servers') }}</span>
  </div>
</template>

<script setup>
defineProps({
  servers: {
    type: Array,
    default: () => []
  },
  latestVersion: {
    type: String,
    default: ''
  }
});

defineEmits(['update-server', 'open-logs']);
</script>

<style scoped>
.server-info-bar {
  margin-top: 0.5rem;
  padding: 0.5rem;
  background: #2c3e50;
  color: #ecf0f1;
  border-radius: 4px;
  font-size: 0.8rem;
  text-align: center;
  flex-shrink: 0;
  white-space: normal; /* Allow wrapping */
  overflow-y: auto;
  max-height: 80px; /* Limit height */
}

.server-label {
  font-weight: bold;
  color: #bdc3c7;
}

.server-item {
  font-family: monospace;
}

.server-version {
  color: #95a5a6;
  font-size: 0.75rem;
  margin-left: 2px;
}

.update-btn {
  background: #e67e22;
  color: white;
  border: none;
  border-radius: 2px;
  padding: 0 4px;
  margin-left: 4px;
  cursor: pointer;
  font-size: 0.7rem;
  line-height: 1.2;
}

.update-btn:hover {
  background: #d35400;
}

.log-btn {
  background: #34495e;
  color: #bdc3c7;
  border: 1px solid #7f8c8d;
  border-radius: 2px;
  padding: 0 4px;
  margin-left: 4px;
  cursor: pointer;
  font-size: 0.7rem;
  line-height: 1.2;
}

.log-btn:hover {
  background: #2c3e50;
  color: #ecf0f1;
}

@media (max-width: 768px) {
  .server-info-bar {
    white-space: normal;
    overflow: visible;
    text-align: left;
  }
}
</style>