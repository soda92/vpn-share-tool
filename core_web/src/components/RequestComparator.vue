<template>
  <div class="comparator-layout">
    <div class="request-pane" :class="{ collapsed: req1Collapsed }">
      <h2 @click="req1Collapsed = !req1Collapsed">
        Request 1
        <span class="collapse-icon">{{ req1Collapsed ? '+' : '-' }}</span>
      </h2>
      <pre v-show="!req1Collapsed">{{ req1 }}</pre>
    </div>
    <div class="request-pane" :class="{ collapsed: req2Collapsed }">
      <h2 @click="req2Collapsed = !req2Collapsed">
        Request 2
        <span class="collapse-icon">{{ req2Collapsed ? '+' : '-' }}</span>
      </h2>
      <pre v-show="!req2Collapsed">{{ req2 }}</pre>
    </div>
    <div class="diff-pane">
      <h2>Diff</h2>
      <DiffViewer v-if="diffData" :data1="diffData.data1" :data2="diffData.data2" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { useRoute } from 'vue-router';
import DiffViewer from './DiffViewer.vue';

interface CapturedRequest {
  id: number;
  timestamp: string;
  method: string;
  url: string;
  request_headers: Record<string, string[]>;
  request_body: string;
  response_status: number;
  response_headers: Record<string, string[]>;
  response_body: string;
}

const route = useRoute();
const req1 = ref<CapturedRequest | null>(null);
const req2 = ref<CapturedRequest | null>(null);
const req1Collapsed = ref(false);
const req2Collapsed = ref(false);

const fetchRequest = async (id: number): Promise<CapturedRequest | null> => {
  try {
    const response = await fetch(`/debug/requests/${id}`);
    if (response.ok) {
      return await response.json();
    } else {
      console.error(`Failed to fetch request ${id}`);
      return null;
    }
  } catch (error) {
    console.error(`Error fetching request ${id}:`, error);
    return null;
  }
};

const diffData = computed(() => {
  if (!req1.value || !req2.value) {
    return null;
  }

  const req1Body = req1.value.request_body;
  const req2Body = req2.value.request_body;

  const req1ContentType = req1.value.request_headers['Content-Type']?.[0] || '';
  const isJson = req1ContentType.includes('application/json');
  const isForm = req1ContentType.includes('application/x-www-form-urlencoded');

  let data1: Record<string, unknown> = {};
  let data2: Record<string, unknown> = {};

  if (isJson) {
    try {
      data1 = JSON.parse(req1Body);
    } catch (e) {
      console.error('Error parsing JSON for request 1:', e);
    }
    try {
      data2 = JSON.parse(req2Body);
    } catch (e) {
      console.error('Error parsing JSON for request 2:', e);
    }
  } else if (isForm) {
    data1 = Object.fromEntries(new URLSearchParams(req1Body));
    data2 = Object.fromEntries(new URLSearchParams(req2Body));
  }

  return { data1, data2 };
});

onMounted(async () => {
  const req1Id = Number(route.query.req1);
  const req2Id = Number(route.query.req2);

  if (req1Id) {
    req1.value = await fetchRequest(req1Id);
  }
  if (req2Id) {
    req2.value = await fetchRequest(req2Id);
  }

  document.title = `Compare Requests ${req1Id} vs ${req2Id}`;
});
</script>

<style scoped>
.comparator-layout {
  display: flex;
  height: 100vh;
  background-color: #f5f5f5;
}

.request-pane, .diff-pane {
  padding: 1rem;
  overflow-y: auto;
  transition: width 0.3s ease;
}

.request-pane {
  width: 33.3%;
  background-color: #fff;
  border-right: 1px solid #ddd;
}

.request-pane.collapsed {
  width: 50px; /* Collapsed width */
}

.diff-pane {
  width: 34%;
  flex-grow: 1;
}

h2 {
  cursor: pointer;
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin: -1rem -1rem 1rem -1rem;
  padding: 1rem;
  background-color: #e9e9e9;
  border-bottom: 1px solid #ddd;
}

.collapse-icon {
  font-family: monospace;
  font-size: 1.2rem;
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
</style>