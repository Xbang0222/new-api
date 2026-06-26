import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const dashboardRoot = resolve(here, '..');
const projectRoot = resolve(dashboardRoot, '..', '..');

const readDashboardFile = (fileName) =>
  readFileSync(resolve(dashboardRoot, fileName), 'utf8');

describe('dashboard redesign structure', () => {
  test('dashboard exposes stable workbench landmarks', () => {
    const source = readDashboardFile('index.jsx');

    expect(source).toContain("data-testid='dashboard-workbench'");
    expect(source).toContain("data-testid='dashboard-summary-grid'");
    expect(source).toContain("data-testid='dashboard-analytics-card'");
  });

  test('dashboard styles are scoped behind the custom marker', () => {
    const css = readFileSync(resolve(projectRoot, 'index.css'), 'utf8');

    expect(css).toContain('custom: dashboard redesign');
    expect(css).toContain('.dashboard-workbench');
    expect(css).toContain('.dashboard-card');
  });
});
