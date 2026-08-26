<template>
  <div class="logs-view">
    <div class="header-card">
      <div class="title-row">
        <div>
          <h2>Instance Logs</h2>
          <p class="subtitle">Live log stream from vpn-share-tool.</p>
        </div>
        <div class="controls">
          <label class="control-item">
            <span>Lines:</span>
            <select v-model.number="lineLimit" @change="fetchLogs">
              <option :value="50">50</option>
              <option :value="100">100</option>
              <option :value="200">200</option>
              <option :value="500">500</option>
              <option :value="1000">1000</option>
            </select>
          </label>
          <label class="control-item checkbox-label">
            <input type="checkbox" v-model="autoRefresh" />
            <span>Auto Refresh (2s)</span>
          </label>
          <button class="btn-refresh" @click="fetchLogs" :disabled="loading">
            {{ loading ? 'Loading...' : '🔄 Refresh' }}
          </button>
        </div>
      </div>
    </div>

    <div class="log-container" ref="logContainerRef">
      <div v-if="loading && lines.length === 0" class="loading-state">
        Fetching logs...
      </div>
      <div v-else-if="lines.length === 0" class="empty-state">
        No log entries found.
      </div>
      <pre v-else class="log-content"><code v-for="(line, idx) in lines" :key="idx" :class="getLineClass(line)">{{ line }}
</code></pre>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue';
import axios from 'axios';

const lines = ref<string[]>([]);
const lineLimit = ref(200);
const loading = ref(false);
const autoRefresh = ref(true);
const logContainerRef = ref<HTMLElement | null>(null);
let refreshTimer: number | undefined;

const fetchLogs = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/debug/logs', {
      params: { lines: lineLimit.value }
    });
    lines.value = res.data.lines || [];
    await nextTick();
    scrollToBottom();
  } catch (err) {
    console.error('Failed to fetch logs:', err);
  } finally {
    loading.value = false;
  }
};

const scrollToBottom = () => {
  if (logContainerRef.value) {
    logContainerRef.value.scrollTop = logContainerRef.value.scrollHeight;
  }
};

const getLineClass = (line: string) => {
  if (line.includes('ERROR') || line.includes('Failed') || line.includes('Error')) {
    return 'log-error';
  }
  if (line.includes('WARN') || line.includes('warning') || line.includes('Warning')) {
    return 'log-warn';
  }
  if (line.includes('Health check successful') || line.includes('Starting proxy') || line.includes('Re-using historical port')) {
    return 'log-success';
  }
  return 'log-info';
};

onMounted(() => {
  fetchLogs();
  refreshTimer = window.setInterval(() => {
    if (autoRefresh.value) {
      fetchLogs();
    }
  }, 2000);
});

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer);
});
</script>

<style scoped>
.logs-view {
  max-width: 1200px;
  margin: 0 auto;
  padding: 1.5rem 1rem;
  display: flex;
  flex-direction: column;
  height: calc(100vh - 90px);
}

.header-card {
  background: white;
  padding: 1rem 1.5rem;
  border-radius: 8px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
  margin-bottom: 1rem;
  flex-shrink: 0;
}

.title-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 1rem;
}

.title-row h2 {
  margin: 0 0 0.2rem 0;
  font-size: 1.25rem;
}

.subtitle {
  color: #777;
  font-size: 0.85rem;
  margin: 0;
}

.controls {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.control-item {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.85rem;
  color: #555;
}

select {
  padding: 0.3rem 0.6rem;
  border: 1px solid #ccc;
  border-radius: 4px;
  outline: none;
}

.btn-refresh {
  background: #f5f5f5;
  border: 1px solid #ddd;
  padding: 0.35rem 0.8rem;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.85rem;
}

.btn-refresh:hover:not(:disabled) {
  background: #e0e0e0;
}

.log-container {
  background: #1e1e1e;
  border-radius: 8px;
  box-shadow: inset 0 2px 6px rgba(0, 0, 0, 0.3);
  padding: 1rem;
  overflow-y: auto;
  flex: 1;
}

.log-content {
  margin: 0;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 0.82rem;
  line-height: 1.45;
  white-space: pre-wrap;
  word-break: break-all;
}

.log-info {
  color: #d4d4d4;
}

.log-error {
  color: #f48771;
  font-weight: bold;
}

.log-warn {
  color: #cca700;
}

.log-success {
  color: #89d185;
}

.loading-state, .empty-state {
  color: #888;
  text-align: center;
  padding: 3rem;
  font-style: italic;
}
</style>
