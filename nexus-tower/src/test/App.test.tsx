import { describe, expect, it } from 'vitest';

import { App } from '../app/App';

describe('App', () => {
  it('exports app component', () => {
    expect(App).toBeTypeOf('function');
  });
});
