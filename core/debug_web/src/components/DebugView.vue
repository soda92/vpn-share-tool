<template>
  <div class="debug-view" @click="hideContextMenu">
    <div class="main-layout">
      <div 
        class="pane-container list-pane" 
        :style="!isMobileView ? { width: sidebarWidth + 'px' } : {}"
        :class="{ 'hidden-on-mobile': showMobileDetails }"
      >
        <RequestList
          :requests="requests"
          :selectedRequest="selectedRequest"
          v-model:searchQuery="searchQuery"
          v-model:methodFilter="methodFilter"
          v-model:hideErrors="hideErrors"
          v-model:resourceTypeFilter="resourceTypeFilter"
          :has-more="hasMoreRequests"
          :loading-more="loadingMore"
          @select-request="selectRequest"
          @show-context-menu="showContextMenu"
          @toggle-bookmark="toggleBookmark"
          @clear="clearHistory"
          @load-more="loadMoreRequests"
        />
      </div>
      <div 
        v-if="!isMobileView" 
        class="resizer" 
        :class="{ dragging: isDragging }" 
        @mousedown="startResize"
      ></div>
      <div class="pane-container details-pane" :class="{ 'active-on-mobile': showMobileDetails }">
        <RequestDetails
          :request="selectedRequest"
          v-model:note="selectedRequestNote"
          @close="closeMobileDetails"
        />
      </div>
      <ContextMenu
        :menuData="contextMenu"
        :isCompareEnabled="!!selectedForCompare"
        :isDeleteEnabled="!props.isLive"
        @select-for-compare="selectForCompare"
        @compare-with-selected="compareWithSelected"
        @share-request="shareRequest"
        @delete-request="deleteRequest"
        @toggle-bookmark="toggleBookmarkFromContext"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick, computed } from 'vue';
import axios from 'axios';
import type { CapturedRequest } from '../types';
import RequestList from './RequestList.vue';
import RequestDetails from './RequestDetails.vue';
import ContextMenu from './ContextMenu.vue';
import { useToast } from 'vue-toastification';

const props = defineProps<{
  isLive?: boolean;
  sessionId: string;
}>();

const requests = ref<CapturedRequest[]>([]);
const totalRequests = ref(0);
const limit = ref(50);
const loadingMore = ref(false);

const hasMoreRequests = computed(() => requests.value.length < totalRequests.value);
const selectedRequest = ref<CapturedRequest | null>(null);
const searchQuery = ref('');
const methodFilter = ref('ALL');
const hideErrors = ref(true);
const resourceTypeFilter = ref<string[]>(['DOC', 'XHR']);
const selectedForCompare = ref<CapturedRequest | null>(null);
const contextMenu = ref({
  visible: false,
  x: 0,
  y: 0,
  request: null as CapturedRequest | null,
});
const selectedRequestNote = ref('');
const showMobileDetails = ref(false); // Mobile view state
let noteUpdateTimeout: number | undefined;
const toast = useToast();

const activeSessionId = computed(() => props.isLive ? 'live_session' : props.sessionId);

watch(selectedRequest, (newReq) => {
  selectedRequestNote.value = newReq?.note || '';
});

watch(selectedRequestNote, () => {
  clearTimeout(noteUpdateTimeout);
  noteUpdateTimeout = window.setTimeout(() => {
    saveNote();
  }, 1500);
});

const saveNote = async () => {
  if (!selectedRequest.value) return;
  selectedRequest.value.note = selectedRequestNote.value;
  try {
    await axios.put(`/api/debug/requests/${activeSessionId.value}/${selectedRequest.value.id}`, { note: selectedRequestNote.value });
  } catch (error) {
    console.error('Error saving note:', error);
  }
};

const fetchRequests = async () => {
  if (!activeSessionId.value) return;
  try {
    const response = await axios.get(`/api/debug/sessions/${activeSessionId.value}/requests`, {
      params: { 
        page: 1, 
        limit: limit.value,
        search: searchQuery.value,
        hide_errors: hideErrors.value,
        types: resourceTypeFilter.value.join(',')
      }
    });
    requests.value = response.data.requests || [];
    totalRequests.value = response.data.total || 0;
  } catch (error) {
    console.error('Error fetching requests:', error);
    requests.value = []; // Clear requests on error
    totalRequests.value = 0;
  }
};

