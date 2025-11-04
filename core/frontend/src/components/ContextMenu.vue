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
