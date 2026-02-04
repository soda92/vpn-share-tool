<template>
  <div class="container">
    <h1 class="app-title">{{ $t('title') }}</h1>

    <div class="main-grid" :class="{ 'single-column': !showAllProxies }">
      <!-- Left Column: Tagged URLs (Primary Action) -->
      <div class="left-column">
        <div class="tagged-list-wrapper">
          <TaggedList
            :tagged-urls="taggedUrls"
            :add-form="newTag"
            :creating-proxy-urls="creatingProxyUrls"
            @save-tag="saveTaggedUrl"
            @create-proxy="createProxy"
            @open-settings="openSettings"
            @rename-tag="renameTag"
            @delete-tag="deleteTag"
          />
        </div>

        <div class="toggle-section">
           <el-button @click="showAllProxies = !showAllProxies">
             {{ showAllProxies ? 'Hide All Active Proxies' : 'Show All Active Proxies' }}
           </el-button>
        </div>
      </div>

      <!-- Right Column: All Active Proxies (Quick Access) -->
      <div v-if="showAllProxies" class="right-column">
        <ProxyList
          :cluster-proxies="clusterProxies"
          @open-settings="openSettings"
        />
      </div>
    </div>

    <!-- Bottom Section: Active Servers (Info) -->
    <ServerInfo 
        :servers="servers" 
        :latest-version="latestVersion" 
        @update-server="handleUpdateServer"
        @open-logs="openLogs"
    />

    <SettingsDialog
      v-model="settingsVisible"
      :proxy-data="currentSettingsProxy"
      @save="handleSaveSettings"
    />

    <LogViewer
      v-model="logsVisible"
      :server-address="currentLogServer"
    />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { ElNotification } from 'element-plus';
import TaggedList from './components/TaggedList.vue';
import ProxyList from './components/ProxyList.vue';
import ServerInfo from './components/ServerInfo.vue';
import SettingsDialog from './components/SettingsDialog.vue';
import LogViewer from './components/LogViewer.vue';

const servers = ref([]);
const taggedUrls = ref([]);
const clusterProxies = ref([]);
const showAllProxies = ref(false);
const newTag = ref({ tag: '', url: '' });
const creatingProxyUrls = ref({});
const latestVersion = ref('');

// Settings Dialog State
const settingsVisible = ref(false);
const currentSettingsProxy = ref(null);

// Log Viewer State
const logsVisible = ref(false);
const currentLogServer = ref('');

const openSettings = (proxy) => {
  currentSettingsProxy.value = proxy;
  settingsVisible.value = true;
};

const openLogs = (address) => {
    currentLogServer.value = address;
    logsVisible.value = true;
};

const handleSaveSettings = async (data) => {
  let success = false;
  // 1. Try New Settings
  try {
    await axios.post('/update-proxy-settings', { 
        url: data.url, 
        settings: data.settings 
    });
    success = true;
  } catch (err) {
    console.warn("New settings API failed:", err);
  }
  
  if (success) {
      ElNotification({ title: 'Success', message: 'Settings updated.', type: 'success' });
      fetchTaggedURLs();
      fetchClusterProxies();
  } else {
      ElNotification({ title: 'Error', message: 'Failed to update settings. Check client connection.', type: 'error' });
  }
};

const fetchServers = async () => {
  try {
    const response = await axios.get('/instances');
    servers.value = (response.data || []).sort((a, b) => a.address.localeCompare(b.address));
  } catch (err) { console.error('Error fetching servers:', err); }
};
const fetchClusterProxies = async () => {
  try {
    const response = await axios.get('/cluster-proxies');
    clusterProxies.value = (response.data || []).sort((a, b) => a.original_url.localeCompare(b.original_url));
  } catch (err) { console.error('Error fetching cluster proxies:', err); }
};
const fetchTaggedURLs = async () => {
  try {
    const response = await axios.get('/tagged-urls');
    taggedUrls.value = (response.data || []).sort((a, b) => new Date(b.created_at) - new Date(a.created_at));
  } catch (err) { console.error('Error fetching tagged URLs:', err); }
};
const fetchLatestVersion = async () => {
  try {
    const response = await axios.get('/latest-version');
    if (response.data && response.data.version) {
      latestVersion.value = response.data.version;
    }
  } catch (err) { console.error('Error fetching latest version:', err); }
};
const handleUpdateServer = async (address) => {
  try {
    await axios.post('/trigger-update-remote', { address });
    ElNotification({ title: 'Success', message: `Update triggered for ${address}`, type: 'success' });
  } catch (err) {
    ElNotification({ title: 'Error', message: err.response?.data?.error || err.message, type: 'error' });
  }
};
const saveTaggedUrl = async () => {
  try {
    await axios.post('/tagged-urls', newTag.value);
    newTag.value = { tag: '', url: '' };
    fetchTaggedURLs();
    ElNotification({ title: 'Success', message: 'Tagged URL saved.', type: 'success' });
  } catch (err) {
    ElNotification({ title: 'Error', message: 'Error saving URL.', type: 'error' });
  }
};
const createProxy = async (url) => {
  creatingProxyUrls.value[url] = true;
  try {
    const response = await axios.post('/create-proxy', { url });
    ElNotification({ title: 'Success', message: `Proxy created: ${response.data.shared_url}`, type: 'success' });
    fetchTaggedURLs(); // Refresh to show new proxy status
    fetchClusterProxies();
  } catch (err) {
    ElNotification({ title: 'Error', message: err.response?.data?.error || err.message, type: 'error' });
  } finally {
    delete creatingProxyUrls.value[url];
  }
};
const toggleDebug = async (url, enable) => {
  try {
    await axios.post('/toggle-debug-proxy', { url, enable });
    ElNotification({ title: 'Success', message: `Debugger ${enable ? 'enabled' : 'disabled'}`, type: 'success' });
    // Refresh to show new status
    fetchTaggedURLs();
    fetchClusterProxies();
  } catch (err) {
    ElNotification({ title: 'Error', message: 'Failed to toggle debugger', type: 'error' });
  }
};

