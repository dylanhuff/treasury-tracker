const TOUR_STORAGE_KEY = 'treasuryApp_tourCompleted';

export function getTourCompleted(): boolean {
  try {
    if (typeof window === 'undefined' || !window.localStorage) {
      return false;
    }

    const value = localStorage.getItem(TOUR_STORAGE_KEY);
    if (value === null) {
      return false;
    }

    return value === 'true';
  } catch {
    return false;
  }
}

export function setTourCompleted(completed: boolean): void {
  try {
    if (typeof window === 'undefined' || !window.localStorage) {
      return;
    }

    localStorage.setItem(TOUR_STORAGE_KEY, String(completed));
  } catch {
    // localStorage may be unavailable in private browsing
  }
}
