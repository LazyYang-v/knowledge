import assert from 'node:assert/strict'
import test from 'node:test'

import {
  buildSettingsRouteQuery,
  integrationSectionKey,
  isIntegrationSection,
  normalizeSettingsSection,
  settingsQueryUnchanged,
} from './settingsRoute'

test('every settings nav item writes only section', () => {
  assert.deepEqual(
    buildSettingsRouteQuery('models', {
      section: 'system-global',
      tab: 'im',
      agentId: 'agt_1',
    }),
    { section: 'models' },
  )
  assert.deepEqual(
    buildSettingsRouteQuery('general', { section: 'system-global' }),
    { section: 'general' },
  )
  assert.deepEqual(
    buildSettingsRouteQuery('runtime-queues', { section: 'system-global' }),
    { section: 'runtime-queues' },
  )
  assert.deepEqual(
    buildSettingsRouteQuery(integrationSectionKey('mcpserver'), {
      section: 'integrations',
      tab: 'im',
    }),
    { section: 'integration-mcpserver' },
  )
})

test('legacy integrations / bare-tab query strings normalize to nav keys', () => {
  assert.equal(normalizeSettingsSection('mcpserver'), 'integration-mcpserver')
  // 'mcp' is the MCP-client settings page, not the MCP server integration tab.
  assert.equal(normalizeSettingsSection('mcp'), 'mcp')
  assert.equal(normalizeSettingsSection('integrations', 'mcpserver'), 'integration-mcpserver')
  assert.equal(normalizeSettingsSection('integrations', 'embed'), 'integration-im')
  assert.equal(normalizeSettingsSection('integrations'), 'integration-im')
  assert.equal(normalizeSettingsSection('system-global'), 'system-global')
  assert.equal(isIntegrationSection('integration-mcpserver'), true)
  assert.equal(isIntegrationSection('models'), false)
  assert.equal(isIntegrationSection('integration-unknown'), false)
})

test('canonical settings query skips a redundant replace', () => {
  assert.equal(
    settingsQueryUnchanged(
      { section: 'integration-mcpserver' },
      { section: 'integration-mcpserver' },
    ),
    true,
  )
  assert.equal(
    settingsQueryUnchanged(
      { section: 'integrations', tab: 'mcpserver' },
      { section: 'integration-mcpserver' },
    ),
    false,
  )
})
