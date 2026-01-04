<template>
  <el-dialog v-model="visible" title="Proxy Settings" width="500px">
    <el-form :model="form" label-width="180px">
      <el-form-item label="Internal URL Rewrite">
        <el-switch v-model="form.enable_url_rewrite" />
        <div class="help-text">Rewrites internal IPs (10.x, 192.168.x) to proxied URLs.</div>
      </el-form-item>

      <el-form-item label="Content Modification">
        <el-switch v-model="form.enable_content_mod" />
        <div class="help-text">Enables system-specific fixes (e.g. Legacy JS fixes, Captcha injection).</div>
      </el-form-item>

      <el-divider content-position="left">Legacy / Overrides</el-divider>

      <el-form-item label="Debug Script">
        <el-switch v-model="form.enable_debug" />
        <div class="help-text">Injects debug overlay (Legacy).</div>
      </el-form-item>

      <el-form-item label="Auto Captcha">
        <el-switch v-model="form.enable_captcha" />
        <div class="help-text">Legacy toggle for captcha solver.</div>
      </el-form-item>
      
      <el-divider v-if="activeSystems.length > 0" content-position="left">Detected Systems</el-divider>
      <div v-if="activeSystems.length > 0">
        <el-tag v-for="sys in activeSystems" :key="sys" type="success" style="margin-right: 5px">{{ sys }}</el-tag>
      </div>
    </el-form>
    
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="visible = false">Cancel</el-button>
        <el-button type="primary" @click="save">Save</el-button>
      </span>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, watch, defineProps, defineEmits } from 'vue';

const props = defineProps({
  modelValue: Boolean,
  proxyData: Object, // The proxy object containing settings
});

const emit = defineEmits(['update:modelValue', 'save']);

const visible = ref(false);
const form = ref({
  enable_url_rewrite: true,
  enable_content_mod: true,
  enable_debug: false,
  enable_captcha: false,
});
const activeSystems = ref([]);

watch(() => props.modelValue, (val) => {
  visible.value = val;
  if (val && props.proxyData) {
    // Initialize form from proxy data
    const s = props.proxyData.settings || {};
    form.value = {
      enable_url_rewrite: s.enable_url_rewrite !== undefined ? s.enable_url_rewrite : true,
      enable_content_mod: s.enable_content_mod !== undefined ? s.enable_content_mod : true,
      // Fallback to legacy fields if settings object is missing (older client)
      // or if we want to show legacy state
      enable_debug: props.proxyData.enable_debug,
      enable_captcha: props.proxyData.enable_captcha,
    };
    activeSystems.value = props.proxyData.active_systems || [];
  }
});

watch(visible, (val) => {
  emit('update:modelValue', val);
});

const save = () => {
  emit('save', {
    url: props.proxyData.original_url || props.proxyData.url, // Handle different naming conventions if any
    settings: {
        enable_url_rewrite: form.value.enable_url_rewrite,
        enable_content_mod: form.value.enable_content_mod,
    },
    legacy: {
        enable_debug: form.value.enable_debug,
        enable_captcha: form.value.enable_captcha,
    }
  });
  visible.value = false;
};
</script>

<style scoped>
.help-text {
  font-size: 12px;
  color: #888;
  line-height: 1.2;
}
</style>
