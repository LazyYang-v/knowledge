<template>
  <div class="login-layout">
    <!-- Header: language switch only -->
    <div class="header-actions">
      <div class="language-switch">
        <button @click="toggleLanguageMenu" class="lang-button" :title="currentLangOption?.label">
          <span class="lang-flag-icon">{{ currentLangOption?.flag }}</span>
          <span class="link-text">{{ currentLangOption?.shortLabel }}</span>
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"
            stroke-linecap="round">
            <polyline points="6 9 12 15 18 9" />
          </svg>
        </button>

        <!-- Language Dropdown -->
        <div v-if="showLanguageMenu" class="language-dropdown">
          <div v-for="lang in languageOptions" :key="lang.value" @click="selectLanguage(lang.value)"
            class="language-option" :class="{ active: currentLanguage === lang.value }">
            <span class="lang-flag">{{ lang.flag }}</span>
            <span class="lang-label">{{ lang.label }}</span>
            <span v-if="currentLanguage === lang.value" class="check-icon">✓</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Centered Auth Card -->
    <div class="form-section">
      <div class="form-panel">
        <!-- Login Card -->
        <div class="form-card" v-if="!isRegisterMode">
          <!-- 登录已有账号时保留邀请上下文。 -->
          <div v-if="inviteLookup" class="invite-banner">
            <t-icon name="link" class="invite-banner__icon" />
            <div class="invite-banner__text">
              <div class="invite-banner__title">
                {{ $t('inviteRegister.bannerTitle', { tenant: inviteLookup.tenant_name || '' }) }}
              </div>
              <div class="invite-banner__hint">
                {{ $t('inviteRegister.bannerHintLogin') }}
              </div>
            </div>
          </div>
          <div v-else-if="inviteLookupError" class="invite-banner invite-banner--error">
            {{ inviteLookupError }}
          </div>
          <div class="form-header">
            <h2 class="form-title">{{ $t('auth.welcomeTitle') }}</h2>
            <p class="form-welcome">{{ $t('auth.welcomeSubtitle') }}</p>
            <p v-if="registrationEnabled" class="form-hint">{{ $t('auth.loginHint') }}</p>
          </div>

          <div class="form-content">
            <t-form ref="formRef" :data="formData" :rules="formRules" @submit="handleLogin" layout="vertical"
              label-align="top">
              <t-form-item :label="$t('auth.email')" name="email">
                <t-input v-model="formData.email" :placeholder="$t('auth.emailPlaceholder')" type="text"
                  autocomplete="email" size="large" :disabled="loading" clearable />
              </t-form-item>

              <t-form-item :label="$t('auth.password')" name="password">
                <t-input v-model="formData.password" :placeholder="$t('auth.passwordPlaceholder')" type="password"
                  autocomplete="current-password" size="large" :disabled="loading" @enter="handleLogin">
                  <template #suffixIcon>
                    <t-icon :name="showPassword ? 'browse' : 'browse-off'" class="password-toggle"
                      @click="showPassword = !showPassword" />
                  </template>
                </t-input>
              </t-form-item>

              <t-button type="submit" theme="primary" size="large" block :loading="loading" class="submit-button">
                {{ loading ? $t('auth.loggingIn') : $t('auth.login') }}
              </t-button>

              <div class="register-cta" v-if="registrationEnabled">
                <t-button theme="default" variant="text" size="large" block class="register-cta__button"
                  :disabled="loading" @click="toggleMode">
                  {{ $t('auth.createAccount') }}
                </t-button>
              </div>

              <div v-if="oidcEnabled" class="oidc-divider">
                <span>{{ $t('auth.orContinueWith') }}</span>
              </div>

              <t-button v-if="oidcEnabled" theme="default" size="large" block :loading="oidcLoading" :disabled="loading"
                class="oidc-button" @click="handleOIDCLogin">
                {{ oidcLoading ? $t('auth.redirectingToOIDC') : oidcLoginText }}
              </t-button>
            </t-form>
          </div>
        </div>

        <!-- Registration must be allowed for this mode and invitation. -->
        <div class="form-card" v-if="isRegisterMode && registrationEnabled">
          <!-- Share-link banner: shown only when ?token= resolved to a
               real invitation row. Sits above the form header so the
               invitee instantly sees who invited them and into which
               workspace, without bumping the existing register UX. -->
          <div v-if="inviteLookup" class="invite-banner">
            <t-icon name="link" class="invite-banner__icon" />
            <div class="invite-banner__text">
              <div class="invite-banner__title">
                {{ $t('inviteRegister.bannerTitle', { tenant: inviteLookup.tenant_name || '' }) }}
              </div>
              <div class="invite-banner__hint">
                {{ $t('inviteRegister.bannerHint') }}
              </div>
            </div>
          </div>
          <div v-else-if="inviteLookupError" class="invite-banner invite-banner--error">
            {{ inviteLookupError }}
          </div>
          <div class="form-header">
            <h2 class="form-title">{{ $t('auth.createAccount') }}</h2>
            <p class="form-subtitle">{{ $t('auth.registerSubtitle') }}</p>
          </div>

          <div class="form-content">
            <t-form ref="registerFormRef" :data="registerData" :rules="registerRules" @submit="handleRegister"
              layout="vertical" label-align="top">
              <t-form-item :label="$t('auth.username')" name="username">
                <t-input v-model="registerData.username" :placeholder="$t('auth.usernamePlaceholder')" size="large"
                  :disabled="loading" clearable />
              </t-form-item>

              <t-form-item :label="$t('auth.email')" name="email">
                <t-input v-model="registerData.email" :placeholder="$t('auth.emailPlaceholder')" type="text"
                  autocomplete="email" size="large" :disabled="loading" clearable />
              </t-form-item>

              <t-form-item :label="$t('auth.password')" name="password">
                <t-input v-model="registerData.password" :placeholder="$t('auth.passwordPlaceholder')" type="password"
                  autocomplete="new-password" size="large" :disabled="loading">
                  <template #suffixIcon>
                    <t-icon :name="showPassword ? 'browse' : 'browse-off'" class="password-toggle"
                      @click="showPassword = !showPassword" />
                  </template>
                </t-input>
              </t-form-item>

              <t-form-item :label="$t('auth.confirmPassword')" name="confirmPassword">
                <t-input v-model="registerData.confirmPassword" :placeholder="$t('auth.confirmPasswordPlaceholder')"
                  type="password" autocomplete="new-password" size="large" :disabled="loading"
                  @enter="handleRegister">
                  <template #suffixIcon>
                    <t-icon :name="showPassword ? 'browse' : 'browse-off'" class="password-toggle"
                      @click="showPassword = !showPassword" />
                  </template>
                </t-input>
              </t-form-item>

              <t-button type="submit" theme="primary" size="large" block :loading="loading" class="submit-button">
                {{ loading ? $t('auth.registering') : $t('auth.register') }}
              </t-button>
            </t-form>

            <div class="form-footer">
              <span>{{ $t('auth.haveAccount') }}</span>
              <a href="#" @click.prevent="toggleMode" class="link-button">
                {{ $t('auth.backToLogin') }}
              </a>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, nextTick, onMounted, onBeforeUnmount, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { useRoleLabel } from '@/composables/useRoleLabel'