const loadMoreRequests = async () => {
  if (loadingMore.value || !hasMoreRequests.value) return;
  loadingMore.value = true;
  try {
    const nextPage = Math.floor(requests.value.length / limit.value) + 1;
    const response = await axios.get(`/api/debug/sessions/${activeSessionId.value}/requests`, {
      params: { 
        page: nextPage, 
        limit: limit.value,
        search: searchQuery.value,
        hide_errors: hideErrors.value,
        types: resourceTypeFilter.value.join(',')
      }
    });
    
    const newItems = response.data.requests || [];
    newItems.forEach((item: CapturedRequest) => {
      if (!requests.value.some(r => r.id === item.id)) {
        requests.value.push(item);
      }
    });
    totalRequests.value = response.data.total || 0;
  } catch (error) {
    console.error('Error loading more requests:', error);
  } finally {
    loadingMore.value = false;
  }
};

let searchDebounceTimeout: number | undefined;
watch([searchQuery, hideErrors, resourceTypeFilter], () => {
  clearTimeout(searchDebounceTimeout);
  searchDebounceTimeout = window.setTimeout(() => {
    fetchRequests();
  }, 300);
});

const clearHistory = async () => {
  if (props.isLive) {
    try {
      await axios.post('/api/debug/clear-live');
      fetchRequests(); // Refetch to show only persisted items
    } catch (error) {
      console.error('Error clearing history:', error);
    }
  } else {
    toast.warning('Cannot clear history for a saved session.');
  }
};

const selectRequest = async (request: CapturedRequest) => {
  selectedRequest.value = request;
  showMobileDetails.value = true; // Show details on mobile

  if (request.response_body === undefined) {
    try {
      const response = await axios.get(`/api/debug/requests/${activeSessionId.value}/${request.id}`);
      
      // Update original item in the reactive requests list
      const originalReq = requests.value.find(r => r.id === request.id);
      if (originalReq) {
        originalReq.request_body = response.data.request_body;
        originalReq.response_body = response.data.response_body;
        originalReq.is_base64 = response.data.is_base64;
      }
      
      // Update selectedRequest and trigger reactive updates in components
      if (selectedRequest.value?.id === request.id) {
        selectedRequest.value.request_body = response.data.request_body;
        selectedRequest.value.response_body = response.data.response_body;
        selectedRequest.value.is_base64 = response.data.is_base64;
        selectedRequest.value = { ...selectedRequest.value };
      }
    } catch (error) {
      console.error('Error fetching request details:', error);
    }
  }
};

const closeMobileDetails = () => {
  showMobileDetails.value = false;
  // Optional: deselect request? maybe not, keeping state is fine.
};

const showContextMenu = (event: MouseEvent, request: CapturedRequest) => {
  hideContextMenu();
  nextTick(() => {
    contextMenu.value.visible = true;
    contextMenu.value.x = event.clientX;
    contextMenu.value.y = event.clientY;
    contextMenu.value.request = request;
  });
};

const hideContextMenu = () => {
  contextMenu.value.visible = false;
};

const selectForCompare = () => {
  if (contextMenu.value.request) {
    selectedForCompare.value = contextMenu.value.request;
  }
  hideContextMenu();
};

const compareWithSelected = () => {
  if (selectedForCompare.value && contextMenu.value.request) {
    window.open(`/debug/compare?req1=${selectedForCompare.value.id}&req2=${contextMenu.value.request.id}`, ' _blank');
  }
  hideContextMenu();
};

const toggleBookmark = async (request: CapturedRequest) => {
  const newStatus = !request.bookmarked;
  try {
    await axios.put(`/api/debug/requests/${activeSessionId.value}/${request.id}`, { bookmarked: newStatus });
    request.bookmarked = newStatus; // Optimistically update UI
  } catch (error) {
    console.error('Error updating bookmark:', error);
  }
};

const toggleBookmarkFromContext = () => {
  if (contextMenu.value.request) {
    toggleBookmark(contextMenu.value.request);
  }
  hideContextMenu();
};

const deleteRequest = async () => {
  if (!contextMenu.value.request) return;
  if (confirm('Are you sure you want to permanently delete this request?')) {
    try {
      await axios.delete(`/api/debug/requests/${activeSessionId.value}/${contextMenu.value.request.id}`);
      const index = requests.value.findIndex((r: CapturedRequest) => r.id === contextMenu.value.request!.id);
      if (index > -1) {
        requests.value.splice(index, 1);
      }
      if (selectedRequest.value?.id === contextMenu.value.request.id) {
        selectedRequest.value = null;
      }
    } catch (error) {
      console.error('Error deleting request:', error);
      toast.error('Failed to delete request.');
    }
  }
  hideContextMenu();
};

const shareRequest = async () => {
  if (!contextMenu.value.request) return;
  const shareUrl = `${window.location.origin}/debug/request/${activeSessionId.value}/${contextMenu.value.request.id}`;
  window.open(shareUrl, ' _blank');
  hideContextMenu();
};

