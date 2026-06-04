<template>
  <div class="container">
    <div class="header-section">
      <h1 class="app-title">{{ $t('title') }}</h1>
      <div v-if="isAuthenticated" class="user-info">
        <span>Logged in as <strong>{{ authUsername }}</strong></span>
        <el-button size="small" type="danger" plain @click="handleLogout">
          Logout
        </el-button>
      </div>
    </div>

    <div v-if="!isAuthenticated" class="login-wrapper">
      <el-card class="login-card">
        <template #header>
          <div class="card-header">
            <h3>Login to Control Panel</h3>
          </div>
        </template>
        <el-form :model="loginForm" label-position="top" @submit.prevent="handleLogin">
          <el-form-item label="Username">
            <el-input v-model="loginForm.username" placeholder="Enter username" />
          </el-form-item>
          <el-form-item label="Password">
            <el-input v-model="loginForm.password" type="password" show-password placeholder="Enter password" />
          </el-form-item>
          <div class="login-btn-wrapper">
            <el-button type="primary" native-type="submit" :loading="loginLoading" class="login-btn">
              Login
            </el-button>
          </div>
        </el-form>
      </el-card>
    </div>

    <div v-else-if="mustChangePassword" class="login-wrapper">
      <el-card class="login-card">
        <template #header>
          <div class="card-header">
            <h3>Change Your Password</h3>
            <p style="margin: 5px 0 0 0; font-size: 0.85rem; color: #909399; text-align: center;">
              You must change your password before continuing.
            </p>
          </div>
        </template>
        <el-form :model="changePasswordForm" label-position="top" @submit.prevent="handleChangePassword">
          <el-form-item label="Current Password">
            <el-input v-model="changePasswordForm.oldPassword" type="password" show-password placeholder="Enter current password" />
          </el-form-item>
          <el-form-item label="New Password">
            <el-input v-model="changePasswordForm.newPassword" type="password" show-password placeholder="Enter new password" />
          </el-form-item>
          <el-form-item label="Confirm New Password">
            <el-input v-model="changePasswordForm.confirmPassword" type="password" show-password placeholder="Confirm new password" />
          </el-form-item>
          <div class="login-btn-wrapper">
            <el-button type="primary" native-type="submit" :loading="changePasswordLoading" class="login-btn">
              Change Password
            </el-button>
          </div>
        </el-form>
      </el-card>
    </div>

    <div v-else>
      <div class="main-grid" :class="{ 'single-column': !showAllProxies }">
        <!-- Left Column: Tagged URLs (Primary Action) -->
        <div class="left-column">
          <div class="tagged-list-wrapper">
            <TaggedList :tagged-urls="taggedUrls" :add-form="newTag" :creating-proxy-urls="creatingProxyUrls" :servers="servers"
              @save-tag="saveTaggedUrl" @create-proxy="createProxy" @open-settings="openSettings" @rename-tag="renameTag"
              @delete-tag="deleteTag" />
          </div>

          <div class="toggle-section">
            <el-button @click="showAllProxies = !showAllProxies">
              {{ showAllProxies ? 'Hide All Active Proxies' : 'Show All Active Proxies' }}
            </el-button>
          </div>
        </div>

        <!-- Right Column: All Active Proxies (Quick Access) -->
        <div v-if="showAllProxies" class="right-column">
          <ProxyList :cluster-proxies="clusterProxies" @open-settings="openSettings" />
        </div>
      </div>

      <!-- Bottom Section: Active Servers (Info) -->
      <ServerInfo :servers="servers" :latest-version="latestVersion" @update-server="handleUpdateServer"
        @open-logs="openLogs" />

      <SettingsDialog v-model="settingsVisible" :proxy-data="currentSettingsProxy" @save="handleSaveSettings" />

      <LogViewer v-model="logsVisible" :server-address="currentLogServer" />
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { ElNotification } from 'element-plus';
import TaggedList from './components/TaggedList.vue';
import ProxyList from './components/ProxyList.vue';
import ServerInfo from './components/ServerInfo.vue';
import SettingsDialog from './components/SettingsDialog.vue';
import LogViewer from './components/LogViewer.vue';

const servers = ref([]);
const isAuthenticated = ref(false);
const authUsername = ref('');
const loginForm = ref({ username: '', password: '' });
const loginLoading = ref(false);