import { notifyLoginSuccess } from '@/utils/loginNotify'
import { canRegister } from './registrationPolicy'
import { newPasswordRules } from '@/utils/passwordPolicy'
import {
  login,
  register,
  getOIDCAuthorizationURL,
  getOIDCConfig,
  autoSetup,
  getAuthConfig,
  userInfoFromApi,
  getInvitationByToken,
  registerByInvite,
  type InviteLookup,
} from '@/api/auth'
import { useAuthStore } from '@/stores/auth'
import { useI18n } from 'vue-i18n'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const { t, tm, locale } = useI18n()
const { formatRole, roleIcon } = useRoleLabel()

// Form references
const formRef = ref()
const registerFormRef = ref()

// State management
const loading = ref(false)
const oidcLoading = ref(false)
const isRegisterMode = ref(false)
const showLanguageMenu = ref(false)
const showPassword = ref(false)
const oidcEnabled = ref(false)
const oidcProviderName = ref('')
// Wait for configuration before showing a registration entry.
const registrationMode = ref('')
const registrationEnabled = computed(() => canRegister(registrationMode.value, !!inviteLookup.value))
const complexPasswordEnabled = ref(false)

// invite-link state. When the URL carries ?token=xxx we resolve it to
// the originating tenant + role and switch the form into a "register
// via invitation" mode when invitation registration is enabled. Submitting the register
// form with this set hits /auth/register-by-invite (auto-login on
// success) instead of /auth/register.
const inviteToken = ref('')
const inviteLookup = ref<InviteLookup | null>(null)
const inviteLookupError = ref('')
const inviteLookupLoading = ref(false)

