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
import { ref, onMounted, watch, defineProps, nextTick } from 'vue';
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
    await axios.put(`/debug/requests/${props.sessionId}/${selectedRequest.value.id}`, { note: selectedRequestNote.value });
  } catch (error) {
    console.error('Error saving note:', error);
  }
};

const fetchRequests = async () => {
  if (!props.sessionId) return;
  try {
    const response = await axios.get(`/debug/sessions/${props.sessionId}/requests`);
    requests.value = response.data || [];
  } catch (error) {
    console.error('Error fetching requests:', error);
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
    await axios.put(`/debug/requests/${props.sessionId}/${request.id}`, { bookmarked: newStatus });
    request.bookmarked = newStatus; // Optimistically update UI
  } catch (error) {
    console.error('Error updating bookmark:', error);
  }
};

const deleteRequest = async () => {
  if (!contextMenu.value.request) return;
  if (confirm('Are you sure you want to permanently delete this request?')) {
    try {
      await axios.delete(`/debug/requests/${props.sessionId}/${contextMenu.value.request.id}`);
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
  const shareUrl = `${window.location.origin}/debug/request/${props.sessionId}/${contextMenu.value.request.id}`;
  window.open(shareUrl, ' _blank');
  hideContextMenu();
};

onMounted(() => {
  fetchRequests();
  if (props.isLive) {
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
</style>



