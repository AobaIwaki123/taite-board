import { render } from 'preact';
import { App } from './App';
import './styles/main.css';

const rootElement = document.getElementById('app');
if (rootElement) {
  render(<App />, rootElement);
} else {
  console.error('Fatal: Mount element #app not found');
}

// Service Worker Registration for offline image caching in underground venues
if ('serviceWorker' in navigator && !import.meta.env.DEV) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js').catch((err) => {
      console.warn('ServiceWorker registration failed:', err);
    });
  });
}