// Language options
const languageOptions = [
  { value: 'zh-CN', label: '简体中文', shortLabel: '中文', flag: '🇨🇳' },
  { value: 'zh-TW', label: '繁體中文', shortLabel: '繁中', flag: '繁' },
  { value: 'en-US', label: 'English', shortLabel: 'EN', flag: '🇺🇸' },
  { value: 'ru-RU', label: 'Русский', shortLabel: 'RU', flag: '🇷🇺' },
  { value: 'ko-KR', label: '한국어', shortLabel: '한국어', flag: '🇰🇷' },
  { value: 'ja-JP', label: '日本語', shortLabel: '日本語', flag: '🇯🇵' }
]

const currentLanguage = computed(() => locale.value)
const oidcLoginText = computed(() => {
  if (oidcProviderName.value) {
    return t('auth.oidcLoginWithProvider', { provider: oidcProviderName.value })
  }
  return t('auth.oidcLogin')
})
const currentLangOption = computed(() => languageOptions.find(l => l.value === currentLanguage.value))

// Login form data
const formData = reactive<{ [key: string]: any }>({
  email: '',
  password: '',
})

// Register form data
const registerData = reactive<{ [key: string]: any }>({
  username: '',
  email: '',
  password: '',
  confirmPassword: ''
})

// Login form validation rules
const formRules = computed(() => ({
  email: [
    { required: true, message: t('auth.emailRequired'), type: 'error' },
    { email: true, message: t('auth.emailInvalid'), type: 'error' }
  ],
  password: [
    { required: true, message: t('auth.passwordRequired'), type: 'error' },
    { min: 8, message: t('auth.passwordMinLength'), type: 'error' },
    { max: 32, message: t('auth.passwordMaxLength'), type: 'error' }
  ],
}))

// Register form validation rules
const registerRules = computed(() => ({
  username: [
    { required: true, message: t('auth.usernameRequired'), type: 'error' },
    { min: 2, message: t('auth.usernameMinLength'), type: 'error' },
    { max: 20, message: t('auth.usernameMaxLength'), type: 'error' },
    {
      pattern: /^[a-zA-Z0-9_\u4e00-\u9fa5]+$/,
      message: t('auth.usernameInvalid'),
      type: 'error'
    }
  ],
  email: [
    { required: true, message: t('auth.emailRequired'), type: 'error' },
    { email: true, message: t('auth.emailInvalid'), type: 'error' }
  ],
  password: newPasswordRules(t, complexPasswordEnabled.value),
  confirmPassword: [
    { required: true, message: t('auth.confirmPasswordRequired'), type: 'error' },
    {
      validator: (val: string) => val === registerData.password,
      message: t('auth.passwordMismatch'),
      type: 'error'
    }
  ]
}))

// Toggle login/register mode
const toggleMode = () => {
  isRegisterMode.value = !isRegisterMode.value && registrationEnabled.value

  Object.keys(registerData).forEach(key => {
    (registerData as any)[key] = ''
  })
}

