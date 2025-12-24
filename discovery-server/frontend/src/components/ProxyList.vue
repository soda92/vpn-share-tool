<template>
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
                  @change="$emit('toggle-debug', proxy.original_url, $event.target.checked)">
                🐞
              </label>
              <label class="captcha-toggle" title="Toggle Captcha">
                <input type="checkbox" :checked="proxy.enable_captcha"
                  @change="$emit('toggle-captcha', proxy.original_url, $event.target.checked)">
                🤖
              </label>
            </div>
          </div>
        </div>
      </li>
      <li v-if="!clusterProxies.length" class="empty-msg">{{ $t('no_active_proxies') }}</li>
    </ul>
  </div>
</template>

<script setup>
defineProps({
  clusterProxies: {
    type: Array,
    default: () => []
  }
});

defineEmits(['toggle-debug']);
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

.debug-toggle {
  cursor: pointer;
  user-select: none;
  display: inline-flex;
  align-items: center;
}

.debug-toggle input {
  margin-right: 2px;
}

.empty-msg {
  text-align: center;
  color: #909399;
  padding: 1rem;
  font-style: italic;
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
}
</style>