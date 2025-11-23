<template>
  <div id="app" @click="hideContextMenu">
    <div class="main-layout">
      <div class="pane-container list-pane" :class="{ 'hidden-on-mobile': showMobileDetails }">
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
      </div>
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
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch, defineProps, nextTick, computed } from 'vue';
import axios from 'axios';
import type { CapturedRequest } from '../types';
import RequestList from './RequestList.vue';
import RequestDetails from './RequestDetails.vue';
import ContextMenu from './ContextMenu.vue';

const props = defineProps<{
  isLive?: boolean;
  sessionId: string;
}>();

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
const showMobileDetails = ref(false); // Mobile view state
let noteUpdateTimeout: number | undefined;

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
    const response = await axios.get(`/debug/sessions/${activeSessionId.value}/requests`);
    requests.value = response.data || [];
  } catch (error) {
    console.error('Error fetching requests:', error);
    requests.value = []; // Clear requests on error
  }
};

const clearHistory = async () => {
  if (props.isLive) {
    try {
      await axios.post('/debug/clear-live');
      fetchRequests(); // Refetch to show only persisted items
    } catch (error) {
      console.error('Error clearing history:', error);
    }
  } else {
    alert('Cannot clear history for a saved session.');
  }
};

const selectRequest = (request: CapturedRequest) => {
  selectedRequest.value = request;
  showMobileDetails.value = true; // Show details on mobile
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
      alert('Failed to delete request.');
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

onMounted(() => {
  fetchRequests();
  if (props.isLive) {
    const ws = new WebSocket(`ws://${window.location.host}/debug/ws`);
    ws.onmessage = (event) => {
      const newRequest = JSON.parse(event.data);
      // Avoid full refetch for performance. Add to list if not present.
      const existingIndex = requests.value.findIndex(r => r.id === newRequest.id);
      if (existingIndex === -1) {
        requests.value.unshift(newRequest);
      } else {
        requests.value[existingIndex] = newRequest;
      }
    };
    ws.onclose = () => console.log('WebSocket connection closed');
    ws.onerror = (error) => console.error('WebSocket error:', error);
    document.title = "Live Session";
  } else {
    // In a real app, you'd fetch the session name
    document.title = `Session ${props.sessionId}`;
  }
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
  width: 35%;
  min-width: 300px;
  /* border-right: 1px solid #ddd; Removed per user request */
}

.details-pane {
  flex-grow: 1;
  width: 65%; /* Default desktop width */
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



