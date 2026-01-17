import { computed } from 'vue'
import type { RouteLocationNormalized, Router, RouteComponent } from 'vue-router'
import { openModal } from './methods'
import type Modal from './utils/Modal'
import ModalError from './utils/ModalError'

interface ModalRouterInterface {
  initialize(): Promise<void>
  _isModal: boolean
  getModalObject(): Modal
  close(v: boolean): Promise<void>
}
interface ModalRouterStateInterface {
  router: Router | null
}

const state: ModalRouterStateInterface = {
  router: null,
}

function init(router: Router) {
  if (state.router) throw ModalError.DuplicatedRouterIntegration()

  state.router = router
  function findModal(routerLocation: RouteLocationNormalized): ModalRouterInterface | null {
    for (let i = routerLocation.matched.length - 1; i >= 0; i--) {
      const components = routerLocation.matched[i].components
      /**
       * Problem:
       * Object.values(components)
       * return (RouteComponent | ModalRouterInterface)[]
       *
       * How to do it in TypeScript
       * */
      // @ts-ignore
      const a: ModalRouterInterface | null = Object.values(components).find((route: RouteComponent) => route._isModal)

      if (a) return a
    }
    return null
  }

  router.beforeEach(async (to: RouteLocationNormalized, from: RouteLocationNormalized) => {
    try {
      const modalRoute = findModal(from)
      if (modalRoute && !modalRoute.getModalObject()?.closed?.value) await modalRoute.close(true)
    } catch (e) {
      return false
    }
  })

  router.afterEach(async (to: any) => {
    const modal: ModalRouterInterface | null = findModal(to)
    if (modal) await modal.initialize()
  })
}

function useModalRouter(component: any) {
  //Ссылка на modalObject
  let modal: Modal | null = null

  let isNavigationClosingGuard = false

  async function initialize(): Promise<void> {
    if (!state.router) throw ModalError.ModalRouterIntegrationNotInitialized()

    isNavigationClosingGuard = false
    modal = null

    modal = await openModal(
      component,
      computed(() => state.router?.currentRoute.value.params),
      { isRoute: true }
    )
    modal.onclose = () => {
      if (!isNavigationClosingGuard) state.router?.back()
    }
  }

  return {
    getModalObject: () => modal,
    _isModal: true,

    async close(v = false) {
      isNavigationClosingGuard = v

      if (modal) return await modal.close()
    },
    initialize,
    setup: () => () => null,
  }
}

useModalRouter.init = init

export default useModalRouter
