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
            <div v-if="url.proxy_url" class="proxy-status active">
              <a :href="url.proxy_url" target="_blank">➤ {{ url.proxy_url }}</a>
              <label class="debug-toggle" title="Toggle Debugger">
                <input type="checkbox" :checked="url.enable_debug"
                  @change="$emit('toggle-debug', url.url, $event.target.checked)">
                🐞
              </label>
              <label class="captcha-toggle" title="Toggle Captcha">
                <input type="checkbox" :checked="url.enable_captcha"
                  @change="$emit('toggle-captcha', url.url, $event.target.checked)">
                🤖
              </label>
            </div>
            <div v-else class="proxy-status inactive">
              Not proxied ({{ url.url.replace('http://', '').replace('https://', '') }})
            </div>
          </div>
          <div class="url-actions compact-actions">
            <button 
              @click="$emit('create-proxy', url.url)" 
              :disabled="!!url.proxy_url || creatingProxyUrls[url.url]" 
              class="action-btn create"
              title="Create Proxy"
            >
              <span v-if="creatingProxyUrls[url.url]" class="spinner"></span>
              <span v-else>⚡</span>
            </button>
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
  }
});

const emit = defineEmits(['save-tag', 'create-proxy', 'toggle-debug', 'toggle-captcha', 'rename-tag', 'delete-tag']);

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
  }

  .dense-list {
    max-height: 400px;
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
}
</style>