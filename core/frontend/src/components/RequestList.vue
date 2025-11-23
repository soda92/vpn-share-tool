<template>
  <div class="request-list-pane">
    <div class="request-list-header">
      <h2>Requests</h2>
      <div class="filter-controls">
        <input type="text" :value="searchQuery" @input="$emit('update:searchQuery', ($event.target as HTMLInputElement).value)" placeholder="Search URL..." />
        <select :value="methodFilter" @change="$emit('update:methodFilter', ($event.target as HTMLSelectElement).value)">
          <option value="ALL">All</option>
          <option value="GET">GET</option>
          <option value="POST">POST</option>
        </select>
        <button @click="$emit('clear')">Clear</button>
      </div>
    </div>
    <div v-if="filteredRequests.length === 0" class="no-requests">
      No requests found.
    </div>
    <ul v-else class="request-list">
      <template v-for="(group, groupName) in groupedRequests" :key="groupName">
        <li class="group-header">{{ groupName }}</li>
        <li
          v-for="request in group"
          :key="request.id"
          :class="{ selected: selectedRequest?.id === request.id }"
          @click="$emit('select-request', request)"
          @contextmenu.prevent="$emit('show-context-menu', $event, request)"
        >
          <span class="bookmark-star" @click.stop="$emit('toggle-bookmark', request)">
            {{ request.bookmarked ? '★' : '☆' }}
          </span>
          <span class="timestamp">{{ new Date(request.timestamp).toLocaleTimeString() }}</span>
          <span class="method">{{ request.method }}</span>
          <span class="url">{{ getUrlPath(request.url) }}</span>
        </li>
      </template>
    </ul>
  </div>
</template>

<style scoped>
.request-list-pane {
  width: 35%;
  min-width: 300px;
  /* border-right: 1px solid #ddd; Removed */
  display: flex;
  flex-direction: column;
  background-color: #fff;
  height: 100%; /* Ensure full height in flex container */
}

@media (max-width: 768px) {
  .request-list-pane {
    width: 100%;
    height: 100%; /* Take full height in toggle mode */
    min-height: 0;
    border-right: none;
    border-bottom: none;
  }
}

.request-list-header {
  padding: 1rem;
  border-bottom: 1px solid #ddd;
}

.request-list-header h2 {
  margin: 0 0 1rem 0;
  font-size: 1.25rem;
}

.filter-controls {
  display: flex;
  gap: 0.5rem;
}

.filter-controls input,
.filter-controls select,
.filter-controls button {
  padding: 0.5rem;
  border: 1px solid #ccc;
  border-radius: 4px;
  font-size: 0.9rem;
}

.filter-controls input {
  flex-grow: 1;
}

.filter-controls button {
  cursor: pointer;
  background-color: #e0e0e0;
  transition: background-color 0.2s;
}

.filter-controls button:hover {
  background-color: #d0d0d0;
}

.request-list {
  list-style: none;
  padding: 0;
  margin: 0;
  overflow-y: auto;
  flex-grow: 1;
}

.request-list li {
  padding: 0.75rem 1rem;
  cursor: pointer;
  border-bottom: 1px solid #eee;
  display: flex;
  align-items: center;
  gap: 0.75rem;
  transition: background-color 0.2s;
}

.request-list li:hover {
  background-color: #f0f0f0;
}

.request-list li.selected {
  background-color: #d5e5f5;
  border-left: 4px solid #007bff;
  padding-left: calc(1rem - 4px);
}

.method {
  font-weight: 600;
  padding: 0.2rem 0.4rem;
  border-radius: 4px;
  background-color: #e0e0e0;
  font-size: 0.8rem;
  min-width: 40px;
  text-align: center;
}

.url {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.no-requests {
  padding: 1rem;
  text-align: center;
  color: #777;
}

.request-list li.group-header {
  background-color: #f8f9fa;
  color: #6c757d;
  font-weight: bold;
  padding: 0.5rem 1rem;
  position: sticky;
  top: 0;
  z-index: 10;
  cursor: default;
  white-space: normal; /* Allow multiline */
  word-break: break-all; /* Break long words */
}

.timestamp {
  font-family: monospace;
  font-size: 0.8rem;
  color: #6c757d;
  min-width: 80px;
}

.bookmark-star {
  color: #ffc107;
  font-size: 1.2rem;
  cursor: pointer;
}
</style>
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

const getUrlOrigin = (url: string) => {
  try {
    return new URL(url).origin;
  } catch (e) {
    const parts = url.split('/');
    if (parts.length >= 3) {
      return parts.slice(0, 3).join('/');
    }
    return 'Invalid URL';
  }
};

const getUrlPath = (url: string) => {
  try {
    return new URL(url).pathname;
  } catch (e) {
    const origin = getUrlOrigin(url);
    return url.substring(origin.length);
  }
};

const filteredRequests = computed(() => {
  return props.requests.filter(req => {
    const methodMatch = props.methodFilter === 'ALL' || req.method === props.methodFilter;
    const searchMatch = req.url.toLowerCase().includes(props.searchQuery.toLowerCase());
    return methodMatch && searchMatch;
  });
});

const groupedRequests = computed(() => {
  const groups: Record<string, CapturedRequest[]> = {};
  for (const request of filteredRequests.value) {
    const origin = getUrlOrigin(request.url);
    if (!groups[origin]) {
      groups[origin] = [];
    }
    groups[origin].push(request);
  }
  return groups;
});
</script>
