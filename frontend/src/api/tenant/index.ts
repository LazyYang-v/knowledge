import { del, get, post, put } from '@/utils/request'
import i18n from '@/i18n'

const t = (key: string) => i18n.global.t(key)

// 空间信息接口
export interface TenantInfo {
  id: number
  name: string
  description?: string
  status?: string
  business?: string
  storage_quota?: number
  storage_used?: number
  created_at: string
  updated_at: string
}

// 搜索空间参数
export interface SearchTenantsParams {
  keyword?: string
  tenant_id?: number
  page?: number
  page_size?: number
}

// 搜索空间响应
export interface SearchTenantsResponse {
  success: boolean
  data?: {
    items: TenantInfo[]
    total: number
    page: number
    page_size: number
  }
  message?: string
}

/**
 * 获取所有空间列表（需要跨空间访问权限）
 * @deprecated 建议使用 searchTenants 代替，支持分页和搜索
 */
export async function listAllTenants(): Promise<{ success: boolean; data?: { items: TenantInfo[] }; message?: string }> {
  try {
    const response = await get('/api/v1/tenants/all')
    return response as unknown as { success: boolean; data?: { items: TenantInfo[] }; message?: string }
  } catch (error: any) {
    return {
      success: false,
      message: error.message || t('error.tenant.listFailed')
    }
  }
}

/**
 * 更新空间信息（目前暴露名称、描述两个字段的编辑入口）。
 * 后端 `PUT /tenants/:id` 用指针字段区分"未传"和"显式空串"，未传的列不会
 * 被改动；这里也按需选择性传 `name` / `description`，互不影响。
 * 权限：owner（与 router.go 中的 g.Owner() 守卫保持一致）。
 */
export async function updateTenant(
  tenantId: number,
  payload: { name?: string; description?: string },
): Promise<{ success: boolean; data?: TenantInfo; message?: string }> {
  try {
    const response = await put(`/api/v1/tenants/${tenantId}`, payload)
    return response as unknown as { success: boolean; data?: TenantInfo; message?: string }
  } catch (error: any) {
    return {
      success: false,
      message: error.message || t('error.tenant.updateFailed'),
    }
  }
}

/**
 * 删除当前工作区。权限：owner。
 */
export async function deleteTenant(
  tenantId: number,
): Promise<{ success: boolean; message?: string }> {
  try {
    const response = await del(`/api/v1/tenants/${tenantId}`)
    return response as unknown as { success: boolean; message?: string }
  } catch (error: any) {
    return {
      success: false,
      message: error.message || t('error.tenant.deleteFailed'),
    }
  }
}

/**
 * 创建新工作区（任意已登录用户均可调用）。
 * 后端会自动把调用者写成新空间的 Owner，并填充默认 storage_quota
 * 等服务端字段；API Key 由用户在集成页手动创建。
 * 路由：POST /api/v1/tenants（router 上不挂 g.CrossTenant()，自助场景使用）。
 */
export async function createTenant(
  payload: { name: string; description?: string },
): Promise<{ success: boolean; data?: TenantInfo; message?: string }> {
  try {
    const response = await post('/api/v1/tenants', payload)
    return response as unknown as { success: boolean; data?: TenantInfo; message?: string }
  } catch (error: any) {
    const code = error?.error?.code ?? error?.code
    return {
      success: false,
      message: code === 2005
        ? t('tenant.create.disabled')
        : (error.message || t('error.tenant.createFailed')),
    }
  }
}

/**
 * 搜索空间（支持分页、关键词搜索和空间ID过滤）
 */
export async function searchTenants(params: SearchTenantsParams = {}): Promise<SearchTenantsResponse> {
  try {
    const queryParams = new URLSearchParams()
    if (params.keyword) {
      queryParams.append('keyword', params.keyword)
    }
    if (params.tenant_id) {
      queryParams.append('tenant_id', String(params.tenant_id))
    }
    if (params.page) {
      queryParams.append('page', String(params.page))
    }
    if (params.page_size) {
      queryParams.append('page_size', String(params.page_size))
    }
    
    const queryString = queryParams.toString()
    const url = `/api/v1/tenants/search${queryString ? '?' + queryString : ''}`
    const response = await get(url)
    return response as unknown as SearchTenantsResponse
  } catch (error: any) {
    return {
      success: false,
      message: error.message || t('error.tenant.searchFailed')
    }
  }
}