// Toggle language menu
const toggleLanguageMenu = () => {
  showLanguageMenu.value = !showLanguageMenu.value
}

// Select language
const selectLanguage = (lang: string) => {
  locale.value = lang
  localStorage.setItem('locale', lang)
  showLanguageMenu.value = false
  MessagePlugin.success(t('language.languageSaved'))
}

// Close language menu when clicking outside
const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  if (!target.closest('.language-switch')) {
    showLanguageMenu.value = false
  }
}

// Add click outside listener
onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})

const persistLoginResponse = async (response: any, skipRedirect = false) => {
  // Backend renamed `tenant` to `active_tenant` and added `memberships`
  // when tenant-level RBAC landed (issue #1303). The two are otherwise
  // identical — `active_tenant` is the tenant whose ID is encoded in the
  // JWT, defaulting to the user's home tenant on a fresh login.
  const activeTenant = response.active_tenant || response.tenant
  if (response.user && response.token) {
    // user.tenant_id must be the user's HOME tenant (the immutable row
    // on the users table); useHomeTenant() and the home-badge logic both
    // assume so. The ACTIVE tenant (which can differ from home when the
    // server honoured a remembered last-active-tenant preference) is
    // expressed separately via setSelectedTenant below.
    const homeTenantIdRaw = response.user.tenant_id ?? activeTenant?.id ?? ''
    authStore.setUser(userInfoFromApi(response.user, homeTenantIdRaw))
    authStore.setToken(response.token)
    if (response.refresh_token) {
      authStore.setRefreshToken(response.refresh_token)
    }
    if (activeTenant) {
      authStore.setTenant({
        id: String(activeTenant.id) || '',
        name: activeTenant.name || '',
        owner_id: response.user.id || '',
        created_at: activeTenant.created_at || new Date().toISOString(),
        updated_at: activeTenant.updated_at || new Date().toISOString()
      })
    } else {
      authStore.setTenant(null)
    }
    if (Array.isArray(response.memberships)) {
      authStore.setMemberships(response.memberships)
    }
    // If the backend dropped us into a non-home tenant (honoured a
    // remembered "last active tenant" preference), set the override so
    // subsequent requests carry X-Tenant-ID and the UI stays consistent.
    // Otherwise clear any stale override left in localStorage by a
    // previous session for a different account.
    const activeIdNum = Number(activeTenant?.id)
    const homeIdNum = Number(homeTenantIdRaw)
    if (Number.isFinite(activeIdNum) && Number.isFinite(homeIdNum) && activeIdNum !== homeIdNum) {
      authStore.setSelectedTenant(activeIdNum, activeTenant?.name || null)
    } else {
      authStore.setSelectedTenant(null, null)
    }
  }

  // Pull runtime capabilities (including whether ordinary users may create
  // workspaces) before entering the main UI so create actions never flash
  // briefly when the deployment is invitation-only.
  await authStore.refreshFromAuthMe()
  await nextTick()
  if (skipRedirect) return
  router.replace(authStore.hasValidTenant ? '/platform/knowledge-bases' : '/onboarding/workspace')
}

const getBackendOIDCRedirectURI = () => `${window.location.origin}/api/v1/auth/oidc/callback`

const loadOIDCConfig = async () => {
  try {
    const response = await getOIDCConfig()
    oidcEnabled.value = !!response.success && !!response.enabled
    oidcProviderName.value = response.provider_display_name || ''
  } catch {
    oidcEnabled.value = false
    oidcProviderName.value = ''
  }
}

// Registration stays hidden if configuration cannot be loaded.
const loadAuthConfig = async () => {
  try {
    const response = await getAuthConfig()
    registrationMode.value = response.success ? response.registration_mode : ''
    complexPasswordEnabled.value = response.complex_password_enabled
  } catch {
    registrationMode.value = ''
    complexPasswordEnabled.value = false
  }
}

