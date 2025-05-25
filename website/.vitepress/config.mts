import {DefaultTheme, defineConfig, UserConfig} from 'vitepress'
import {withSidebar} from 'vitepress-sidebar'
import {VitePressSidebarOptions} from "vitepress-sidebar/dist/types"
import {withI18n} from "vitepress-i18n";
import {VitePressI18nOptions} from "vitepress-i18n/dist/types";

const rootLocale = 'en'
const supportedLocales = [rootLocale, 'zhHans']

// https://vitepress.dev/reference/site-config
const vitePressConfig: UserConfig<DefaultTheme.Config> = {
  title: 'Pavonis',
  titleTemplate: true,
  description: 'Pavonis website',
  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    socialLinks: [
      { icon: 'github', link: 'https://github.com/Fallen-Breath/pavonis' }
    ],
  },
  cleanUrls: true,
  srcDir: 'src',
  rewrites: {
    'en/:rest*': ':rest*'
  },
}

const docsRoot: string = 'docs'

// https://vitepress-sidebar.cdget.com/guide/options
const vitePressSidebarCommonOptions: VitePressSidebarOptions = {
  documentRootPath: 'src',
  scanStartPath: docsRoot,
  collapsed: false,
  capitalizeFirst: true,
  useTitleFromFrontmatter: true,
  sortMenusByFrontmatterOrder: true,
}

const vitePressSidebarOptions: VitePressSidebarOptions[] = supportedLocales.map((lang) => {
  return {
    ...vitePressSidebarCommonOptions,
    ...(rootLocale === lang ? {} : { basePath: `/${lang}/` }), // If using `rewrites` option
    documentRootPath: `${vitePressSidebarCommonOptions.documentRootPath}/${lang}`,
    resolvePath: (rootLocale === lang ? '/' : `/${lang}/`) + `${docsRoot}/`,
  };
})


// https://vitepress-i18n.cdget.com/introduction
const vitePressI18nConfig: VitePressI18nOptions = {
  locales: supportedLocales,
  searchProvider: 'local',
  themeConfig: {
    en: {
      nav: [
        { text: 'Home', link: '/' },
        { text: 'Document', link: '/docs/' }
      ],
    },
    zhHans: {
      nav: [
        { text: '首页', link: '/zhHans/' },
        { text: '文档', link: '/zhHans/docs/' }
      ],
    }
  },
}

// noinspection JSUnusedGlobalSymbols
export default defineConfig(
    withSidebar(
        withI18n(vitePressConfig, vitePressI18nConfig),
        vitePressSidebarOptions
    )
)
