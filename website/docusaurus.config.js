const {themes} = require('prism-react-renderer');
const lightCodeTheme = themes.github;
const darkCodeTheme = themes.dracula;

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'Kuargogo',
  tagline: 'El centro de mando para tu homelab de nueva generación',
  url: 'https://DannyStrelok.github.io',
  baseUrl: '/kuargogo/',
  onBrokenLinks: 'warn',
  favicon: 'img/logo.svg',
  organizationName: 'DannyStrelok', // GitHub org/user name.
  projectName: 'kuargogo', // Repo name.
  trailingSlash: false,

  markdown: {
    mermaid: true,
  },
  themes: ['@docusaurus/theme-mermaid'],

  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          path: '../docs',
          sidebarPath: require.resolve('./sidebars.js'),
          routeBasePath: 'docs',
          editUrl: 'https://github.com/DannyStrelok/kuargogo/edit/main/website/',
        },
        blog: {
          showReadingTime: true,
          editUrl: 'https://github.com/DannyStrelok/kuargogo/edit/main/website/',
        },
        theme: {
          customCss: require.resolve('./src/css/custom.css'),
        },
      }),
    ],
  ],

  plugins: [
    [
      '@docusaurus/plugin-content-docs',
      /** @type {import('@docusaurus/plugin-content-docs').Options} */
      ({
        id: 'guides',
        path: '../internal/help/docs',
        routeBasePath: 'guides',
        sidebarPath: require.resolve('./sidebarsGuides.js'),
        editUrl: 'https://github.com/DannyStrelok/kuargogo/edit/main/website/',
      }),
    ],
    [
      '@easyops-cn/docusaurus-search-local',
      /** @type {import('@easyops-cn/docusaurus-search-local').PluginOptions} */
      ({
        hashed: true,
        language: ['es', 'en'],
        indexBlog: true,
        indexDocs: true,
        indexPages: true,
        docsRouteBasePath: ['docs', 'guides'],
      }),
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      colorMode: {
        defaultMode: 'dark',
        disableSwitch: false,
        respectPrefersColorScheme: true,
      },
      navbar: {
        title: 'Kuargogo',
        logo: {
          alt: 'Kuargogo Logo',
          src: 'img/logo.svg',
        },
        items: [
          {
            type: 'docSidebar',
            sidebarId: 'docsSidebar',
            position: 'left',
            label: 'Documentación',
          },
          {
            to: '/guides/workflow-roadmap',
            position: 'left',
            label: 'Guías SRE',
          },
          {
            to: '/blog',
            label: 'Blog',
            position: 'left',
          },
          {
            href: 'https://github.com/DannyStrelok/kuargogo',
            label: 'GitHub',
            position: 'right',
          },
        ],
      },
      footer: {
        style: 'dark',
        links: [
          {
            title: 'Documentación',
            items: [
              {
                label: 'Portal de Despliegue',
                to: '/docs/DEPLOYMENT_GUIDE',
              },
              {
                label: 'Referencia de Comandos',
                to: '/docs/COMMANDS',
              },
              {
                label: 'Arquitectura del Sistema',
                to: '/docs/ARCHITECTURE',
              },
            ],
          },
          {
            title: 'Guías Prácticas',
            items: [
              {
                label: 'Fase 1: Preparación Hardware',
                to: '/guides/hardware-preparation',
              },
              {
                label: 'Fase 2: Aprovisionamiento',
                to: '/guides/provisioning',
              },
              {
                label: 'Fase 3: Cluster K3s',
                to: '/guides/cluster-and-services',
              },
            ],
          },
          {
            title: 'Comunidad & Más',
            items: [
              {
                label: 'Roadmap del Proyecto',
                to: '/docs/ROADMAP',
              },
              {
                label: 'Blog',
                to: '/blog',
              },
              {
                label: 'GitHub',
                href: 'https://github.com/DannyStrelok/kuargogo',
              },
            ],
          },
        ],
        copyright: `Copyright © ${new Date().getFullYear()} Kuargogo. Licencia AGPLv3.`,
      },
      prism: {
        theme: lightCodeTheme,
        darkTheme: darkCodeTheme,
        additionalLanguages: ['bash', 'yaml', 'ini', 'go'],
      },
    }),
};

module.exports = config;