const handleOIDCLogin = async () => {
  try {
    oidcLoading.value = true
    const response = await getOIDCAuthorizationURL(getBackendOIDCRedirectURI())
    const authorizationURL = response.authorization_url

    if (!response.success || !authorizationURL) {
      MessagePlugin.error(response.message || t('auth.oidcLoginFailed'))
      return
    }

    // 跳转 IdP 会丢失 URL 中的 token，暂存到 sessionStorage，回调后由 App.vue 兑换。
    if (inviteToken.value) {
      sessionStorage.setItem('weknora_pending_invite_token', inviteToken.value)
    }
    window.location.href = authorizationURL
  } catch (error: any) {
    console.error('OIDC 登录跳转失败:', error)
    MessagePlugin.error(error.message || t('auth.oidcLoginFailed'))
  } finally {
    oidcLoading.value = false
  }
}

// 用 token 加入空间并进入应用。会话此时已有效，故即便 token 失效也照常进入（避免困在登录页）。
const acceptAndEnter = async (token: string) => {
  loading.value = true
  try {
    const result = await authStore.acceptInvitationByTokenAndRefresh(token)
    if (result.ok) {
      MessagePlugin.success(t('inviteRegister.joined'))
    } else {
      MessagePlugin.warning(t('inviteRegister.invalidBody'))
    }
  } catch {
    MessagePlugin.warning(t('inviteRegister.invalidBody'))
  } finally {
    loading.value = false
    await nextTick()
    router.replace('/platform/knowledge-bases')
  }
}

// Handle login
const handleLogin = async () => {
  try {
    const valid = await formRef.value?.validate()
    if (valid !== true) return

    loading.value = true

    const response = await login({
      email: formData.email,
      password: formData.password,
    })

    if (response.success) {
      if (inviteToken.value) {
        // 从邀请链接登录：持久化会话后兑换 token 并进入对应空间。
        await persistLoginResponse(response, true)
        await acceptAndEnter(inviteToken.value)
        return
      }
      await persistLoginResponse(response)
      notifyLoginSuccess(response, t, tm, formatRole, roleIcon)
    } else {
      MessagePlugin.error(response.message || t('auth.loginError'))
    }
  } catch (error: any) {
    console.error('登录错误:', error)
    MessagePlugin.error(error.message || t('auth.loginErrorRetry'))
  } finally {
    loading.value = false
  }
}

// Handle registration. Dispatches based on whether the user arrived
// with a share-link token: with token -> register-by-invite (auto-
// login on success); without -> the normal self-service register
// (drops back to the login form for the user to sign in).
const handleRegister = async () => {
  if (!registrationEnabled.value) return
  try {
    const valid = await registerFormRef.value?.validate()
    if (valid !== true) return

    loading.value = true

    if (inviteToken.value) {
      const response = await registerByInvite({
        token: inviteToken.value,
        username: registerData.username,
        email: registerData.email,
        password: registerData.password,
      })
      if (!response.success) {
        MessagePlugin.error(response.message || t('auth.registerFailed'))
        return
      }
      MessagePlugin.success(t('auth.registerSuccess'))
      // register-by-invite returns the same shape as login (token +
      // active_tenant + memberships), so reuse the login persistence
      // path — same store writes, same redirect target.
      await persistLoginResponse(response)
      return
    }

    const response = await register({
      username: registerData.username,
      email: registerData.email,
      password: registerData.password
    })

    if (response.success) {
      MessagePlugin.success(t('auth.registerSuccess'))

      // Switch to login mode and fill in email
      isRegisterMode.value = false
      formData.email = registerData.email

      // Clear register form
      Object.keys(registerData).forEach(key => {
        (registerData as any)[key] = ''
      })
    } else {
      MessagePlugin.error(response.message || t('auth.registerFailed'))
    }
  } catch (error: any) {
    console.error('注册错误:', error)
    MessagePlugin.error(error.message || t('auth.registerError'))
  } finally {
    loading.value = false
  }
}

