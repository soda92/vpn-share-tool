<template>
  <div class="request-list-pane">
    <div class="request-list-header">
      <h2>Requests</h2>
      <div class="filter-controls">
        <input type="text" :value="searchQuery"
          @input="$emit('update:searchQuery', ($event.target as HTMLInputElement).value)" placeholder="Filter..."
          class="search-input" />
        <label class="error-filter">
          <input type="checkbox" v-model="hideErrors"> Hide Errors
        </label>
        <button @click="$emit('clear')" title="Clear">🚫</button>
      </div>
      <div class="type-filters">
        <button :class="{ active: resourceTypeFilter.has('ALL') }" @click="toggleFilter('ALL')">All</button>
        <button :class="{ active: resourceTypeFilter.has('XHR') }" @click="toggleFilter('XHR')">XHR</button>
        <button :class="{ active: resourceTypeFilter.has('JS') }" @click="toggleFilter('JS')">JS</button>
        <button :class="{ active: resourceTypeFilter.has('CSS') }" @click="toggleFilter('CSS')">CSS</button>
        <button :class="{ active: resourceTypeFilter.has('IMG') }" @click="toggleFilter('IMG')">Img</button>
        <button :class="{ active: resourceTypeFilter.has('DOC') }" @click="toggleFilter('DOC')">Doc</button>
        <button :class="{ active: resourceTypeFilter.has('OTHER') }" @click="toggleFilter('OTHER')">Other</button>
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
          <div class="req-meta">
            <span class="method" :class="request.method">{{ request.method }}</span>
            <span class="status" :class="getStatusClass(request.response_status)">{{ request.response_status }}</span>
          </div>
          <div class="req-main">
            <div class="req-path">{{ getUrlDirectory(request.url) }}</div>
            <div class="req-name-row">
               <div class="req-name" :title="request.url">{{ getRequestName(request.url) }}</div>
               <span v-if="request.repeatCount && request.repeatCount > 1" class="repeat-badge">{{ request.repeatCount }}</span>
            </div>
          </div>
        </li>
      </template>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import type { CapturedRequest } from '../types';

interface DisplayRequest extends CapturedRequest {
  repeatCount?: number;
}

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

const resourceTypeFilter = ref<Set<string>>(new Set(['DOC', 'XHR']));
const hideErrors = ref(true);

const getUrlOrigin = (url: string) => {
  try {
    return new URL(url).origin;
  } catch (e) {
    return 'Unknown';
  }
};

const getUrlDirectory = (url: string) => {
  try {
    const u = new URL(url);
    const path = u.pathname;
    const lastSlash = path.lastIndexOf('/');
    if (lastSlash === -1) return '/';
    return path.substring(0, lastSlash + 1);
  } catch (e) {
    return '/';
  }
};

const getBarePath = (url: string) => {
   try {
    return new URL(url).pathname;
  } catch (e) {
    return url;
  }
};

