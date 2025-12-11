<template>
  <div class="container">
    <h1 class="app-title">{{ $t('title') }}</h1>

    <div class="main-grid">
      <!-- Left Column: Tagged URLs (Primary Action) -->
      <TaggedList :tagged-urls="taggedUrls" :add-form="newTag" :creating-proxy-url="creatingProxyUrl"
        @save-tag="saveTaggedUrl" @create-proxy="createProxy" @toggle-debug="toggleDebug" @rename-tag="renameTag"
        @delete-tag="deleteTag" />

      <!-- Right Column: All Active Proxies (Quick Access) -->
      <ProxyList :cluster-proxies="clusterProxies" @toggle-debug="toggleDebug" />
    </div>

    <!-- Bottom Section: Active Servers (Info) -->
    <ServerInfo :servers="servers" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { ElNotification } from 'element-plus';
import TaggedList from './components/TaggedList.vue';
import ProxyList from './components/ProxyList.vue';
import ServerInfo from './components/ServerInfo.vue';

const servers = ref([]);
const taggedUrls = ref([]);
const clusterProxies = ref([]);
const newTag = ref({ tag: '', url: '' });
const creatingProxyUrl = ref(null);

const fetchServers = async () => {
  try {
    const response = await axios.get('/instances');
    servers.value = response.data || [];
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
  creatingProxyUrl.value = url;
  try {
    const response = await axios.post('/create-proxy', { url });
    ElNotification({ title: 'Success', message: `Proxy created: ${response.data.shared_url}`, type: 'success' });
    fetchTaggedURLs(); // Refresh to show new proxy status
    fetchClusterProxies();
  } catch (err) {
    ElNotification({ title: 'Error', message: err.response?.data?.error || err.message, type: 'error' });
  } finally {
    creatingProxyUrl.value = null;
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
  height: 100%;
  box-sizing: border-box;
  overflow: hidden;
  /* Desktop default */
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

*,
*::before,
*::after {
  box-sizing: border-box;
}

@media (max-width: 768px) {

  html,
  body {
    height: auto !important;
    overflow: visible !important;
    /* Ensure document scroll */
    display: block;
  }
}

@media (max-height: 600px) {

  html,
  body {
    height: auto !important;
    overflow: auto !important;
    display: block;
  }
}
</style>



<style scoped>
.container {
  max-width: 1400px;
  width: calc(100% - 1rem);
  height: calc(100% - 1rem);
  background-color: #ffffff;
  padding: 1rem;
  border-radius: 8px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.05);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  /* Prevent container scroll */
}

.app-title {
  color: #2c3e50;
  text-align: center;
  margin: 0 0 1rem 0;
  font-size: 1.5rem;
  font-weight: 700;
  flex-shrink: 0;
}

.main-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1rem;
  flex-grow: 1;
  overflow: hidden;
  /* Contains the scrollable sections */

  min-height: 0;
  /* Crucial for nested scrolling */

}

@media (max-width: 768px) {
  .container {
    height: auto;
    /* Grow with content */
    min-height: 100vh;
    padding: 1rem;
    overflow: visible;
    border-radius: 0;
    /* Full width look */
    box-shadow: none;
    width: 100%;
  }

  .main-grid {
    grid-template-columns: 1fr;
    /* Single column */
    overflow: visible;
    /* Allow page scroll */
    display: flex;
    flex-direction: column;
  }
}

@media (max-height: 600px) {
  .container {
    height: auto;
    overflow: visible;
  }

  .main-grid {
    overflow: visible;
  }
}
</style>