const mustChangePassword = ref(false);
const changePasswordForm = ref({ oldPassword: '', newPassword: '', confirmPassword: '' });
const changePasswordLoading = ref(false);

// Response interceptor to catch 401 Unauthorized
axios.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response && error.response.status === 401) {
      isAuthenticated.value = false;
    }
    return Promise.reject(error);
  }
);

const checkAuth = async () => {
  try {
    const response = await axios.get('/check-auth');
    if (response.data && response.data.authenticated) {
      isAuthenticated.value = true;
      authUsername.value = response.data.username;
      mustChangePassword.value = !!response.data.must_change_password;
      return true;
    }
  } catch (err) {
    isAuthenticated.value = false;
  }
  return false;
};

const handleLogin = async () => {
  loginLoading.value = true;
  try {
    const response = await axios.post('/login', loginForm.value);
    isAuthenticated.value = true;
    authUsername.value = loginForm.value.username;
    mustChangePassword.value = !!response.data.must_change_password;
    ElNotification({ title: 'Success', message: 'Logged in successfully.', type: 'success' });
    
    // Start data loading and polling if we don't need to change password
    if (!mustChangePassword.value) {
      fetchLatestVersion();
      startPolling();
    }
  } catch (err) {
    ElNotification({
      title: 'Error',
      message: err.response?.data?.error || err.response?.data?.message || 'Invalid username or password.',
      type: 'error'
    });
  } finally {
    loginLoading.value = false;
  }
};

const handleLogout = async () => {
  try {
    await axios.post('/logout');
    isAuthenticated.value = false;
    authUsername.value = '';
    mustChangePassword.value = false;
    ElNotification({ title: 'Success', message: 'Logged out successfully.', type: 'success' });
  } catch (err) {
    ElNotification({ title: 'Error', message: 'Logout failed.', type: 'error' });
  }
};

const handleChangePassword = async () => {
  if (changePasswordForm.value.newPassword !== changePasswordForm.value.confirmPassword) {
    ElNotification({ title: 'Error', message: 'New passwords do not match.', type: 'error' });
    return;
  }
  if (changePasswordForm.value.newPassword.length < 6) {
    ElNotification({ title: 'Error', message: 'New password must be at least 6 characters long.', type: 'error' });
    return;
  }
  changePasswordLoading.value = true;
  try {
    await axios.post('/change-password', {
      old_password: changePasswordForm.value.oldPassword,
      new_password: changePasswordForm.value.newPassword
    });
    mustChangePassword.value = false;
    changePasswordForm.value = { oldPassword: '', newPassword: '', confirmPassword: '' };
    ElNotification({ title: 'Success', message: 'Password changed successfully.', type: 'success' });
    
    // Start standard flows
    fetchLatestVersion();
    startPolling();
  } catch (err) {
    ElNotification({
      title: 'Error',
      message: err.response?.data || err.response?.data?.error || 'Failed to change password.',
      type: 'error'
    });
  } finally {
    changePasswordLoading.value = false;
  }
};
const taggedUrls = ref([]);
const clusterProxies = ref([]);
const showAllProxies = ref(false);
const newTag = ref({ tag: '', url: '' });
const creatingProxyUrls = ref({});
const latestVersion = ref('');

// Settings Dialog State
const settingsVisible = ref(false);
const currentSettingsProxy = ref(null);

// Log Viewer State
const logsVisible = ref(false);
const currentLogServer = ref('');

const openSettings = (proxy) => {
  currentSettingsProxy.value = proxy;
  settingsVisible.value = true;
};

const openLogs = (address) => {
  currentLogServer.value = address;
  logsVisible.value = true;
};

const handleSaveSettings = async (data) => {
  let success = false;
  // 1. Try New Settings
  try {
    await axios.post('/update-proxy-settings', {
      url: data.url,
      settings: data.settings,
      node_address: data.node_address || ''
    });
    success = true;
  } catch (err) {
    console.warn("New settings API failed:", err);
  }

  if (success) {
    ElNotification({ title: 'Success', message: 'Settings updated.', type: 'success' });
    fetchTaggedURLs();
    fetchClusterProxies();
  } else {
    ElNotification({ title: 'Error', message: 'Failed to update settings. Check client connection.', type: 'error' });
  }
};

