import { render } from 'preact';
import { App } from './App';
import './styles/main.css';

const rootElement = document.getElementById('app');
if (rootElement) {
  render(<App />, rootElement);
} else {
  console.error('Fatal: Mount element #app not found');
}
