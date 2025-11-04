<template>
  <div
    v-if="menuData.visible"
    class="context-menu"
    :style="{ top: menuData.y + 'px', left: menuData.x + 'px' }"
    @click.stop
  >
    <ul>
      <li @click="$emit('select-for-compare')">Select for Compare</li>
      <li @click="$emit('compare-with-selected')" :class="{ disabled: !isCompareEnabled }">Compare with Selected</li>
      <li @click="$emit('share-request')">Open in New Tab</li>
      <li v-if="isDeleteEnabled" class="delete-option" @click="$emit('delete-request')">Delete Request</li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { defineProps, defineEmits } from 'vue';
import type { CapturedRequest } from '../types';

interface MenuData {
  visible: boolean;
  x: number;
  y: number;
  request: CapturedRequest | null;
}

defineProps<{
  menuData: MenuData;
  isCompareEnabled: boolean;
  isDeleteEnabled: boolean;
}>();

defineEmits<{
  (e: 'select-for-compare'): void;
  (e: 'compare-with-selected'): void;
  (e: 'share-request'): void;
  (e: 'delete-request'): void;
}>();
</script>

<style scoped>
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

.delete-option {
  color: #dc3545;
}

.delete-option:hover {
  background-color: #f8d7da !important;
}
</style>