const fetchServers = async () => {
  try {
    const response = await axios.get('/instances');
    servers.value = (response.data || []).sort((a, b) => a.address.localeCompare(b.address));
  } catch (err) { console.error('Error fetching servers:', err); }
};
const fetchClusterProxies = async () => {
  try {
    const response = await axios.get('/cluster-proxies');
    clusterProxies.value = (response.data || []).sort((a, b) => a.original_url.localeCompare(b.original_url));
  } catch (err) { console.error('Error fetching cluster proxies:', err); }
};
const fetchTaggedURLs = async () => {
  try {
    const response = await axios.get('/tagged-urls');
    taggedUrls.value = (response.data || []).sort((a, b) => new Date(b.created_at) - new Date(a.created_at));
  } catch (err) { console.error('Error fetching tagged URLs:', err); }
};
const fetchLatestVersion = async () => {
  try {
    const response = await axios.get('/latest-app-version?format=zip');
    if (response.data && response.data.version) {
      latestVersion.value = response.data.version;
    }
  } catch (err) { console.error('Error fetching latest version:', err); }
};
const handleUpdateServer = async (address) => {
  try {
    await axios.post('/trigger-update-remote', { address });
    ElNotification({ title: 'Success', message: `Update triggered for ${address}`, type: 'success' });
  } catch (err) {
    ElNotification({ title: 'Error', message: err.response?.data?.error || err.message, type: 'error' });
  }
};
const saveTaggedUrl = async () => {
  try {
    await axios.post('/tagged-urls', newTag.value);
    newTag.value = { tag: '', url: '' };
    fetchTaggedURLs();
    ElNotification({ title: 'Success', message: 'Tagged URL saved.', type: 'success' });
  } catch (err) {
    ElNotification({ title: 'Error', message: 'Error saving URL.', type: 'error' });
  }
};
const createProxy = async (url, nodeAddress = '') => {
  creatingProxyUrls.value[url] = true;
  try {
    const response = await axios.post('/create-proxy', { url, node_address: nodeAddress });
    let sharedUrl = response.data.shared_url;

    // Client-side fix for older nodes or partial backend responses:
    // Reconstruct the full shared URL using the host/port from the response
    // and the path/query/hash from the original requested URL.
    try {
      const originalObj = new URL(url);
      const sharedObj = new URL(sharedUrl);

      // We trust the backend for the scheme, host, and port (where the proxy lives)
      // We trust the original request for the path, query, and hash (what the user wants)
      sharedObj.pathname = originalObj.pathname;
      sharedObj.search = originalObj.search;
      sharedObj.hash = originalObj.hash;

      sharedUrl = sharedObj.toString();
    } catch (e) {
      // If parsing fails, fall back to the backend's response
      console.warn("Failed to reconstruct shared URL, using backend response:", e);
    }

    ElNotification({ title: 'Success', message: `Proxy created: ${sharedUrl}`, type: 'success' });
    fetchTaggedURLs(); // Refresh to show new proxy status
    fetchClusterProxies();
  } catch (err) {
    ElNotification({ title: 'Error', message: err.response?.data?.error || err.message, type: 'error' });
  } finally {
    delete creatingProxyUrls.value[url];
  }
};
const toggleDebug = async (url, enable) => {
  try {
    await axios.post('/toggle-debug-proxy', { url, enable });
    ElNotification({ title: 'Success', message: `Debugger ${enable ? 'enabled' : 'disabled'}`, type: 'success' });
    // Refresh to show new status
    fetchTaggedURLs();
    fetchClusterProxies();
  } catch (err) {
    ElNotification({ title: 'Error', message: 'Failed to toggle debugger', type: 'error' });
  }
};