const toggleFilter = (type: string) => {
  if (type === 'ALL') {
    resourceTypeFilter.value.clear();
    resourceTypeFilter.value.add('ALL');
  } else {
    if (resourceTypeFilter.value.has('ALL')) {
      resourceTypeFilter.value.delete('ALL');
    }
    if (resourceTypeFilter.value.has(type)) {
      resourceTypeFilter.value.delete(type);
      if (resourceTypeFilter.value.size === 0) {
        resourceTypeFilter.value.add('ALL'); // Default back to ALL if empty
      }
    } else {
      resourceTypeFilter.value.add(type);
    }
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

  // Prioritize URL patterns for static assets to correctly classify 404s
  if (url.match(/\.(js|jsx|ts|tsx)(\?.*)?$/) || contentType.includes('javascript') || contentType.includes('application/x-javascript')) return 'JS';
  if (url.match(/\.(css|less|scss)(\?.*)?$/) || contentType.includes('css')) return 'CSS';
  if (url.match(/\.(png|jpg|jpeg|gif|ico|svg|webp|bmp)(\?.*)?$/) || contentType.includes('image')) return 'IMG';

  if (contentType.includes('text/html')) return 'DOC';
  if (contentType.includes('json') || contentType.includes('xml') || req.request_headers['X-Requested-With']) return 'XHR';

  return 'OTHER';
};

const filteredRequests = computed(() => {
  return props.requests.filter(req => {
    if (hideErrors.value && req.response_status >= 400) return false;

    const searchMatch = req.url.toLowerCase().includes(props.searchQuery.toLowerCase());

    if (!searchMatch) return false;

    // Resource Type Filter
    if (resourceTypeFilter.value.has('ALL')) return true;

    const type = getResourceType(req);
    return resourceTypeFilter.value.has(type);
  });
});

const collapseRequests = (reqs: CapturedRequest[]): DisplayRequest[] => {
  const res: DisplayRequest[] = [];
  if (reqs.length === 0) return res;

  // Clone first to start
  let current: DisplayRequest = { ...reqs[0]!, repeatCount: 1 };

  for (let i = 1; i < reqs.length; i++) {
    const next = reqs[i]!;
    const currPath = getBarePath(current.url);
    const nextPath = getBarePath(next.url);

    if (current.method === next.method && 
        currPath === nextPath && 
        current.response_status === next.response_status) {
       current.repeatCount = (current.repeatCount || 1) + 1;
       current.id = next.id;
       current.timestamp = next.timestamp;
       current.url = next.url;
    } else {
       res.push(current);
       current = { ...next, repeatCount: 1 };
    }
  }
  res.push(current);
  return res;
};

const groupedRequests = computed(() => {
  const groups: Record<string, DisplayRequest[]> = {};
  const rawGroups: Record<string, CapturedRequest[]> = {};
  for (const request of filteredRequests.value) {
    const origin = getUrlOrigin(request.url);
    if (!rawGroups[origin]) {
      rawGroups[origin] = [];
    }
    rawGroups[origin].push(request);
  }
  
  for (const origin in rawGroups) {
      groups[origin] = collapseRequests(rawGroups[origin]!);
  }
  return groups;
});
</script>



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
    height: auto;
    /* Grow with content */
    min-height: 0;
    border-right: none;
    border-bottom: none;
    overflow: visible;
    /* Let window scroll */
  }

  .request-list {
    overflow-y: visible;
    /* Let window scroll */
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

.error-filter {
  display: flex;
  align-items: center;
  font-size: 0.8rem;
  white-space: nowrap;
  gap: 4px;
  cursor: pointer;
  user-select: none;
}

.type-filters {
  display: flex;
  gap: 0.2rem;
  overflow-x: auto;
  padding-bottom: 2px;
  scrollbar-width: none;
  /* Firefox */
  -ms-overflow-style: none;
  /* IE 10+ */
}

.type-filters::-webkit-scrollbar {
  display: none;
  /* Chrome/Safari */
}

.type-filters button {
  padding: 0.2rem 0.4rem;
  /* Slightly reduced horizontal padding */
  font-size: 0.75rem;
  border: 1px solid #ccc;
  background-color: #fff;
  color: #333;
  /* Explicitly set text color */
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
  padding: 0.5rem 0.8rem 0.5rem 0.5rem;
  /* More right padding */
  cursor: pointer;
  border-bottom: 1px solid #eee;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.5rem;
  transition: background-color 0.2s;
  min-width: 0;
  /* Allow flex children to shrink */
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
  display: flex;
  flex-direction: column;
}

.req-name {
  font-weight: 600;
  font-size: 0.9rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: #333;
}

.req-name-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.repeat-badge {
  background-color: #6c757d;
  color: white;
  font-size: 0.7rem;
  padding: 1px 5px;
  border-radius: 10px;
  margin-left: 4px;
  font-weight: bold;
  flex-shrink: 0;
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
  align-items: center;
  justify-content: center;
  gap: 2px;
  font-size: 0.75rem;
  flex-shrink: 0;
  width: 45px;
  margin-right: 0.5rem;
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