<template>
  <div class="container">
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem;">
      <h2 class="view-title" style="margin: 0;">Recorded Archives</h2>
    </div>
    <div class="session-list-container">
      <ul v-if="sessions.length" class="session-list">
        <li v-for="session in sessions" :key="session.id" class="session-item">
          <div class="session-info">
            <span class="session-name">{{ session.name }}</span>
            <div class="session-meta">
              Original URL: <span class="meta-val">{{ session.proxy_url }}</span> | 
              Created: <span class="meta-val">{{ formatDate(session.created_at) }}</span>
            </div>
          </div>
          <div class="session-actions">
            <button @click="viewArchive(session)" class="action-btn view-btn">
              View 👁️
            </button>
            <button @click="downloadArchive(session.id, 'windows')" class="action-btn download-btn">
              Download (Win) 🪟
            </button>
            <button @click="downloadArchive(session.id, 'linux')" class="action-btn download-btn">
              Download (Linux) 🐧
            </button>
            <button @click="deleteSession(session.id)" class="action-btn delete-btn">Delete</button>
          </div>
        </li>
      </ul>
      <div v-else class="empty-state">No recorded archives.</div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { useToast } from 'vue-toastification';

const sessions = ref([]);
const toast = useToast();

const fetchSessions = async () => {
  try {
    const response = await axios.get('/archive/sessions');
    const data = response.data || [];
    // Sort by created_at (latest first)
    data.sort((a, b) => {
      const timeA = new Date(a.created_at).getTime();
      const timeB = new Date(b.created_at).getTime();
      return timeB - timeA;
    });
    sessions.value = data;
  } catch (e) {
    console.error('Failed to fetch archive sessions', e);
  }
};

const viewArchive = (session) => {
  const timestamp = new Date(session.created_at).getTime() * 1000000; // convert to nanoseconds
  window.open(`/archive/view/${session.id}/${timestamp}/${session.proxy_url}`, '_blank');
};

const deleteSession = async (id) => {
  if (confirm('Are you sure you want to delete this recorded archive session? All captured pages/data for this session will be permanently lost.')) {
    try {
      await axios.delete(`/archive/sessions/${id}`);
      toast.success('Archive session deleted successfully.');
      fetchSessions();
    } catch (e) {
      console.error('Failed to delete session', e);
      toast.error('Failed to delete archive session.');
    }
  }
};

const downloadArchive = (id, platform) => {
  window.open(`/archive/sessions/${id}/download?platform=${platform}`, '_blank');
};

const formatDate = (dateStr) => {
  if (!dateStr) return '-';
  try {
    const date = new Date(dateStr);
    return date.toLocaleString();
  } catch (e) {
    return dateStr;
  }
};

onMounted(fetchSessions);
</script>

<style scoped>
.container {
  height: 100%;
  display: flex;
  flex-direction: column;
  position: relative;
  padding: 1rem;
  overflow: hidden;
}

.view-title {
  margin: 0 0 1rem 0;
  color: #333;
}

.session-list-container {
  flex-grow: 1;
  overflow-y: auto;
}

.session-list {
  list-style: none;
  padding: 0;
  margin: 0;
}

.session-item {
  background-color: #ffffff;
  padding: 1rem;
  border-radius: 6px;
  margin-bottom: 0.8rem;
  display: flex;
  justify-content: space-between;
  align-items: center;
  box-shadow: 0 1px 3px rgba(0,0,0,0.1);
  border: 1px solid #e0e0e0;
}

.session-info {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.session-name {
  font-weight: 600;
  color: #673ab7;
  font-size: 1.1rem;
}

.session-meta {
  color: #666;
  font-size: 0.85rem;
}

.meta-val {
  font-weight: 500;
  color: #333;
}

.session-actions {
  display: flex;
  gap: 0.5rem;
}

.action-btn {
  background-color: #6c757d;
  color: white;
  border: none;
  padding: 0.4rem 0.8rem;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.9rem;
  transition: background-color 0.2s;
}

.action-btn:hover {
  background-color: #5a6268;
}

.download-btn {
  background-color: #007bff;
}

.download-btn:hover {
  background-color: #0056b3;
}

.view-btn {
  background-color: #673ab7;
}

.view-btn:hover {
  background-color: #512da8;
}

.delete-btn {
  background-color: #dc3545;
}

.delete-btn:hover {
  background-color: #c82333;
}

.empty-state {
  text-align: center;
  color: #777;
  margin-top: 2rem;
}

@media (max-width: 768px) {
  .session-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.8rem;
  }
  
  .session-actions {
    width: 100%;
    justify-content: flex-end;
  }
}
</style>