const toggleCaptcha = async (url, enable) => {
  try {
    await axios.post('/toggle-captcha-proxy', { url, enable });
    ElNotification({ title: 'Success', message: `Auto captcha ${enable ? 'enabled' : 'disabled'}`, type: 'success' });
    // Refresh to show new status
    fetchTaggedURLs();
    fetchClusterProxies();
  } catch (err) {
    ElNotification({ title: 'Error', message: 'Failed to toggle debugger', type: 'error' });
  }
};
const renameTag = async (id, newTagValue) => {
  try {
    await axios.put(`/tagged-urls/${id}`, { tag: newTagValue });
    fetchTaggedURLs();
    ElNotification({ title: 'Success', message: 'Tag renamed.', type: 'success' });
  } catch (err) {
    ElNotification({ title: 'Error', message: err.response?.data?.error || 'Error renaming tag.', type: 'error' });
  }
};
const deleteTag = async (id) => {
  try {
    await axios.delete(`/tagged-urls/${id}`);
    fetchTaggedURLs();
    ElNotification({ title: 'Success', message: 'Tag deleted.', type: 'success' });
  } catch (err) {
    ElNotification({ title: 'Error', message: err.response?.data?.error || 'Error deleting tag.', type: 'error' });
  }
};
let pollingStarted = false;
const startPolling = () => {
  if (pollingStarted) return;
  pollingStarted = true;

  const pollServers = () => {
    if (!isAuthenticated.value || mustChangePassword.value) { pollingStarted = false; return; }
    fetchServers().finally(() => setTimeout(pollServers, 5000));
  };
  const pollTaggedURLs = () => {
    if (!isAuthenticated.value || mustChangePassword.value) { pollingStarted = false; return; }
    fetchTaggedURLs().finally(() => setTimeout(pollTaggedURLs, 5000));
  };
  const pollClusterProxies = () => {
    if (!isAuthenticated.value || mustChangePassword.value) { pollingStarted = false; return; }
    fetchClusterProxies().finally(() => setTimeout(pollClusterProxies, 5000));
  };

  pollServers();
  pollTaggedURLs();
  pollClusterProxies();
};

onMounted(async () => {
  const authed = await checkAuth();
  if (authed && !mustChangePassword.value) {
    fetchLatestVersion();
    startPolling();
  }
});
</script>

<style>
html,
body {
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
  background-color: #f4f7f6;
  margin: 0;
  padding: 0;
  min-height: 100vh;
  box-sizing: border-box;
}

*,
*::before,
*::after {
  box-sizing: border-box;
}
</style>

<style scoped>
.container {
  max-width: 1400px;
  width: 100%;
  min-height: 100vh;
  background-color: #ffffff;
  padding: 1rem;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.05);
  display: flex;
  flex-direction: column;
  margin: 0 auto;
  /* Center naturally */
}

@media (min-width: 769px) {
  .container {
    width: calc(100% - 2rem);
    min-height: auto;
    /* Allow content to dictate height, but keep min for look */
    margin: 1rem auto;
    border-radius: 8px;
  }
}

.header-section {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1.5rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid #eef2f5;
}

.app-title {
  color: #2c3e50;
  text-align: left;
  margin: 0;
  font-size: 1.5rem;
  font-weight: 700;
}

.user-info {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 0.9rem;
  color: #606266;
}

.login-wrapper {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 3rem 1rem;
  min-height: 50vh;
}

.login-card {
  width: 100%;
  max-width: 420px;
  border-radius: 12px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08) !important;
  border: 1px solid #e4e7ed;
}

.card-header h3 {
  margin: 0;
  font-size: 1.25rem;
  color: #303133;
  text-align: center;
}

.login-btn-wrapper {
  margin-top: 2rem;
}

.login-btn {
  width: 100%;
  padding: 12px 0;
  font-size: 1rem;
  font-weight: 600;
  border-radius: 6px;
}

.main-grid {
  display: grid;
  gap: 1rem;
  /* Removed overflow and flex-grow */
}

.main-grid.single-column {
  grid-template-columns: 1fr;
}

@media (min-width: 769px) {
  .main-grid:not(.single-column) {
    grid-template-columns: 1.2fr 1fr;
  }
}

.left-column {
  display: grid;
  grid-template-rows: 1fr auto;
  gap: 1rem;
  height: 100%;
  min-height: 0;
}

.tagged-list-wrapper {
  min-height: 0;
  /* Critical for scrolling inside grid item */
  /* No flex needed here anymore */
}

.toggle-section {
  text-align: center;
  /* margin-top is handled by gap in grid */
  margin-top: 0;
}

@media (max-width: 768px) {
  .container {
    width: 100%;
    margin: 0;
    border-radius: 0;
    box-shadow: none;
    padding: 0.5rem;
  }

  .main-grid {
    grid-template-columns: 1fr;
    display: flex;
    flex-direction: column;
  }
}
</style>