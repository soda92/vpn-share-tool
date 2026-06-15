<template>
  <div class="section tagged-section">
    <div class="section-header">
      <h2 v-t="'tagged_urls_title'"></h2>
      <form @submit.prevent="$emit('save-tag')" class="inline-form">
        <input type="text" v-model="addForm.tag" :placeholder="$t('tag_placeholder')" required class="compact-input">
        <input type="text" v-model="addForm.url" :placeholder="$t('url_placeholder')" required class="compact-input">
        <button type="submit" class="compact-btn">{{ $t('save_tagged_url_button') }}</button>
      </form>
    </div>
    <ul id="tagged-list-ul" class="dense-list">
      <li v-for="url in taggedUrls" :key="url.id">
        <div class="url-row">
          <div class="url-info">
            <div class="tag-name">{{ url.tag }}</div>
            <div class="url-sub">{{ url.url }}</div>
            <!-- Multiple proxies support -->
            <div v-if="url.proxies && url.proxies.length > 0" class="proxies-container">
              <div v-for="p in url.proxies" :key="p.proxy_url" class="proxy-status active">
                <a :href="p.proxy_url" target="_blank">➤ {{ p.proxy_url }}</a>
                <div class="proxy-meta-row">
                  <span class="node-badge" :title="p.node_address">
                    Node: {{ p.node_address }}
                  </span>
                  <span class="stats-badge" :title="'Total Requests: ' + p.total_requests">
                    ⚡ {{ p.request_rate ? p.request_rate.toFixed(1) : 0 }}/s
                  </span>
                  <button @click="$emit('open-settings', p)" class="action-btn settings" title="Settings">⚙️</button>
                </div>
              </div>
            </div>
            <div v-else-if="url.proxy_url" class="proxy-status active">
              <a :href="url.proxy_url" target="_blank">➤ {{ url.proxy_url }}</a>
              <div class="proxy-meta-row">
                <span class="stats-badge" :title="'Total Requests: ' + url.total_requests">
                  ⚡ {{ url.request_rate ? url.request_rate.toFixed(1) : 0 }}/s
                </span>
                <button @click="$emit('open-settings', url)" class="action-btn settings" title="Settings">⚙️</button>
              </div>
            </div>
            <div v-else class="proxy-status inactive">
              Not proxied ({{ url.url.replace('http://', '').replace('https://', '') }})
            </div>
          </div>
          <div class="url-actions compact-actions">
            <div class="btn-group">
              <button 
                :disabled="!servers.length || creatingProxyUrls[url.url]" 
                class="action-btn create btn-group-left"
                title="Create Proxy Fast"
                @click="handleCreateProxyCommand(url.url, 'auto')"
              >
                <span v-if="creatingProxyUrls[url.url]" class="spinner"></span>
                <span v-else>⚡</span>
              </button>
              <el-dropdown trigger="click" @command="(cmd) => handleCreateProxyCommand(url.url, cmd)">
                <button 
                  :disabled="!servers.length || creatingProxyUrls[url.url]" 
                  class="action-btn create btn-group-right"
                  title="Select Node"
                >
                  <span>▼</span>
                </button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="auto">Auto (First Reachable)</el-dropdown-item>
                    <el-dropdown-item command="auto_another">Auto (Create Another)</el-dropdown-item>
                    <el-dropdown-item 
                      v-for="server in servers" 
                      :key="server.address" 
                      :command="server.address"
                      :disabled="isProxyActiveOnNode(url, server.address)"
                    >
                      Node: {{ server.address }}
                      <span v-if="isProxyActiveOnNode(url, server.address)" class="active-indicator"> (active)</span>
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
            <button @click="handleRename(url.id, url.tag)" class="action-btn rename" title="Rename">✎</button>
            <button @click="handleDelete(url.id)" class="action-btn delete" title="Delete">✕</button>
          </div>
        </div>
      </li>
    </ul>
  </div>
</template>

<script setup>
import { ElMessageBox } from 'element-plus';

defineProps({
  taggedUrls: {
    type: Array,
    default: () => []
  },
  addForm: {
    type: Object,
    required: true
  },
  creatingProxyUrls: {
    type: Object,
    default: () => ({})
  },
  servers: {
    type: Array,
    default: () => []
  }
});

const emit = defineEmits(['save-tag', 'create-proxy', 'toggle-debug', 'toggle-captcha', 'rename-tag', 'delete-tag']);

const isProxyActiveOnNode = (url, serverAddress) => {
  if (!url.proxies) return false;
  return url.proxies.some(p => p.node_address === serverAddress);
};

const handleCreateProxyCommand = (targetUrl, command) => {
  const nodeAddress = command === 'auto' ? '' : command;
  emit('create-proxy', targetUrl, nodeAddress);
};