let ws: WebSocket | null = null;
let reconnectTimeout: number | undefined;

const connectWebSocket = () => {
  if (!props.isLive) return;

  clearTimeout(reconnectTimeout);

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/debug/ws`;
  console.log(`Connecting to WebSocket at ${wsUrl}...`);
  ws = new WebSocket(wsUrl);

  ws.onmessage = (event) => {
    try {
      const newRequest = JSON.parse(event.data);
      const existingIndex = requests.value.findIndex(r => r.id === newRequest.id);
      if (existingIndex === -1) {
        requests.value.unshift(newRequest);
        totalRequests.value++;
      } else {
        requests.value[existingIndex] = newRequest;
      }
    } catch (e) {
      console.error('Failed to parse WebSocket message:', e);
    }
  };

  ws.onopen = () => {
    console.log('WebSocket connection established');
    fetchRequests(); // Refetch to catch up on any requests missed while disconnected
  };

  ws.onclose = (event) => {
    console.log('WebSocket connection closed. Reconnecting in 3s...', event.reason);
    reconnectTimeout = window.setTimeout(connectWebSocket, 3000);
  };

  ws.onerror = (error) => {
    console.error('WebSocket error:', error);
  };
};

const isMobileView = ref(typeof window !== 'undefined' ? window.innerWidth <= 768 : false);
const sidebarWidth = ref(parseInt(localStorage.getItem('debug_sidebar_width') || '360', 10));
const isDragging = ref(false);

const handleResize = () => {
  isMobileView.value = window.innerWidth <= 768;
};

const startResize = (event: MouseEvent) => {
  event.preventDefault();
  isDragging.value = true;
  document.addEventListener('mousemove', doResize);
  document.addEventListener('mouseup', stopResize);
};

const doResize = (event: MouseEvent) => {
  if (!isDragging.value) return;
  const newWidth = event.clientX;
  if (newWidth >= 280 && newWidth <= 600) {
    sidebarWidth.value = newWidth;
  }
};

const stopResize = () => {
  isDragging.value = false;
  document.removeEventListener('mousemove', doResize);
  document.removeEventListener('mouseup', stopResize);
  localStorage.setItem('debug_sidebar_width', sidebarWidth.value.toString());
};

onMounted(() => {
  fetchRequests();
  if (props.isLive) {
    connectWebSocket();
    document.title = "Live Session";
  } else {
    document.title = `Session ${props.sessionId}`;
  }
  window.addEventListener('resize', handleResize);
});

onUnmounted(() => {
  clearTimeout(reconnectTimeout);
  if (ws) {
    ws.close();
  }
  window.removeEventListener('resize', handleResize);
});
</script>

<style scoped>
.debug-view {
  font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
  height: 100%;
  margin: 0;
  background-color: #f5f5f5;
  color: #333;
  overflow: hidden; /* Default desktop */
}

@media (max-width: 768px) {
  .debug-view {
    height: auto;
    overflow: visible;
  }
}

.main-layout {
  display: flex;
  height: 100%;
  overflow: hidden;
  position: relative; /* Context for absolute positioning if needed */
}

.pane-container {
  height: 100%;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.list-pane {
  min-width: 280px;
  border-right: 1px solid #e0e0e0;
}

.resizer {
  width: 6px;
  cursor: col-resize;
  background-color: #e0e0e0;
  transition: background-color 0.2s, width 0.2s;
  height: 100%;
  z-index: 10;
  flex-shrink: 0;
}

.resizer:hover,
.resizer.dragging {
  background-color: #673ab7;
  width: 8px;
}

.details-pane {
  flex-grow: 1;
  /* width is automatic via flex-grow */
  overflow: hidden; /* Ensure scrolling happens inside */
}

button {
  background-color: #007bff;
  color: white;
  border: none;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  cursor: pointer;
  transition: background-color 0.2s;
}

button:hover {
  background-color: #0056b3;
}

@media (max-width: 768px) {
  .main-layout {
    flex-direction: column;
    height: auto; /* Allow growth */
    overflow: visible; /* Allow spillover to body scroll */
  }

  .pane-container {
    height: auto; /* Allow growth */
    overflow: visible;
  }

  .list-pane {
    width: 100%; /* Full width on mobile */
    border-right: none;
  }

  .details-pane {
    width: 100%;
    display: none; /* Hidden by default on mobile */
  }

  .list-pane.hidden-on-mobile {
    display: none;
  }

  .details-pane.active-on-mobile {
    display: flex; /* Show when active */
  }
}
</style>



