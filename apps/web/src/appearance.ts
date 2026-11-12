import catalog from '../../desktop/theme-palettes.json' with { type: 'json' };
import { readPreference, writePreference } from './lib/preferences';
export type Palette = keyof typeof catalog.themes;
export type Appearance = { version: 1; light: Palette; dark: Palette; textSize: number; reduceMotion: boolean };
export const appearanceKey = 'offgrid.appearance.v1';
export const appearanceDefaults: Appearance = { version: 1, light: 'offgrid', dark: 'offgrid', textSize: 100, reduceMotion: false };
export const palettes = Object.keys(catalog.themes) as Palette[];
export type PaletteMode = 'light' | 'dark';
type Tokens = typeof catalog.themes.offgrid.light;
const themes: Record<string, Partial<Record<PaletteMode, Tokens>>> = catalog.themes;
const labels: Record<string, Partial<Record<PaletteMode, string>>> = catalog.labels;
export const availablePalettes = (mode: PaletteMode) => palettes.filter(id => themes[id][mode]);
export const paletteTokens = (id: Palette, mode: PaletteMode) => themes[id]?.[mode] ?? themes.offgrid[mode]!;
export const paletteLabel = (id: Palette, mode: PaletteMode) => labels[id]?.[mode] ?? id;
export function normalizeAppearance(value: unknown): Appearance {
  const v = value as Partial<Appearance> | null;
  if (!v || v.version !== 1) return { ...appearanceDefaults };
  const palette = (id: unknown, mode: PaletteMode): Palette => typeof id === 'string' && Object.hasOwn(themes, id) && themes[id][mode] ? id as Palette : 'offgrid';
  return { version: 1, light: palette(v.light, 'light'), dark: palette(v.dark, 'dark'), textSize: [100,110,125,150].includes(v.textSize ?? 0) ? v.textSize! : 100, reduceMotion: v.reduceMotion === true };
}
export function readAppearance(): Appearance {
  try { return normalizeAppearance(JSON.parse(readPreference(appearanceKey) ?? 'null')); } catch { return { ...appearanceDefaults }; }
}
export function applyAppearance(value = readAppearance()) {
  const root = document.documentElement;
  const mode = root.dataset.theme === 'light' ? 'light' : 'dark';
  root.dataset.palette = value[mode]; root.dataset.reduceMotion = String(value.reduceMotion);
  const tokens = paletteTokens(value[mode], mode);
  for (const key of Object.keys(tokens)) {
    if (value[mode] === 'offgrid' || matchMedia('(forced-colors: active)').matches) root.style.removeProperty('--' + key);
    else root.style.setProperty('--' + key, tokens[key as keyof typeof tokens]);
  }
  for (const [key, size] of Object.entries({caption:12,meta:13,ui:14,body:16,section:18,title:24})) root.style.setProperty('--type-' + key, `${size * value.textSize / 100}px`);
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', tokens['surface-0']);
}
export function saveAppearance(value: Appearance) {
  const normalized = normalizeAppearance(value);
  const saved = writePreference(appearanceKey, JSON.stringify(normalized));
  applyAppearance(normalized); window.dispatchEvent(new Event('offgrid-appearance'));
  return saved;
}
