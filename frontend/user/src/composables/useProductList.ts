import { computed, ref, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { productAPI, categoryAPI } from '../api'
import { buildCategoryGroups, createCategoryMap, normalizeCategoryParentId, type PublicCategory } from '../utils/category'
import { debounceAsync } from '../utils/debounce'
import { loadProductSearchPage } from '../utils/productSearch'
import { useUserAuthStore } from '../stores/userAuth'

export interface UseProductListOptions {
  pageSize?: number
  homeRouteName?: string
  categoryRouteName?: string
}

export function useProductList(options: UseProductListOptions = {}) {
  const {
    pageSize: defaultPageSize = 20,
    homeRouteName = 'home',
    categoryRouteName = 'category-products',
  } = options

  const router = useRouter()
  const route = useRoute()
  const authStore = useUserAuthStore()

  const loading = ref(true)
  const products = ref<any[]>([])
  const categories = ref<PublicCategory[]>([])
  const selectedCategory = ref<number | null>(null)
  const searchQuery = ref('')
  const currentPage = ref(1)
  const pageSize = ref(defaultPageSize)
  const totalPages = ref(0)
  const totalItems = ref(0)
  const loadFailed = ref(false)
  const showFilterDrawer = ref(false)
  const expandedParentIds = ref<number[]>([])

  const categoryGroups = computed(() => buildCategoryGroups(categories.value))
  const categoryMap = computed(() => createCategoryMap(categories.value))

  let initializing = true
  let requestId = 0

  const isParentExpanded = (categoryId: number) => {
    return expandedParentIds.value.includes(categoryId)
  }

  const expandParentCategory = (categoryId: number) => {
    if (!categoryId || isParentExpanded(categoryId)) return
    expandedParentIds.value = [...expandedParentIds.value, categoryId]
  }

  const toggleParentCategory = (categoryId: number) => {
    if (isParentExpanded(categoryId)) {
      expandedParentIds.value = expandedParentIds.value.filter((id) => id !== categoryId)
      return
    }
    expandParentCategory(categoryId)
  }

  const getParentToggleButtonClass = (categoryId: number) => {
    return isParentExpanded(categoryId)
      ? 'bg-primary text-primary-foreground border-transparent'
      : 'bg-card/70 text-muted-foreground hover:text-foreground'
  }

  const syncExpandedCategoryState = () => {
    if (!selectedCategory.value) return

    const matched = categoryMap.value.get(selectedCategory.value)
    if (!matched) return

    const parentId = normalizeCategoryParentId(matched.parent_id)
    if (parentId > 0) {
      expandParentCategory(parentId)
      return
    }

    const selectedGroup = categoryGroups.value.find((group) => group.id === matched.id)
    if (selectedGroup?.children.length) {
      expandParentCategory(selectedGroup.id)
    }
  }

  const selectCategory = (categoryId: number | null, closeDrawer = false) => {
    selectedCategory.value = categoryId
    if (closeDrawer) {
      showFilterDrawer.value = false
    }
  }

  const loadProducts = async () => {
    const request = ++requestId
    const token = authStore.token
    const isCurrent = () => request === requestId && token === authStore.token
    loading.value = true
    loadFailed.value = false
    try {
      const params: any = {
        page: currentPage.value,
        page_size: pageSize.value,
      }
      if (selectedCategory.value) {
        params.category_id = selectedCategory.value
      }
      const keyword = searchQuery.value.trim()
      if (keyword) {
        const result = await loadProductSearchPage(async (page) => {
          const response = await productAPI.list({ ...params, page, page_size: 100 })
          return response.data
        }, keyword, params.page, params.page_size)
        if (!isCurrent()) return
        products.value = result.products
        totalPages.value = result.totalPages
        totalItems.value = result.totalItems
      } else {
        const response = await productAPI.list(params)
        if (!isCurrent()) return
        if (response.data.status_code !== 0 || !Array.isArray(response.data.data)) throw new Error('Invalid product response')
        products.value = response.data.data
        totalPages.value = response.data.pagination?.total_page || 0
        totalItems.value = response.data.pagination?.total ?? products.value.length
      }
    } catch (error) {
      if (!isCurrent()) return
      products.value = []
      totalItems.value = 0
      totalPages.value = 0
      loadFailed.value = true
      console.error('Failed to load products:', error)
    } finally {
      if (isCurrent()) loading.value = false
    }
  }

  const loadCategories = async () => {
    try {
      const response = await categoryAPI.list()
      categories.value = response.data.data || []
    } catch (error) {
      console.error('Failed to load categories:', error)
    }
  }

  const debouncedLoadProducts = debounceAsync(loadProducts, 300)
  const scheduleLoadProducts = () => {
    // Invalidate an earlier response immediately, including during the debounce.
    ++requestId
    loading.value = true
    debouncedLoadProducts()
  }

  const changePage = (page: number) => {
    if (page < 1 || page > totalPages.value) return
    currentPage.value = page
    scheduleLoadProducts()
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  const clearSearch = () => {
    if (!searchQuery.value) return
    searchQuery.value = ''
    currentPage.value = 1
    scheduleLoadProducts()
  }

  const onSearch = () => {
    currentPage.value = 1
    scheduleLoadProducts()
  }

  const syncSelectedCategoryFromRoute = () => {
    if (route.name !== categoryRouteName) {
      if (selectedCategory.value !== null) {
        selectedCategory.value = null
      }
      return false
    }

    const slugParam = route.params.slug as string | undefined
    if (!slugParam || categories.value.length === 0) return false

    const matched = categories.value.find((category) => category.slug === slugParam)
    if (!matched) return false

    if (selectedCategory.value !== matched.id) {
      selectedCategory.value = matched.id
    }

    return true
  }

  watch(selectedCategory, () => {
    if (initializing) return
    currentPage.value = 1
    syncExpandedCategoryState()
    scheduleLoadProducts()

    if (selectedCategory.value) {
      const matched = categories.value.find((category) => category.id === selectedCategory.value)
      if (matched?.slug && route.params.slug !== matched.slug) {
        router.replace({ name: categoryRouteName, params: { slug: matched.slug } })
      }
    } else if (route.name === categoryRouteName) {
      router.replace({ name: homeRouteName })
    }
  })

  watch(searchQuery, () => {
    if (initializing) return
    currentPage.value = 1
    scheduleLoadProducts()
  })

  watch(() => authStore.token, () => {
    if (initializing) return
    products.value = []
    currentPage.value = 1
    scheduleLoadProducts()
  })

  watch(
    () => route.params.slug,
    () => {
      if (initializing) return
      if (categories.value.length === 0) return
      syncSelectedCategoryFromRoute()
    },
  )

  const initialize = async () => {
    await loadCategories()
    if (syncSelectedCategoryFromRoute()) {
      syncExpandedCategoryState()
    }
    initializing = false
    await loadProducts()
  }

  const cleanup = () => {
    ++requestId
    debouncedLoadProducts.cancel()
  }

  return {
    loading,
    products,
    categories,
    selectedCategory,
    searchQuery,
    currentPage,
    pageSize,
    totalPages,
    totalItems,
    loadFailed,
    showFilterDrawer,
    expandedParentIds,
    categoryGroups,
    categoryMap,
    isParentExpanded,
    toggleParentCategory,
    getParentToggleButtonClass,
    selectCategory,
    loadProducts,
    loadCategories,
    changePage,
    clearSearch,
    onSearch,
    syncSelectedCategoryFromRoute,
    syncExpandedCategoryState,
    initialize,
    cleanup,
  }
}
