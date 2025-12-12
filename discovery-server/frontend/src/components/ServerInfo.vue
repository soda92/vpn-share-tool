<template>
  <div class="server-info-bar">
    <span class="server-label" v-t="'active_servers_title'"></span>:
    <span v-for="(server, index) in servers" :key="server.address" class="server-item">
      {{ server.address }}
      <span v-if="server.version" class="server-version">(v{{ server.version }})</span>
      <button 
        v-if="latestVersion && server.version && server.version !== latestVersion" 
        class="update-btn"
        @click="$emit('update-server', server.address)"
        :title="'Update to ' + latestVersion"
      >
        ↑
      </button>
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

defineEmits(['update-server']);
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
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
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

@media (max-width: 768px) {
  .server-info-bar {
    white-space: normal;
    overflow: visible;
    text-align: left;
  }
}
</style>