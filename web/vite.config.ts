/// <reference types="vitest/config" />
import path from 'node:path'
import {fileURLToPath} from 'node:url'
import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'

const webRoot = path.dirname(fileURLToPath(import.meta.url))

// Wails 开发模式会注入 Overlay.svelte，它要求页面上存在 #wails-spinner。
// Vite 直出 HTML 时经常没有这个节点，导致 Cannot read properties of null (reading 'nodes')。
function wailsSpinnerAnchor() {
  return {
    name: 'wails-spinner-anchor',
    transformIndexHtml(html: string) {
      const patch = `<script>
(function () {
  var orig = Document.prototype.querySelector;
  Document.prototype.querySelector = function (sel) {
    var el = orig.call(this, sel);
    if (sel === '#wails-spinner' && !el) {
      el = document.getElementById('wails-spinner');
      if (!el) {
        el = document.createElement('div');
        el.id = 'wails-spinner';
        el.style.display = 'none';
        if (document.body) document.body.insertBefore(el, document.body.firstChild);
      }
    }
    return el;
  };
})();
</script>`
      if (!html.includes('id="wails-spinner"')) {
        html = html.replace(/<body[^>]*>/i, (m) => m + '<div id="wails-spinner"></div>')
      }
      return html.replace(/<head[^>]*>/i, (m) => m + patch)
    },
  }
}

export default defineConfig({
  plugins: [vue(), wailsSpinnerAnchor()],
  build: {
    // Go embed 不能引用 .. ，产物写到 server/frontend/dist
    outDir: path.resolve(webRoot, '../server/frontend/dist'),
    emptyOutDir: true,
  },
  test: {
    environment: 'happy-dom',
    include: ['src/**/*.test.ts'],
  },
  server: {
    watch: {
      // Windows 下 wailsjs 绑定更新后，Vite 内存缓存常不失效
      usePolling: true,
      interval: 500,
    },
  },
  optimizeDeps: {
    exclude: ['@wailsjs-go-main-app'],
  },
})
