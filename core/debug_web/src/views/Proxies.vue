<template>
  <div class="proxies-view">
    <div class="header-card">
      <h2>Proxy Management</h2>
      <p class="subtitle">Share target URLs on this instance or stop active proxy endpoints.</p>

      <form @submit.prevent="shareUrl" class="share-form">
        <div class="form-group url-input">
          <input
            v-model="newUrl"
            type="text"
            placeholder="Target URL (e.g. http://10.216.11.24:8306/phis/app/login)"
            required
            :disabled="isSubmitting"
          />
        </div>
        <div class="form-group port-input">
          <input
            v-model.number="requestedPort"
            type="number"
            placeholder="Port (0 = auto)"
            min="0"
            max="65535"
            :disabled="isSubmitting"
          />
        </div>
        <button type="submit" class="btn-share" :disabled="isSubmitting">
          {{ isSubmitting ? 'Sharing...' : 'Share URL' }}
        </button>
      </form>
      <p v-if="errorMessage" class="error-msg">{{ errorMessage }}</p>
    </div>

    <div class="proxies-list-container">
      <div class="list-header">
        <h3>Active Proxies ({{ proxies.length }})</h3>
        <button class="btn-refresh" @click="fetchProxies" :disabled="loading">
          {{ loading ? 'Refreshing...' : '🔄 Refresh' }}
        </button>
      </div>

      <div v-if="loading && proxies.length === 0" class="loading-state">
        Loading active proxies...
      </div>

      <div v-else-if="proxies.length === 0" class="empty-state">
        No active proxies currently shared on this instance.
      </div>

      <div v-else class="proxy-cards">
        <div v-for="p in proxies" :key="p.remote_port" class="proxy-card">
          <div class="proxy-header">
            <span class="badge-port">:{{ p.remote_port }}</span>
            <div class="proxy-systems" v-if="p.active_systems && p.active_systems.length">
              <span v-for="sys in p.active_systems" :key="sys" class="badge-system">{{ sys }}</span>
            </div>
            <div class="proxy-actions">
              <a :href="p.shared_url" target="_blank" class="btn-action btn-open">↗ Open</a>
              <button @click="copyUrl(p.shared_url)" class="btn-action btn-copy">📋 Copy</button>
              <button @click="stopProxy(p.remote_port)" class="btn-action btn-stop" :disabled="stoppingPort === p.remote_port">
                {{ stoppingPort === p.remote_port ? 'Stopping...' : '⛔ Stop' }}
              </button>
            </div>
          </div>

          <div class="proxy-body">
            <div class="row">
              <span class="label">Shared URL:</span>
              <a :href="p.shared_url" target="_blank" class="value shared-link">{{ p.shared_url }}</a>
            </div>
            <div class="row">
              <span class="label">Target URL:</span>
              <span class="value target-link">{{ p.original_url }}</span>
            </div>
            <div class="stats-row">
              <span class="stat">Total Requests: <strong>{{ p.total_requests || 0 }}</strong></span>
              <span class="stat">Rate: <strong>{{ (p.request_rate || 0).toFixed(1) }} req/s</strong></span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
import axios from 'axios';
import { useToast } from 'vue-toastification';

interface ProxyItem {
  original_url: string;
  remote_port: number;
  path: string;
  shared_url: string;
  active_systems?: string[];
  total_requests?: number;
  request_rate?: number;
}

const toast = useToast();
const proxies = ref<ProxyItem[]>([]);
const loading = ref(false);
const isSubmitting = ref(false);
const stoppingPort = ref<number | null>(null);
const newUrl = ref('');
const requestedPort = ref<number | null>(null);
const errorMessage = ref('');
let timer: number | undefined;

const fetchProxies = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/debug/proxies');
    proxies.value = res.data || [];
  } catch (err: any) {
    console.error('Failed to fetch proxies:', err);
  } finally {
    loading.value = false;
  }
};

const shareUrl = async () => {
  if (!newUrl.value.trim()) return;
  isSubmitting.value = true;
  errorMessage.value = '';

  try {
    const res = await axios.post('/api/debug/proxies', {
      url: newUrl.value.trim(),
      requested_port: requestedPort.value || 0
    });
    toast.success(`Proxy created on port :${res.data.remote_port}`);
    newUrl.value = '';
    requestedPort.value = null;
    fetchProxies();
  } catch (err: any) {
    const msg = err.response?.data || err.message || 'Failed to create proxy';
    errorMessage.value = typeof msg === 'string' ? msg : JSON.stringify(msg);
    toast.error(errorMessage.value);
  } finally {
    isSubmitting.value = false;
  }
};

