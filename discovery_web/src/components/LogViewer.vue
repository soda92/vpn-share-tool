<template>
  <el-dialog v-model="visible" title="Instance Logs" width="800px">
    <div class="log-container" ref="logContainer">
      <div v-if="loading" class="loading">Loading logs...</div>
      <pre v-else>{{ logs || 'No logs available.' }}</pre>
    </div>
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="fetchLogs">Refresh</el-button>
        <el-button @click="visible = false">Close</el-button>
      </span>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, watch, nextTick, onBeforeUnmount } from 'vue';
import { ElNotification } from 'element-plus';

const props = defineProps({
  modelValue: Boolean,
  serverAddress: String,
});

const emit = defineEmits(['update:modelValue']);

const visible = ref(false);
const logs = ref('');
const loading = ref(false);
const logContainer = ref(null);
let ws = null;

watch(() => props.modelValue, (val) => {
  visible.value = val;
  if (val && props.serverAddress) {
    connectWS();
  } else {
    closeWS();
  }
});

watch(visible, (val) => {
  emit('update:modelValue', val);
  if (!val) closeWS();
});

const connectWS = () => {
  closeWS();
  loading.value = true;
  logs.value = '';

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const url = `${protocol}//${window.location.host}/logs?address=${encodeURIComponent(props.serverAddress)}`;

  ws = new WebSocket(url);

  ws.onopen = () => {
    loading.value = false;
    // console.log("WS Connected");
  };

  ws.onmessage = (event) => {
    // Append new logs
    logs.value += event.data;
    nextTick(() => {
        if (logContainer.value) {
            logContainer.value.scrollTop = logContainer.value.scrollHeight;
        }
    });
  };

  ws.onerror = (err) => {
    console.error("WS Error:", err);
    ElNotification({ title: 'Error', message: 'Log stream error', type: 'error' });
    loading.value = false;
  };
  
  ws.onclose = () => {
    loading.value = false;
  };
};

const closeWS = () => {
  if (ws) {
    ws.close();
    ws = null;
  }
};

const fetchLogs = () => {
    // Reconnect to refresh
    connectWS();
};

onBeforeUnmount(() => {
  closeWS();
});
</script>

<style scoped>
.log-container {
  height: 500px;
  overflow-y: auto;
  background-color: #1e1e1e;
  color: #d4d4d4;
  padding: 10px;
  font-family: 'Consolas', 'Monaco', monospace;
  font-size: 12px;
  border-radius: 4px;
}
.loading {
    color: #888;
    text-align: center;
    margin-top: 20px;
}
pre {
    margin: 0;
    white-space: pre-wrap;
    word-break: break-all;
}
</style>
