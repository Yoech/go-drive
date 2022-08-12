export interface SearchConfig {
  enabled: boolean
  examples: string[]
}

export interface ThumbnailConfig {
  extensions: O<boolean>
}

export interface VersionConfig {
  buildAt: string
  rev: string
  version: string
}

export interface Config {
  version: VersionConfig
  thumbnail: ThumbnailConfig
  options: O<string>

  search?: SearchConfig
}