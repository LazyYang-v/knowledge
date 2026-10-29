<template>
  <div class="integrations-settings">
    <div class="integrations-settings__body">
      <div v-if="tab === 'im'" class="section">
        <div class="section-header">
          <h2>{{ $t('agentEditor.im.title') }}</h2>
          <p class="section-description">
            {{ $t('agentEditor.im.description') }}
          </p>
        </div>
        <IMChannelPanel v-model:filter-agent-id="filterAgentId" />
      </div>

      <div v-if="tab === 'mcpserver'" class="section">
        <div class="section-header">
          <h2>{{ $t('integrations.mcpserver.title') }}</h2>
          <p class="section-description">{{ $t('integrations.mcpserver.subtitle') }}</p>
        </div>
        <McpServerIntegrationSettings />
      </div>

    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import IMChannelPanel from '@/components/IMChannelPanel.vue'
import McpServerIntegrationSettings from '@/views/integrations/McpServerIntegrationSettings.vue'
import type { IntegrationTab } from '@/config/integrations'

const filterAgentId = ref('')

defineProps<{
  tab: IntegrationTab
}>()

const route = useRoute()

function applyAgentFilterFromRoute() {
  filterAgentId.value = (route.query.agentId as string) || ''
}

watch(
  () => route.query.agentId,
  applyAgentFilterFromRoute,
  { immediate: true },
)
</script>

<style scoped lang="less">
@import (reference) '@/components/css/settings-section.less';

.integrations-settings {
  display: flex;
  flex-direction: column;
}

.integrations-settings__body {
  min-width: 0;
}

.section-header {
  .settings-section-header();
}

.doc-link {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  margin-left: 6px;
  color: var(--td-brand-color);
  text-decoration: none;

  &:hover {
    text-decoration: underline;
  }
}

.link-icon {
  font-size: var(--app-text-md);
}
</style>
