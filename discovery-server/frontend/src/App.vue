<template>
  <div class="container">
    <h1 class="app-title">{{ $t('title') }}</h1>

    <div class="main-grid">
      <!-- Left Column: Tagged URLs (Primary Action) -->
      <div class="section tagged-section">
        <div class="section-header">
          <h2 v-t="'tagged_urls_title'"></h2>
          <form @submit.prevent="saveTaggedUrl" class="inline-form">
            <input type="text" v-model="newTag.tag" :placeholder="$t('tag_placeholder')" required class="compact-input">
            <input type="text" v-model="newTag.url" :placeholder="$t('url_placeholder')" required class="compact-input">
            <button type="submit" class="compact-btn">{{ $t('save_tagged_url_button') }}</button>
          </form>
        </div>
        <ul id="tagged-list-ul" class="dense-list">
          <li v-for="url in taggedUrls" :key="url.id">
            <div class="url-row">
              <div class="url-info">
                <div class="tag-name">{{ url.tag }}</div>
                <div class="url-sub">{{ url.url }}</div>
                <div v-if="url.proxy_url" class="proxy-status active">
                  <a :href="url.proxy_url" target="_blank">➤ {{ url.proxy_url }}</a>
                  <label class="debug-toggle" title="Toggle Debugger">
                    <input type="checkbox" :checked="url.enable_debug"
                      @change="toggleDebug(url.url, $event.target.checked)">
                    🐞
                  </label>
                </div>
                <div v-else class="proxy-status inactive">
                  Not proxied ({{ url.url.replace('http://', '').replace('https://', '') }})
                </div>
              </div>
              <div class="url-actions compact-actions">
                <button @click="createProxy(url.url)" :disabled="!!url.proxy_url" class="action-btn create"
                  title="Create Proxy">⚡</button>
                <button @click="renameTag(url.id, url.tag)" class="action-btn rename" title="Rename">✎</button>
                <button @click="deleteTag(url.id)" class="action-btn delete" title="Delete">✕</button>
              </div>
            </div>
          </li>
        </ul>
      </div>

      <!-- Right Column: All Active Proxies (Quick Access) -->
      <div class="section proxies-section">
        <h2 v-t="'active_proxies_title'"></h2>
        <ul id="proxy-list-ul" class="dense-list">
          <li v-for="proxy in clusterProxies" :key="proxy.shared_url">
            <div class="url-row">
              <div class="url-info">
                <div class="tag-name">{{ proxy.original_url }}</div>
                <div class="proxy-status active">
                  <a :href="proxy.shared_url" target="_blank">➤ {{ proxy.shared_url }}</a>
                  <label class="debug-toggle" title="Toggle Debugger">
                    <input type="checkbox" :checked="proxy.enable_debug"
                      @change="toggleDebug(proxy.original_url, $event.target.checked)">
                    🐞
                  </label>
                </div>
              </div>
            </div>
          </li>
          <li v-if="!clusterProxies.length" class="empty-msg">{{ $t('no_active_proxies') }}</li>
        </ul>
      </div>
    </div>

    <!-- Bottom Section: Active Servers (Info) -->
    <div class="server-info-bar">
      <span class="server-label" v-t="'active_servers_title'"></span>:
      <span v-for="(server, index) in servers" :key="server.address" class="server-item">
        {{ server.address }}<span v-if="index < servers.length - 1">, </span>
      </span>
      <span v-if="!servers.length">{{ $t('no_active_servers') }}</span>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { ElNotification, ElMessageBox } from 'element-plus';