const stopProxy = async (port: number) => {
  if (!confirm(`Are you sure you want to stop the proxy on port ${port}?`)) {
    return;
  }
  stoppingPort.value = port;
  try {
    await axios.delete('/api/debug/proxies', { params: { port } });
    toast.info(`Proxy on port :${port} stopped.`);
    fetchProxies();
  } catch (err: any) {
    toast.error(`Failed to stop proxy: ${err.message}`);
  } finally {
    stoppingPort.value = null;
  }
};

const copyUrl = async (url: string) => {
  try {
    await navigator.clipboard.writeText(url);
    toast.success('Shared URL copied to clipboard!');
  } catch {
    toast.error('Failed to copy URL');
  }
};

onMounted(() => {
  fetchProxies();
  timer = window.setInterval(fetchProxies, 4000);
});

onUnmounted(() => {
  if (timer) clearInterval(timer);
});
</script>

<style scoped>
.proxies-view {
  max-width: 1000px;
  margin: 0 auto;
  padding: 1.5rem 1rem;
  overflow-y: auto;
  height: 100%;
  width: 100%;
  box-sizing: border-box;
}

.header-card {
  background: white;
  padding: 1.5rem;
  border-radius: 8px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
  margin-bottom: 1.5rem;
}

.header-card h2 {
  margin: 0 0 0.25rem 0;
  color: #333;
}

.subtitle {
  color: #777;
  font-size: 0.9rem;
  margin-bottom: 1.25rem;
}

.share-form {
  display: flex;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.url-input {
  flex: 1;
  min-width: 280px;
}

.port-input {
  width: 140px;
}

input {
  width: 100%;
  padding: 0.6rem 0.8rem;
  border: 1px solid #ccc;
  border-radius: 6px;
  font-size: 0.95rem;
  outline: none;
  box-sizing: border-box;
}

input:focus {
  border-color: #673ab7;
}

.btn-share {
  background: #673ab7;
  color: white;
  border: none;
  padding: 0.6rem 1.2rem;
  font-size: 0.95rem;
  font-weight: 600;
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.2s;
}

.btn-share:hover:not(:disabled) {
  background: #5e35b1;
}

.btn-share:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.error-msg {
  color: #d32f2f;
  margin: 0.5rem 0 0 0;
  font-size: 0.85rem;
}

.proxies-list-container {
  background: white;
  padding: 1.5rem;
  border-radius: 8px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
}

.list-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.list-header h3 {
  margin: 0;
  color: #444;
}

.btn-refresh {
  background: transparent;
  border: 1px solid #ddd;
  padding: 0.4rem 0.8rem;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.85rem;
}

.btn-refresh:hover {
  background: #f5f5f5;
}

.empty-state, .loading-state {
  text-align: center;
  padding: 2.5rem;
  color: #888;
  font-style: italic;
}

.proxy-cards {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.proxy-card {
  border: 1px solid #e0e0e0;
  border-radius: 6px;
  padding: 1rem;
  background: #fafafa;
  transition: box-shadow 0.2s;
}

.proxy-card:hover {
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.06);
}

.proxy-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
  flex-wrap: wrap;
}

.badge-port {
  background: #673ab7;
  color: white;
  font-weight: bold;
  padding: 0.2rem 0.6rem;
  border-radius: 4px;
  font-size: 0.9rem;
}

.badge-system {
  background: #00897b;
  color: white;
  font-size: 0.75rem;
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  font-weight: 600;
}

.proxy-actions {
  margin-left: auto;
  display: flex;
  gap: 0.4rem;
}

.btn-action {
  padding: 0.35rem 0.7rem;
  border-radius: 4px;
  font-size: 0.85rem;
  font-weight: 500;
  cursor: pointer;
  text-decoration: none;
  border: 1px solid #ccc;
  background: white;
  color: #333;
}

.btn-open {
  background: #e8eaf6;
  color: #3f51b5;
  border-color: #c5cae9;
}

.btn-copy {
  background: #ede7f6;
  color: #5e35b1;
  border-color: #d1c4e9;
}

.btn-stop {
  background: #ffebee;
  color: #c62828;
  border-color: #ffcdd2;
}

.btn-stop:hover:not(:disabled) {
  background: #ffcdd2;
}

.proxy-body .row {
  display: flex;
  margin-bottom: 0.4rem;
  font-size: 0.9rem;
}

.proxy-body .label {
  width: 90px;
  font-weight: 600;
  color: #666;
  flex-shrink: 0;
}

.proxy-body .value {
  color: #333;
  word-break: break-all;
}

.shared-link {
  color: #1976d2;
  text-decoration: none;
  font-weight: 500;
}

.shared-link:hover {
  text-decoration: underline;
}

.stats-row {
  display: flex;
  gap: 1.5rem;
  margin-top: 0.6rem;
  padding-top: 0.6rem;
  border-top: 1px dashed #e0e0e0;
  font-size: 0.85rem;
  color: #777;
}
</style>
