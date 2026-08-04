import { renderToStaticMarkup } from 'react-dom/server';
import React from 'react';
const ui = await import('@hanzo/ui');
const gui = await import('@hanzo/gui');
const { config } = await import('@hanzo/ui/gui-config');

console.log('--- NO PROVIDER ---');
try {
  console.log(renderToStaticMarkup(React.createElement(ui.Button, null, 'Hi')).slice(0, 600));
} catch (e) { console.log('ERR', e.message); }

console.log('\n--- WITH PROVIDER ---');
try {
  console.log(renderToStaticMarkup(
    React.createElement(gui.GuiProvider, { config, defaultTheme: 'light' },
      React.createElement(ui.Button, null, 'Hi'))
  ).slice(0, 900));
} catch (e) { console.log('ERR', e.message); }