// Check if already logged in; for lite edition, attempt transparent auto-setup
onMounted(async () => {
  // Share-link landing: ?token=xxx switches the form into invite-
  // register mode before any other auto-flow (logged-in redirect /
  // auto-setup / OIDC) gets a chance to redirect. Resolution failure
  // surfaces inline; the user can still log in normally if they
  // already have an account. We check this BEFORE the isLoggedIn
  // redirect so an existing session doesn't bounce the user to
  // /platform (and possibly back to /login if the session is stale),
  // dropping the invite token along the way.
  const tokenFromQuery = String(route.query.token || '').trim()
  if (tokenFromQuery) {
    inviteToken.value = tokenFromQuery
    inviteLookupLoading.value = true
    // 1. 先校验 token：无效/过期则停在登录页报错，不进注册模式。
    try {
      const resp = await getInvitationByToken(tokenFromQuery)
      if (resp.success && resp.data) {
        inviteLookup.value = resp.data
      } else {
        inviteLookupError.value = resp.message || t('inviteRegister.invalidBody')
        loadOIDCConfig()
        loadAuthConfig()
        return
      }
    } catch {
      inviteLookupError.value = t('inviteRegister.invalidBody')
      loadOIDCConfig()
      loadAuthConfig()
      return
    } finally {
      inviteLookupLoading.value = false
    }

    // 2. 已登录则直接兑换 token 进入空间（三种模式通用）。
    if (authStore.isLoggedIn && (await authStore.refreshFromAuthMe())) {
      await acceptAndEnter(tokenFromQuery)
      return
    }

    // 3. Only enabled registration modes may create an invited account.
    await loadAuthConfig()
    isRegisterMode.value = registrationEnabled.value
    loadOIDCConfig()
    return
  }

  if (authStore.isLoggedIn) {
    router.replace('/platform/knowledge-bases')
    return
  }

  localStorage.removeItem('weknora_auto_setup_failed')
  try {
    const response = await autoSetup()
    if (response.success) {
      authStore.setLiteMode(true)
      await persistLoginResponse(response)
      return
    }
  } catch {
    // Auto-setup may be unavailable outside the native Lite shell.
  }

  loadOIDCConfig()
  loadAuthConfig()
})
</script>

<style lang="less" scoped>
// 登录/注册页独立于应用的绿色品牌主题：在页面作用域内把 TDesign 品牌与
// 文本 token 固定为蓝色企业风（#2563EB）+ 浅色文字，避免暗色模式把白卡片
// 上的文字变成不可读的浅色。CSS 自定义属性会自然继承到卡片内所有子组件。
.login-layout {
  --td-brand-color: #2563eb;
  --td-brand-color-hover: #1d4ed8;
  --td-brand-color-focus: #1d4ed8;
  --td-brand-color-active: #1e40af;
  --td-brand-color-disabled: rgba(37, 99, 235, 0.4);
  --td-brand-color-light: #eff6ff;
  --td-brand-color-light-hover: #dbeafe;
  --td-brand-color-light-active: #bfdbfe;
  --td-text-color-primary: rgba(0, 0, 0, 0.9);
  --td-text-color-secondary: rgba(0, 0, 0, 0.6);
  --td-text-color-placeholder: rgba(0, 0, 0, 0.34);
  --td-text-color-anti: #ffffff;
  --td-text-color-disabled: rgba(0, 0, 0, 0.26);
  --td-component-stroke: #e7e9ec;
  --td-border-level-2-color: #e7e9ec;
  --td-bg-color-secondarycontainer: #f3f4f6;
  --td-bg-color-container-hover: #f3f5f9;
  --td-error-color: #d54941;
  --td-error-color-1: #fff0ed;
  --td-error-color-2: #ffd6d2;
  --td-error-color-3: #f78a76;

  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 100%;
  min-height: 100%;
  overflow-x: hidden;
  position: relative;
  background: #F5F7FA;
  padding: 24px;
  box-sizing: border-box;
}

