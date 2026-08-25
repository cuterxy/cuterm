/* Shared terminal appearance presets, used by both the app page (app.js)
 * and the config page (config.js).
 *
 * CUTERM_FONTS: fontFamily stacks offered in the config page. The stored
 * config value is the stack string itself.
 * CUTERM_THEMES: named xterm color schemes. The stored config value is the
 * theme key; an unknown or empty key falls back to "default".
 */
var CUTERM_FONTS = [
  { label: 'Menlo', value: 'Menlo, Consolas, "Courier New", monospace' },
  { label: 'SF Mono', value: '"SF Mono", Menlo, Consolas, monospace' },
  { label: 'Consolas', value: 'Consolas, Menlo, "Courier New", monospace' },
  { label: 'Courier New', value: '"Courier New", Courier, monospace' },
  { label: 'JetBrains Mono', value: '"JetBrains Mono", Menlo, Consolas, monospace' },
  { label: 'Fira Code', value: '"Fira Code", Menlo, Consolas, monospace' },
  { label: 'Cascadia Code', value: '"Cascadia Code", Consolas, Menlo, monospace' },
  { label: { en: 'System default mono', 'zh-CN': '系统默认等宽' }, value: 'monospace' }
];

var CUTERM_DEFAULT_FONT = CUTERM_FONTS[0].value;
var CUTERM_DEFAULT_FONT_SIZE = 14;
var CUTERM_DEFAULT_THEME = 'default';
var CUTERM_DEFAULT_SCROLLBACK = 1000;

var CUTERM_THEMES = {
  'default': {
    label: { en: 'Default Dark', 'zh-CN': '默认暗色' },
    theme: {
      background: '#1e1e1e',
      foreground: '#d4d4d4',
      cursor: '#d4d4d4',
      selectionBackground: '#264f78'
    }
  },
  'light': {
    label: { en: 'Default Light', 'zh-CN': '默认亮色' },
    theme: {
      background: '#ffffff',
      foreground: '#333333',
      cursor: '#333333',
      selectionBackground: '#add6ff'
    }
  },
  'solarized-dark': {
    label: 'Solarized Dark',
    theme: {
      background: '#002b36',
      foreground: '#839496',
      cursor: '#839496',
      selectionBackground: '#073642',
      black: '#073642',
      red: '#dc322f',
      green: '#859900',
      yellow: '#b58900',
      blue: '#268bd2',
      magenta: '#d33682',
      cyan: '#2aa198',
      white: '#eee8d5',
      brightBlack: '#002b36',
      brightRed: '#cb4b16',
      brightGreen: '#586e75',
      brightYellow: '#657b83',
      brightBlue: '#839496',
      brightMagenta: '#6c71c4',
      brightCyan: '#93a1a1',
      brightWhite: '#fdf6e3'
    }
  },
  'solarized-light': {
    label: 'Solarized Light',
    theme: {
      background: '#fdf6e3',
      foreground: '#657b83',
      cursor: '#657b83',
      selectionBackground: '#eee8d5',
      black: '#073642',
      red: '#dc322f',
      green: '#859900',
      yellow: '#b58900',
      blue: '#268bd2',
      magenta: '#d33682',
      cyan: '#2aa198',
      white: '#eee8d5',
      brightBlack: '#002b36',
      brightRed: '#cb4b16',
      brightGreen: '#586e75',
      brightYellow: '#657b83',
      brightBlue: '#839496',
      brightMagenta: '#6c71c4',
      brightCyan: '#93a1a1',
      brightWhite: '#fdf6e3'
    }
  },
  'dracula': {
    label: 'Dracula',
    theme: {
      background: '#282a36',
      foreground: '#f8f8f2',
      cursor: '#f8f8f2',
      selectionBackground: '#44475a',
      black: '#21222c',
      red: '#ff5555',
      green: '#50fa7b',
      yellow: '#f1fa8c',
      blue: '#bd93f9',
      magenta: '#ff79c6',
      cyan: '#8be9fd',
      white: '#f8f8f2',
      brightBlack: '#6272a4',
      brightRed: '#ff6e6e',
      brightGreen: '#69ff94',
      brightYellow: '#ffffa5',
      brightBlue: '#d6acff',
      brightMagenta: '#ff92df',
      brightCyan: '#a4ffff',
      brightWhite: '#ffffff'
    }
  },
  'monokai': {
    label: 'Monokai',
    theme: {
      background: '#272822',
      foreground: '#f8f8f2',
      cursor: '#f8f8f2',
      selectionBackground: '#49483e',
      black: '#272822',
      red: '#f92672',
      green: '#a6e22e',
      yellow: '#f4bf75',
      blue: '#66d9ef',
      magenta: '#ae81ff',
      cyan: '#a1efe4',
      white: '#f8f8f2',
      brightBlack: '#75715e',
      brightRed: '#f92672',
      brightGreen: '#a6e22e',
      brightYellow: '#e6db74',
      brightBlue: '#66d9ef',
      brightMagenta: '#ae81ff',
      brightCyan: '#a1efe4',
      brightWhite: '#f9f8f5'
    }
  },
  'one-dark': {
    label: 'One Dark',
    theme: {
      background: '#282c34',
      foreground: '#abb2bf',
      cursor: '#528bff',
      selectionBackground: '#3e4451',
      black: '#282c34',
      red: '#e06c75',
      green: '#98c379',
      yellow: '#e5c07b',
      blue: '#61afef',
      magenta: '#c678dd',
      cyan: '#56b6c2',
      white: '#abb2bf',
      brightBlack: '#5c6370',
      brightRed: '#e06c75',
      brightGreen: '#98c379',
      brightYellow: '#e5c07b',
      brightBlue: '#61afef',
      brightMagenta: '#c678dd',
      brightCyan: '#56b6c2',
      brightWhite: '#ffffff'
    }
  },
  'nord': {
    label: 'Nord',
    theme: {
      background: '#2e3440',
      foreground: '#d8dee9',
      cursor: '#d8dee9',
      selectionBackground: '#434c5e',
      black: '#3b4252',
      red: '#bf616a',
      green: '#a3be8c',
      yellow: '#ebcb8b',
      blue: '#81a1c1',
      magenta: '#b48ead',
      cyan: '#88c0d0',
      white: '#e5e9f0',
      brightBlack: '#4c566a',
      brightRed: '#bf616a',
      brightGreen: '#a3be8c',
      brightYellow: '#ebcb8b',
      brightBlue: '#81a1c1',
      brightMagenta: '#b48ead',
      brightCyan: '#8fbcbb',
      brightWhite: '#eceff4'
    }
  }
};

/* Look up a theme entry by key, falling back to the default. */
function cutermTheme(key) {
  return CUTERM_THEMES[key] || CUTERM_THEMES[CUTERM_DEFAULT_THEME];
}
