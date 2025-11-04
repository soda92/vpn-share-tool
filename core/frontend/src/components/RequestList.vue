<template>
  <div class="request-list-pane">
    <div class="request-list-header">
      <h2>Requests</h2>
      <div class="filter-controls">
        <input type="text" :value="searchQuery" @input="$emit('update:searchQuery', $event.target.value)" placeholder="Search URL..." />
        <select :value="methodFilter" @change="$emit('update:methodFilter', $event.target.value)">
          <option value="ALL">All</option>
          <option value="GET">GET</option>
          <option value="POST">POST</option>
        </select>
        <button @click="$emit('clear')">Clear</button>
      </div>
    </div>
    <div v-if="groupedRequests.length === 0" class="no-requests">
      No requests match the filter.
    </div>
    <ul v-else class="request-list">
      <li
        v-for="item in groupedRequests"
        :key="item.id"
        :class="item.type === 'request' ? { selected: selectedRequest && selectedRequest.id === item.request.id } : 'group-header'"
        @click="item.type === 'request' && $emit('select-request', item.request)"
        @contextmenu.prevent="item.type === 'request' && $emit('show-context-menu', $event, item.request)"
      >
        <template v-if="item.type === 'request'">
          <span class="bookmark-star" @click.stop="$emit('toggle-bookmark', item.request)">
            {{ item.request.bookmarked ? '★' : '☆' }}
          </span>
          <span class="timestamp">{{ new Date(item.request.timestamp).toLocaleTimeString() }}</span>
          <span class="method">{{ item.request.method }}</span>
          <span class="url">{{ item.request.url.substring(item.groupName.length) }}</span>
        </template>
        <template v-else>
          <span>{{ item.groupName }}</span>
        </template>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { computed, defineProps, defineEmits } from 'vue';
import type { CapturedRequest } from '../types';

const props = defineProps<{
  requests: CapturedRequest[];
  selectedRequest: CapturedRequest | null;
  searchQuery: string;
  methodFilter: string;
}>();

defineEmits<{
  (e: 'select-request', request: CapturedRequest): void;
  (e: 'show-context-menu', event: MouseEvent, request: CapturedRequest): void;
  (e: 'toggle-bookmark', request: CapturedRequest): void;
  (e: 'update:searchQuery', value: string): void;
  (e: 'update:methodFilter', value: string): void;
  (e: 'clear'): void;
}>();

const getUrlPrefix = (url: string) => {
  try {
    const urlObj = new URL(url);
    const pathParts = urlObj.pathname.split('/').filter(p => p);
    if (pathParts.length > 1) {
      return `${urlObj.origin}/${pathParts.slice(0, -1).join('/')}/`;
    }
    return urlObj.origin + '/';
  } catch (e) {
    const parts = url.split('/');
    if (parts.length > 3) {
      return parts.slice(0, parts.length - 1).join('/') + '/';
    }
    return url;
  }
};

const groupedRequests = computed(() => {
  const result: any[] = [];
  let lastPrefix = '';

  const filtered = props.requests.filter(req => {
    const methodMatch = props.methodFilter === 'ALL' || req.method === props.methodFilter;
    const searchMatch = req.url.toLowerCase().includes(props.searchQuery.toLowerCase());
    return methodMatch && searchMatch;
  });

  for (const request of filtered) {
    const currentPrefix = getUrlPrefix(request.url);
    if (currentPrefix !== lastPrefix) {
      result.push({ id: `group-${currentPrefix}-${request.id}`, type: 'group-header', groupName: currentPrefix });
      lastPrefix = currentPrefix;
    }
    result.push({ id: request.id, type: 'request', request: request, groupName: currentPrefix });
  }

  return result;
});
</script>
