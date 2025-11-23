<template>
  <div class="request-list-pane">
    <div class="request-list-header">
      <h2>Requests</h2>
      <div class="filter-controls">
        <input type="text" :value="searchQuery"
          @input="$emit('update:searchQuery', ($event.target as HTMLInputElement).value)" placeholder="Filter..."
          class="search-input" />
        <button @click="$emit('clear')" title="Clear">🚫</button>
      </div>
      <div class="type-filters">
        <button :class="{ active: resourceTypeFilter === 'ALL' }" @click="resourceTypeFilter = 'ALL'">All</button>
        <button :class="{ active: resourceTypeFilter === 'XHR' }" @click="resourceTypeFilter = 'XHR'">XHR</button>
        <button :class="{ active: resourceTypeFilter === 'JS' }" @click="resourceTypeFilter = 'JS'">JS</button>
        <button :class="{ active: resourceTypeFilter === 'CSS' }" @click="resourceTypeFilter = 'CSS'">CSS</button>
        <button :class="{ active: resourceTypeFilter === 'IMG' }" @click="resourceTypeFilter = 'IMG'">Img</button>
        <button :class="{ active: resourceTypeFilter === 'DOC' }" @click="resourceTypeFilter = 'DOC'">Doc</button>
        <button :class="{ active: resourceTypeFilter === 'OTHER' }" @click="resourceTypeFilter = 'OTHER'">Other</button>
      </div>
    </div>
    <div v-if="filteredRequests.length === 0" class="no-requests">
      No requests found.
    </div>
    <ul v-else class="request-list">
      <template v-for="(group, groupName) in groupedRequests" :key="groupName">
        <li class="group-header">{{ groupName }}</li>
        <li v-for="request in group" :key="request.id"
          :class="{ selected: selectedRequest?.id === request.id, error: request.response_status >= 400 }"
          @click="$emit('select-request', request)" @contextmenu.prevent="$emit('show-context-menu', $event, request)">
          <div class="req-main">
            <div class="req-name" :title="request.url">{{ getRequestName(request.url) }}</div>
            <div class="req-path">{{ getUrlPath(request.url) }}</div>
          </div>
          <div class="req-meta">
            <span class="method" :class="request.method">{{ request.method }}</span>
            <span class="status" :class="getStatusClass(request.response_status)">{{ request.response_status }}</span>
          </div>
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
  height: 100%;
  /* Ensure full height in flex container */
}

@media (max-width: 768px) {
  .request-list-pane {
    width: 100%;
    height: auto; /* Grow with content */
    min-height: 0;
    border-right: none;
    border-bottom: none;
    overflow: visible; /* Let window scroll */
  }
  
  .request-list {
    overflow-y: visible; /* Let window scroll */
    height: auto;
  }
}

.request-list-header {
  padding: 0.5rem;
  border-bottom: 1px solid #ddd;
  background-color: #f8f9fa;
}

.request-list-header h2 {
  margin: 0 0 0.5rem 0;
  font-size: 1rem;
  display: none;
  /* Hide title to save space */
}

.filter-controls {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
}

.search-input {
  flex-grow: 1;
  padding: 0.4rem;
  border: 1px solid #ccc;
  border-radius: 4px;
  font-size: 0.9rem;
  min-width: 0;
  /* Prevent overflow */
}

.type-filters {
  display: flex;
  gap: 0.2rem;
  overflow-x: auto;
  /* Scrollable filters on mobile */
  padding-bottom: 2px;
}

.type-filters button {
  padding: 0.2rem 0.5rem;
  font-size: 0.75rem;
  border: 1px solid #ccc;
  background-color: #fff;
  border-radius: 10px;
  cursor: pointer;
  white-space: nowrap;
}

.type-filters button.active {
  background-color: #007bff;
  color: white;
  border-color: #007bff;
}

.request-list {
  list-style: none;
  padding: 0;
  margin: 0;
  overflow-y: auto;
  flex-grow: 1;
}

.request-list li {
  padding: 0.5rem;
  cursor: pointer;
  border-bottom: 1px solid #eee;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.5rem;
  transition: background-color 0.2s;
}

.request-list li:hover {
  background-color: #f0f0f0;
}

.request-list li.selected {
  background-color: #d5e5f5;
}

.request-list li.error .req-name {
  color: #d32f2f;
}

.req-main {
  flex-grow: 1;
  min-width: 0;
  overflow: hidden;
}

.req-name {
  font-weight: 600;
  font-size: 0.9rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: #333;
}

.req-path {
  font-size: 0.75rem;
  color: #888;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.req-meta {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
  font-size: 0.75rem;
  flex-shrink: 0;
}

.status {
  font-weight: bold;
}

.status.ok {
  color: #28a745;
}

.status.error {
  color: #dc3545;
}

.status.redirect {
  color: #ffc107;
}

.method {
  color: #555;
  font-weight: bold;
}

.request-list li.group-header {
  background-color: #e9ecef;
  color: #495057;
  font-weight: bold;
  font-size: 0.8rem;
  padding: 0.3rem 0.5rem;
  position: sticky;
  top: 0;
  z-index: 10;
}

.no-requests {
  padding: 1rem;
  text-align: center;
  color: #777;
}
</style>
<script setup lang="ts">
import { computed, defineProps, defineEmits, ref } from 'vue';
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

const resourceTypeFilter = ref('ALL');

const getUrlOrigin = (url: string) => {
  try {
    return new URL(url).origin;
  } catch (e) {
    return 'Unknown';
  }
};

const getUrlPath = (url: string) => {
  try {
    return new URL(url).pathname + new URL(url).search;
  }
  catch (e) {
    return url;
  }
};

const getRequestName = (url: string) => {
  try {
    const u = new URL(url);
    const segments = u.pathname.split('/').filter(Boolean);
    let name = segments.pop() || '/';
    if (u.search) name += u.search;
    return name;
  } catch (e) {
    return url;
  }
};

const getStatusClass = (status: number) => {
  if (status >= 200 && status < 300) return 'ok';
  if (status >= 300 && status < 400) return 'redirect';
  return 'error';
};

const getResourceType = (req: CapturedRequest): string => {
  const contentType = (req.response_headers['Content-Type']?.[0] || '').toLowerCase();
  const url = req.url.toLowerCase();

  if (contentType.includes('text/html')) return 'DOC';
  if (contentType.includes('javascript') || contentType.includes('application/x-javascript') || url.endsWith('.js')) return 'JS';
  if (contentType.includes('css') || url.endsWith('.css')) return 'CSS';
  if (contentType.includes('image') || url.match(/\.(png|jpg|jpeg|gif|ico|svg|webp)$/)) return 'IMG';
  if (contentType.includes('json') || contentType.includes('xml') || req.request_headers['X-Requested-With']) return 'XHR';

  return 'OTHER';
};

const filteredRequests = computed(() => {
  return props.requests.filter(req => {
    const searchMatch = req.url.toLowerCase().includes(props.searchQuery.toLowerCase());

    // Resource Type Filter
    if (resourceTypeFilter.value !== 'ALL') {
      const type = getResourceType(req);
      if (type === 'XHR' && (getResourceType(req) !== 'XHR' && getResourceType(req) !== 'OTHER')) return false; // Loose XHR? No, let's be strict.

      if (resourceTypeFilter.value === 'XHR') {
        // Special case: XHR often implies JSON/API calls not covered by others
        const t = getResourceType(req);
        if (t !== 'XHR') return false;
      } else if (getResourceType(req) !== resourceTypeFilter.value) {
        return false;
      }
    }

    return searchMatch;
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