.header-actions {
  position: absolute;
  top: 20px;
  right: 24px;
  display: flex;
  align-items: center;
  z-index: 100;
}

.lang-button {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px;
  border-radius: 8px;
  background: transparent;
  border: 1px solid var(--td-component-stroke);
  color: var(--td-text-color-secondary);
  text-decoration: none;
  font-size: var(--app-text-md);
  font-weight: 500;
  font-family: var(--app-font-family);
  cursor: pointer;
  transition: all var(--app-motion-base);

  svg {
    flex-shrink: 0;
  }

  .link-text {
    line-height: 1;
  }

  .lang-flag-icon {
    font-size: var(--app-text-lg);
    line-height: 1;
    flex-shrink: 0;
  }

  &:hover {
    border-color: var(--td-brand-color);
    color: var(--td-brand-color);
    background: #fff;
  }
}

.language-switch {
  position: relative;
}

.language-dropdown {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  min-width: 160px;
  background: #fff;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-md);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
  overflow: hidden;
  z-index: 1000;
}

.language-option {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  cursor: pointer;
  font-size: var(--app-text-md);
  font-family: var(--app-font-family);
  color: var(--td-text-color-primary);

  .lang-flag {
    font-size: var(--app-text-xl);
    flex-shrink: 0;
  }

  .lang-label {
    flex: 1;
  }

  .check-icon {
    color: var(--td-brand-color);
    font-weight: 700;
    font-size: var(--app-text-base);
    flex-shrink: 0;
  }

  &:hover {
    background: var(--td-bg-color-secondarycontainer);
  }

  &.active {
    background: #EFF6FF;
    color: var(--td-brand-color);
  }
}

.form-section {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  min-height: 100%;
  position: relative;
}

.form-panel {
  width: 100%;
  max-width: 420px;
  position: relative;
  z-index: 2;
}

.form-card {
  background: #fff;
  border-radius: 12px;
  padding: 40px;
  box-shadow: 0 1px 3px rgba(16, 24, 40, 0.06), 0 8px 24px rgba(16, 24, 40, 0.08);
  box-sizing: border-box;
  border: 1px solid rgba(16, 24, 40, 0.06);
  width: 100%;
}

/* Share-link invitation banner. Sits above the register form when the
 * user arrived via /register?token=xxx; gives them confirmation of who
 * invited them before they fill anything in. Subtle, neutral card —
 * we lean on the form's own surface tokens. */
.invite-banner {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  margin-bottom: 20px;
  border-radius: var(--app-radius-lg);
  background: var(--td-bg-color-container-hover);
  border: 1px solid var(--td-component-stroke);
  color: var(--td-text-color-primary);
}

.invite-banner__icon {
  margin-top: 2px;
  font-size: var(--app-text-2xl);
  flex-shrink: 0;
  color: var(--td-text-color-secondary);
}

.invite-banner__text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.invite-banner__title {
  font-size: var(--app-text-base);
  font-weight: 600;
  line-height: 1.4;
  color: var(--td-text-color-primary);
}

.invite-banner__hint {
  font-size: var(--app-text-sm);
  color: var(--td-text-color-secondary);
  line-height: 1.5;
}

.invite-banner--error {
  background: var(--td-error-color-1);
  border-color: var(--td-error-color-3);
  color: var(--td-error-color);
  font-size: var(--app-text-md);
}

.form-header {
  text-align: center;
  margin-bottom: 32px;
}

.form-title {
  font-size: 24px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  margin: 0 0 8px 0;
  font-family: var(--app-font-family);
}

.form-welcome {
  font-size: var(--app-text-md);
  color: var(--td-text-color-secondary);
  margin: 0;
  font-family: var(--app-font-family);
}

.form-hint {
  margin: 14px 0 0;
  padding: 8px 12px;
  border-radius: var(--app-radius-md);
  background: #EFF6FF;
  color: var(--td-brand-color);
  font-size: 12.5px;
  line-height: 1.5;
  font-family: var(--app-font-family);
}