const toggleCaptcha = async (url, enable) => {
  try {
    await axios.post('/toggle-captcha-proxy', { url, enable });
    ElNotification({ title: 'Success', message: `Auto captcha ${enable ? 'enabled' : 'disabled'}`, type: 'success' });
    // Refresh to show new status
    fetchTaggedURLs();
    fetchClusterProxies();
  } catch (err) {
    ElNotification({ title: 'Error', message: 'Failed to toggle debugger', type: 'error' });
  }
};
const renameTag = async (id, newTagValue) => {
  try {
    await axios.put(`/tagged-urls/${id}`, { tag: newTagValue });
    fetchTaggedURLs();
    ElNotification({ title: 'Success', message: 'Tag renamed.', type: 'success' });
  } catch (err) {
    ElNotification({ title: 'Error', message: err.response?.data?.error || 'Error renaming tag.', type: 'error' });
  }
};
const deleteTag = async (id) => {
  try {
    await axios.delete(`/tagged-urls/${id}`);
    fetchTaggedURLs();
    ElNotification({ title: 'Success', message: 'Tag deleted.', type: 'success' });
  } catch (err) {
    ElNotification({ title: 'Error', message: err.response?.data?.error || 'Error deleting tag.', type: 'error' });
  }
};
onMounted(() => {
  fetchLatestVersion(); // Fetch once on mount
  const pollServers = () => {
    fetchServers().finally(() => setTimeout(pollServers, 5000));
  };
  const pollTaggedURLs = () => {
    fetchTaggedURLs().finally(() => setTimeout(pollTaggedURLs, 5000));
  };
  const pollClusterProxies = () => {
    fetchClusterProxies().finally(() => setTimeout(pollClusterProxies, 5000));
  };
  pollServers();
  pollTaggedURLs();
  pollClusterProxies();
});
</script>

<style>
html,
body {
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
  background-color: #f4f7f6;
  margin: 0;
  padding: 0;
  min-height: 100vh;
  box-sizing: border-box;
}

*,
*::before,
*::after {
  box-sizing: border-box;
}
</style>

<style scoped>
.container {
  max-width: 1400px;
  width: 100%;
  min-height: 100vh;
  background-color: #ffffff;
  padding: 1rem;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.05);
  display: flex;
  flex-direction: column;
  margin: 0 auto; /* Center naturally */
}

@media (min-width: 769px) {
  .container {
    width: calc(100% - 2rem);
    min-height: auto; /* Allow content to dictate height, but keep min for look */
    margin: 1rem auto;
    border-radius: 8px;
  }
}

.app-title {
  color: #2c3e50;
  text-align: center;
  margin: 0 0 1rem 0;
  font-size: 1.5rem;
  font-weight: 700;
}

.main-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1rem;
  /* Removed overflow and flex-grow */
}

.main-grid.single-column {
  grid-template-columns: 1fr;
}

.left-column {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.tagged-list-wrapper {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.toggle-section {
  margin-top: 1rem;
  text-align: center;
}

@media (max-width: 768px) {
  .container {
    width: 100%;
    margin: 0;
    border-radius: 0;
    box-shadow: none;
  }

  .main-grid {
    grid-template-columns: 1fr;
    display: flex;
    flex-direction: column;
  }
}
</style>