const handleRename = async (id, oldTag) => {
  try {
    const { value } = await ElMessageBox.prompt('Enter new tag name:', 'Rename Tag', {
      confirmButtonText: 'Save',
      cancelButtonText: 'Cancel',
      inputValue: oldTag,
    });
    if (value && value !== oldTag) {
      emit('rename-tag', id, value);
    }
  } catch (action) {
    // cancelled
  }
};

const handleDelete = async (id) => {
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
    emit('delete-tag', id);
  } catch (action) {
    // cancelled
  }
};
</script>

<style scoped>
.section {
  display: flex;
  flex-direction: column;
  border: 1px solid #ebeef5;
  border-radius: 6px;
  padding: 0.8rem;
  background: #fafafa;
  overflow: hidden;
  min-width: 0;
  height: 100%; /* Ensure it fills grid cell */
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
  flex-grow: 1;
}

.dense-list li {
  background-color: white;
  padding: 0.6rem;
  border-radius: 4px;
  margin-bottom: 0.4rem;
  border: 1px solid #e0e0e0;
  transition: background-color 0.2s;
  margin-right: 4px;
  min-width: 0;
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
  min-width: 0;
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
  white-space: normal;
  word-break: break-all;
  overflow: visible;
  text-overflow: clip;
  display: block;
}

.proxy-status {
  font-size: 0.85rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  width: 100%;
  padding: 0.2rem 0;
}

.proxy-status a {
  color: #2ecc71;
  font-weight: 500;
  text-decoration: none;
  white-space: normal;
  word-break: break-all;
  overflow: visible;
  text-overflow: clip;
  flex-grow: 1;
  min-width: 0;
}

.proxy-status a:hover {
  text-decoration: underline;
}

.proxy-meta-row {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex-shrink: 0;
  flex-wrap: wrap;
}

.stats-badge {
  background-color: #f0f9eb;
  color: #67c23a;
  padding: 0 4px;
  border-radius: 4px;
  font-size: 0.75rem;
  border: 1px solid #e1f3d8;
  cursor: help;
}

.proxy-status.inactive {
  color: #bdc3c7;
  font-style: italic;
  font-size: 0.75rem;
  white-space: normal;
  word-break: break-all;
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
  transition: opacity 0.15s ease, transform 0.1s ease, background-color 0.15s ease;
}

.action-btn:hover:not(:disabled) {
  opacity: 0.85;
}

.action-btn:active:not(:disabled) {
  transform: scale(0.95);
}

.create {
  background-color: #67c23a;
  color: white;
}

.create:disabled {
  background-color: #e1f3d8;
  cursor: not-allowed;
}

.btn-group {
  display: inline-flex;
  align-items: stretch;
}

.btn-group :deep(.el-dropdown) {
  display: inline-flex;
}

.btn-group-left {
  border-top-right-radius: 0 !important;
  border-bottom-right-radius: 0 !important;
}

.btn-group-right {
  border-top-left-radius: 0 !important;
  border-bottom-left-radius: 0 !important;
  border-left: 1px solid rgba(255, 255, 255, 0.3) !important;
  padding-left: 0.25rem !important;
  padding-right: 0.25rem !important;
  min-width: 18px !important;
}

.btn-group-right:disabled {
  border-left-color: rgba(0, 0, 0, 0.05) !important;
}

.settings {
  background-color: #909399;
  color: white;
}

.rename {
  background-color: #e6a23c;
  color: white;
}

.delete {
  background-color: #f56c6c;
  color: white;
}

.spinner {
  width: 12px;
  height: 12px;
  border: 2px solid #ffffff;
  border-bottom-color: transparent;
  border-radius: 50%;
  display: inline-block;
  box-sizing: border-box;
  animation: rotation 1s linear infinite;
}

@keyframes rotation {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

@media (max-width: 768px) {
  .section {
    height: auto;
    max-height: none;
    overflow: visible;
    margin-bottom: 1rem;
    padding: 0.5rem;
  }

  .dense-list {
    max-height: none;
    overflow-y: visible;
  }

  .dense-list li {
    padding: 0.5rem;
    margin-right: 0;
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
}

.proxies-container {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  margin-top: 0.3rem;
}

.node-badge {
  background-color: #e9eef3;
  color: #5a738e;
  padding: 0 6px;
  border-radius: 4px;
  font-size: 0.7rem;
  border: 1px solid #d3dbe2;
  font-family: monospace;
}

.active-indicator {
  color: #67c23a;
  font-size: 0.75rem;
  margin-left: 4px;
}

@media (max-width: 600px) {
  .inline-form {
    flex-direction: column;
    align-items: stretch;
    gap: 0.4rem;
  }
  .compact-input {
    width: 100%;
    min-width: 0;
  }
  .compact-btn {
    width: 100%;
    justify-content: center;
  }
  .proxy-status {
    flex-direction: column;
    align-items: stretch;
    gap: 0.3rem;
  }
  .proxy-status a {
    white-space: normal;
    word-break: break-all;
  }
  .proxy-meta-row {
    justify-content: flex-start;
  }
}
</style>