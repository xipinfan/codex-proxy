import { ConsoleSettings, defaultSettings } from './types';

const STORAGE_KEY = 'codex-proxy-console-settings';

export function loadConsoleSettings(): ConsoleSettings {
  if (typeof window === 'undefined') {
    return { ...defaultSettings };
  }

  const raw = window.localStorage.getItem(STORAGE_KEY);
  if (!raw) {
    return { ...defaultSettings };
  }

  try {
    return { ...defaultSettings, ...(JSON.parse(raw) as Partial<ConsoleSettings>) };
  } catch {
    return { ...defaultSettings };
  }
}

export function saveConsoleSettings(settings: ConsoleSettings): void {
  if (typeof window === 'undefined') {
    return;
  }

  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
}
