// Joyride tour styles matching the Tremor/Tailwind design system.
export const TOUR_STYLES = {
  options: {
    primaryColor: '#3B82F6',
    backgroundColor: '#FFFFFF',
    textColor: '#374151',
    arrowColor: '#FFFFFF',
    overlayColor: 'rgba(0, 0, 0, 0.5)',
    zIndex: 10000,
    width: 420,
    beaconSize: 36,
  },

  tooltip: {
    borderRadius: '0.5rem',
    boxShadow: '0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1)',
    padding: 0,
    maxWidth: '90vw',
    transition: 'all 0.3s ease-in-out',
  },

  tooltipContent: {
    padding: '1.25rem',
    fontSize: '0.875rem',
    lineHeight: '1.25rem',
    color: '#374151',
    backgroundColor: '#FFFFFF',
  },

  tooltipTitle: {
    fontSize: '1.125rem',
    lineHeight: '1.75rem',
    fontWeight: '600',
    color: '#111827',
    marginBottom: '0.5rem',
  },

  tooltipFooter: {
    marginTop: '0.75rem',
    padding: '0 1.25rem 1.25rem 1.25rem',
    display: 'flex',
    gap: '0.75rem',
    justifyContent: 'space-between',
    alignItems: 'center',
  },

  buttonNext: {
    backgroundColor: '#3B82F6',
    borderRadius: '0.375rem',
    color: '#FFFFFF',
    fontSize: '0.875rem',
    fontWeight: '500',
    lineHeight: '1.25rem',
    padding: '0.5rem 1rem',
    outline: 'none',
    border: 'none',
    cursor: 'pointer',
    transition: 'background-color 0.2s ease-in-out, transform 0.1s ease-in-out',
  },

  buttonBack: {
    color: '#6B7280',
    fontSize: '0.875rem',
    fontWeight: '500',
    lineHeight: '1.25rem',
    marginRight: '0.5rem',
    padding: '0.5rem 0.75rem',
    outline: 'none',
    border: 'none',
    backgroundColor: 'transparent',
    cursor: 'pointer',
    transition: 'color 0.2s ease-in-out',
  },

  buttonSkip: {
    color: '#6B7280',
    fontSize: '0.875rem',
    fontWeight: '500',
    lineHeight: '1.25rem',
    padding: '0.5rem 0.75rem',
    outline: 'none',
    border: 'none',
    backgroundColor: 'transparent',
    cursor: 'pointer',
    transition: 'color 0.2s ease-in-out',
  },

  beacon: {
    outline: 'none',
  },

  beaconInner: {
    backgroundColor: '#3B82F6',
  },

  beaconOuter: {
    backgroundColor: 'rgba(59, 130, 246, 0.2)',
    border: '2px solid #3B82F6',
  },

  spotlight: {
    borderRadius: '0.5rem',
    padding: 8,
    transition: 'all 0.3s ease-in-out',
  },

  overlay: {
    mixBlendMode: 'hard-light' as const,
    transition: 'opacity 0.3s ease-in-out',
  },
};
