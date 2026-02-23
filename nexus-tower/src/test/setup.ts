import '@testing-library/jest-dom/vitest';
import { afterEach, vi } from 'vitest';
import { cleanup } from '@testing-library/react';

// Automatically cleanup after each test
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

// Mock global fetch
vi.stubGlobal('fetch', vi.fn());

// Silence ResizeObserver errors from recharts in DOM test environment
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
vi.stubGlobal('ResizeObserver', ResizeObserverStub);

Object.defineProperty(HTMLCanvasElement.prototype, 'getContext', {
  value: vi.fn(() => {
    const gradient = { addColorStop: vi.fn() };
    return {
      setTransform: vi.fn(),
      clearRect: vi.fn(),
      fillText: vi.fn(),
      createLinearGradient: vi.fn(() => gradient),
      beginPath: vi.fn(),
      moveTo: vi.fn(),
      lineTo: vi.fn(),
      closePath: vi.fn(),
      fill: vi.fn(),
      stroke: vi.fn(),
      ellipse: vi.fn(),
      arc: vi.fn(),
      fillStyle: '',
      strokeStyle: '',
      lineWidth: 1,
      font: '',
    };
  }),
});
