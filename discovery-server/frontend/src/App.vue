<template>
  <div class="container">
    <h1>{{ $t('title') }}</h1>
    <div class="grid">
      <div>
        <h2 v-t="'active_servers_title'"></h2>
        <ul id="server-list-ul">
          <li v-for="server in servers" :key="server.address">
            {{ server.address }} ({{ $t('last_seen') }}: {{ new Date(server.last_seen).toLocaleString() }})
          </li>
          <li v-if="!servers.length">{{ $t('no_active_servers') }}</li>
        </ul>

        <h2>All Active Proxies</h2>
        <ul id="proxy-list-ul">
          <li v-for="proxy in clusterProxies" :key="proxy.shared_url">
            <div class="url-info">
              <strong>{{ proxy.original_url }}</strong><br>
              <small class="proxy-link"><a :href="proxy.shared_url" target="_blank">{{ proxy.shared_url }}</a></small>
            </div>
          </li>
          <li v-if="!clusterProxies.length">No active proxies found.</li>
        </ul>
      </div>
      <div>
        <h2 v-t="'tagged_urls_title'"></h2>
        <form @submit.prevent="saveTaggedUrl">
          <input type="text" v-model="newTag.tag" :placeholder="$t('tag_placeholder')" required>
          <input type="text" v-model="newTag.url" :placeholder="$t('url_placeholder')" required>
          <input type="submit" :value="$t('save_tagged_url_button')">
        </form>
        <ul id="tagged-list-ul">
          <li v-for="url in taggedUrls" :key="url.id">
            <div class="url-info">
              <strong>{{ url.tag }}</strong><br>
              <small>{{ url.url }}</small><br>
              <small v-if="url.proxy_url" class="proxy-link">Proxied at: <a :href="url.proxy_url" target="_blank">{{ url.proxy_url }}</a></small>
              <small v-else class="no-proxy">Not currently proxied (Matched against: {{ url.url.replace('http://', '').replace('https://', '') }})</small>
            </div>
            <div class="url-actions">
              <button @click="createProxy(url.url)" :disabled="!!url.proxy_url">{{ $t('create_proxy_button') }}</button>
              <button class="rename" @click="renameTag(url.id, url.tag)">{{ $t('rename_button') }}</button>
              <button class="delete" @click="deleteTag(url.id)">{{ $t('delete_button') }}</button>
            </div>
          </li>
        </ul>
      </div>
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
body { font-family: 'Inter', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background-color: #f4f7f6; margin: 0; padding: 1rem; }
</style>
<style scoped>
.container { margin: auto; background-color: #ffffff; padding: 2.5rem; border-radius: 12px; box-shadow: 0 6px 20px rgba(0, 0, 0, 0.08); }
h1 { color: #2c3e50; text-align: center; margin-bottom: 2rem; font-size: 2.5rem; font-weight: 700; }
h2 { color: #34495e; border-bottom: 2px solid #eceff1; padding-bottom: 0.75rem; margin-top: 2.5rem; margin-bottom: 1.5rem; font-size: 1.75rem; font-weight: 600; }
.grid { display: grid; grid-template-columns: 1fr 1fr; gap: 3rem; }
input[type="text"], input[type="submit"], button { padding: 0.85rem; border: 1px solid #dcdfe6; border-radius: 6px; font-size: 1rem; width: 100%; box-sizing: border-box; margin-bottom: 0.75rem; transition: all 0.3s ease; }
input[type="text"]:focus { border-color: #409eff; box-shadow: 0 0 0 2px rgba(64, 158, 255, 0.2); outline: none; }
input[type="submit"], button { background-color: #409eff; color: white; cursor: pointer; border: none; font-weight: 500; }
input[type="submit"]:hover, button:hover { background-color: #66b1ff; box-shadow: 0 4px 10px rgba(64, 158, 255, 0.2); }
button.delete { background-color: #f56c6c; }
button.delete:hover { background-color: #f78989; }
button.rename { background-color: #e6a23c; }
button.rename:hover { background-color: #ebb563; }
ul { list-style: none; padding: 0; }
li { background-color: #fdfdfd; padding: 1.2rem; border-radius: 8px; margin-bottom: 0.75rem; border: 1px solid #ebeef5; box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05); transition: all 0.3s ease; }
li:hover { transform: translateY(-3px); box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1); }
.url-info { word-break: break-all; line-height: 1.6; }
.url-info strong { color: #2c3e50; font-size: 1.1rem; }
.url-info small { color: #7f8c8d; }
.url-actions { display: flex; gap: 0.75rem; margin-top: 1.2rem; justify-content: flex-end; }
.proxy-link a { color: #2ecc71; font-weight: 500; text-decoration: none; }
.proxy-link a:hover { text-decoration: underline; }
.no-proxy { color: #95a5a6; font-style: italic; }
#server-list-ul li { display: flex; justify-content: space-between; align-items: center; }
#server-list-ul li span { color: #7f8c8d; font-size: 0.9rem; }
</style>