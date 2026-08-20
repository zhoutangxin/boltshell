import {createApp} from 'vue'
import App from './App.vue'
// 必须在自定义样式之前引入：缺失时 xterm 的字符测量元素（32 个 W）和
// 输入用 textarea 会失去隐藏样式，直接显示在终端里。
import 'xterm/css/xterm.css'
import './style.css';

createApp(App).mount('#app')
