<template>
  <div id="app" @click="hideContextMenu">
    <div class="main-layout">
      <RequestList
        :requests="requests"
        :selectedRequest="selectedRequest"
        v-model:searchQuery="searchQuery"
        v-model:methodFilter="methodFilter"
        @select-request="selectRequest"
        @show-context-menu="showContextMenu"
        @toggle-bookmark="toggleBookmark"
        @clear="clearHistory"
      />
      <RequestDetails
        :request="selectedRequest"
        v-model:note="selectedRequestNote"
      />
      <ContextMenu
        :menuData="contextMenu"
        :isCompareEnabled="!!selectedForCompare"
        :isDeleteEnabled="!props.isLive"
        @select-for-compare="selectForCompare"
        @compare-with-selected="compareWithSelected"
        @share-request="shareRequest"
        @delete-request="deleteRequest"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch, defineProps } from 'vue';
import axios from 'axios';
import { useRouter } from 'vue-router';
import type { CapturedRequest } from '../types';
import RequestList from './RequestList.vue';
import RequestDetails from './RequestDetails.vue';
import ContextMenu from './ContextMenu.vue';

const props = defineProps<{
  isLive: boolean;
  sessionId?: string;
}>();

const router = useRouter();

const requests = ref<CapturedRequest[]>([]);
const selectedRequest = ref<CapturedRequest | null>(null);
const searchQuery = ref('');
const methodFilter = ref('ALL');
const selectedForCompare = ref<CapturedRequest | null>(null);
const contextMenu = ref({
  visible: false,
  x: 0,
  y: 0,
  request: null as CapturedRequest | null,
});
const selectedRequestNote = ref('');
let noteUpdateTimeout: number | undefined;

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
    await axios.put(`/debug/requests/${selectedRequest.value.id}`, { note: selectedRequestNote.value });
  } catch (error) {
    console.error('Error saving note:', error);
  }
};

const fetchRequests = async () => {
  try {
    let url = '';
    if (props.isLive) {
      url = '/debug/live-requests';
    } else if (props.sessionId) {
      url = `/debug/sessions/${props.sessionId}/requests`;
    }
    const response = await axios.get(url);
    requests.value = (response.data || []).sort((a: CapturedRequest, b: CapturedRequest) => b.id - a.id);
  } catch (error) {
    console.error('Error fetching requests:', error);
  }
};

const clearHistory = async () => {
  if (props.isLive) {
    try {
      await axios.post('/debug/clear');
      requests.value = [];
      selectedRequest.value = null;
      selectedForCompare.value = null;
    } catch (error) {
      console.error('Error clearing history:', error);
    }
  } else {
    alert('Cannot clear history for a saved session.');
  }
};

const selectRequest = (request: CapturedRequest) => {
  selectedRequest.value = request;
};

const showContextMenu = (event: MouseEvent, request: CapturedRequest) => {
  contextMenu.value.visible = true;
  contextMenu.value.x = event.clientX;
  contextMenu.value.y = event.clientY;
  contextMenu.value.request = request;
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
    window.open(`/debug/compare?req1=${selectedForCompare.value.id}&req2=${contextMenu.value.request.id}`, '_blank');
  }
  hideContextMenu();
};

const toggleBookmark = async (request: CapturedRequest) => {
  const newStatus = !request.bookmarked;
  request.bookmarked = newStatus;
  try {
    await axios.put(`/debug/requests/${request.id}`, { bookmarked: newStatus });
  } catch (error) {
    console.error('Error updating bookmark:', error);
    request.bookmarked = !newStatus; // Revert on error
  }
};

const deleteRequest = async () => {
  if (!contextMenu.value.request) return;
  if (confirm('Are you sure you want to permanently delete this request?')) {
    try {
      await axios.delete(`/debug/requests/${contextMenu.value.request.id}`);
      const index = requests.value.findIndex((r: CapturedRequest) => r.id === contextMenu.value.request!.id);
      if (index > -1) {
        requests.value.splice(index, 1);
      }
      if (selectedRequest.value?.id === contextMenu.value.request.id) {
        selectedRequest.value = null;
      }
    } catch (error) {
      console.error('Error deleting request:', error);
      alert('Failed to delete request.');
    }
  }
  hideContextMenu();
};

const shareRequest = async () => {
  if (!contextMenu.value.request) return;
  let requestIdToShare = contextMenu.value.request.id;

  if (props.isLive) {
    try {
      const response = await axios.post('/debug/share-request', contextMenu.value.request);
      requestIdToShare = response.data.id;
    } catch (error) {
      console.error('Error saving request for sharing:', error);
      alert('Failed to save request for sharing.');
      return;
    }
  }

  const shareUrl = `${window.location.origin}/debug/request/${requestIdToShare}`;
  window.open(shareUrl, '_blank');
  hideContextMenu();
};

onMounted(() => {
  if (props.isLive) {
    fetchRequests();
    const ws = new WebSocket(`ws://${window.location.host}/debug/ws`);
    ws.onmessage = (event) => {
      const newRequest: CapturedRequest = JSON.parse(event.data);
      requests.value.unshift(newRequest);
      if (requests.value.length > 1000) {
        requests.value.pop();
      }
    };
    ws.onclose = () => console.log('WebSocket connection closed');
    ws.onerror = (error) => console.error('WebSocket error:', error);
  } else {
    fetchRequests();
  }
  document.title = props.isLive ? "Live Session" : `Session ${props.sessionId}`;
});
</script>

<style scoped>
#app {
  font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
  height: 100vh;
  margin: 0;
  background-color: #f5f5f5;
  color: #333;
}

.main-layout {
  display: flex;
  height: 100%;
}

.request-list-pane {
  width: 35%;
  min-width: 300px;
  border-right: 1px solid #ddd;
  display: flex;
  flex-direction: column;
  background-color: #fff;
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

.request-details-pane {
  width: 65%;
  padding: 1.5rem;
  overflow-y: auto;
}

.details-grid {
  display: grid;
  grid-template-columns: 120px 1fr;
  gap: 0.75rem;
  margin-bottom: 1.5rem;
  background-color: #fff;
  padding: 1rem;
  border-radius: 6px;
  border: 1px solid #ddd;
}

.details-grid div {
  word-break: break-all;
}

h2, h3 {
  border-bottom: 2px solid #007bff;
  padding-bottom: 0.5rem;
  margin-bottom: 1rem;
  color: #0056b3;
}

pre {
  background-color: #fff;
  padding: 1rem;
  border: 1px solid #ddd;
  border-radius: 6px;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: 'Fira Code', 'Courier New', Courier, monospace;
}

.no-selection {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100%;
  font-size: 1.2rem;
  color: #777;
}

.context-menu {
  position: absolute;
  background-color: white;
  border: 1px solid #ccc;
  box-shadow: 2px 2px 5px rgba(0, 0, 0, 0.15);
  border-radius: 4px;
  padding: 0.5rem 0;
  margin: 0;
  list-style: none;
  z-index: 1000;
}

.context-menu ul {
  list-style: none;
  padding: 0;
  margin: 0;
}

.context-menu li {
  padding: 0.6rem 1.2rem;
  cursor: pointer;
  transition: background-color 0.2s;
}

.context-menu li:hover {
  background-color: #f0f0f0;
}

.context-menu li.disabled {
  color: #aaa;
  cursor: not-allowed;
  background-color: #fff;
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

textarea {
  width: 100%;
  min-height: 100px;
  padding: 0.5rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-family: inherit;
}

.delete-option {
  color: #dc3545;
}

.delete-option:hover {
  background-color: #f8d7da !important;
}
</style>
