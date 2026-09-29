export interface GoogleCredentialResponse {
  credential?: string
  select_by?: string
  clientId?: string
}

export interface GoogleIdentityConfiguration {
  client_id: string
  callback?: (response: GoogleCredentialResponse) => void
  ux_mode?: 'popup' | 'redirect'
  login_uri?: string
  auto_select?: boolean
  cancel_on_tap_outside?: boolean
  use_fedcm_for_button?: boolean
}

export interface GoogleIdentityButtonConfiguration {
  type: 'standard' | 'icon'
  theme?: 'outline' | 'filled_blue' | 'filled_black'
  size?: 'large' | 'medium' | 'small'
  text?: 'signin_with' | 'signup_with' | 'continue_with' | 'signin'
  shape?: 'rectangular' | 'pill' | 'circle' | 'square'
  logo_alignment?: 'left' | 'center'
  width?: string
  locale?: string
  state?: string
}

export interface GoogleAccountsID {
  initialize: (configuration: GoogleIdentityConfiguration) => void
  renderButton: (parent: HTMLElement, configuration: GoogleIdentityButtonConfiguration) => void
  cancel?: () => void
}

export interface GoogleIdentityServices {
  accounts?: {
    id?: GoogleAccountsID
  }
}

declare global {
  interface Window {
    google?: GoogleIdentityServices
  }
}
