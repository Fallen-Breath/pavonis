import {DefaultTheme, defineConfig, UserConfig} from 'vitepress'
import {withSidebar} from 'vitepress-sidebar'
import {VitePressSidebarOptions} from "vitepress-sidebar/dist/types"

// https://vitepress.dev/reference/site-config
const vitePressOptions: UserConfig<DefaultTheme.Config> = {
  title: "My Awesome Project",
  description: "A VitePress Site",
  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Examples', link: '/markdown-examples' }
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/Fallen-Breath/pavonis' }
    ]
  },
  cleanUrls: true,
  srcDir: 'src',
}

// https://vitepress-sidebar.cdget.com/guide/options
const vitePressSidebarOptions: VitePressSidebarOptions = {
  documentRootPath: 'src',
  collapsed: false,
  capitalizeFirst: true,
  useTitleFromFrontmatter: true,
};

// noinspection JSUnusedGlobalSymbols
export default defineConfig(withSidebar(vitePressOptions, vitePressSidebarOptions))