const servers = ref([]);
const taggedUrls = ref([]);
const clusterProxies = ref([]);
const newTag = ref({ tag: '', url: '' });
const fetchServers = async () => {
  try {
    const response = await axios.get('/instances');
    servers.value = response.data || [];
  } catch (err) { console.error('Error fetching servers:', err); }
};
const fetchClusterProxies = async () => {
  try {
    const response = await axios.get('/cluster-proxies');
    clusterProxies.value = response.data || [];
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
  try {
    const response = await axios.post('/create-proxy', { url });
    ElNotification({ title: 'Success', message: `Proxy created: ${response.data.shared_url}`, type: 'success' });
    fetchTaggedURLs(); // Refresh to show new proxy status
    fetchClusterProxies();
  } catch (err) {
    ElNotification({ title: 'Error', message: err.response?.data?.error || err.message, type: 'error' });
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
const renameTag = async (id, oldTag) => {
  try {
    const { value } = await ElMessageBox.prompt('Enter new tag name:', 'Rename Tag', {
      confirmButtonText: 'Save',
      cancelButtonText: 'Cancel',
      inputValue: oldTag,
    });
    if (value && value !== oldTag) {
      await axios.put(`/tagged-urls/${id}`, { tag: value });
      fetchTaggedURLs();
      ElNotification({ title: 'Success', message: 'Tag renamed.', type: 'success' });
    }
  } catch (action) {
    if (action === 'cancel') {
      ElNotification({ message: 'Rename cancelled.', type: 'info' });
    }
  }
};
const deleteTag = async (id) => {
  try {
    await ElMessageBox.confirm(
      'Are you sure you want to delete this tagged URL?',
      'Warning',
      {
        confirmButtonText: 'Delete',
        cancelButtonText: 'Cancel',
        type: 'warning',
      }
    );
    await axios.delete(`/tagged-urls/${id}`);
    fetchTaggedURLs();
    ElNotification({ title: 'Success', message: 'Tag deleted.', type: 'success' });
  } catch (action) {
    if (action === 'cancel') {
      ElNotification({ message: 'Delete cancelled.', type: 'info' });
    }
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

html, body { font-family: 'Inter', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background-color: #f4f7f6; margin: 0; padding: 0; height: 100%; box-sizing: border-box; }

body { padding: 0.5rem; overflow: hidden; } /* Desktop default */

*, *::before, *::after { box-sizing: border-box; }



@media (max-width: 768px) {

  html, body {

    height: auto !important;

    overflow: visible !important; /* Ensure document scroll */

    padding: 0; /* Reset padding on body, handle in container */

  }

}

</style>



<style scoped>
.container {
  max-width: 1400px;
  /* Increased max-width */
  width: 100%;
  margin: auto;
  background-color: #ffffff;
  padding: 1rem;
  border-radius: 8px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.05);
  display: flex;
  flex-direction: column;
  height: 100%;
  /* Fill available body height (minus body padding) */
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

.section {
  display: flex;
  flex-direction: column;
  border: 1px solid #ebeef5;
  border-radius: 6px;
  padding: 0.8rem;
  background: #fafafa;
  overflow: hidden;
  /* Prevent section itself from growing */

  min-width: 0;
  /* Allow flex/grid shrink */

}

.section-header {
  flex-shrink: 0;
  margin-bottom: 0.5rem;
}

h2 {
  color: #34495e;
  border-bottom: 1px solid #dcdfe6;
  padding-bottom: 0.4rem;
  margin: 0 0 0.5rem 0;
  font-size: 1.1rem;
  font-weight: 600;
  flex-shrink: 0;
}

.inline-form {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.compact-input {
  flex: 1;
  padding: 0.4rem;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  font-size: 0.9rem;
  min-width: 150px;
}

.compact-btn {
  padding: 0.4rem 0.8rem;
  background-color: #409eff;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.9rem;
  white-space: nowrap;
}

.compact-btn:hover {
  background-color: #66b1ff;
}

.dense-list {
  list-style: none;
  padding: 0;
  margin: 0;
  overflow-y: auto;
  /* The scrollable part */

  flex-grow: 1;
  height: 100%;
  /* Ensure it takes up remaining space */

}

.dense-list li {
  background-color: white;
  padding: 0.6rem;
  border-radius: 4px;
  margin-bottom: 0.4rem;
  border: 1px solid #e0e0e0;
  transition: background-color 0.2s;
  margin-right: 4px;
  /* Space for scrollbar */
}

.dense-list li:hover {
  background-color: #f0f9eb;
}

.url-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 0.5rem;
  overflow: hidden;
}

.url-info {
  flex-grow: 1;
  min-width: 0;
  overflow: hidden;
}

.tag-name {
  font-weight: 600;
  color: #2c3e50;
  font-size: 0.95rem;
  margin-bottom: 0.1rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.url-sub {
  color: #909399;
  font-size: 0.75rem;
  margin-bottom: 0.2rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  display: block;
}

.proxy-status {
  font-size: 0.85rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.proxy-status.active a {
  color: #2ecc71;
  font-weight: 500;
  text-decoration: none;
  overflow: hidden;
  text-overflow: ellipsis;
}

.proxy-status.active a:hover {
  text-decoration: underline;
}

.proxy-status.inactive {
  color: #bdc3c7;
  font-style: italic;
  font-size: 0.75rem;
}

.debug-toggle {
  cursor: pointer;
  user-select: none;
  display: inline-flex;
  align-items: center;
}

.debug-toggle input {
  margin-right: 2px;
}

.compact-actions {
  display: flex;
  gap: 0.3rem;
  flex-shrink: 0;
}

.action-btn {
  padding: 0.2rem 0.5rem;
  border: none;
  border-radius: 3px;
  cursor: pointer;
  font-size: 0.8rem;
  min-width: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.create {
  background-color: #67c23a;
  color: white;
}

.create:disabled {
  background-color: #e1f3d8;
  cursor: not-allowed;
}

.rename {
  background-color: #e6a23c;
  color: white;
}

.delete {
  background-color: #f56c6c;
  color: white;
}

.empty-msg {
  text-align: center;
  color: #909399;
  padding: 1rem;
  font-style: italic;
}

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

@media (max-width: 768px) {

  .container {

    height: auto; /* Grow with content */

    min-height: 100vh;

    padding: 1rem; /* Move padding here */

    overflow: visible;

    border-radius: 0; /* Full width look */

    box-shadow: none;

  }

  

  .main-grid {

    grid-template-columns: 1fr; /* Single column */

    overflow: visible; /* Allow page scroll */

    display: flex;

    flex-direction: column;

  }



  .section {

    height: auto; /* Auto height for sections */

    max-height: none; /* Remove max-height to allow full content display */

    overflow: visible;

    margin-bottom: 1rem;

  }



  .dense-list {

    max-height: 400px; /* Keep internal scroll for very long lists to save vertical space */

    overflow-y: auto; 

  }

  

  .url-row {

    flex-direction: column;

    align-items: stretch;

  }

  

  .url-actions {

    margin-top: 0.5rem;

    justify-content: flex-end;

    align-self: flex-end;

  }



  .server-info-bar {

    white-space: normal; /* Allow wrapping */

    overflow: visible;

    text-align: left;

  }

}

</style>