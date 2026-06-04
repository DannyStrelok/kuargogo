const mdx = require('./website/node_modules/eslint-plugin-mdx');

module.exports = [
  {
    name: 'mdx/recommended',
    files: [
      'docs/**/*.md',
      'docs/**/*.mdx',
      'internal/help/docs/**/*.md',
      'internal/help/docs/**/*.mdx'
    ],
    ...mdx.flat,
    processor: mdx.createRemarkProcessor({
      lintCodeBlocks: true,
    }),
  },
  {
    name: 'mdx/code-blocks',
    files: [
      'docs/**/*.md',
      'docs/**/*.mdx',
      'internal/help/docs/**/*.md',
      'internal/help/docs/**/*.mdx'
    ],
    ...mdx.flatCodeBlocks,
    rules: {
      ...mdx.flatCodeBlocks.rules,
    },
  },
];
