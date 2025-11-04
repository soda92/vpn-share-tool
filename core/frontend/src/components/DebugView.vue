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
  }
};

const selectRequest = (request: CapturedRequest) => {
  selectedRequest.value = request;
};

const showContextMenu = (event: MouseEvent, request: CapturedRequest) => {
  hideContextMenu(); // Hide any existing menu first
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
  window.open(shareUrl, ' _blank');
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



