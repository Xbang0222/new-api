/**
 * Brand customization layer
 *
 * All brand-specific overrides go here. NEVER modify upstream utility
 * functions (utils.jsx, render.jsx, data.js, etc.) for branding purposes.
 *
 * Safe to edit freely — this file has no upstream counterpart and will
 * never conflict during upstream merges.
 */

// Brand logo paths (matched to files in web/public/)
export const BRAND_LOGO_DAY = '/logo_day.ico';
export const BRAND_LOGO_NIGHT = '/logo_night.ico';
export const BRAND_FAVICON = '/favicon.ico';

/**
 * Get the brand logo for the current theme, ignoring backend logo settings.
 * Use this in components where you always want the local brand asset,
 * regardless of what the admin configured in the dashboard.
 */
export function getBrandLogo(themeMode) {
  if (themeMode === 'dark') return BRAND_LOGO_NIGHT;
  return BRAND_LOGO_DAY;
}

/**
 * Get the brand favicon for the current theme.
 */
export function getBrandFavicon(themeMode) {
  if (themeMode === 'dark') return BRAND_LOGO_NIGHT;
  return BRAND_FAVICON;
}
