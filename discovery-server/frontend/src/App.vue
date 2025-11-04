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
              <small v-else class="no-proxy">Not currently proxied</small>
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

const servers = ref([]);
const taggedUrls = ref([]);
const newTag = ref({ tag: '', url: '' });

const fetchServers = async () => {
  try {
    const response = await axios.get('/instances');
    servers.value = response.data || [];
  } catch (err) { console.error('Error fetching servers:', err); }
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
  } catch (err) { alert('Error saving URL.'); }
};

const createProxy = async (url) => {
  try {
    const response = await axios.post('/create-proxy', { url });
    alert(`Proxy created: ${response.data.shared_url}`);
    fetchTaggedURLs(); // Refresh to show new proxy status
  } catch (err) { alert(`Error: ${err.response?.data?.error || err.message}`); }
};

const renameTag = async (id, oldTag) => {
  const newTag = prompt('Enter new tag name:', oldTag);
  if (newTag && newTag !== oldTag) {
    try {
      await axios.put(`/tagged-urls/${id}`, { tag: newTag });
      fetchTaggedURLs();
    } catch (err) { alert('Error renaming tag.'); }
  }
};

const deleteTag = async (id) => {
  if (confirm('Are you sure you want to delete this tagged URL?')) {
    try {
      await axios.delete(`/tagged-urls/${id}`);
      fetchTaggedURLs();
    } catch (err) { alert('Error deleting tag.'); }
  }
};

onMounted(() => {
  fetchServers();
  fetchTaggedURLs();
  setInterval(fetchServers, 5000);
  setInterval(fetchTaggedURLs, 5000); // Refresh tagged URLs and their proxy status
});
</script>

<style scoped>
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background-color: #f0f2f5; margin: 0; padding: 1rem; }
.container { max-width: 1200px; margin: auto; background-color: white; padding: 2rem; border-radius: 8px; box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1); }
h1, h2 { color: #333; }
.grid { display: grid; grid-template-columns: 1fr 1fr; gap: 2rem; }
input[type="text"], input[type="submit"], button { padding: 0.75rem; border: 1px solid #ccc; border-radius: 4px; font-size: 1rem; width: 100%; box-sizing: border-box; margin-bottom: 0.5rem; }
input[type="submit"], button { background-color: #007bff; color: white; cursor: pointer; border: none; }
button.delete { background-color: #dc3545; }
button.rename { background-color: #ffc107; }
ul { list-style: none; padding: 0; }
li { background-color: #f8f9fa; padding: 1rem; border-radius: 4px; margin-bottom: 0.5rem; }
.url-info { word-break: break-all; }
.url-actions { display: flex; gap: 0.5rem; margin-top: 1rem; justify-content: flex-end; }
.proxy-link a { color: #28a745; }
.no-proxy { color: #6c757d; font-style: italic; }
</style>