/* 注册入口：次级文字按钮，保持极简。 */
.register-cta {
  margin-top: 8px;

  &__button {
    height: 40px;
    border-radius: var(--app-radius-md);
    font-size: var(--app-text-md);
    font-weight: 500;
    color: var(--td-brand-color);

    &:hover {
      color: var(--td-brand-color-active);
    }
  }
}

.form-subtitle {
  font-size: var(--app-text-md);
  color: var(--td-text-color-secondary);
  margin: 0;
  font-family: var(--app-font-family);
}

.form-content {
  :deep(.t-form-item__label) {
    font-size: var(--app-text-base);
    color: var(--td-text-color-primary);
    font-weight: 500;
    margin-bottom: 8px;
    font-family: var(--app-font-family);
    display: block;
    text-align: left;
  }

  :deep(.t-input) {
    border: 1px solid var(--td-component-stroke);
    border-radius: var(--app-radius-md);
    background: #fff;
    transition: all var(--app-motion-base);

    &:focus-within {
      border-color: var(--td-brand-color);
      box-shadow: 0 0 0 3px color-mix(in srgb, var(--td-brand-color) 10%, transparent);
    }

    &:hover {
      border-color: var(--td-brand-color);
    }

    .t-input__inner {
      border: none !important;
      box-shadow: none !important;
      outline: none !important;
      background: transparent;
      font-size: var(--app-text-lg);
      font-family: var(--app-font-family);

      &:focus {
        border: none !important;
        box-shadow: none !important;
        outline: none !important;
      }
    }

    .t-input__wrap {
      border: none !important;
      box-shadow: none !important;
    }
  }

  :deep(.t-form-item) {
    margin-bottom: 18px;

    &:last-child {
      margin-bottom: 0;
    }
  }

  :deep(.t-form-item__control) {
    width: 100%;
  }
}

.password-toggle {
  cursor: pointer;
  font-size: 18px;
  color: var(--td-text-color-secondary);

  &:hover {
    color: var(--td-brand-color);
  }
}

.submit-button {
  height: 46px;
  border-radius: var(--app-radius-md);
  font-size: var(--app-text-lg);
  font-weight: 500;
  font-family: var(--app-font-family);
  margin: 20px 0 8px 0;
  background: var(--td-brand-color);
  border-color: var(--td-brand-color);
}

.oidc-divider {
  position: relative;
  margin: 16px 0 10px;
  text-align: center;
  color: var(--td-text-color-placeholder);
  font-size: var(--app-text-sm);

  span {
    position: relative;
    z-index: 1;
    padding: 0 12px;
    background: #fff;
  }

  &::before {
    content: '';
    position: absolute;
    left: 0;
    right: 0;
    top: 50%;
    border-top: 1px solid var(--td-component-stroke);
  }
}

.oidc-button {
  height: 46px;
  border-radius: var(--app-radius-md);
  font-size: var(--app-text-lg);
  font-weight: 500;
}

.form-footer {
  text-align: center;
  font-size: var(--app-text-base);
  color: var(--td-text-color-secondary);
  font-family: var(--app-font-family);
  margin-top: 16px;

  .link-button {
    color: var(--td-brand-color);
    text-decoration: none;
    margin-left: 4px;
    font-weight: 500;
    transition: all var(--app-motion-base);

    &:hover {
      color: var(--td-brand-color-active);
      text-decoration: underline;
    }
  }
}

/* Responsive Design */
@media (max-width: 768px) {
  .login-layout {
    padding: 16px;
  }

  .header-actions {
    top: 14px;
    right: 16px;
  }

  .form-card {
    padding: 28px 20px;
  }

  .form-header {
    margin-bottom: 24px;
  }

  .form-title {
    font-size: 20px;
  }
}

@media (max-width: 480px) {
  .form-panel {
    max-width: 100%;
  }

  .form-card {
    padding: 24px 16px;
  }

  .link-text {
    display: none;
  }
}
</style>
